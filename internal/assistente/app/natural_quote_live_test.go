package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	"github.com/gadevsbr/tino/internal/capability"
	"github.com/gadevsbr/tino/internal/flow"
)

// Real inference, temporary database, synthetic prices. No WhatsApp or Bitz.
func TestLiveNaturalCommercialJourney(t *testing.T) {
	if os.Getenv("TINO_AI_LIVE_TEST") != "1" {
		t.Skip("explicit live AI test required")
	}
	base, err := filepath.Abs(filepath.Join("..", "..", "..", "dist", "data"))
	if err != nil {
		t.Fatal(err)
	}
	ai, err := flow.ConfiguredAI(capability.NewStore(filepath.Join(base, "capabilities.json")))
	if err != nil || ai == nil {
		t.Fatal("AI unavailable", err)
	}
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := New(rooms.NewRepository(db), conversation.NewRepository(db)).EnableGuestAssistant(ai.Converse)
	quotes := 0
	fetch := func(_ context.Context, search omnibees.Search) (omnibees.Result, error) {
		quotes++
		if search.Adults != 2 || fmt.Sprint(search.Ages) != "[3]" {
			t.Fatal(search)
		}
		return omnibees.Result{Text: "Preço de teste: R$ 2.290,36", Categories: []omnibees.Category{{Key: "deluxe", Name: "Suíte Deluxe com varanda"}, {Key: "varanda", Name: "Suíte com varanda"}}}, nil
	}
	s, err := commercial.New(ctx, db, func(ctx context.Context, account, contact, text string) (commercial.QuoteReply, error) {
		return a.publicQuote(ctx, account, contact, text, fetch)
	})
	if err != nil {
		t.Fatal(err)
	}
	s.SetAssistant(ai.Converse)
	if err := s.Configure(ctx, "test-account", commercial.Config{Mode: commercial.Public}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	inputs := []string{"oi", "queria um orçamento de hospedagem", "para 01", "10/12/2099", "12/12/2099", "somos um casal", "uma criança", "minha filha tem três anos", "quero a suíte deluxe com varanda"}
	var result commercial.Reply
	for i, input := range inputs {
		result, err = s.Handle(ctx, commercial.Input{Account: "test-account", Contact: "synthetic-guest", MessageID: fmt.Sprint(i), Text: input, Now: now.Add(time.Duration(i) * time.Second)})
		if err != nil {
			t.Fatal(input, err)
		}
		t.Logf("%s => %s %s", input, result.Text, result.SelectionPrompt)
		if i == 0 && (!strings.Contains(result.Text, "assistente virtual") || strings.Contains(result.Text, "Responda com 1 ou 2")) {
			t.Fatal("greeting did not use natural AI", result.Text)
		}
		if i == 7 && (len(result.Messages) == 0 || !strings.Contains(result.Messages[0], "R$ 2.290,36")) {
			t.Fatal("verified price changed", result.Messages)
		}
	}
	if quotes != 1 || result.PreReservation == nil || len(result.PreReservation.Rooms) != 1 || len(result.PreReservation.Categories) != 1 || result.PreReservation.Categories[0].Key != "deluxe" || result.PreReservation.Rooms[0].Adults != 2 || fmt.Sprint(result.PreReservation.Rooms[0].Ages) != "[3]" {
		t.Fatalf("journey incomplete: %#v, quotes=%d", result, quotes)
	}
	t.Log("Natural conversation produced only a validated local preregistration request; no booking provider invoked")
}
