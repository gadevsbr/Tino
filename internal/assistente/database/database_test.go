package database

import (
	"context"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	"path/filepath"
	"testing"
)

func TestSeedExactlyOfficialRoomsAndIdempotent(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := SeedRooms(ctx, db); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT number FROM rooms ORDER BY number`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		got = append(got, n)
	}
	if len(got) != 42 {
		t.Fatalf("got %d rooms", len(got))
	}
	for i, n := range rooms.OfficialNumbers {
		if got[i] != n {
			t.Fatalf("room %d=%d want %d", i, got[i], n)
		}
	}
}
