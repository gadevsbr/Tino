package backup

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/database"
)

func TestRunCreatesVerifiedSnapshot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(dir, "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := New(db, dir, 30, time.UTC)
	receiptsDir := filepath.Join(dir, "comprovantes")
	if err := os.MkdirAll(receiptsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(receiptsDir, "example.pdf"), []byte("receipt"), 0o600); err != nil {
		t.Fatal(err)
	}
	weeklyDir := filepath.Join(dir, "extratos-semanais", "week-id")
	if err := os.MkdirAll(weeklyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(weeklyDir, "extrato.pdf"), []byte("weekly"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := m.Run(ctx, time.Date(2026, 8, 15, 17, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(path); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	foundDB, foundReceipt, foundWeekly := false, false, false
	for _, file := range archive.File {
		foundDB = foundDB || file.Name == "hotel.db"
		foundReceipt = foundReceipt || file.Name == "comprovantes/example.pdf"
		foundWeekly = foundWeekly || file.Name == "extratos-semanais/week-id/extrato.pdf"
	}
	if !foundDB || !foundReceipt || !foundWeekly {
		t.Fatalf("backup entries: db=%t receipt=%t weekly=%t", foundDB, foundReceipt, foundWeekly)
	}
	status, err := m.Latest(ctx)
	if err != nil || status.Path != path {
		t.Fatalf("status=%#v err=%v", status, err)
	}
}
