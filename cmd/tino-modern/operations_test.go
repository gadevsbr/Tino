package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/advances"
	"github.com/gadevsbr/tino/internal/assistente/cash"
	assistdb "github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/extratos"
)

func TestOperationalDashboardReceiptsAndAdvances(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := assistdb.Open(ctx, filepath.Join(dir, "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	zone := time.UTC
	cashRepo := cash.NewRepository(db, zone)
	if err := cashRepo.OpenDate(ctx, "2026-09-28", 10000, "admin"); err != nil {
		t.Fatal(err)
	}
	if err := cashRepo.AddDate(ctx, "2026-09-28", "ENTRY", "PIX", 25000, "hospedagem", "admin", "m1"); err != nil {
		t.Fatal(err)
	}
	if err := cashRepo.AddDate(ctx, "2026-09-28", "EXIT", "", 5000, "compra", "admin", "m2"); err != nil {
		t.Fatal(err)
	}
	receiptDir := filepath.Join(dir, "comprovantes")
	if err := os.MkdirAll(receiptDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := []byte("%PDF-1.4 test")
	path := filepath.Join(receiptDir, "pix.pdf")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(content))
	if err := cashRepo.AddReceipt(ctx, "2026-09-28", "PIX", 1500, "reserva", "admin", hash, "receipt-1", filepath.Join("comprovantes", "pix.pdf"), "application/pdf"); err != nil {
		t.Fatal(err)
	}
	advanceRepo := advances.NewRepository(db, zone)
	if _, err := advanceRepo.Create(ctx, "Maria", 2000, "adiantamento", false, "admin", "a1", time.Date(2026, 9, 28, 12, 0, 0, 0, zone)); err != nil {
		t.Fatal(err)
	}
	a := &App{ctx: ctx, hotel: &hotelRuntime{db: db, dataDir: dir, zone: zone, cash: cashRepo, advances: advanceRepo, extratos: extratos.NewRepository(db)}}
	dashboard, err := a.FinanceDashboard("2026-09-28", "2026-09-28")
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.EntryCents != 26500 || dashboard.ExitCents != 5000 || dashboard.BalanceCents != 31500 || dashboard.Receipts != 1 {
		t.Fatalf("totais inesperados: %#v", dashboard)
	}
	receipts, err := a.ListReceipts("2026-09-28", "2026-09-28")
	if err != nil || len(receipts) != 1 || !receipts[0].HasFile {
		t.Fatalf("comprovantes inesperados: %#v %v", receipts, err)
	}
	preview, err := a.GetReceiptPreview(receipts[0].ID)
	if err != nil || preview.DataURL == "" {
		t.Fatalf("preview ausente: %#v %v", preview, err)
	}
	items, err := a.ListAdvances("2026-09", "Maria")
	if err != nil || len(items) != 1 || items[0].Cents != 2000 {
		t.Fatalf("vales inesperados: %#v %v", items, err)
	}
}

func TestOperationalRangeValidation(t *testing.T) {
	if _, _, err := validRange("2026-09-30", "2026-09-01"); err == nil {
		t.Fatal("intervalo invertido deveria falhar")
	}
	if _, _, err := validRange("", "2026-09-01"); err == nil {
		t.Fatal("intervalo vazio deveria falhar")
	}
}
