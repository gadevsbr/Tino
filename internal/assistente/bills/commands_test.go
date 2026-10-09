package bills

import (
	"context"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandsRegisterConfigureAndCloseBill(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := New(db)
	for _, text := range []string{"boleto 20/10/2026 1.250,90 | Energia", "lembretes boletos 5573999999999 3 09:00", "boletos", "boleto pago 1"} {
		reply, err := r.Command(ctx, text)
		if err != nil || reply == "" {
			t.Fatal(text, reply, err)
		}
	}
	reply, err := r.Command(ctx, "boletos")
	if err != nil || !strings.Contains(reply, "Nenhum boleto pendente") {
		t.Fatal(reply, err)
	}
	cfg, err := r.Settings(ctx)
	if err != nil || !cfg.Enabled || cfg.Hour != 9 {
		t.Fatal(cfg, err)
	}
	if !IsCommand("boleto pago 1") || IsCommand("oi") {
		t.Fatal("classification")
	}
	for _, bad := range []string{"1.25", "1,234", "-1", "999999999999999999999999"} {
		if _, err := ParseMoney(bad); err == nil {
			t.Fatal("bad value accepted", bad)
		}
	}
}
