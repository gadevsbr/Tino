package app

import (
	"context"
	"fmt"
	"github.com/gadevsbr/tino/internal/assistente/advances"
	"github.com/gadevsbr/tino/internal/assistente/cash"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) (*App, *rooms.Repository) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	rr := rooms.NewRepository(db)
	return New(rr, conversation.NewRepository(db)), rr
}

func TestCashCommands(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := New(rooms.NewRepository(db), conversation.NewRepository(db)).EnableCash(cash.NewRepository(db, time.UTC))
	for i, input := range []string{"abrir caixa 191,85", "entrada dinheiro 2100 hospedagem qto 104", "saida 50 material"} {
		if _, err = a.Handle(ctx, "u", fmt.Sprint(i), input); err != nil {
			t.Fatal(err)
		}
	}
	reply, err := a.Handle(ctx, "u", "4", "caixa hoje")
	if err != nil || !strings.Contains(reply, "R$ 2241,85") {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
}

func TestAdvanceGuidedFlowAndReportDeliverySelection(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cashRepo := cash.NewRepository(db, time.UTC)
	a := New(rooms.NewRepository(db), conversation.NewRepository(db)).EnableCash(cashRepo).EnableAdvances(advances.NewRepository(db, time.UTC))
	var sentKind string
	var sentPhones, sentGroups []string
	var authorized string
	a.EnableMessaging(
		func(_ context.Context, _, number string) (bool, error) { authorized = number; return true, nil },
		func(context.Context) ([]GroupOption, error) {
			return []GroupOption{{JID: "123@g.us", Name: "Recepção"}}, nil
		},
		func(_ context.Context, kind string, phones, groups []string) (string, error) {
			sentKind, sentPhones, sentGroups = kind, phones, groups
			return "enviado", nil
		},
	)
	for i, input := range []string{"vale", "Maria", "150,00", "adiantamento", "sim"} {
		if _, err = a.Handle(ctx, "u", fmt.Sprintf("v%d", i), input); err != nil {
			t.Fatalf("vale step %q: %v", input, err)
		}
	}
	day, err := cashRepo.Today(ctx, time.Now())
	if err != nil || len(day.Exits) != 1 || day.Exits[0].Cents != 15000 {
		t.Fatalf("day=%#v err=%v", day, err)
	}
	report, err := a.Handle(ctx, "u", "vr", "vales Maria")
	if err != nil || !strings.Contains(report, "Maria") || !strings.Contains(report, "R$ 150,00") {
		t.Fatalf("report=%q err=%v", report, err)
	}
	for i, input := range []string{"enviar relatorios", "3", "1 5573999999999"} {
		if _, err = a.Handle(ctx, "u", fmt.Sprintf("r%d", i), input); err != nil {
			t.Fatalf("reports step %q: %v", input, err)
		}
	}
	if sentKind != "BOTH" || len(sentPhones) != 1 || len(sentGroups) != 1 || sentGroups[0] != "123@g.us" {
		t.Fatalf("kind=%s phones=%v groups=%v", sentKind, sentPhones, sentGroups)
	}
	reply, err := a.Handle(ctx, "u", "a1", "autorizar numero 5573988888888")
	if err != nil || authorized != "5573988888888" || !strings.Contains(reply, "efeito imediato") {
		t.Fatalf("reply=%q authorized=%s err=%v", reply, authorized, err)
	}
}

func TestCashMovementEditAndConfirmedDelete(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := cash.NewRepository(db, time.UTC)
	a := New(rooms.NewRepository(db), conversation.NewRepository(db)).EnableCash(repo)
	for i, input := range []string{"abrir caixa 100", "entrada dinheiro 200 hospedagem", "saida 50 material"} {
		if _, err = a.Handle(ctx, "u", fmt.Sprint(i), input); err != nil {
			t.Fatal(err)
		}
	}
	list, err := a.Handle(ctx, "u", "l", "movimentos caixa")
	if err != nil || !strings.Contains(list, "#1") || !strings.Contains(list, "#2") {
		t.Fatalf("list=%q err=%v", list, err)
	}
	reply, err := a.Handle(ctx, "u", "e", "editar movimento 1 entrada pix 250 hospedagem 104")
	if err != nil || !strings.Contains(reply, "PIX") || !strings.Contains(reply, "R$ 250,00") {
		t.Fatalf("edit=%q err=%v", reply, err)
	}
	prompt, err := a.Handle(ctx, "u", "d", "excluir movimento 2")
	if err != nil || !strings.Contains(prompt, "confirmar") {
		t.Fatalf("prompt=%q err=%v", prompt, err)
	}
	reply, err = a.Handle(ctx, "u", "c", "confirmar")
	if err != nil || !strings.Contains(reply, "excluído") {
		t.Fatalf("delete=%q err=%v", reply, err)
	}
	day, err := repo.Today(ctx, time.Now())
	if err != nil || len(day.Entries) != 1 || day.MethodTotal("PIX") != 25000 || len(day.Exits) != 0 {
		t.Fatalf("day=%#v err=%v", day, err)
	}
	var audits int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cash_movement_audit`).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
}

func TestCashMovementDeleteCanBeCancelled(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := cash.NewRepository(db, time.UTC)
	a := New(rooms.NewRepository(db), conversation.NewRepository(db)).EnableCash(repo)
	_, _ = a.Handle(ctx, "u", "1", "saida 50 material")
	_, _ = a.Handle(ctx, "u", "2", "excluir movimento 1")
	reply, err := a.Handle(ctx, "u", "3", "cancelar")
	if err != nil || !strings.Contains(reply, "cancelada") {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
	day, _ := repo.Today(ctx, time.Now())
	if len(day.Exits) != 1 {
		t.Fatalf("movement was deleted: %#v", day)
	}
}

func TestCashCloseReopenHistoryAndOperations(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := New(rooms.NewRepository(db), conversation.NewRepository(db)).EnableCash(cash.NewRepository(db, time.Local))
	a.EnableOperations(func(context.Context) (string, error) { return "backup ok", nil }, func(context.Context) (string, error) { return "backup status", nil }, func(context.Context) string { return "saude ok" })
	_, _ = a.Handle(ctx, "u", "1", "abrir caixa 100")
	_, _ = a.Handle(ctx, "u", "2", "entrada pix 50 hospedagem")
	reply, err := a.Handle(ctx, "u", "3", "fechar caixa")
	if err != nil || !strings.Contains(reply, "CAIXA FECHADO") {
		t.Fatalf("close=%q err=%v", reply, err)
	}
	reply, _ = a.Handle(ctx, "u", "4", "saida 10 material")
	if !strings.Contains(reply, "fechado") {
		t.Fatalf("lock=%q", reply)
	}
	reply, _ = a.Handle(ctx, "u", "5", "reabrir caixa")
	if !strings.Contains(reply, "reaberto") {
		t.Fatalf("reopen=%q", reply)
	}
	reply, _ = a.Handle(ctx, "u", "6", "caixa "+time.Now().Format("02/01/2006"))
	if !strings.Contains(reply, "R$ 150,00") {
		t.Fatalf("history=%q", reply)
	}
	for input, want := range map[string]string{"backup agora": "backup ok", "status backup": "backup status", "saude": "saude ok"} {
		reply, err = a.Handle(ctx, "u", input, input)
		if err != nil || reply != want {
			t.Fatalf("%s=%q err=%v", input, reply, err)
		}
	}
}
func TestOperationalCommands(t *testing.T) {
	ctx := context.Background()
	a, rr := testApp(t)
	if _, err := a.Handle(ctx, "u", "1", "verdes 101 102"); err != nil {
		t.Fatal(err)
	}
	reply, _ := a.Handle(ctx, "u", "2", "verdes")
	if !strings.Contains(reply, "101\n102") || !strings.Contains(reply, "Total: 2") {
		t.Fatalf("list: %q", reply)
	}
	reply, _ = a.Handle(ctx, "u", "3", "status")
	if !strings.Contains(reply, "Disponíveis: 2") {
		t.Fatalf("status: %q", reply)
	}
	if _, err := a.Handle(ctx, "u", "3b", "103 desforrado"); err != nil {
		t.Fatal(err)
	}
	reply, _ = a.Handle(ctx, "u", "3c", "status")
	if !strings.Contains(reply, "Limpos, mas desforrados: 1") {
		t.Fatalf("split status: %q", reply)
	}
	reply, _ = a.Handle(ctx, "u", "4", "obs 101 ar condicionado fazendo barulho")
	if !strings.Contains(reply, "ar condicionado") {
		t.Fatalf("obs: %q", reply)
	}
	room, _ := rr.Get(ctx, 101)
	if room.Observation == "" {
		t.Fatal("observation not persisted")
	}
	reply, _ = a.Handle(ctx, "u", "5", "historico 101")
	if !strings.Contains(reply, "LIMPAR → DISPONÍVEL") {
		t.Fatalf("history: %q", reply)
	}
}
func TestGuidedSkip(t *testing.T) {
	ctx := context.Background()
	a, _ := testApp(t)
	a.Handle(ctx, "u", "1", "atualizar status")
	reply, err := a.Handle(ctx, "u", "2", "pular")
	if err != nil || !strings.Contains(reply, "Quarto 102:") {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
}

func TestPauseAndResume(t *testing.T) {
	ctx := context.Background()
	a, _ := testApp(t)
	a.Handle(ctx, "u", "1", "atualizar status")
	a.Handle(ctx, "u", "2", "verde")
	reply, err := a.Handle(ctx, "u", "3", "parar")
	if err != nil || !strings.Contains(reply, "1 de 42") {
		t.Fatalf("pause=%q err=%v", reply, err)
	}
	reply, err = a.Handle(ctx, "u", "4", "continuar")
	if err != nil || !strings.Contains(reply, "Próximo:\n102") {
		t.Fatalf("resume=%q err=%v", reply, err)
	}
}

func TestMenuDocumentsRealCommandsAndCleaningOK(t *testing.T) {
	ctx := context.Background()
	a, _ := testApp(t)
	menu, err := a.Handle(ctx, "u", "1", "menu")
	if err != nil || !strings.Contains(menu, "ok 101") || !strings.Contains(menu, "relatorio em pdf") || !strings.Contains(menu, "🟠 laranja") || !strings.Contains(menu, "101 desforrado") || !strings.Contains(menu, "enviar relatorios") || !strings.Contains(menu, "autorizar numero") || !strings.Contains(menu, "vales mes") || !strings.Contains(menu, "comprovante 12") || !strings.Contains(menu, "legenda comprovante") {
		t.Fatalf("menu=%q err=%v", menu, err)
	}
	if _, err = a.Handle(ctx, "u", "2", "101 laranja 2 pessoas"); err != nil {
		t.Fatal(err)
	}
	reply, err := a.Handle(ctx, "u", "3", "ok 101")
	if err != nil || !strings.Contains(reply, "concluída") {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
	reply, err = a.Handle(ctx, "u", "4", "101 laranja 2 pessoas")
	if err != nil || !strings.Contains(reply, "amanhã") {
		t.Fatalf("block=%q err=%v", reply, err)
	}
}

func TestGuestCountInDirectAndGuidedUpdates(t *testing.T) {
	ctx := context.Background()
	a, rr := testApp(t)
	reply, err := a.Handle(ctx, "u", "g1", "101 amarelo 3 pessoas")
	if err != nil || !strings.Contains(reply, "3 pessoas") {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
	room, _ := rr.Get(ctx, 101)
	if room.Status != rooms.Entry || room.GuestCount != 3 {
		t.Fatalf("room=%#v", room)
	}
	a.Handle(ctx, "u", "g2", "atualizar status")
	reply, err = a.Handle(ctx, "u", "g3", "laranja")
	if err != nil || !strings.Contains(reply, "Quantas pessoas") {
		t.Fatalf("prompt=%q err=%v", reply, err)
	}
	reply, err = a.Handle(ctx, "u", "g4", "2")
	if err != nil || !strings.Contains(reply, "Quarto 102:") {
		t.Fatalf("advance=%q err=%v", reply, err)
	}
	room, _ = rr.Get(ctx, 101)
	if room.GuestCount != 2 {
		t.Fatalf("room=%#v", room)
	}
}
