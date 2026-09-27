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
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("abrir CSV: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	head, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("cabeçalho CSV: %w", err)
	}
	idx := map[string]int{}
	for i, h := range head {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, required := range []string{"phone", "message", "consent"} {
		if _, ok := idx[required]; !ok {
			return nil, fmt.Errorf("coluna obrigatória ausente: %s", required)
		}
	}
	var items []Item
	for line := 2; ; line++ {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("linha %d: %w", line, err)
		}
		get := func(k string) string {
			if i, ok := idx[k]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		consent := strings.ToLower(get("consent"))
		approved := consent == "true" || consent == "yes" || consent == "sim" || consent == "1"
		items = append(items, Item{Phone: digits.ReplaceAllString(get("phone"), ""), Name: get("name"), Message: get("message"), Consented: approved})
	}
	return items, nil
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
