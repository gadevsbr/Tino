package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/omnibees"
)

func TestPublicQuoteCollectsAllRoomsBeforeFetching(t *testing.T) {
	a, _ := testApp(t)
	ctx := context.Background()
	calls := []omnibees.Search{}
	fetch := func(_ context.Context, s omnibees.Search) (omnibees.Result, error) {
		calls = append(calls, s)
		return omnibees.Result{Text: fmt.Sprintf("total para %d", s.Adults), Categories: []omnibees.Category{{Key: fmt.Sprintf("cat-%d", s.Adults), Name: "Categoria"}}}, nil
	}
	steps := []string{"orçamento", "2", "10/12/2099", "12/12/2099", "2", "1", "7", "3", "0"}
	var lastText string
	var messages int
	for i, input := range steps {
		r, err := a.publicQuote(ctx, "a", "g", input, fetch)
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		lastText = r.Text
		messages = len(r.Messages)
		if i < len(steps)-1 && len(calls) != 0 {
			t.Fatalf("consultou antes de reunir todos os quartos no passo %d", i)
		}
	}
	if len(calls) != 2 || calls[0].Adults != 2 || fmt.Sprint(calls[0].Ages) != "[7]" || calls[1].Adults != 3 {
		t.Fatal(calls)
	}
	if messages != 2 || lastText != "" {
		t.Fatalf("messages=%d text=%q", messages, lastText)
	}
	r, _ := a.publicQuote(ctx, "a", "g", "qualquer", fetch)
	if r.Active {
		t.Fatal("session should be complete")
	}
}

func TestPublicQuoteForwardsGroupAfterGeneralInformation(t *testing.T) {
	a, _ := testApp(t)
	ctx := context.Background()
	fetchCalls := 0
	fetch := func(context.Context, omnibees.Search) (omnibees.Result, error) {
		fetchCalls++
		return omnibees.Result{}, nil
	}
	steps := []string{"orçamento", "7", "10/12/2099", "12/12/2099", "18", "2", "4, 9"}
	var resultText, request string
	for _, input := range steps {
		r, err := a.publicQuote(ctx, "a", "grupo", input, fetch)
		if err != nil {
			t.Fatal(err)
		}
		resultText, request = r.Text, r.GroupRequest
	}
	if fetchCalls != 0 || !strings.Contains(resultText, "setor de grupos") || !strings.Contains(request, "Quartos: 7") || !strings.Contains(request, "Adultos: 18") || !strings.Contains(request, "Idades: 4, 9") {
		t.Fatal(resultText, request, fetchCalls)
	}
}

func TestPublicQuoteNeverRunsOperatorCommands(t *testing.T) {
	a, _ := testApp(t)
	ctx := context.Background()
	for _, text := range []string{"101 verde", "abrir caixa 100", "autorizar numero 5573999999999", "relatorio em pdf"} {
		r, err := a.PublicQuote(ctx, "account", "guest", text)
		if err != nil || r.Active || !strings.Contains(r.Text, "Envie orçamento") {
			t.Fatalf("operator command accepted: %s %+v %v", text, r, err)
		}
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
