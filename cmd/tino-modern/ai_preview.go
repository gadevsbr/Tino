package main

import (
	"context"
	"fmt"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/flow"
	"strconv"
	"strings"
)

// PreviewCommercialAI exercises the actual guest dialogue without WhatsApp or
// operational writes, booking providers, notifications, or reservation creation.
func (a *App) PreviewCommercialAI(text string) (string, error) {
	if err := a.requireCapability("ai"); err != nil {
		return "", err
	}
	ai, err := flow.ConfiguredAI(a.capabilities)
	if err != nil {
		return "", err
	}
	if ai == nil {
		return "", fmt.Errorf("IA desativada")
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 8000 {
		return "", fmt.Errorf("informe uma frase sobre a quantidade de quartos")
	}
	answer, err := ai.Converse(context.Background(), commercial.DialogueRequest{Task: "interpret", Step: "PUBLIC_QUOTE_ROOMS", Message: text, Prompt: "Quantidade de quartos, de 1 a 50", Today: commercial.HotelToday()})
	if err != nil {
		return "", err
	}
	rooms, e := strconv.Atoi(answer.Value)
	if !answer.Understood || e != nil || rooms < 1 || rooms > 50 {
		return "", fmt.Errorf("não ficou claro quantos quartos você deseja")
	}
	phrase := commercial.HumanizeDialogue(context.Background(), ai.Converse, text, "Qual a data de entrada no hotel?")
	return fmt.Sprintf("IA entendeu: %d quarto(s).\n\n%s", rooms, phrase), nil
}
