package app

import (
	"context"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuidedQuotePersistsAndUsesRealPriceFetcher(t *testing.T) {
	a, _ := testApp(t)
	ctx := context.Background()
	var got omnibees.Search
	a.quoteFetch = func(_ context.Context, s omnibees.Search) (string, error) {
		got = s
		return "Orçamento de teste com preço consultado", nil
	}
	user := "operator"
	steps := []struct{ input, want string }{
		{"orçamento", "check-in"},
		{"31/09/2026", "inválida"},
		{"19/09/2026", "check-out"},
		{"19/09/2026", "posterior"},
		{"20/09/2026", "adultos"},
		{"2", "crianças"},
		{"2", "criança 1"},
		{"7", "criança 2"},
		{"10", "Orçamento de teste"},
	}
	for i, step := range steps {
		reply, err := a.Handle(ctx, user, string(rune('a'+i)), step.input)
		if err != nil || !strings.Contains(reply, step.want) {
			t.Fatalf("step %d: reply=%q err=%v", i, reply, err)
		}
	}
	if got.Adults != 2 || got.Children != 2 || len(got.Ages) != 2 || got.Ages[0] != 7 || got.Ages[1] != 10 || got.URL.Query().Get("q") != "17134" {
		t.Fatalf("search=%#v", got)
	}
}

func TestVerticalFlowPersistsNextRoom(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rr := rooms.NewRepository(db)
	sr := conversation.NewRepository(db)
	a := New(rr, sr)
	reply, err := a.Handle(ctx, "557300000000", "m1", "atualizar status")
	if err != nil || !strings.Contains(reply, "Quarto 101:") {
		t.Fatalf("start: %q %v", reply, err)
	}
	reply, err = a.Handle(ctx, "557300000000", "m2", "verde")
	if err != nil || !strings.Contains(reply, "Quarto 102:") {
		t.Fatalf("advance: %q %v", reply, err)
	}
	room, err := rr.Get(ctx, 101)
	if err != nil || room.Status != rooms.AvailableClean {
		t.Fatalf("room=%#v err=%v", room, err)
	}
	s, ok, err := sr.Active(ctx, "557300000000")
	if err != nil || !ok || s.CurrentRoom != 102 {
		t.Fatalf("session=%#v active=%v err=%v", s, ok, err)
	}
}

func TestUnknownCommandIsSilent(t *testing.T) {
	a, _ := testApp(t)
	reply, err := a.Handle(context.Background(), "operator", "unknown-1", "mensagem sem comando")
	if err != nil || reply != "" {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
}
