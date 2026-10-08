package app

import (
	"context"
	"fmt"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
	"testing"
)

func TestNaturalQuoteUsesValidatedDataAndDoesNotExecuteOperatorCommands(t *testing.T) {
	a, _ := testApp(t)
	ctx := context.Background()
	values := map[string]string{"para 01": "1", "dia dez de dezembro": "10/12/2099", "até dia doze": "12/12/2099", "somos um casal": "2", "uma criança": "1", "minha filha tem três anos": "3"}
	a.EnableGuestAssistant(func(_ context.Context, r commercial.DialogueRequest) (commercial.DialogueAnswer, error) {
		return commercial.DialogueAnswer{Value: values[r.Message], Understood: values[r.Message] != ""}, nil
	})
	calls := 0
	fetch := func(_ context.Context, s omnibees.Search) (omnibees.Result, error) {
		calls++
		if s.Adults != 2 || fmt.Sprint(s.Ages) != "[3]" {
			t.Fatal(s)
		}
		return omnibees.Result{Text: "Preço confirmado: R$ 2.290,36", Categories: []omnibees.Category{{Key: "verified", Name: "Suíte verificada"}}}, nil
	}
	var result commercial.QuoteReply
	for _, input := range []string{"orçamento", "para 01", "dia dez de dezembro", "até dia doze", "somos um casal", "uma criança", "minha filha tem três anos"} {
		var err error
		result, err = a.publicQuote(ctx, "a", "g", input, fetch)
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 || len(result.Messages) != 1 || len(result.Plan.Rooms) != 1 {
		t.Fatal(result, calls)
	}
}

func TestNaturalQuoteStillRejectsBadDatesAndOverCapacity(t *testing.T) {
	a, _ := testApp(t)
	ctx := context.Background()
	a.EnableGuestAssistant(func(_ context.Context, r commercial.DialogueRequest) (commercial.DialogueAnswer, error) {
		return commercial.DialogueAnswer{Value: "99", Understood: true}, nil
	})
	if _, err := a.PublicQuote(ctx, "a", "g", "orçamento"); err != nil {
		t.Fatal(err)
	}
	result, err := a.PublicQuote(ctx, "a", "g", "quero muitos quartos")
	if err != nil || !result.Active || result.Text != "Informe a quantidade de quartos, de 1 a 50." {
		t.Fatal(result, err)
	}
}

func TestNaturalHumanRequestDuringQuoteStopsCollection(t *testing.T) {
	a, _ := testApp(t)
	ctx := context.Background()
	a.EnableGuestAssistant(func(context.Context, commercial.DialogueRequest) (commercial.DialogueAnswer, error) {
		return commercial.DialogueAnswer{Value: "atendente", Understood: true}, nil
	})
	a.PublicQuote(ctx, "a", "g", "orçamento")
	r, err := a.PublicQuote(ctx, "a", "g", "prefiro falar com alguém da equipe")
	if err != nil || !r.Handoff || r.Active {
		t.Fatal(r, err)
	}
	_, active, err := a.sessions.Active(ctx, publicQuoteKey("a", "g"))
	if err != nil || active {
		t.Fatal(active, err)
	}
}
