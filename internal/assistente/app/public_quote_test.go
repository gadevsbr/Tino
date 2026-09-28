package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
)

func TestPublicQuoteNeverRunsOperatorCommands(t *testing.T) {
	a, _ := testApp(t)
	ctx := context.Background()
	if _, err := a.Handle(ctx, "guest", "operator-start", "atualizar status"); err != nil {
		t.Fatal(err)
	}
	before, err := a.sessions.Get(ctx, "guest")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"101 verde", "abrir caixa 100", "autorizar numero 5573999999999", "relatorio em pdf", "vale", "enviar relatorios", "extratos"} {
		r, err := a.PublicQuote(ctx, "account", "guest", text)
		if err != nil || r.Active || !strings.Contains(r.Text, "Envie orçamento") {
			t.Fatalf("operator command accepted: %s %+v %v", text, r, err)
		}
	}
	a.PublicQuote(ctx, "account", "guest", "orçamento")
	r, err := a.PublicQuote(ctx, "account", "guest", "101 verde")
	if err != nil || !r.Active || !strings.Contains(r.Text, "Data inválida") {
		t.Fatal(r, err)
	}
	after, err := a.sessions.Get(ctx, "guest")
	if err != nil || before.Type != after.Type || before.CurrentRoom != after.CurrentRoom || before.CurrentIndex != after.CurrentIndex {
		t.Fatal("operator session modified", after, err)
	}
	r, err = a.PublicQuote(ctx, "other-account", "guest", "10/12/2099")
	if err != nil || r.Active {
		t.Fatal("account isolation failed", r, err)
	}
	r, err = a.PublicQuote(ctx, "account", "other-guest", "10/12/2099")
	if err != nil || r.Active {
		t.Fatal("contact isolation failed", r, err)
	}
}

func TestPublicQuoteGuidanceCategoriesAndFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			a, _ := testApp(t)
			ctx := context.Background()
			calls := 0
			fetch := func(_ context.Context, s omnibees.Search) (omnibees.Result, error) {
				calls++
				if s.Adults != 2 || s.Children != 2 || fmt.Sprint(s.Ages) != "[7 12]" {
					t.Fatal(s)
				}
				result := omnibees.Result{Text: "totais reais", Categories: []omnibees.Category{{Key: "varanda", SourceKey: "quadruploVaranda", TotalCents: 12345}}}
				if fail {
					return result, errors.New("provider unavailable")
				}
				return result, nil
			}
			var last commercial.QuoteReply
			for i, step := range []struct{ input, want string }{
				{"orçamento", "check-in"}, {"01/01/2000", "passado"}, {"31/02/2099", "inválida"}, {"10/12/2099", "check-out"}, {"09/12/2099", "posterior"}, {"12/12/2099", "adultos"}, {"0", "1 a 5"}, {"2", "crianças"}, {"4", "0 a 3"}, {"2", "criança 1"}, {"18", "0 a 17"}, {"7", "criança 2"}, {"12", ""},
			} {
				var err error
				last, err = a.publicQuote(ctx, "a", "g", step.input, fetch)
				if err != nil || !strings.Contains(last.Text, step.want) {
					t.Fatalf("step %d: %+v %v", i, last, err)
				}
			}
			if calls != 1 || last.Active {
				t.Fatal(calls, last)
			}
			if fail {
				if len(last.Categories) != 0 || !strings.Contains(last.Text, "Nenhum preço foi estimado") {
					t.Fatal(last)
				}
			} else {
				if len(last.Categories) != 1 || last.Categories[0].SourceKey != "quadruploVaranda" {
					t.Fatal(last)
				}
			}
		})
	}
}

func TestPublicHotelDateBoundary(t *testing.T) {
	date, _ := time.Parse("2006-01-02", "2030-01-01")
	if !publicCheckInAllowed(date, time.Date(2030, 1, 2, 2, 59, 59, 0, time.UTC)) {
		t.Fatal("UTC date used instead of hotel date")
	}
	if publicCheckInAllowed(date, time.Date(2030, 1, 2, 3, 0, 0, 0, time.UTC)) {
		t.Fatal("past hotel date accepted")
	}
}

func TestCommercialPublicFlowResetsPersistedSessionAfter24Hours(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := New(rooms.NewRepository(db), conversation.NewRepository(db))
	s, err := commercial.New(ctx, db, a.PublicQuote)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Configure(ctx, "a", commercial.Config{Mode: commercial.Public}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i, text := range []string{"orçamento", "10/12/2099"} {
		if _, err = s.Handle(ctx, commercial.Input{Account: "a", Contact: "g", MessageID: fmt.Sprint(i), Text: text, Now: now}); err != nil {
			t.Fatal(err)
		}
	}
	r, err := s.Handle(ctx, commercial.Input{Account: "a", Contact: "g", MessageID: "expired", Text: "orçamento", Now: now.Add(24 * time.Hour)})
	if err != nil || !strings.Contains(r.Text, "check-in") || strings.Contains(r.Text, "inválid") {
		t.Fatal(r, err)
	}
	session, err := a.sessions.Get(ctx, publicQuoteKey("a", "g"))
	if err != nil || session.Type != "QUOTE_CHECKIN" || session.Payload != "{}" {
		t.Fatal(session, err)
	}
	// Restart the service with the same database and continue the fresh session.
	s, err = commercial.New(ctx, db, a.PublicQuote)
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.Handle(ctx, commercial.Input{Account: "a", Contact: "g", MessageID: "next", Text: "11/12/2099", Now: now.Add(24*time.Hour + time.Minute)})
	if err != nil || !strings.Contains(r.Text, "check-out") {
		t.Fatal(r, err)
	}
}
