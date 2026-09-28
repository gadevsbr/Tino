package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestClaimMessageIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	first, err := ClaimMessage(ctx, db, "abc", "5511")
	if err != nil || !first {
		t.Fatalf("first=%v err=%v", first, err)
	}
	second, err := ClaimMessage(ctx, db, "abc", "5511")
	if err != nil || second {
		t.Fatalf("second=%v err=%v", second, err)
	}
}
