package batch

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

var digits = regexp.MustCompile(`\D`)

type Item struct {
	Phone, Name, Message string
	Consented            bool
}
type CSVImport struct {
	Items      []Item
	HasMessage bool
	HasConsent bool
	PhoneField string
}
type Result struct {
	Phone                    string
	SentAt                   time.Time
	MessageID, Status, Error string
}
type Processor struct {
	Client                   *whatsmeow.Client
	MinInterval, MaxInterval time.Duration
	MaxPerRun                int
	mu                       sync.Mutex
}

func LoadCSV(path string) ([]Item, error) {
	result, err := LoadCSVFlexible(path)
	if err != nil {
		return nil, err
	}
	if !result.HasMessage {
		return nil, errors.New("coluna obrigatória ausente: message")
	}
	if !result.HasConsent {
		return nil, errors.New("coluna obrigatória ausente: consent")
	}
	return result.Items, nil
}

func LoadCSVFlexible(path string) (CSVImport, error) {
	f, err := os.Open(path)
	if err != nil {
		return CSVImport{}, fmt.Errorf("abrir CSV: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	r.ReuseRecord = false
	head, err := r.Read()
	if err != nil {
		return CSVImport{}, fmt.Errorf("cabeçalho CSV: %w", err)
	}
	if len(head) == 1 {
		line := strings.TrimPrefix(head[0], "\ufeff")
		for _, delimiter := range []rune{';', '\t'} {
			if strings.ContainsRune(line, delimiter) {
				if _, err := f.Seek(0, io.SeekStart); err != nil {
					return CSVImport{}, err
				}
				r = csv.NewReader(f)
				r.TrimLeadingSpace = true
				r.Comma = delimiter
				head, err = r.Read()
				if err != nil {
					return CSVImport{}, fmt.Errorf("cabeçalho CSV: %w", err)
				}
				break
			}
		}
	}
	idx := map[string]int{}
	for i, h := range head {
		idx[normalizeHeader(h)] = i
	}
	phoneField, phoneIndex := "", -1
	for _, alias := range []string{"phone", "telefone", "celular", "whatsapp", "numero", "numero_telefone"} {
		if i, ok := idx[alias]; ok {
			phoneField, phoneIndex = alias, i
			break
		}
	}
	if phoneIndex < 0 {
		return CSVImport{}, errors.New("coluna de telefone não encontrada; use telefone, celular, whatsapp ou phone")
	}
	messageIndex, hasMessage := firstIndex(idx, "message", "mensagem", "texto")
	consentIndex, hasConsent := firstIndex(idx, "consent", "consentimento", "autorizado", "opt_in")
	nameIndex, _ := firstIndex(idx, "name", "nome", "cliente")
	var items []Item
	for line := 2; ; line++ {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return CSVImport{}, fmt.Errorf("linha %d: %w", line, err)
		}
		get := func(i int) string {
			if i >= 0 && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		consent := strings.ToLower(get(consentIndex))
		approved := consent == "true" || consent == "yes" || consent == "sim" || consent == "1"
		phone := digits.ReplaceAllString(get(phoneIndex), "")
		if phone == "" {
			continue
		}
		items = append(items, Item{Phone: phone, Name: get(nameIndex), Message: get(messageIndex), Consented: approved})
	}
	if len(items) == 0 {
		return CSVImport{}, errors.New("nenhum telefone válido encontrado")
	}
	return CSVImport{Items: items, HasMessage: hasMessage, HasConsent: hasConsent, PhoneField: phoneField}, nil
}

func firstIndex(indexes map[string]int, aliases ...string) (int, bool) {
	for _, alias := range aliases {
		if i, ok := indexes[alias]; ok {
			return i, true
		}
	}
	return -1, false
}

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "\ufeff")))
	replacer := strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c", " ", "_")
	return replacer.Replace(value)
}

func (p *Processor) Run(ctx context.Context, items []Item, onResult func(Result)) error {
	if p.MinInterval <= 0 || p.MaxInterval < p.MinInterval || p.MaxPerRun <= 0 {
		return errors.New("configuração inválida do processador")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	sent := 0
	for i, item := range items {
		if !item.Consented {
			onResult(Result{Phone: item.Phone, Status: "skipped_no_consent"})
			continue
		}
		if item.Phone == "" || item.Message == "" {
			onResult(Result{Phone: item.Phone, Status: "invalid", Error: "phone/message vazio"})
			continue
		}
		if sent >= p.MaxPerRun {
			return fmt.Errorf("limite por execução atingido (%d)", p.MaxPerRun)
		}
		if sent > 0 {
			timer := time.NewTimer(randomDuration(p.MinInterval, p.MaxInterval))
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		jid := types.NewJID(item.Phone, types.DefaultUserServer)
		resp, err := p.Client.SendMessage(ctx, jid, &waE2E.Message{Conversation: proto.String(item.Message)})
		if err != nil {
			onResult(Result{Phone: item.Phone, Status: "failed", Error: err.Error()})
			continue
		}
		sent++
		onResult(Result{Phone: item.Phone, SentAt: time.Now().UTC(), MessageID: string(resp.ID), Status: "sent"})
		_ = i
	}
	return nil
}

func randomDuration(min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return min
	}
	return min + time.Duration(binary.LittleEndian.Uint64(b[:])%uint64(max-min+1))
}
