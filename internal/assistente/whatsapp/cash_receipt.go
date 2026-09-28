package whatsapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/cash"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/utils"
	"github.com/ledongthuc/pdf"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

const maxReceiptDocumentBytes = 15 << 20

type receiptDraft struct {
	Hash      string `json:"hash"`
	Date      string `json:"date"`
	Method    string `json:"method"`
	Cents     int64  `json:"cents"`
	Text      string `json:"text,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	MediaType string `json:"media_type,omitempty"`
}

var receiptMoneyRE = regexp.MustCompile(`(?i)R\$\s*([0-9.]+,[0-9]{2})`)
var receiptLooseMoneyRE = regexp.MustCompile(`\b([0-9][0-9.]*,[0-9]{2})\b`)
var receiptDateRE = regexp.MustCompile(`\b([0-3]?\d/[01]?\d/(?:20)?\d{2})\b`)

func (s *Service) receiveCashReceiptImage(ctx context.Context, sender string, imageMessage *waE2E.ImageMessage, chat types.JID) (string, bool, error) {
	var data []byte
	var err error
	if s.downloadOverride != nil {
		data, err = s.downloadOverride(ctx, imageMessage)
	} else {
		data, err = s.client.Download(ctx, imageMessage)
	}
	if err != nil {
		return "", true, err
	}
	return s.processCashReceipt(ctx, sender, data, "image")
}

func (s *Service) receiveCashReceiptPDF(ctx context.Context, sender string, doc *waE2E.DocumentMessage) (string, bool, error) {
	name := strings.ToLower(filepath.Base(doc.GetFileName()))
	if doc.GetMimetype() != "application/pdf" && !strings.HasSuffix(name, ".pdf") {
		return "", false, nil
	}
	if doc.GetFileLength() == 0 || doc.GetFileLength() > maxReceiptDocumentBytes {
		return "O comprovante PDF estÃ¡ vazio ou passa de 15 MB. Envie um arquivo menor.", true, nil
	}
	if session, active, err := conversation.NewRepository(s.domainDB).Active(ctx, sender); err != nil {
		return "", true, err
	} else if active && session.Type == "EXTRATOS" {
		return "", false, nil
	} else if active && !strings.HasPrefix(session.Type, "CASH_RECEIPT_") {
		return "Conclua ou cancele a operaÃ§Ã£o atual antes de enviar um comprovante.", true, nil
	}
	var data []byte
	var err error
	if s.downloadOverride != nil {
		data, err = s.downloadOverride(ctx, doc)
	} else {
		data, err = s.client.Download(ctx, doc)
	}
	if err != nil {
		return "", true, err
	}
	return s.processCashReceipt(ctx, sender, data, "pdf")
}

func (s *Service) processCashReceipt(ctx context.Context, sender string, data []byte, kind string) (string, bool, error) {
	repo := conversation.NewRepository(s.domainDB)
	if session, active, err := repo.Active(ctx, sender); err != nil {
		return "", true, err
	} else if active {
		if strings.HasPrefix(session.Type, "CASH_RECEIPT_") {
			return "Existe um comprovante aguardando descrição ou confirmação. Responda à etapa atual ou envie `cancelar comprovante`.", true, nil
		}
		return "Conclua ou cancele a operação atual antes de enviar um comprovante.", true, nil
	}
	if len(data) == 0 || len(data) > maxReceiptDocumentBytes {
		return "A imagem/PDF está vazia ou passa de 15 MB. Envie um arquivo menor.", true, nil
	}
	hashBytes := sha256.Sum256(data)
	digest := hex.EncodeToString(hashBytes[:])
	if exists, err := s.cash.ReceiptExists(ctx, digest); err != nil {
		return "", true, err
	} else if exists {
		return "Esse comprovante já foi lançado no caixa; não registrei novamente.", true, nil
	}
	draft := receiptDraft{Hash: digest}
	var extracted receiptDraft
	var extractErr error
	if kind == "pdf" {
		draft.MediaType = "application/pdf"
		extracted, extractErr = parseReceiptPDF(data)
	} else {
		imageConfig, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || (format != "jpeg" && format != "png") || imageConfig.Width < 100 || imageConfig.Height < 100 || int64(imageConfig.Width)*int64(imageConfig.Height) > 40_000_000 {
			return "Não reconheci a foto. Envie uma imagem JPEG ou PNG nítida do comprovante.", true, nil
		}
		var ocrText string
		draft.MediaType = "image/" + format
		ocrText, extractErr = runReceiptOCR(ctx, data, format)
		if extractErr == nil {
			extracted = parseReceiptFields(ocrText)
		}
	}
	if extractErr == nil {
		draft = mergeReceiptDraft(draft, extracted)
	} else {
		slog.Warn("cash receipt extraction failed", "source", kind, "error", extractErr)
	}
	extension := ".pdf"
	if kind == "image" {
		extension = ".jpg"
		if draft.MediaType == "image/png" {
			extension = ".png"
		}
	}
	relativePath := filepath.Join("comprovantes", digest+extension)
	absolutePath := filepath.Join(s.cfg.DataDir, relativePath)
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o700); err != nil {
		return "", true, err
	}
	if err := os.WriteFile(absolutePath, data, 0o600); err != nil {
		return "", true, err
	}
	draft.FilePath = relativePath
	if draft.Method == "" || draft.Cents <= 0 || draft.Date == "" {
		slog.Info("cash receipt fields incomplete", "source", kind, "method_found", draft.Method != "", "amount_found", draft.Cents > 0, "date_found", draft.Date != "")
		session := conversation.Session{User: sender, Type: "CASH_RECEIPT_REVIEW", Status: conversation.Active}
		payload, _ := json.Marshal(draft)
		session.Payload = string(payload)
		if err := repo.Save(ctx, session); err != nil {
			s.removePendingReceiptFile(draft.FilePath)
			return "", true, err
		}
		return receiptReviewPrompt(draft), true, nil
	}
	session := conversation.Session{User: sender, Type: "CASH_RECEIPT_DESCRIPTION", Status: conversation.Active}
	payload, _ := json.Marshal(draft)
	session.Payload = string(payload)
	if err := repo.Save(ctx, session); err != nil {
		s.removePendingReceiptFile(draft.FilePath)
		return "", true, err
	}
	return receiptSummary(draft) + "\n\nDigite a descrição manual do lançamento, por exemplo: `hospedagem quarto 104`.", true, nil
}

func (s *Service) handleCashReceiptText(ctx context.Context, sender, text string, chat types.JID) (string, bool, error) {
	repo := conversation.NewRepository(s.domainDB)
	session, active, err := repo.Active(ctx, sender)
	if err != nil || !active || !strings.HasPrefix(session.Type, "CASH_RECEIPT_") {
		return "", false, err
	}
	input := strings.TrimSpace(text)
	if utils.Normalize(input) == "cancelar comprovante" || utils.Normalize(input) == "cancelar" {
		var draft receiptDraft
		_ = json.Unmarshal([]byte(session.Payload), &draft)
		s.removePendingReceiptFile(draft.FilePath)
		session.Status = "COMPLETED"
		if err := repo.Save(ctx, session); err != nil {
			return "", true, err
		}
		return "Comprovante cancelado; nada foi lançado no caixa.", true, nil
	}
	var draft receiptDraft
	if err := json.Unmarshal([]byte(session.Payload), &draft); err != nil {
		return "", true, err
	}
	switch session.Type {
	case "CASH_RECEIPT_REVIEW", "CASH_RECEIPT_MANUAL":
		if strings.EqualFold(input, "confirmar") || input == "1" {
			if draft.Method == "" || draft.Cents <= 0 || draft.Date == "" {
				return receiptReviewPrompt(draft), true, nil
			}
			session.Type = "CASH_RECEIPT_DESCRIPTION"
			session.Payload = marshalReceiptDraft(draft)
			if err := repo.Save(ctx, session); err != nil {
				return "", true, err
			}
			return receiptSummary(draft) + "\n\nDigite a descrição manual do lançamento, por exemplo: `hospedagem quarto 104`.", true, nil
		}
		updated, ok := applyReceiptCorrections(draft, input)
		if !ok {
			return receiptReviewPrompt(draft), true, nil
		}
		draft = updated
		session.Type = "CASH_RECEIPT_REVIEW"
		session.Payload = marshalReceiptDraft(draft)
		if err := repo.Save(ctx, session); err != nil {
			return "", true, err
		}
		return receiptReviewPrompt(draft), true, nil
	case "CASH_RECEIPT_DESCRIPTION":
		if len([]rune(input)) < 3 || len([]rune(input)) > 120 {
			return "A descrição deve ter de 3 a 120 caracteres.", true, nil
		}
		draft.Text = input
		session.Type = "CASH_RECEIPT_CONFIRM"
		payload, _ := json.Marshal(draft)
		session.Payload = string(payload)
		if err := repo.Save(ctx, session); err != nil {
			return "", true, err
		}
		return receiptSummary(draft) + "\nDescrição: " + draft.Text + "\n\nAdicionar ao caixa do dia " + formatReceiptDate(draft.Date) + "? Responda `1` para confirmar ou `2` para cancelar.", true, nil
	case "CASH_RECEIPT_CONFIRM":
		if input != "1" && input != "2" {
			return "Responda `1` para lançar ou `2` para cancelar.", true, nil
		}
		if input == "2" {
			s.removePendingReceiptFile(draft.FilePath)
			session.Status = "COMPLETED"
			if err := repo.Save(ctx, session); err != nil {
				return "", true, err
			}
			return "Comprovante cancelado; nada foi lançado no caixa.", true, nil
		}
		if err := s.cash.AddReceipt(ctx, draft.Date, draft.Method, draft.Cents, draft.Text, sender, draft.Hash, "receipt:"+draft.Hash, draft.FilePath, draft.MediaType); err != nil {
			if errors.Is(err, cash.ErrReceiptDuplicate) {
				session.Status = "COMPLETED"
				_ = repo.Save(ctx, session)
				return "Esse comprovante já foi lançado; evitei duplicar o movimento.", true, nil
			}
			if errors.Is(err, cash.ErrCashClosed) {
				return "O caixa dessa data está fechado. Reabra esse dia com auditoria e depois tente confirmar novamente.", true, nil
			}
			return "", true, err
		}
		session.Status = "COMPLETED"
		if err := repo.Save(ctx, session); err != nil {
			return "", true, err
		}
		return "✅ Lançamento registrado no caixa de " + formatReceiptDate(draft.Date) + ".\n" + draft.Method + " — " + cash.Money(draft.Cents) + "\nDescrição: " + draft.Text, true, nil
	default:
		return "Etapa do comprovante inválida. Envie `cancelar comprovante` e tente novamente.", true, nil
	}
}

func (s *Service) removePendingReceiptFile(relativePath string) {
	if relativePath == "" {
		return
	}
	base, err := filepath.Abs(filepath.Join(s.cfg.DataDir, "comprovantes"))
	if err != nil {
		return
	}
	target, err := filepath.Abs(filepath.Join(s.cfg.DataDir, relativePath))
	if err != nil {
		return
	}
	rel, err := filepath.Rel(base, target)
	if err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
		_ = os.Remove(target)
	}
}

func receiptSummary(d receiptDraft) string {
	method := d.Method
	if method == "CARTAO" {
		method = "CARTÃO"
	}
	amount := "não identificado"
	if d.Cents > 0 {
		amount = cash.Money(d.Cents)
	}
	date := "não identificada"
	if d.Date != "" {
		date = formatReceiptDate(d.Date)
	}
	if method == "" {
		method = "não identificado"
	}
	return fmt.Sprintf("Comprovante identificado:\n\nMétodo: %s\nValor: %s\nData: %s", method, amount, date)
}

func receiptReviewPrompt(d receiptDraft) string {
	return "Conferi o comprovante; veja o que consegui identificar:\n\n" + receiptSummary(d) + "\n\n" +
		"Responda `confirmar` se estiver correto. Para corrigir, envie um ou mais campos: `método PIX`, `valor 24,00`, `data 21/09/2026`. Também aceito os três juntos: `PIX 24,00 21/09/2026`.\n" +
		"Se algum campo aparecer como `não identificado`, informe só aquele campo. Para desistir: `cancelar comprovante`."
}

func marshalReceiptDraft(d receiptDraft) string {
	payload, _ := json.Marshal(d)
	return string(payload)
}

func mergeReceiptDraft(base, extracted receiptDraft) receiptDraft {
	base.Method, base.Cents, base.Date = extracted.Method, extracted.Cents, extracted.Date
	return base
}

func parseReceiptFields(text string) receiptDraft {
	draft := receiptDraft{}
	normalized := utils.Normalize(text)
	if strings.Contains(normalized, "pix") {
		draft.Method = "PIX"
	} else if strings.Contains(normalized, "debito") || strings.Contains(normalized, "credito") || strings.Contains(normalized, "mastercard") || strings.Contains(normalized, "visa") || strings.Contains(normalized, "elo") || strings.Contains(normalized, "cartao") {
		draft.Method = "CARTAO"
	}
	moneyMatches := receiptMoneyRE.FindAllStringSubmatch(text, -1)
	if len(moneyMatches) == 0 {
		moneyMatches = receiptLooseMoneyRE.FindAllStringSubmatch(text, -1)
	}
	amounts := map[int64]bool{}
	for _, match := range moneyMatches {
		if cents, ok := parseReceiptMoney(match[1]); ok && cents > 0 {
			amounts[cents] = true
		}
	}
	if len(amounts) == 1 {
		for amount := range amounts {
			draft.Cents = amount
		}
	} else if draft.Method == "CARTAO" {
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if strings.Contains(utils.Normalize(line), "valor venda") {
				for j := i; j < len(lines) && j <= i+2; j++ {
					matches := receiptMoneyRE.FindStringSubmatch(lines[j])
					if len(matches) != 2 {
						matches = receiptLooseMoneyRE.FindStringSubmatch(lines[j])
					}
					if len(matches) == 2 {
						if cents, ok := parseReceiptMoney(matches[1]); ok {
							draft.Cents = cents
							break
						}
					}
				}
			}
		}
	}
	dateMatches := receiptDateRE.FindAllStringSubmatch(text, -1)
	dates := map[string]bool{}
	for _, match := range dateMatches {
		dateText := match[1]
		if len(strings.Split(dateText, "/")[2]) == 2 {
			dateText = dateText[:6] + "20" + dateText[6:]
		}
		parsed, err := time.Parse("02/01/2006", dateText)
		if err == nil && parsed.Year() >= 2020 && !parsed.After(time.Now().AddDate(0, 0, 1)) {
			dates[parsed.Format("2006-01-02")] = true
		}
	}
	if len(dates) == 1 {
		for date := range dates {
			draft.Date = date
		}
	}
	return draft
}

func parseReceiptPDF(data []byte) (receiptDraft, error) {
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return receiptDraft{}, errors.New("arquivo invÃ¡lido; envie PDF legÃ­vel")
	}
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return receiptDraft{}, err
	}
	plain, err := reader.GetPlainText()
	if err != nil {
		return receiptDraft{}, err
	}
	text, err := io.ReadAll(io.LimitReader(plain, 2<<20))
	if err != nil {
		return receiptDraft{}, err
	}
	content := string(text)
	if strings.TrimSpace(content) == "" {
		return receiptDraft{}, errors.New("PDF escaneado sem camada de texto; envie uma foto ou PDF com texto")
	}
	return parseReceiptFields(content), nil
}

func applyReceiptCorrections(d receiptDraft, input string) (receiptDraft, bool) {
	if method, cents, date, ok := parseManualReceipt(input); ok {
		d.Method, d.Cents, d.Date = method, cents, date
		return d, true
	}
	// Quando somente um campo ficou sem leitura, aceite apenas o valor dele.
	// O resumo orienta o operador a informar "só aquele campo", então exigir
	// novamente método/valor/data ou o nome do campo contradiz o próprio fluxo.
	missing := 0
	if d.Method == "" {
		missing++
	}
	if d.Cents <= 0 {
		missing++
	}
	if d.Date == "" {
		missing++
	}
	if missing == 1 {
		value := strings.TrimSpace(input)
		switch {
		case d.Method == "":
			switch utils.Normalize(value) {
			case "pix":
				d.Method = "PIX"
				return d, true
			case "cartao":
				d.Method = "CARTAO"
				return d, true
			}
		case d.Cents <= 0:
			value = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(value), "R$"))
			if cents, ok := parseReceiptMoney(value); ok && cents > 0 {
				d.Cents = cents
				return d, true
			}
		case d.Date == "":
			if parsed, err := time.Parse("02/01/2006", value); err == nil && parsed.Format("02/01/2006") == value {
				d.Date = parsed.Format("2006-01-02")
				return d, true
			}
		}
	}
	parts := strings.Split(input, ";")
	if len(parts) == 1 {
		parts = strings.Split(input, "\n")
	}
	updated := false
	for _, part := range parts {
		field := strings.TrimSpace(part)
		lower := utils.Normalize(field)
		switch {
		case strings.HasPrefix(lower, "metodo ") || strings.HasPrefix(lower, "forma "):
			value := strings.TrimSpace(field[strings.Index(field, " ")+1:])
			switch utils.Normalize(value) {
			case "pix":
				d.Method, updated = "PIX", true
			case "cartao":
				d.Method, updated = "CARTAO", true
			default:
				return d, false
			}
		case strings.HasPrefix(lower, "valor "):
			value := strings.TrimSpace(field[strings.Index(field, " ")+1:])
			value = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(value), "R$"))
			cents, ok := parseReceiptMoney(value)
			if !ok || cents <= 0 {
				return d, false
			}
			d.Cents, updated = cents, true
		case strings.HasPrefix(lower, "data "):
			value := strings.TrimSpace(field[strings.Index(field, " ")+1:])
			parsed, err := time.Parse("02/01/2006", value)
			if err != nil || parsed.Format("02/01/2006") != value {
				return d, false
			}
			d.Date, updated = parsed.Format("2006-01-02"), true
		default:
			return d, false
		}
	}
	return d, updated
}

func formatReceiptDate(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.Format("02/01/2006")
}

func parseManualReceipt(text string) (string, int64, string, bool) {
	parts := strings.Fields(text)
	if len(parts) != 3 {
		return "", 0, "", false
	}
	method := utils.Normalize(parts[0])
	if method == "cartao" {
		method = "CARTAO"
	} else if method == "pix" {
		method = "PIX"
	} else {
		return "", 0, "", false
	}
	cents, ok := parseReceiptMoney(parts[1])
	if !ok || cents <= 0 {
		return "", 0, "", false
	}
	date, err := time.Parse("02/01/2006", parts[2])
	if err != nil || date.Format("02/01/2006") != parts[2] {
		return "", 0, "", false
	}
	return method, cents, date.Format("2006-01-02"), true
}

func parseReceiptText(text string) (string, int64, string, bool) {
	normalized := utils.Normalize(text)
	method := ""
	if strings.Contains(normalized, "pix") {
		method = "PIX"
	}
	if strings.Contains(normalized, "debito") || strings.Contains(normalized, "credito") || strings.Contains(normalized, "mastercard") || strings.Contains(normalized, "visa") || strings.Contains(normalized, "elo") {
		if method != "" {
			return "", 0, "", false
		}
		method = "CARTAO"
	}
	if method == "" {
		return "", 0, "", false
	}
	moneyMatches := receiptMoneyRE.FindAllStringSubmatch(text, -1)
	amounts := map[int64]bool{}
	for _, match := range moneyMatches {
		if cents, ok := parseReceiptMoney(match[1]); ok && cents > 0 {
			amounts[cents] = true
		}
	}
	if method == "CARTAO" {
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if strings.Contains(utils.Normalize(line), "valor venda") {
				for j := i; j < len(lines) && j <= i+2; j++ {
					if m := receiptMoneyRE.FindStringSubmatch(lines[j]); len(m) == 2 {
						if cents, ok := parseReceiptMoney(m[1]); ok {
							amounts = map[int64]bool{cents: true}
							break
						}
					}
				}
			}
		}
	}
	if len(amounts) != 1 {
		return "", 0, "", false
	}
	var cents int64
	for candidate := range amounts {
		cents = candidate
	}
	dateMatches := receiptDateRE.FindAllStringSubmatch(text, -1)
	dates := map[string]bool{}
	for _, match := range dateMatches {
		dateText := match[1]
		if len(strings.Split(dateText, "/")[2]) == 2 {
			dateText = dateText[:6] + "20" + dateText[6:]
		}
		parsed, err := time.Parse("02/01/2006", dateText)
		if err == nil && parsed.Year() >= 2020 && !parsed.After(time.Now().AddDate(0, 0, 1)) {
			dates[parsed.Format("2006-01-02")] = true
		}
	}
	if len(dates) != 1 {
		return "", 0, "", false
	}
	var date string
	for candidate := range dates {
		date = candidate
	}
	return method, cents, date, true
}

func parseReceiptMoney(text string) (int64, bool) {
	text = strings.TrimSpace(strings.ReplaceAll(text, ".", ""))
	parts := strings.Split(text, ",")
	if len(parts) != 2 || len(parts[1]) != 2 {
		return 0, false
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, false
	}
	fraction, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return whole*100 + fraction, true
}

func runReceiptOCR(ctx context.Context, data []byte, format string) (string, error) {
	tesseractPath, err := findReceiptOCRBinary(os.Getenv("PATH"))
	if err != nil {
		return "", errors.New("OCR Tesseract não instalado; instale com pkg install tesseract")
	}
	work, err := os.MkdirTemp("", "hotel-receipt-ocr-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	path := filepath.Join(work, "receipt."+format)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	var results []string
	var failures []string
	ocrCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	for _, segmentation := range []string{"6", "11", "4"} {
		command := exec.CommandContext(ocrCtx, tesseractPath, path, "stdout", "-l", "eng", "--psm", segmentation, "-c", "preserve_interword_spaces=1")
		output, runErr := command.Output()
		if runErr != nil {
			failures = append(failures, fmt.Sprintf("PSM %s: %v", segmentation, runErr))
			continue
		}
		results = append(results, string(output))
	}
	if len(results) == 0 {
		return "", fmt.Errorf("tesseract OCR failed: %s", strings.Join(failures, "; "))
	}
	return mergeReceiptOCRResults(results), nil
}

// findReceiptOCRBinary deliberately avoids exec.LookPath. Termux runs this
// ARMv7 executable as GOOS=linux; Go's Unix LookPath uses faccessat2 there,
// which Android's seccomp policy kills with SIGSYS instead of returning EPERM.
// Resolve using stat and pass an absolute path to exec.CommandContext so it
// does not perform a second LookPath internally.
func findReceiptOCRBinary(pathEnv string) (string, error) {
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, "tesseract")
		info, err := os.Stat(candidate)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			continue
		}
		return candidate, nil
	}
	return "", exec.ErrNotFound
}

func mergeReceiptOCRResults(results []string) string {
	methods := map[string]bool{}
	amounts := map[int64]bool{}
	dates := map[string]bool{}
	for _, result := range results {
		draft := parseReceiptFields(result)
		if draft.Method != "" {
			methods[draft.Method] = true
		}
		if draft.Cents > 0 {
			amounts[draft.Cents] = true
		}
		if draft.Date != "" {
			dates[draft.Date] = true
		}
	}
	var combined strings.Builder
	if len(methods) == 1 {
		for method := range methods {
			combined.WriteString(method + "\n")
		}
	}
	if len(amounts) == 1 {
		for cents := range amounts {
			fmt.Fprintf(&combined, "R$ %d,%02d\n", cents/100, cents%100)
		}
	}
	if len(dates) == 1 {
		for date := range dates {
			if parsed, err := time.Parse("2006-01-02", date); err == nil {
				combined.WriteString(parsed.Format("02/01/2006") + "\n")
			}
		}
	}
	return combined.String()
}

var _ whatsmeow.DownloadableMessage = (*waE2E.ImageMessage)(nil)
