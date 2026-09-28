package cash

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/database"
)

func TestCashTotalsAndPDF(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	repo := NewRepository(db, time.UTC)
	if err = repo.Open(ctx, 19185, "u", now); err != nil {
		t.Fatal(err)
	}
	if err = repo.Add(ctx, "ENTRY", "DINHEIRO", 210000, "hospedagem qto 104", "u", "1", now); err != nil {
		t.Fatal(err)
	}
	if err = repo.Add(ctx, "ENTRY", "PIX", 10000, "hospedagem qto 101", "u", "2", now); err != nil {
		t.Fatal(err)
	}
	if err = repo.Add(ctx, "EXIT", "", 5000, "material", "u", "3", now); err != nil {
		t.Fatal(err)
	}
	day, err := repo.Today(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if day.EntryTotal() != 220000 || day.ExitTotal() != 5000 || day.FinalCents() != 234185 {
		t.Fatalf("day=%#v final=%d", day, day.FinalCents())
	}
	if day.MethodTotal("DINHEIRO") != 210000 || day.MethodTotal("PIX") != 10000 || day.MethodTotal("CARTAO") != 0 {
		t.Fatalf("method totals incorrect")
	}
	if day.FinalMethodTotal("DINHEIRO") != 224185 || day.FinalMethodTotal("PIX") != 10000 || day.FinalMethodTotal("CARTAO") != 0 {
		t.Fatalf("final method totals incorrect")
	}
	if got := movementText(day.Exits[0]); got != "R$ 50,00 - material" {
		t.Fatalf("exit line=%q", got)
	}
	data, err := PDF(day, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatal("invalid pdf")
	}
	for _, want := range [][]byte{[]byte("CAIXA FINAL"), []byte("DINHEIRO"), []byte("CART"), []byte("PIX"), []byte("2241,85")} {
		if !bytes.Contains(data, want) {
			t.Fatalf("pdf missing %q", want)
		}
	}
}

func TestMoney(t *testing.T) {
	if got := Money(210050); got != "R$ 2100,50" {
		t.Fatalf("got %s", got)
	}
}

func TestRangePDFContainsEachDay(t *testing.T) {
	days := []Day{{Date: "2026-09-18", OpeningCents: 10000}, {Date: "2026-09-19", OpeningCents: 20000}}
	data, err := RangePDF(days, time.UTC)
	if err != nil || !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("range pdf: %v", err)
	}
	for _, date := range [][]byte{[]byte("18/09/2026"), []byte("19/09/2026")} {
		if !bytes.Contains(data, date) {
			t.Fatalf("missing date %s", date)
		}
	}
}

func TestRangeWithSingleDatabaseConnection(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewRepository(db, time.UTC)
	date := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	if err := repo.Open(ctx, 10000, "u", date); err != nil {
		t.Fatal(err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	days, err := repo.Range(queryCtx, "2026-09-18", "2026-09-18")
	if err != nil || len(days) != 1 || days[0].Date != "2026-09-18" {
		t.Fatalf("days=%#v err=%v", days, err)
	}
}

func TestCloseLocksAndReopenAudits(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	repo := NewRepository(db, time.UTC)
	if err := repo.Open(ctx, 10000, "u", now); err != nil {
		t.Fatal(err)
	}
	if err := repo.Add(ctx, "ENTRY", "PIX", 5000, "hospedagem", "u", "1", now); err != nil {
		t.Fatal(err)
	}
	day, err := repo.CloseToday(ctx, "u", now)
	if err != nil || day.ClosedAt == "" {
		t.Fatalf("day=%#v err=%v", day, err)
	}
	if err := repo.Add(ctx, "EXIT", "", 1000, "material", "u", "2", now); !errors.Is(err, ErrCashClosed) {
		t.Fatalf("add err=%v", err)
	}
	if err := repo.EditToday(ctx, 1, "ENTRY", "PIX", 6000, "editada", "u", now); !errors.Is(err, ErrCashClosed) {
		t.Fatalf("edit err=%v", err)
	}
	if err := repo.ReopenToday(ctx, "u", now); err != nil {
		t.Fatal(err)
	}
	if err := repo.Add(ctx, "EXIT", "", 1000, "material", "u", "2", now); err != nil {
		t.Fatal(err)
	}
	var audits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cash_day_audit`).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
	days, err := repo.Range(ctx, "2026-08-01", "2026-08-31")
	if err != nil || len(days) != 1 {
		t.Fatalf("days=%#v err=%v", days, err)
	}
}

func TestReceiptStoresAndQueriesOriginalMetadata(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewRepository(db, time.UTC)
	hash := strings.Repeat("a", 64)
	if err := repo.AddReceipt(ctx, "2026-09-21", "PIX", 2400, "hospedagem 101", "u", hash, "receipt:"+hash, filepath.Join("comprovantes", hash+".pdf"), "application/pdf"); err != nil {
		t.Fatal(err)
	}
	items, err := repo.ReceiptsByDate(ctx, "2026-09-21")
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	got, err := repo.Receipt(ctx, items[0].ID)
	if err != nil || got.Cents != 2400 || got.FilePath == "" || got.MediaType != "application/pdf" {
		t.Fatalf("receipt=%#v err=%v", got, err)
	}
}
