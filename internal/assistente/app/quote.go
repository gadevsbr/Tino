package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
)

type quotePayload struct {
	CheckIn  string `json:"check_in"`
	CheckOut string `json:"check_out"`
	Adults   int    `json:"adults"`
	Children int    `json:"children"`
	Ages     []int  `json:"ages"`
}

func parseQuoteDate(text string) (time.Time, bool) {
	text = strings.TrimSpace(text)
	date, err := time.Parse("02/01/2006", text)
	return date, err == nil && date.Format("02/01/2006") == text
}

func (a *App) handleQuoteSession(ctx context.Context, session conversation.Session, text string) (string, error) {
	if strings.EqualFold(strings.TrimSpace(text), "cancelar") {
		session.Status = "COMPLETED"
		return "Orçamento cancelado.", a.sessions.Save(ctx, session)
	}
	var p quotePayload
	if err := json.Unmarshal([]byte(session.Payload), &p); err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	switch session.Type {
	case "QUOTE_CHECKIN":
		date, ok := parseQuoteDate(text)
		if !ok {
			return "Data inválida. Envie o check-in em DD/MM/AAAA.", nil
		}
		p.CheckIn = date.Format("02/01/2006")
		session.Type = "QUOTE_CHECKOUT"
		return a.saveQuoteStep(ctx, session, p, "Qual a data de check-out? Envie DD/MM/AAAA.")
	case "QUOTE_CHECKOUT":
		date, ok := parseQuoteDate(text)
		checkIn, _ := parseQuoteDate(p.CheckIn)
		if !ok || !date.After(checkIn) {
			return "Check-out inválido. Informe uma data posterior ao check-in em DD/MM/AAAA.", nil
		}
		p.CheckOut = date.Format("02/01/2006")
		session.Type = "QUOTE_ADULTS"
		return a.saveQuoteStep(ctx, session, p, "Quantos adultos? Envie um número de 1 a 5.")
	case "QUOTE_ADULTS":
		n, err := strconv.Atoi(text)
		if err != nil || n < 1 || n > 5 {
			return "Informe de 1 a 5 adultos.", nil
		}
		p.Adults = n
		session.Type = "QUOTE_CHILDREN"
		return a.saveQuoteStep(ctx, session, p, fmt.Sprintf("Quantas crianças? Envie de 0 a %d.", min(4, 5-n)))
	case "QUOTE_CHILDREN":
		n, err := strconv.Atoi(text)
		if err != nil || n < 0 || n > 4 || n+p.Adults > 5 {
			return fmt.Sprintf("Informe de 0 a %d crianças (máximo de 5 hóspedes no total).", min(4, 5-p.Adults)), nil
		}
		p.Children = n
		if n == 0 {
			return a.finishQuote(ctx, session, p)
		}
		session.Type = "QUOTE_AGES"
		return a.saveQuoteStep(ctx, session, p, fmt.Sprintf("Qual a idade da criança 1 de %d? Envie de 0 a 17 anos.", n))
	case "QUOTE_AGES":
		age, err := strconv.Atoi(text)
		if err != nil || age < 0 || age > 17 {
			return "Informe somente a idade da criança, de 0 a 17 anos.", nil
		}
		p.Ages = append(p.Ages, age)
		if len(p.Ages) < p.Children {
			return a.saveQuoteStep(ctx, session, p, fmt.Sprintf("Qual a idade da criança %d de %d?", len(p.Ages)+1, p.Children))
		}
		return a.finishQuote(ctx, session, p)
	}
	return "", fmt.Errorf("etapa de orçamento inválida: %s", session.Type)
}

func (a *App) saveQuoteStep(ctx context.Context, session conversation.Session, p quotePayload, prompt string) (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	session.Payload = string(data)
	if err := a.sessions.Save(ctx, session); err != nil {
		return "", err
	}
	return prompt, nil
}

func (a *App) finishQuote(ctx context.Context, session conversation.Session, p quotePayload) (string, error) {
	checkIn, _ := parseQuoteDate(p.CheckIn)
	checkOut, _ := parseQuoteDate(p.CheckOut)
	search, err := omnibees.NewSearch(checkIn, checkOut, p.Adults, p.Ages)
	if err != nil {
		return "❌ Dados do orçamento inválidos: " + err.Error(), nil
	}
	session.Status = "COMPLETED"
	if err := a.sessions.Save(ctx, session); err != nil {
		return "", err
	}
	quote, err := a.quoteFetch(ctx, search)
	if err != nil {
		return "❌ Não consegui consultar os valores totais na OmniBees agora. Nenhum preço foi estimado. Envie orçamento para tentar de novo.", nil
	}
	return quote, nil
}
