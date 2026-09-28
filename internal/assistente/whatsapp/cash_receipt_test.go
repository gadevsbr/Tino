package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/cash"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/go-pdf/fpdf"
	"go.mau.fi/whatsmeow/types"
)

func TestParseReceiptFieldsInterPixExample(t *testing.T) {
	date := time.Now().Format("02/01/2006")
	text := fmt.Sprintf("Inter\nPix enviado\nR$ 30,00\nSobre a transação\nData do pagamento\nSexta, %s\nHorário\n08h41\nID da transação\nE0041698202609251139M7ZB6VcNWY\nQuem recebeu\nNome\nBrena Guerra da Rocha\nInstituição\nBco Bradesco S.A.\nQuem pagou\nNome\nMIKELLY VITORIA OLIVEIRA JUNKER\nInstituição\nBanco Inter S.A.", date)

	draft := parseReceiptFields(text)
	if draft.Method != "PIX" {
		t.Errorf("method = %q, want PIX", draft.Method)
	}
	if draft.Cents != 3000 {
		t.Errorf("amount = %d cents, want 3000", draft.Cents)
	}
	if draft.Date != time.Now().Format("2006-01-02") {
		t.Errorf("date = %q, want today's date", draft.Date)
	}
}

func TestParseReceiptFieldsCardThousands(t *testing.T) {
	date := time.Now().Format("02/01/2006")
	draft := parseReceiptFields(fmt.Sprintf("VISA CREDITO\nVALOR VENDA\nR$ 1.265,80\nDATA %s", date))
	if draft.Method != "CARTAO" || draft.Cents != 126580 || draft.Date != time.Now().Format("2006-01-02") {
		t.Fatalf("unexpected extracted receipt: %#v", draft)
	}
}

func TestParseReceiptFieldsKeepsPartialValuesAndRejectsAmbiguity(t *testing.T) {
	draft := parseReceiptFields("Pix enviado\nR$ 30,00\nData indisponível")
	if draft.Method != "PIX" || draft.Cents != 3000 || draft.Date != "" {
		t.Fatalf("expected partial PIX and amount only, got %#v", draft)
	}

	draft = parseReceiptFields("Pix enviado\nR$ 30,00\nR$ 50,00\nData 25/09/2026")
	if draft.Method != "PIX" || draft.Cents != 0 {
		t.Fatalf("ambiguous amounts must not be guessed, got %#v", draft)
	}
}

func TestMergeReceiptOCRResultsCombinesFieldsAcrossLayouts(t *testing.T) {
	date := time.Now().Format("02/01/2006")
	merged := mergeReceiptOCRResults([]string{
		"Pix enviado\nR$ 30,00",
		fmt.Sprintf("Data do pagamento\nSexta, %s", date),
	})
	draft := parseReceiptFields(merged)
	if draft.Method != "PIX" || draft.Cents != 3000 || draft.Date != time.Now().Format("2006-01-02") {
		t.Fatalf("did not combine unambiguous fields from OCR layouts: %#v (text %q)", draft, merged)
	}
}

func TestMergeReceiptOCRResultsDoesNotChooseConflictingAmounts(t *testing.T) {
	merged := mergeReceiptOCRResults([]string{"Pix enviado\nR$ 30,00", "Pix enviado\nR$ 80,00"})
	draft := parseReceiptFields(merged)
	if draft.Cents != 0 {
		t.Fatalf("conflicting OCR amounts must remain unknown, got %#v", draft)
	}
}

func TestParseReceiptPDFTextLayer(t *testing.T) {
	date := time.Now().Format("02/01/2006")
	pdfBytes := makeReceiptPDF(t, fmt.Sprintf("Inter\nPix enviado\nR$ 30,00\nData do pagamento\nSexta, %s\nID da transação\nE0041698202609251139M7ZB6VcNWY", date))

	draft, err := parseReceiptPDF(pdfBytes)
	if err != nil {
		t.Fatalf("parseReceiptPDF returned error: %v", err)
	}
	if draft.Method != "PIX" || draft.Cents != 3000 || draft.Date != time.Now().Format("2006-01-02") {
		t.Fatalf("unexpected extracted receipt: %#v", draft)
	}
}

func TestParseReceiptPDFRejectsInvalidAndScannedFiles(t *testing.T) {
	if _, err := parseReceiptPDF([]byte("not a PDF")); err == nil {
		t.Fatal("expected invalid PDF error")
	}
	if _, err := parseReceiptPDF(makeReceiptPDF(t, "")); err == nil {
		t.Fatal("expected empty text layer error for scanned/blank PDF")
	}
}

