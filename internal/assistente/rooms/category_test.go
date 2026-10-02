package rooms_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
)

func TestBitzCandidatesExcludeMaintenanceRooms(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := rooms.NewRepository(db)
	for _, number := range []int{201, 202} {
		if err := repo.SetBitzCategory(ctx, number, "triploDeluxe"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := repo.UpdateStatus(ctx, 201, rooms.Maintenance, "test", "test", "maintenance-201"); err != nil {
		t.Fatal(err)
	}
	configured, eligible, err := repo.BitzAllocationCandidates(ctx, "triploDeluxe")
	if err != nil {
		t.Fatal(err)
	}
	if len(configured) != 2 || len(eligible) != 1 || eligible[0] != 202 {
		t.Fatalf("configured=%v eligible=%v", configured, eligible)
	}
}

func TestRejectsUnknownBitzCategory(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := rooms.NewRepository(db).SetBitzCategory(ctx, 101, "texto-livre"); err == nil {
		t.Fatal("unknown category accepted")
	}
}
