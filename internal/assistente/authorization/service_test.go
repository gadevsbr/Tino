package authorization

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gadevsbr/tino/internal/assistente/config"
	"github.com/gadevsbr/tino/internal/assistente/database"
)

func TestDynamicAuthorizationIsImmediateAndPersistent(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Config{Authorized: map[string]struct{}{"5573999999999": {}}}
	service, err := New(cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	added, err := service.Add(ctx, "5573999999999", "5573988888888")
	if err != nil || !added || !service.Allowed("5573988888888") {
		t.Fatalf("added=%v allowed=%v err=%v", added, service.Allowed("5573988888888"), err)
	}
	reloaded, err := New(cfg, db)
	if err != nil || !reloaded.Allowed("5573988888888") {
		t.Fatalf("persistent allowed=%v err=%v", reloaded != nil && reloaded.Allowed("5573988888888"), err)
	}
}