func TestApplyReceiptCorrectionsChangesOnlyRequestedField(t *testing.T) {
	draft := receiptDraft{Method: "PIX", Cents: 3000, Date: "2026-09-25"}
	updated, ok := applyReceiptCorrections(draft, "valor R$ 45,90")
	if !ok {
		t.Fatal("expected amount correction to be accepted")
	}
	if updated.Method != "PIX" || updated.Cents != 4590 || updated.Date != draft.Date {
		t.Fatalf("unrequested fields changed: %#v", updated)
	}
}

func TestApplyReceiptCorrectionsAcceptsOnlyMissingFieldValue(t *testing.T) {
	tests := []struct {
		name  string
		draft receiptDraft
		input string
		want  receiptDraft
	}{
		{
			name:  "date only",
			draft: receiptDraft{Method: "PIX", Cents: 2400},
			input: "21/09/2026",
			want:  receiptDraft{Method: "PIX", Cents: 2400, Date: "2026-09-21"},
		},
		{
			name:  "amount only",
			draft: receiptDraft{Method: "PIX", Date: "2026-09-21"},
			input: "24,00",
			want:  receiptDraft{Method: "PIX", Cents: 2400, Date: "2026-09-21"},
		},
		{
			name:  "method only",
			draft: receiptDraft{Cents: 2400, Date: "2026-09-21"},
			input: "PIX",
			want:  receiptDraft{Method: "PIX", Cents: 2400, Date: "2026-09-21"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := applyReceiptCorrections(tt.draft, tt.input)
			if !ok {
				t.Fatalf("single missing field value %q was rejected", tt.input)
			}
			if got.Method != tt.want.Method || got.Cents != tt.want.Cents || got.Date != tt.want.Date {
				t.Fatalf("updated receipt = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestFindReceiptOCRBinaryResolvesWithoutLookPath(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "tesseract")
	if err := os.WriteFile(want, []byte("test executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := findReceiptOCRBinary(dir)
	if err != nil {
		t.Fatalf("findReceiptOCRBinary returned error: %v", err)
	}
	if got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}
}

func TestFindReceiptOCRBinarySkipsNonExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use Unix executable permission bits")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tesseract"), []byte("not executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := findReceiptOCRBinary(dir); err == nil {
		t.Fatal("expected non-executable file to be skipped")
	}
}

func TestReceiptPDFIsStoredAndLinkedAfterConfirmation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(dir, "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Service{domainDB: db, cash: cash.NewRepository(db, time.UTC)}
	s.cfg.DataDir = dir
	s.cfg.Timezone = time.UTC
	pdf := makeReceiptPDF(t, "Pix enviado\nR$ 24,00\nData do pagamento\n21/09/2026")
	reply, handled, err := s.processCashReceipt(ctx, "operator", pdf, "pdf")
	if err != nil || !handled || !strings.Contains(reply, "descrição") {
		t.Fatalf("receive reply=%q handled=%t err=%v", reply, handled, err)
	}
	session, active, err := conversation.NewRepository(db).Active(ctx, "operator")
	if err != nil || !active {
		t.Fatalf("session active=%t err=%v", active, err)
	}
	var draft receiptDraft
	if err := json.Unmarshal([]byte(session.Payload), &draft); err != nil {
		t.Fatal(err)
	}
	if draft.FilePath == "" {
		t.Fatal("stored file path missing from receipt draft")
	}
	if _, err := os.Stat(filepath.Join(dir, draft.FilePath)); err != nil {
		t.Fatalf("stored receipt missing: %v", err)
	}
	chat := types.NewJID("557300000000", types.DefaultUserServer)
	if _, handled, err = s.handleCashReceiptText(ctx, "operator", "hospedagem 101", chat); err != nil || !handled {
		t.Fatalf("description handled=%t err=%v", handled, err)
	}
	if _, handled, err = s.handleCashReceiptText(ctx, "operator", "1", chat); err != nil || !handled {
		t.Fatalf("confirmation handled=%t err=%v", handled, err)
	}
	items, err := s.cash.ReceiptsByDate(ctx, "2026-09-21")
	if err != nil || len(items) != 1 || items[0].FilePath != draft.FilePath {
		t.Fatalf("stored items=%#v err=%v", items, err)
	}
}

func makeReceiptPDF(t *testing.T, text string) []byte {
	t.Helper()
	doc := fpdf.New("P", "mm", "A4", "")
	doc.AddPage()
	doc.SetFont("Arial", "", 12)
	for _, line := range bytes.Split([]byte(text), []byte("\n")) {
		doc.Cell(0, 7, string(line))
		doc.Ln(7)
	}
	var out bytes.Buffer
	if err := doc.Output(&out); err != nil {
		t.Fatalf("create test PDF: %v", err)
	}
	return out.Bytes()
}
