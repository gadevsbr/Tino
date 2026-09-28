package advances

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/cash"
	"github.com/gadevsbr/tino/internal/assistente/database"
)

func TestCreateWithAndWithoutCashAndMonthlyReport(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	repo := NewRepository(db, time.UTC)
	first, err := repo.Create(ctx, "Maria", 15000, "adiantamento", true, "operador", "m1", now)
	if err != nil || !first.DeductCash || !first.CashMovementID.Valid {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	if _, err = repo.Create(ctx, "Maria", 5000, "", false, "operador", "m2", now); err != nil {
		t.Fatal(err)
	}
	day, err := cash.NewRepository(db, time.UTC).Today(ctx, now)
	if err != nil || len(day.Exits) != 1 || day.Exits[0].Cents != 15000 || !strings.Contains(day.Exits[0].Description, "VALE - Maria") {
		t.Fatalf("day=%#v err=%v", day, err)
	}
	items, err := repo.Month(ctx, "Maria", now)
	if err != nil || len(items) != 2 {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	report := Report(items, "09/2026")
	if !strings.Contains(report, "R$ 200,00") || !strings.Contains(report, "não descontado do caixa") {
		t.Fatalf("report=%q", report)
	}
}

func TestPDF(t *testing.T) {
	data, err := PDF([]Advance{{ID: 1, Employee: "Maria", Cents: 15000, Note: "adiantamento", DeductCash: true, Date: "2026-09-15"}}, "09/2026", "", time.UTC)
	if err != nil || !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("pdf err=%v", err)
	}
}
