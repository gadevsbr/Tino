package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"go.mau.fi/whatsmeow/types"
)

func TestExtratosConversationStartCancel(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(dir, "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Service{domainDB: db}
	s.cfg.DataDir = dir
	chat := types.NewJID("557300000000", types.DefaultUserServer)
	reply, handled, err := s.handleExtratosText(ctx, "operator", "extratos", chat)
	if err != nil || !handled || !strings.Contains(reply, "EXTRATOS BITZ") {
		t.Fatalf("reply=%q handled=%t err=%v", reply, handled, err)
	}
	session, active, err := conversation.NewRepository(db).Active(ctx, "operator")
	if err != nil || !active || session.Type != "EXTRATOS" {
		t.Fatalf("session=%#v active=%t err=%v", session, active, err)
	}
	job, err := loadExtratoJob(session)
	if err != nil {
		t.Fatal(err)
	}
	jobDir, err := s.jobDir(job)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jobDir); err != nil {
		t.Fatal(err)
	}
	reply, handled, err = s.handleExtratosText(ctx, "operator", "processar extratos", chat)
	if err != nil || !handled || !strings.Contains(reply, "pelo menos um PDF") {
		t.Fatalf("reply=%q handled=%t err=%v", reply, handled, err)
	}
	reply, handled, err = s.handleExtratosText(ctx, "operator", "cancelar extratos", chat)
	if err != nil || !handled || !strings.Contains(reply, "removidos") {
		t.Fatalf("reply=%q handled=%t err=%v", reply, handled, err)
	}
	if _, err := os.Stat(jobDir); !os.IsNotExist(err) {
		t.Fatalf("job dir remains: %v", err)
	}
}

func TestWeeklyExtratosGuidedDates(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(dir, "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Service{domainDB: db}
	s.cfg.DataDir = dir
	chat := types.NewJID("557300000000", types.DefaultUserServer)
	for _, step := range []struct{ input, want string }{
		{"extrato semana 1 setembro 2026", "data inicial"},
		{"02/09/2026", "data final"},
		{"09/09/2026", "1 a 7 dias"},
		{"08/09/2026", "Período definido"},
		{"processar extratos", "pelo menos um PDF"},
		{"cancelar extratos", "continuam salvos"},
	} {
		reply, handled, err := s.handleExtratosText(ctx, "operator", step.input, chat)
		if err != nil || !handled || !strings.Contains(strings.ToLower(reply), strings.ToLower(step.want)) {
			t.Fatalf("input=%q reply=%q handled=%t err=%v", step.input, reply, handled, err)
		}
	}
	reply, handled, err := s.handleExtratosText(ctx, "operator", "extrato semana 1 setembro 2026", chat)
	if err != nil || !handled || !strings.Contains(reply, "Semana retomada") || !strings.Contains(reply, "0 PDF(s) acumulado(s)") {
		t.Fatalf("resume reply=%q handled=%t err=%v", reply, handled, err)
	}
}
