package commercial

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
)

func fixture(t *testing.T, quote QuoteFunc) *Service {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s, err := New(ctx, db, quote)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func handle(t *testing.T, s *Service, id, text string, now time.Time) Reply {
	t.Helper()
	r, err := s.Handle(context.Background(), Input{Account: "a", Contact: "guest", MessageID: id, Text: text, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func configure(t *testing.T, s *Service, c Config) {
	t.Helper()
	if err := s.Configure(context.Background(), "a", c); err != nil {
		t.Fatal(err)
	}
}

func TestGreetingChoiceHandoffAnd24HourReset(t *testing.T) {
	s := fixture(t, func(context.Context, string, string, string) (QuoteReply, error) { return QuoteReply{}, nil })
	configure(t, s, Config{Mode: Public})
	now := time.Now()
	r := handle(t, s, "1", "oi", now)
	if !strings.Contains(r.Text, greeting) || !strings.Contains(r.Text, "Tratar de outros assuntos") {
		t.Fatal(r)
	}
	r = handle(t, s, "2", "quero falar sobre um evento", now.Add(time.Minute))
	if !r.Handoff || !strings.Contains(r.Text, "atendente humano") {
		t.Fatal(r)
	}
	if err := s.Resume(context.Background(), "a", "guest"); err != nil {
		t.Fatal(err)
	}
	r = handle(t, s, "3", "oi", now.Add(24*time.Hour+time.Minute))
	if !strings.Contains(r.Text, greeting) {
		t.Fatal(r)
	}
}

func TestBudgetSequenceAddsTwoConfiguredMessages(t *testing.T) {
	categories := []omnibees.Category{{Key: "interna", Name: "Interna"}}
	calls := 0
	s := fixture(t, func(_ context.Context, _, _, text string) (QuoteReply, error) {
		calls++
		if text == "orçamento" {
			return QuoteReply{Text: "quantos quartos", Active: true}, nil
		}
		return QuoteReply{Messages: []string{"*Quarto 1*\nvalores"}, Categories: categories}, nil
	})
	configure(t, s, Config{Mode: Public, GroupPhone: "5573988240413", FinalMessage1: "mensagem um", FinalMessage2: "mensagem dois"})
	now := time.Now()
	handle(t, s, "1", "oi", now)
	r := handle(t, s, "2", "1", now.Add(time.Minute))
	if !strings.Contains(r.Text, "quantos quartos") {
		t.Fatal(r)
	}
	r = handle(t, s, "3", "fim", now.Add(2*time.Minute))
	if calls != 2 || len(r.Messages) != 3 || r.Messages[1] != "mensagem um" || r.Messages[2] != "mensagem dois" || len(r.Categories) != 1 {
		t.Fatal(r)
	}
}

func TestNumericOneIsMenuChoiceOnlyDuringTriage(t *testing.T) {
	inputs := []string{}
	s := fixture(t, func(_ context.Context, _, _, text string) (QuoteReply, error) {
		inputs = append(inputs, text)
		if text == "orçamento" {
			return QuoteReply{Text: "Para quantos quartos?", Active: true}, nil
		}
		return QuoteReply{Text: "próxima etapa", Active: true}, nil
	})
	configure(t, s, Config{Mode: Public})
	now := time.Now()
	handle(t, s, "1", "oi", now)
	handle(t, s, "2", "1", now.Add(time.Minute))
	r := handle(t, s, "3", "1", now.Add(2*time.Minute))
	if r.Text != "próxima etapa" {
		t.Fatal(r)
	}
	if strings.Join(inputs, ",") != "orçamento,1" {
		t.Fatalf("a quantidade reiniciou o orçamento: %v", inputs)
	}
}

func TestConcurrentDuplicateIsHandledOnce(t *testing.T) {
	s := fixture(t, func(context.Context, string, string, string) (QuoteReply, error) { return QuoteReply{}, nil })
	configure(t, s, Config{Mode: Public})
	var wg sync.WaitGroup
	replies := make(chan Reply, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _ := s.Handle(context.Background(), Input{Account: "a", Contact: "g", MessageID: "same", Text: "oi"})
			replies <- r
		}()
	}
	wg.Wait()
	close(replies)
	n := 0
	for r := range replies {
		if r.Text != "" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("sent %d replies", n)
	}
}

func TestConfigurationDefaultsAndValidation(t *testing.T) {
	s := fixture(t, func(context.Context, string, string, string) (QuoteReply, error) { return QuoteReply{}, nil })
	cfg, err := s.Configuration(context.Background(), "a")
	if err != nil || cfg.GroupPhone != defaultGroupPhone {
		t.Fatal(cfg, err)
	}
	if err = s.Configure(context.Background(), "a", Config{Mode: Public, GroupPhone: "123"}); err == nil {
		t.Fatal("invalid group phone accepted")
	}
}
