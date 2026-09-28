package commercial

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
)

func fixture(t *testing.T, quote QuoteFunc) (*Service, func() *Service) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "hotel.db")
	db, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := New(ctx, db, quote)
	if err != nil {
		t.Fatal(err)
	}
	reopen := func() *Service {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		db, err = database.Open(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		next, err := New(ctx, db, quote)
		if err != nil {
			t.Fatal(err)
		}
		return next
	}
	return s, reopen
}
func noQuote(context.Context, string, string, string) (QuoteReply, error) {
	return QuoteReply{}, fmt.Errorf("unexpected quote access")
}
func handle(t *testing.T, s *Service, account, contact, id, text string, now time.Time) Reply {
	t.Helper()
	r, err := s.Handle(context.Background(), Input{Account: account, Contact: contact, MessageID: id, Text: text, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func configure(t *testing.T, s *Service, account string, c Config) {
	t.Helper()
	if err := s.Configure(context.Background(), account, c); err != nil {
		t.Fatal(err)
	}
}

func TestModesAccountContactIsolationAndRestart(t *testing.T) {
	s, reopen := fixture(t, noQuote)
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	if r := handle(t, s, "a", "guest", "1", "oi", now); r.Handled {
		t.Fatal("default must be disabled")
	}
	configure(t, s, "a", Config{Mode: Test, Allowlist: []string{"guest"}})
	if r := handle(t, s, "a", "other", "1", "oi", now); r.Handled {
		t.Fatal("test leaked to unlisted guest")
	}
	if r := handle(t, s, "b", "guest", "1", "oi", now); r.Handled {
		t.Fatal("account config leaked")
	}
	if r := handle(t, s, "a", "guest", "1", "oi", now); !strings.Contains(r.Text, greeting) {
		t.Fatal(r)
	}
	s = reopen()
	if r := handle(t, s, "a", "guest", "2", "menu", now.Add(time.Hour)); strings.Contains(r.Text, greeting) {
		t.Fatal("restart lost last interaction")
	}
	configure(t, s, "a", Config{Mode: Public})
	if r := handle(t, s, "a", "other", "2", "oi", now); !strings.Contains(r.Text, greeting) {
		t.Fatal("contacts share greeting")
	}
	configure(t, s, "b", Config{Mode: Public})
	if r := handle(t, s, "b", "guest", "2", "oi", now); !strings.Contains(r.Text, greeting) {
		t.Fatal("accounts share greeting")
	}
}

func TestGreeting24HoursSlidingAndFirstAudio(t *testing.T) {
	s, _ := fixture(t, noQuote)
	configure(t, s, "a", Config{Mode: Public})
	now := time.Now()
	for i, step := range []struct {
		delta time.Duration
		text  string
		greet bool
	}{
		{0, "", true}, {24*time.Hour - time.Nanosecond, "menu", false},
		{48*time.Hour - time.Nanosecond, "menu", true}, {48 * time.Hour, "menu", false},
	} {
		r := handle(t, s, "a", "guest", fmt.Sprint(i), step.text, now.Add(step.delta))
		if strings.Contains(r.Text, greeting) != step.greet {
			t.Fatalf("step %d: %q", i, r.Text)
		}
	}
}

func TestHumanHandoffExactPhraseActiveAndIdlePersists(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(fmt.Sprint(active), func(t *testing.T) {
			calls := []string{}
			s, reopen := fixture(t, func(_ context.Context, _, _, text string) (QuoteReply, error) {
				calls = append(calls, text)
				return QuoteReply{Text: "check-in", Active: text != "cancelar"}, nil
			})
			configure(t, s, "a", Config{Mode: Public})
			now := time.Now()
			if active {
				handle(t, s, "a", "guest", "start", "orçamento", now)
			}
			r := handle(t, s, "a", "guest", "handoff", "atendimento humano", now)
			if !r.Handoff || !strings.Contains(r.Text, "pausado") {
				t.Fatal(r)
			}
			s = reopen()
			configure(t, s, "a", Config{Mode: Test, Allowlist: []string{"guest"}})
			for i, text := range []string{"oi", "orçamento", "menu", "continuar", "retomar"} {
				r = handle(t, s, "a", "guest", fmt.Sprint(i), text, now.Add(72*time.Hour))
				if r.Text != "" || r.Handoff {
					t.Fatalf("pause lost: %+v", r)
				}
			}
			r = handle(t, s, "a", "guest", "resume", "retomar atendimento", now.Add(73*time.Hour))
			if !strings.Contains(r.Text, "retomado") {
				t.Fatal(r)
			}
			if active && strings.Join(calls, ",") != "orçamento,cancelar" {
				t.Fatal(calls)
			}
			handle(t, s, "a", "guest", "pause-again", "atendimento humano", now.Add(73*time.Hour))
			if err := s.Resume(context.Background(), "b", "guest"); err != nil {
				t.Fatal(err)
			}
			if r = handle(t, s, "a", "guest", "still-paused", "oi", now.Add(74*time.Hour)); r.Text != "" {
				t.Fatal("cross-account resume")
			}
			if err := s.Resume(context.Background(), "a", "guest"); err != nil {
				t.Fatal(err)
			}
			if r = handle(t, s, "a", "guest", "operator-resumed", "oi", now.Add(74*time.Hour)); r.Text == "" {
				t.Fatal("resume failed")
			}
		})
	}
}

func TestAllCategoriesOnlyOnceSelectionAndDuplicates(t *testing.T) {
	categories := []omnibees.Category{{Key: "interna", SourceKey: "duplo", Name: "Suíte interna", TotalCents: 12345}, {Key: "varanda", SourceKey: "triploVaranda", Name: "Suíte com varanda", TotalCents: 23456}}
	calls := 0
	s, reopen := fixture(t, func(_ context.Context, _, _, text string) (QuoteReply, error) {
		calls++
		if text == "orçamento" {
			return QuoteReply{Text: "check-in", Active: true}, nil
		}
		return QuoteReply{Text: "totais consultados", Categories: categories}, nil
	})
	configure(t, s, "a", Config{Mode: Public})
	now := time.Now()
	handle(t, s, "a", "guest", "1", "orçamento", now)
	r := handle(t, s, "a", "guest", "2", "finish", now)
	if len(r.Categories) != 2 || r.SelectedCategory != nil {
		t.Fatal(r)
	}
	s = reopen()
	r = handle(t, s, "a", "guest", "2", "finish", now)
	if len(r.Categories) != 0 || r.Text != "" || calls != 2 {
		t.Fatal("duplicate quote replayed")
	}
	r = handle(t, s, "a", "guest", "3", "3", now)
	if r.Handoff {
		t.Fatal("invalid category selected")
	}
	r = handle(t, s, "a", "guest", "4", "2", now)
	if !r.Handoff || r.SelectedCategory == nil || r.SelectedCategory.Key != "varanda" || len(r.Categories) != 0 {
		t.Fatal(r)
	}
	if !strings.Contains(r.Text, "Nenhuma reserva foi confirmada") {
		t.Fatal(r.Text)
	}
}

func TestExpiryAndConfigChangeCancelPersistedQuote(t *testing.T) {
	for _, change := range []bool{false, true} {
		t.Run(fmt.Sprint(change), func(t *testing.T) {
			calls := []string{}
			s, _ := fixture(t, func(_ context.Context, _, _, text string) (QuoteReply, error) {
				calls = append(calls, text)
				return QuoteReply{Text: "check-in", Active: text != "cancelar"}, nil
			})
			configure(t, s, "a", Config{Mode: Public})
			now := time.Now()
			handle(t, s, "a", "guest", "1", "orçamento", now)
			if change {
				configure(t, s, "a", Config{Mode: Test, Allowlist: []string{"guest"}})
			} else {
				now = now.Add(24 * time.Hour)
			}
			handle(t, s, "a", "guest", "2", "orçamento", now)
			if strings.Join(calls, ",") != "orçamento,cancelar,orçamento" {
				t.Fatal(calls)
			}
		})
	}
}

func TestConcurrentDuplicateIsHandledOnce(t *testing.T) {
	s, _ := fixture(t, noQuote)
	configure(t, s, "a", Config{Mode: Public})
	var wg sync.WaitGroup
	replies := make(chan Reply, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.Handle(context.Background(), Input{Account: "a", Contact: "g", MessageID: "same", Text: "oi"})
			replies <- r
			errs <- err
		}()
	}
	wg.Wait()
	close(replies)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
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
