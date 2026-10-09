package bills

import (
	"context"
	"errors"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemindersPersistRetryAndStopAfterPayment(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := New(db)
	b, err := r.Add(ctx, "Energia", 12345, "2026-10-11")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Configure(ctx, Settings{Enabled: true, Target: "5573999999999", DaysBefore: 3, Hour: 9}); err != nil {
		t.Fatal(err)
	}
	zone := time.FixedZone("BRT", -3*3600)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	sends := 0
	send := func(_ context.Context, target, text string) error {
		sends++
		if target != "5573999999999" || !strings.Contains(text, "R$ 123,45") {
			t.Fatal(target, text)
		}
		return nil
	}
	if err := r.Remind(ctx, now.Add(-time.Minute), zone, send); err != nil || sends != 0 {
		t.Fatal(err, sends)
	}
	if err := r.Remind(ctx, now, zone, func(context.Context, string, string) error { return errors.New("offline") }); err == nil {
		t.Fatal("failure ignored")
	}
	if err := r.Remind(ctx, now, zone, send); err != nil || sends != 1 {
		t.Fatal(err, sends)
	}
	if err := New(db).Remind(ctx, now.Add(time.Minute), zone, send); err != nil || sends != 1 {
		t.Fatal("duplicate after restart", err, sends)
	}
	if err := r.Remind(ctx, now.AddDate(0, 0, 1), zone, send); err != nil || sends != 2 {
		t.Fatal(err, sends)
	}
	if err := r.SetStatus(ctx, b.ID, "PAID"); err != nil {
		t.Fatal(err)
	}
	if err := r.Remind(ctx, now.AddDate(0, 0, 6), zone, send); err != nil || sends != 2 {
		t.Fatal("paid bill notified", err, sends)
	}
}
func TestNoDestinationNoReminderAndInvalidBillsRejected(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := New(db)
	if _, err := r.Add(ctx, "bad", 100, "2026-02-30"); err == nil {
		t.Fatal("invalid date accepted")
	}
	if err := r.Configure(ctx, Settings{Enabled: true, Target: "", Hour: 9}); err == nil {
		t.Fatal("no destination accepted")
	}
	if err := r.Remind(ctx, time.Now(), time.UTC, func(context.Context, string, string) error { t.Fatal("unexpected send"); return nil }); err != nil {
		t.Fatal(err)
	}
}
