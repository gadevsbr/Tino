package main

import (
	"context"
	"github.com/gadevsbr/tino/internal/assistente/bills"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"path/filepath"
	"testing"
)

func TestBillsDesktopUsesPersistentOperationalDatabase(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := &App{ctx: ctx, hotel: &hotelRuntime{db: db}}
	b, err := a.AddBill(BillInput{Description: "Energia", Amount: "1.250,90", DueDate: "2026-10-20"})
	if err != nil || b.AmountCents != 125090 {
		t.Fatal(b, err)
	}
	if err := a.SaveBillReminders(bills.Settings{Enabled: true, Target: "5573999999999", DaysBefore: 3, Hour: 9}); err != nil {
		t.Fatal(err)
	}
	view, err := a.GetBills()
	if err != nil || len(view.Bills) != 1 || !view.Settings.Enabled {
		t.Fatal(view, err)
	}
	if err := a.SetBillStatus(b.ID, "PAID"); err != nil {
		t.Fatal(err)
	}
	view, err = a.GetBills()
	if err != nil || view.Bills[0].Status != "PAID" || view.Bills[0].PaidAt == "" {
		t.Fatal(view, err)
	}
}
