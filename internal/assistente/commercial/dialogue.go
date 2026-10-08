package commercial

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DialogueRequest struct {
	Task    string   `json:"task"`
	Step    string   `json:"step"`
	Message string   `json:"message"`
	Prompt  string   `json:"prompt"`
	Options []string `json:"options,omitempty"`
	Today   string   `json:"today"`
}
type DialogueAnswer struct {
	Value      string `json:"value"`
	Text       string `json:"text"`
	Understood bool   `json:"understood"`
}
type AssistantFunc func(context.Context, DialogueRequest) (DialogueAnswer, error)

func HotelToday() string {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.FixedZone("BRT", -3*60*60)
	}
	return time.Now().In(loc).Format("02/01/2006")
}

// NormalizeDialogue proposes a single value. The owning state machine still
// validates bounds, dates, availability, and whether any action is permitted.
func NormalizeDialogue(ctx context.Context, assist AssistantFunc, req DialogueRequest) string {
	fallback := req.Message
	if !strings.Contains(req.Step, "CHECK") && len(req.Options) == 0 {
		m := regexp.MustCompile(`(?i)^\s*(?:(?:para|pra|somos|só|apenas|quero|vai ser|são)\s+)*(\d+)(?:\s*(?:quartos?|adultos?|crianças?|anos?|pessoas?))?[.!]?\s*$`).FindStringSubmatch(req.Message)
		if len(m) > 0 {
			if n, err := strconv.Atoi(m[1]); err == nil && n <= 300 {
				fallback = strconv.Itoa(n)
			}
		}
	}
	if assist == nil {
		return fallback
	}
	req.Task = "interpret"
	req.Today = HotelToday()
	answer, err := assist(ctx, req)
	if err != nil || !answer.Understood {
		return fallback
	}
	value := strings.TrimSpace(answer.Value)
	if value == "atendente" || value == "sair" {
		return value
	}
	if len(req.Options) > 0 {
		for _, option := range req.Options {
			if value == option {
				return value
			}
		}
		return req.Message
	}
	if strings.Contains(req.Step, "CHECKIN") || strings.Contains(req.Step, "CHECKOUT") {
		date, err := time.Parse("02/01/2006", value)
		if err != nil || date.Format("02/01/2006") != value {
			return req.Message
		}
	} else {
		values := strings.Split(value, ",")
		if len(values) > 1 && !strings.Contains(req.Step, "GROUP_AGES") {
			return req.Message
		}
		for _, item := range values {
			n, err := strconv.Atoi(strings.TrimSpace(item))
			if err != nil || n < 0 || n > 300 {
				return req.Message
			}
		}
	}
	return value
}

var roomHeader = regexp.MustCompile(`(?m)^\*Quarto [^\n]+\*\n?`)
var choiceLine = regexp.MustCompile(`(?m)^\d+ — [^\n]+`)
var unsafeDialogue = regexp.MustCompile(`(?i)(R\$|https?://|confirmad[ao]|reservad[ao]|pagamento|pix|pré-reserva criada|pre-reserva criada)`)

// HumanizeDialogue lets the model phrase conversational prose only. Room
// identifiers and offered category lines are reattached unchanged locally.
func HumanizeDialogue(ctx context.Context, assist AssistantFunc, guest, source string) string {
	if assist == nil || strings.TrimSpace(source) == "" {
		return source
	}
	header := roomHeader.FindString(source)
	options := choiceLine.FindAllString(source, -1)
	prose := roomHeader.ReplaceAllString(source, "")
	prose = strings.ReplaceAll(prose, "Envie DD/MM/AAAA.", "Pode informar a data com suas palavras.")
	if len(options) > 0 && !strings.Contains(source, "Fazer um orçamento") {
		prose = "Pergunte de forma natural qual destas categorias o hóspede prefere. Diga que ele pode escrever o nome ou número, ou pedir um atendente."
	} else if strings.Contains(source, "Fazer um orçamento") {
		prose = "Cumprimente como assistente virtual do Hotel Paraíso Tropical e pergunte se a pessoa quer um orçamento de hospedagem ou falar com um atendente. Aceite resposta por palavras; não exija 1 ou 2."
		options = nil
	}
	answer, err := assist(ctx, DialogueRequest{Task: "phrase", Step: "guest_reply", Message: guest, Prompt: prose, Today: HotelToday()})
	text := strings.TrimSpace(answer.Text)
	if err != nil || text == "" || len([]rune(text)) > 650 || unsafeDialogue.MatchString(text) {
		return source
	}
	if strings.Contains(source, "vou criar sua pré-reserva") {
		lower := strings.ToLower(text)
		if strings.Contains(text, "?") || !strings.Contains(lower, "pré-reserva") || !strings.Contains(lower, "equipe") {
			return source
		}
	}
	// Questions must remain inclusive and cannot invent hotel facilities.
	for _, term := range []string{"filho", "filha", "marido", "esposa", "piscina", "estacionamento", "café da manhã", "pet", "desconto", "cortesia"} {
		if strings.Contains(strings.ToLower(text), term) && !strings.Contains(strings.ToLower(prose), term) {
			return source
		}
	}
	// Don't accept model-created dates, quantities, or category lists in prose.
	sourceNumbers := regexp.MustCompile(`\d+`).FindAllString(prose, -1)
	for _, n := range regexp.MustCompile(`\d+`).FindAllString(text, -1) {
		found := false
		for _, allowed := range sourceNumbers {
			if allowed == n {
				found = true
			}
		}
		if !found {
			return source
		}
	}
	if strings.Contains(source, "pausei") || strings.Contains(source, "Encaminhei") || strings.Contains(source, "encaminhada") {
		if !strings.Contains(strings.ToLower(text), "equipe") && !strings.Contains(strings.ToLower(text), "atendente") {
			return source
		}
	}
	if strings.Contains(source, "Sou o assistente virtual") && !strings.Contains(strings.ToLower(text), "assistente virtual") {
		return source
	}
	if len(options) > 0 {
		text += "\n\n" + strings.Join(options, "\n") + "\n\nSe preferir, escreva atendente. Para encerrar, escreva sair."
	}
	if header != "" {
		text = header + "\n" + text
	}
	return text
}
