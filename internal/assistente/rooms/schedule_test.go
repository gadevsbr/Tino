package rooms_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
)

func TestScheduledEntryBecomesCleaningThenCheckout(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := rooms.NewRepository(db, time.UTC)
	entry := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	if _, _, err = repo.ScheduleEntry(ctx, 101, 3, "2026-11-12", "operator", "test", "entry-1", entry); err != nil {
		t.Fatal(err)
	}
	if changed, err := repo.ApplyScheduledStatuses(ctx, entry); err != nil || changed != 0 {
		t.Fatalf("entry day changed=%d err=%v", changed, err)
	}
	if changed, err := repo.ApplyScheduledStatuses(ctx, time.Date(2026, 11, 11, 0, 1, 0, 0, time.UTC)); err != nil || changed != 1 {
		t.Fatalf("cleaning changed=%d err=%v", changed, err)
	}
	room, _ := repo.Get(ctx, 101)
	if room.Status != rooms.OccupiedClean || room.GuestCount != 3 {
		t.Fatalf("cleaning room=%#v", room)
	}
	if changed, err := repo.ApplyScheduledStatuses(ctx, time.Date(2026, 11, 12, 11, 59, 0, 0, time.UTC)); err != nil || changed != 0 {
		t.Fatalf("before checkout changed=%d err=%v", changed, err)
	}
	if changed, err := repo.ApplyScheduledStatuses(ctx, time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)); err != nil || changed != 1 {
		t.Fatalf("checkout changed=%d err=%v", changed, err)
	}
	room, _ = repo.Get(ctx, 101)
	if room.Status != rooms.CheckoutToday || room.GuestCount != 0 {
		t.Fatalf("checkout room=%#v", room)
	}
}

func TestManualStatusCancelsScheduledStay(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := rooms.NewRepository(db, time.UTC)
	entry := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	if _, _, err = repo.ScheduleEntry(ctx, 101, 2, "2026-11-15", "operator", "test", "entry-2", entry); err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.UpdateStatus(ctx, 101, rooms.AvailableClean, "operator", "test", "manual-1"); err != nil {
		t.Fatal(err)
	}
	if changed, err := repo.ApplyScheduledStatuses(ctx, entry.AddDate(0, 0, 2)); err != nil || changed != 0 {
		t.Fatalf("stale schedule changed=%d err=%v", changed, err)
	}
	room, _ := repo.Get(ctx, 101)
	if room.Status != rooms.AvailableClean {
		t.Fatalf("manual status overwritten: %#v", room)
	}
}
