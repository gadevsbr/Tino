package rooms_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
)

func TestCleaningCompletionBlocksSameDayAndAllowsNextDay(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := rooms.NewRepository(db, time.UTC)
	if _, _, err = repo.UpdateStatusWithGuests(ctx, 101, rooms.OccupiedClean, 2, "u", "test", "1"); err != nil {
		t.Fatal(err)
	}
	if err = repo.CompleteCleaning(ctx, 101, "u", "2", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.UpdateStatusWithGuests(ctx, 101, rooms.OccupiedClean, 2, "u", "test", "3"); !errors.Is(err, rooms.ErrCleaningCompletedToday) {
		t.Fatalf("err=%v", err)
	}
	if _, err = db.Exec(`UPDATE rooms SET cleaning_completed_date=? WHERE number=101`, time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	changed, _, err := repo.UpdateStatusWithGuests(ctx, 101, rooms.OccupiedClean, 2, "u", "test", "4")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
}
