package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
)

type publicRoom struct {
	Adults int   `json:"adults"`
	Ages   []int `json:"ages"`
}
type publicQuotePayload struct {
	Rooms, CurrentRoom, CurrentAdults, ExpectedAges int
	CheckIn, CheckOut                               string
	CurrentAges                                     []int
	RoomData                                        []publicRoom
	TotalAdults, TotalChildren                      int
	GroupAges                                       []int
}

func publicQuoteKey(account, contact string) string {
	return "commercial:" + base64.RawURLEncoding.EncodeToString([]byte(account)) + ":" + base64.RawURLEncoding.EncodeToString([]byte(contact))
}
func (a *App) PublicQuote(ctx context.Context, account, contact, text string) (commercial.QuoteReply, error) {
	return a.publicQuote(ctx, account, contact, text, omnibees.FetchResult)
}
func (a *App) publicQuote(ctx context.Context, account, contact, text string, fetch func(context.Context, omnibees.Search) (omnibees.Result, error)) (commercial.QuoteReply, error) {
	if strings.TrimSpace(account) == "" || strings.TrimSpace(contact) == "" {
		return commercial.QuoteReply{}, errors.New("conta e contato obrigatórios")
	}
	key := publicQuoteKey(account, contact)
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "orçamento" || normalized == "orcamento" {
		payload, _ := json.Marshal(publicQuotePayload{CurrentRoom: 1})
		err := a.sessions.Save(ctx, conversation.Session{User: key, Type: "PUBLIC_QUOTE_ROOMS", Status: conversation.Active, Payload: string(payload)})
		return commercial.QuoteReply{Text: "Para quantos quartos você deseja fazer o orçamento?", Active: true}, err
	}
	session, active, err := a.sessions.Active(ctx, key)
	if err != nil {
		return commercial.QuoteReply{}, err
	}
	if !active || !strings.HasPrefix(session.Type, "PUBLIC_QUOTE_") {
		return commercial.QuoteReply{Text: "Envie orçamento para iniciar uma consulta."}, nil
	}
	if normalized == "cancelar" {
		session.Status = "COMPLETED"
		return commercial.QuoteReply{Text: "Orçamento cancelado."}, a.sessions.Save(ctx, session)
	}
	if a.guestAssistant != nil {
		canonical := false
		if strings.Contains(session.Type, "CHECKIN") || strings.Contains(session.Type, "CHECKOUT") {
			_, canonical = parseQuoteDate(text)
		} else {
			_, e := strconv.Atoi(strings.TrimSpace(text))
			canonical = e == nil
		}
		if !canonical {
			text = commercial.NormalizeDialogue(ctx, a.guestAssistant, commercial.DialogueRequest{Step: session.Type, Message: text, Prompt: session.Type + "\nDados já confirmados: " + session.Payload})
		}
		if text == "atendente" || text == "sair" {
			session.Status = "COMPLETED"
			reply := "Tudo bem, encerrei este orçamento. Quando precisar, é só me chamar."
			if text == "atendente" {
				reply = "Vou encaminhar sua conversa para nossa equipe continuar por aqui."
			}
			return commercial.QuoteReply{Text: reply, Handoff: text == "atendente"}, a.sessions.Save(ctx, session)
		}
	}
	var p publicQuotePayload
	if err = json.Unmarshal([]byte(session.Payload), &p); err != nil {
		return commercial.QuoteReply{}, err
	}
	save := func(prompt string) (commercial.QuoteReply, error) {
		data, e := json.Marshal(p)
		if e != nil {
			return commercial.QuoteReply{}, e
		}
		session.Payload = string(data)
		if e = a.sessions.Save(ctx, session); e != nil {
			return commercial.QuoteReply{}, e
		}
		return commercial.QuoteReply{Text: prompt, Active: true}, nil
	}
	switch session.Type {
	case "PUBLIC_QUOTE_ROOMS":
		n, e := strconv.Atoi(strings.TrimSpace(text))
		if e != nil || n < 1 || n > 50 {
			return commercial.QuoteReply{Text: "Informe a quantidade de quartos, de 1 a 50.", Active: true}, nil
		}
		p.Rooms = n
		session.Type = "PUBLIC_QUOTE_CHECKIN"
		return save("Qual a data de check-in? Envie DD/MM/AAAA.")
	case "PUBLIC_QUOTE_CHECKIN":
		date, ok := parseQuoteDate(text)
		if !ok || !publicCheckInAllowed(date, time.Now()) {
			return commercial.QuoteReply{Text: "A data de check-in deve ser válida e não pode estar no passado. Envie DD/MM/AAAA.", Active: true}, nil
		}
		p.CheckIn = date.Format("02/01/2006")
		session.Type = "PUBLIC_QUOTE_CHECKOUT"
		return save("Qual a data de check-out? Envie DD/MM/AAAA.")
	case "PUBLIC_QUOTE_CHECKOUT":
		date, ok := parseQuoteDate(text)
		checkIn, _ := parseQuoteDate(p.CheckIn)
		if !ok || !date.After(checkIn) {
			return commercial.QuoteReply{Text: "Check-out inválido. Informe uma data posterior ao check-in em DD/MM/AAAA.", Active: true}, nil
		}
		p.CheckOut = date.Format("02/01/2006")
		if p.Rooms > 6 {
			session.Type = "PUBLIC_QUOTE_GROUP_ADULTS"
			return save("Quantos adultos participarão do grupo no total?")
		}
		session.Type = "PUBLIC_QUOTE_ROOM_ADULTS"
		return save(roomPrompt(p.CurrentRoom, p.Rooms, "Quantos adultos ficarão neste quarto? Envie de 1 a 5."))
	case "PUBLIC_QUOTE_GROUP_ADULTS":
		n, e := strconv.Atoi(strings.TrimSpace(text))
		if e != nil || n < 1 || n > 300 {
			return commercial.QuoteReply{Text: "Informe a quantidade total de adultos, de 1 a 300.", Active: true}, nil
		}
		p.TotalAdults = n
		session.Type = "PUBLIC_QUOTE_GROUP_CHILDREN"
		return save("Quantas crianças participarão do grupo no total?")
	case "PUBLIC_QUOTE_GROUP_CHILDREN":
		n, e := strconv.Atoi(strings.TrimSpace(text))
		if e != nil || n < 0 || n > 100 {
			return commercial.QuoteReply{Text: "Informe a quantidade total de crianças, de 0 a 100.", Active: true}, nil
		}
		p.TotalChildren = n
		if n == 0 {
			return a.finishGroupQuote(ctx, session, p)
		}
		session.Type = "PUBLIC_QUOTE_GROUP_AGES"
		return save("Informe as idades das crianças separadas por vírgula. Ex.: 2, 7, 12.")
	case "PUBLIC_QUOTE_GROUP_AGES":
		ages, ok := parseAges(text, p.TotalChildren)
		if !ok {
			return commercial.QuoteReply{Text: fmt.Sprintf("Informe exatamente %d idade(s), de 0 a 17 anos, separadas por vírgula.", p.TotalChildren), Active: true}, nil
		}
		p.GroupAges = ages
		return a.finishGroupQuote(ctx, session, p)
	case "PUBLIC_QUOTE_ROOM_ADULTS":
		n, e := strconv.Atoi(strings.TrimSpace(text))
		if e != nil || n < 1 || n > 5 {
			return commercial.QuoteReply{Text: "Informe de 1 a 5 adultos para este quarto.", Active: true}, nil
		}
		p.CurrentAdults = n
		session.Type = "PUBLIC_QUOTE_ROOM_CHILDREN"
		return save(roomPrompt(p.CurrentRoom, p.Rooms, fmt.Sprintf("Quantas crianças ficarão neste quarto? Envie de 0 a %d.", 5-n)))
	case "PUBLIC_QUOTE_ROOM_CHILDREN":
		n, e := strconv.Atoi(strings.TrimSpace(text))
		if e != nil || n < 0 || n+p.CurrentAdults > 5 {
			return commercial.QuoteReply{Text: fmt.Sprintf("Informe de 0 a %d crianças para este quarto.", 5-p.CurrentAdults), Active: true}, nil
		}
		p.ExpectedAges = n
		p.CurrentAges = nil
		if n == 0 {
			return a.advancePublicRoom(ctx, session, p, fetch)
		}
		session.Type = "PUBLIC_QUOTE_ROOM_AGES"
		return save(roomPrompt(p.CurrentRoom, p.Rooms, fmt.Sprintf("Qual a idade da criança 1 de %d?", n)))
	case "PUBLIC_QUOTE_ROOM_AGES":
		age, e := strconv.Atoi(strings.TrimSpace(text))
		if e != nil || age < 0 || age > 17 {
			return commercial.QuoteReply{Text: "Informe somente a idade da criança, de 0 a 17 anos.", Active: true}, nil
		}
		p.CurrentAges = append(p.CurrentAges, age)
		if len(p.CurrentAges) < p.ExpectedAges {
			return save(roomPrompt(p.CurrentRoom, p.Rooms, fmt.Sprintf("Qual a idade da criança %d de %d?", len(p.CurrentAges)+1, p.ExpectedAges)))
		}
		return a.advancePublicRoom(ctx, session, p, fetch)
	default:
		return commercial.QuoteReply{}, fmt.Errorf("etapa pública de orçamento inválida: %s", session.Type)
	}
}
func (a *App) advancePublicRoom(ctx context.Context, session conversation.Session, p publicQuotePayload, fetch func(context.Context, omnibees.Search) (omnibees.Result, error)) (commercial.QuoteReply, error) {
	p.RoomData = append(p.RoomData, publicRoom{Adults: p.CurrentAdults, Ages: append([]int(nil), p.CurrentAges...)})
	p.CurrentAdults, p.ExpectedAges, p.CurrentAges = 0, 0, nil
	if len(p.RoomData) < p.Rooms {
		p.CurrentRoom = len(p.RoomData) + 1
		session.Type = "PUBLIC_QUOTE_ROOM_ADULTS"
		data, _ := json.Marshal(p)
		session.Payload = string(data)
		if err := a.sessions.Save(ctx, session); err != nil {
			return commercial.QuoteReply{}, err
		}
		return commercial.QuoteReply{Text: roomPrompt(p.CurrentRoom, p.Rooms, "Quantos adultos ficarão neste quarto? Envie de 1 a 5."), Active: true}, nil
	}
	return a.finishPublicRooms(ctx, session, p, fetch)
}
func (a *App) finishPublicRooms(ctx context.Context, session conversation.Session, p publicQuotePayload, fetch func(context.Context, omnibees.Search) (omnibees.Result, error)) (commercial.QuoteReply, error) {
	checkIn, _ := parseQuoteDate(p.CheckIn)
	checkOut, _ := parseQuoteDate(p.CheckOut)
	messages := make([]string, 0, len(p.RoomData))
	categories := []omnibees.Category{}
	plan := &commercial.QuotePlan{CheckIn: p.CheckIn, CheckOut: p.CheckOut, Rooms: make([]commercial.QuoteRoom, 0, len(p.RoomData))}
	seen := map[string]bool{}
	for i, room := range p.RoomData {
		search, err := omnibees.NewSearch(checkIn, checkOut, room.Adults, room.Ages)
		if err != nil {
			return commercial.QuoteReply{}, err
		}
		result, err := fetch(ctx, search)
		configuration := roomConfiguration(room)
		if err != nil {
			messages = append(messages, fmt.Sprintf("*Quarto %d — %s*\nNão consegui consultar os valores deste quarto agora.", i+1, configuration))
			plan.Rooms = append(plan.Rooms, commercial.QuoteRoom{Adults: room.Adults, Ages: append([]int(nil), room.Ages...)})
			continue
		}
		plan.Rooms = append(plan.Rooms, commercial.QuoteRoom{Adults: room.Adults, Ages: append([]int(nil), room.Ages...), Categories: append([]omnibees.Category(nil), result.Categories...)})
		messages = append(messages, fmt.Sprintf("*Quarto %d — %s*\n%s", i+1, configuration, result.Text))
		for _, category := range result.Categories {
			if !seen[category.Key] {
				seen[category.Key] = true
				categories = append(categories, category)
			}
		}
	}
	session.Status = "COMPLETED"
	if err := a.sessions.Save(ctx, session); err != nil {
		return commercial.QuoteReply{}, err
	}
	for _, room := range plan.Rooms {
		if len(room.Categories) == 0 {
			plan = nil
			break
		}
	}
	return commercial.QuoteReply{Messages: messages, Categories: categories, Plan: plan}, nil
}
func (a *App) finishGroupQuote(ctx context.Context, session conversation.Session, p publicQuotePayload) (commercial.QuoteReply, error) {
	session.Status = "COMPLETED"
	if err := a.sessions.Save(ctx, session); err != nil {
		return commercial.QuoteReply{}, err
	}
	ages := "nenhuma"
	if len(p.GroupAges) > 0 {
		parts := make([]string, len(p.GroupAges))
		for i, age := range p.GroupAges {
			parts[i] = strconv.Itoa(age)
		}
		ages = strings.Join(parts, ", ")
	}
	details := fmt.Sprintf("SOLICITAÇÃO DE ORÇAMENTO DE GRUPO\nQuartos: %d\nPeríodo: %s a %s\nAdultos: %d\nCrianças: %d\nIdades: %s", p.Rooms, p.CheckIn, p.CheckOut, p.TotalAdults, p.TotalChildren, ages)
	return commercial.QuoteReply{Text: "Sua solicitação foi encaminhada ao setor de grupos. Em breve enviaremos o orçamento por aqui.", GroupRequest: details}, nil
}
func roomPrompt(current, total int, prompt string) string {
	return fmt.Sprintf("*Quarto %d de %d*\n%s", current, total, prompt)
}
func roomConfiguration(room publicRoom) string {
	children := "sem crianças"
	if len(room.Ages) > 0 {
		parts := make([]string, len(room.Ages))
		for i, age := range room.Ages {
			parts[i] = strconv.Itoa(age) + " ano(s)"
		}
		children = fmt.Sprintf("%d criança(s): %s", len(room.Ages), strings.Join(parts, ", "))
	}
	return fmt.Sprintf("%d adulto(s), %s", room.Adults, children)
}
func parseAges(text string, expected int) ([]int, bool) {
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ';' || r == ' ' })
	if len(parts) != expected {
		return nil, false
	}
	ages := make([]int, len(parts))
	for i, part := range parts {
		age, err := strconv.Atoi(part)
		if err != nil || age < 0 || age > 17 {
			return nil, false
		}
		ages[i] = age
	}
	return ages, true
}
func publicCheckInAllowed(date, now time.Time) bool {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		location = time.FixedZone("America/Sao_Paulo", -3*60*60)
	}
	return date.Format("2006-01-02") >= now.In(location).Format("2006-01-02")
}
