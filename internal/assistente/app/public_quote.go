package app

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
)

func publicQuoteKey(account, contact string) string {
	return "commercial:" + base64.RawURLEncoding.EncodeToString([]byte(account)) + ":" + base64.RawURLEncoding.EncodeToString([]byte(contact))
}

// PublicQuote is a quote-only boundary, never App.Handle or commands.Parse.
// The caller serializes calls for each account/contact (commercial.Service does).
func (a *App) PublicQuote(ctx context.Context, account, contact, text string) (commercial.QuoteReply, error) {
	return a.publicQuote(ctx, account, contact, text, omnibees.FetchResult)
}

func (a *App) publicQuote(ctx context.Context, account, contact, text string, fetch func(context.Context, omnibees.Search) (omnibees.Result, error)) (commercial.QuoteReply, error) {
	if strings.TrimSpace(account) == "" || strings.TrimSpace(contact) == "" {
		return commercial.QuoteReply{}, errors.New("conta e contato obrigatórios")
	}
	key := publicQuoteKey(account, contact)
	if strings.EqualFold(strings.TrimSpace(text), "orçamento") || strings.EqualFold(strings.TrimSpace(text), "orcamento") {
		err := a.sessions.Save(ctx, conversation.Session{User: key, Type: "QUOTE_CHECKIN", Status: conversation.Active, Payload: "{}"})
		return commercial.QuoteReply{Text: "Qual a data de check-in? Envie DD/MM/AAAA. Para desistir, envie cancelar.", Active: true}, err
	}
	session, active, err := a.sessions.Active(ctx, key)
	if err != nil {
		return commercial.QuoteReply{}, err
	}
	if !active || !strings.HasPrefix(session.Type, "QUOTE_") {
		return commercial.QuoteReply{Text: "Envie orçamento para iniciar uma consulta."}, nil
	}
	if session.Type == "QUOTE_CHECKIN" {
		if date, ok := parseQuoteDate(text); ok {
			if !publicCheckInAllowed(date, time.Now()) {
				return commercial.QuoteReply{Text: "A data de check-in não pode estar no passado. Envie DD/MM/AAAA.", Active: true}, nil
			}
		}
	}
	var result omnibees.Result
	// A per-call copy keeps structured results out of shared mutable callbacks.
	clone := *a
	clone.quoteFetch = func(ctx context.Context, search omnibees.Search) (string, error) {
		if !publicCheckInAllowed(search.CheckIn, time.Now()) {
			return "", errors.New("check-in no passado")
		}
		var e error
		result, e = fetch(ctx, search)
		if e != nil {
			result = omnibees.Result{}
		}
		return result.Text, e
	}
	text, err = clone.handleQuoteSession(ctx, session, text)
	if err != nil {
		return commercial.QuoteReply{}, err
	}
	_, active, err = a.sessions.Active(ctx, key)
	return commercial.QuoteReply{Text: text, Active: active, Categories: result.Categories}, err
}

func publicCheckInAllowed(date, now time.Time) bool {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		location = time.FixedZone("America/Sao_Paulo", -3*60*60)
	}
	return date.Format("2006-01-02") >= now.In(location).Format("2006-01-02")
}
