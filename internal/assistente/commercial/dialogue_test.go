package commercial

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDialogueUnderstandsWordsAndKeepsStateActionsOwnedByMotor(t *testing.T) {
	s := fixture(t, func(_ context.Context, _, _, text string) (QuoteReply, error) {
		if text != "orçamento" {
			t.Fatalf("invalid canonical action: %q", text)
		}
		return QuoteReply{Text: "Para quantos quartos você deseja fazer o orçamento?", Active: true}, nil
	})
	configure(t, s, Config{Mode: Public})
	calls := 0
	s.SetAssistant(func(_ context.Context, r DialogueRequest) (DialogueAnswer, error) {
		calls++
		if r.Task == "interpret" {
			return DialogueAnswer{Value: "orçamento", Understood: true}, nil
		}
		if strings.Contains(r.Prompt, "Cumprimente") {
			return DialogueAnswer{Text: "Olá! Sou o assistente virtual do Hotel Paraíso Tropical. Quer preparar um orçamento ou prefere falar com um atendente?"}, nil
		}
		return DialogueAnswer{Text: "Quantos quartos você vai precisar para sua estadia?"}, nil
	})
	now := time.Now()
	r := handle(t, s, "hi", "oi", now)
	if strings.Contains(r.Text, "Responda com 1 ou 2") {
		t.Fatal(r.Text)
	}
	r = handle(t, s, "budget", "queria saber os valores", now.Add(time.Second))
	if !strings.Contains(r.Text, "estad") || calls != 3 {
		t.Fatal(r, calls)
	}
}

func TestDialogueProtectsCategoryLinesAndRejectsInventedFacts(t *testing.T) {
	source := "*Quarto 1 de 2*\nEscolha uma categoria pelo número:\n1 — Suíte Deluxe\n2 — Suíte com varanda\nOu envie atendente."
	assist := func(context.Context, DialogueRequest) (DialogueAnswer, error) {
		return DialogueAnswer{Text: "Qual destas opções combina melhor com o que você procura?"}, nil
	}
	got := HumanizeDialogue(context.Background(), assist, "prefiro varanda", source)
	for _, fact := range []string{"*Quarto 1 de 2*", "1 — Suíte Deluxe", "2 — Suíte com varanda"} {
		if !strings.Contains(got, fact) {
			t.Fatal(got)
		}
	}
	for _, unsafe := range []string{"Sua reserva está confirmada.", "Custa R$ 10,00.", "Pode trazer 7 pessoas.", "Vocês têm filhos?", "Nosso hotel tem piscina."} {
		fn := func(context.Context, DialogueRequest) (DialogueAnswer, error) {
			return DialogueAnswer{Text: unsafe}, nil
		}
		if got := HumanizeDialogue(context.Background(), fn, "oi", source); got != source {
			t.Fatal("invented fact accepted", got)
		}
	}
}

func TestDialogueDoesNotCollectDatesAgainWhenCreatingPreReservation(t *testing.T) {
	source := "Tudo certo com as categorias. Agora vou criar sua pré-reserva e avisar nossa equipe para concluir o atendimento."
	fn := func(context.Context, DialogueRequest) (DialogueAnswer, error) {
		return DialogueAnswer{Text: "Vamos finalizar sua pré-reserva. Qual a data de entrada e saída?"}, nil
	}
	if got := HumanizeDialogue(context.Background(), fn, "quero deluxe", source); got != source {
		t.Fatal(got)
	}
}

func TestDialogueAmbiguityAndProviderFailureKeepSafeFallback(t *testing.T) {
	fail := func(context.Context, DialogueRequest) (DialogueAnswer, error) {
		return DialogueAnswer{}, errors.New("offline")
	}
	if got := NormalizeDialogue(context.Background(), fail, DialogueRequest{Step: "PUBLIC_QUOTE_ROOMS", Message: "para 01"}); got != "1" {
		t.Fatal(got)
	}
	ambiguous := func(context.Context, DialogueRequest) (DialogueAnswer, error) {
		return DialogueAnswer{Value: "2", Understood: false}, nil
	}
	if got := NormalizeDialogue(context.Background(), ambiguous, DialogueRequest{Step: "PUBLIC_QUOTE_ROOMS", Message: "não sei ainda"}); got != "não sei ainda" {
		t.Fatal(got)
	}
	if got := HumanizeDialogue(context.Background(), fail, "oi", "Pergunta original"); got != "Pergunta original" {
		t.Fatal(got)
	}
}

func TestPausedGuestAndAudioDoNotNormalizeOrResumeThroughAI(t *testing.T) {
	s := fixture(t, func(context.Context, string, string, string) (QuoteReply, error) { return QuoteReply{}, nil })
	configure(t, s, Config{Mode: Public})
	count := 0
	s.SetAssistant(func(_ context.Context, r DialogueRequest) (DialogueAnswer, error) {
		if r.Task == "interpret" {
			count++
		}
		return DialogueAnswer{}, errors.New("disabled")
	})
	now := time.Now()
	handle(t, s, "hi", "oi", now)
	r := handle(t, s, "human", "atendente", now.Add(time.Second))
	if !r.Handoff {
		t.Fatal(r)
	}
	count = 0
	r = handle(t, s, "paused", "quero orçamento", now.Add(2*time.Second))
	if r.Text != "" || count != 0 {
		t.Fatal(r, count)
	}
}
