package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/queue"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

type Rule struct {
	Name          string `yaml:"name"`
	Contains      string `yaml:"contains"`
	Reply         string `yaml:"reply"`
	CaseSensitive bool   `yaml:"case_sensitive"`
}
type Definition struct {
	Rules        []Rule `yaml:"rules"`
	DefaultReply string `yaml:"default_reply"`
}
type AIService struct {
	Endpoint  string
	Token     string
	Model     string
	MaxTokens int
	Timeout   time.Duration
	Client    *http.Client
}

type Engine struct {
	client    *whatsmeow.Client
	def       Definition
	mu        sync.Mutex
	seen      map[string]struct{}
	ai        *AIService
	queue     *queue.PerKey
	Allow     func(*events.Message) bool
	ResolveAI func() (*AIService, error)
}

func Load(path string, client *whatsmeow.Client, ai *AIService) (*Engine, error) {
	def, err := LoadDefinition(path)
	if err != nil {
		return nil, err
	}
	return New(client, def, ai)
}

func New(client *whatsmeow.Client, def Definition, ai *AIService) (*Engine, error) {
	if client == nil {
		return nil, fmt.Errorf("cliente WhatsApp é obrigatório")
	}
	if err := Validate(def); err != nil {
		return nil, err
	}
	return &Engine{client: client, def: def, seen: make(map[string]struct{}), ai: ai, queue: queue.New()}, nil
}

func LoadDefinition(path string) (Definition, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, fmt.Errorf("ler fluxo: %w", err)
	}
	var def Definition
	if err := yaml.Unmarshal(b, &def); err != nil {
		return Definition{}, fmt.Errorf("decodificar fluxo: %w", err)
	}
	if err := Validate(def); err != nil {
		return Definition{}, err
	}
	return def, nil
}

func Validate(def Definition) error {
	for i, r := range def.Rules {
		if strings.TrimSpace(r.Name) == "" || strings.TrimSpace(r.Contains) == "" || strings.TrimSpace(r.Reply) == "" {
			return fmt.Errorf("regra %d requer nome, condição e resposta", i+1)
		}
	}
	return nil
}

func SaveDefinition(path string, def Definition) error {
	if err := Validate(def); err != nil {
		return err
	}
	b, err := yaml.Marshal(def)
	if err != nil {
		return fmt.Errorf("codificar fluxo: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("criar diretório do fluxo: %w", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("salvar fluxo: %w", err)
	}
	return nil
}

func Match(def Definition, text string) string {
	for _, r := range def.Rules {
		hay, needle := text, r.Contains
		if !r.CaseSensitive {
			hay, needle = strings.ToLower(hay), strings.ToLower(needle)
		}
		if strings.Contains(hay, needle) {
			return r.Reply
		}
	}
	return def.DefaultReply
}

func (e *Engine) Handle(evt any) {
	msg, ok := evt.(*events.Message)
	if !ok || msg.Info.IsFromMe || msg.Info.IsGroup {
		return
	}
	e.mu.Lock()
	if _, exists := e.seen[msg.Info.ID]; exists {
		e.mu.Unlock()
		return
	}
	e.seen[msg.Info.ID] = struct{}{}
	e.mu.Unlock()
	text := extractText(msg)
	if text == "" {
		return
	}
	e.queue.Enqueue(msg.Info.Chat.String(), func() {
		if e.Allow != nil && !e.Allow(msg) {
			return
		}
		reply := e.Reply(text)
		if reply == "" {
			return
		}
		ctx := context.Background()
		if _, err := e.client.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{Conversation: proto.String(reply)}); err != nil {
			fmt.Fprintf(os.Stderr, "erro ao responder %s: %v\n", msg.Info.Chat, err)
		}
	})
}

func extractText(m *events.Message) string {
	if m.Message == nil {
		return ""
	}
	if s := m.Message.GetConversation(); s != "" {
		return strings.TrimSpace(s)
	}
	if x := m.Message.GetExtendedTextMessage(); x != nil {
		return strings.TrimSpace(x.GetText())
	}
	return ""
}

func (e *Engine) match(text string) string {
	return Match(e.def, text)
}

func (e *Engine) generateAIReply(text string) (string, error) {
	return e.ai.Generate(text)
}

func (ai *AIService) Generate(text string) (string, error) {
	if ai == nil {
		return "", errors.New("serviço de IA não configurado")
	}

	// Prepare the request payload
	payload := map[string]interface{}{
		"model": ai.Model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "Você é um assistente prestativo para um negócio de hotelaria. Responda em português do Brasil, de forma clara e objetiva. Se não souber a resposta, diga que não sabe e sugira entrar em contato com a equipe humana.",
			},
			{
				"role":    "user",
				"content": text,
			},
		},
		"max_tokens": ai.MaxTokens,
	}

	return ai.Complete(context.Background(), payload)
}

func (ai *AIService) Complete(ctx context.Context, payload any) (string, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("erro ao codificar payload: %w", err)
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ai.Endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("erro ao criar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ai.Token)

	// Set timeout from AI service
	ctx, cancel := context.WithTimeout(ctx, ai.Timeout)
	defer cancel()
	req = req.WithContext(ctx)

	// Make the request
	resp, err := ai.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao chamar serviço de IA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("serviço de IA retornou status %d", resp.StatusCode)
	}

	// Parse the response
	var aiResponse struct {
		Result struct {
			Response string `json:"response"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&aiResponse); err != nil {
		return "", fmt.Errorf("erro ao decodificar resposta da IA: %w", err)
	}

	if !aiResponse.Success {
		return "", fmt.Errorf("serviço de IA retornou sucesso falso")
	}

	reply := strings.TrimSpace(aiResponse.Result.Response)
	if reply == "" {
		return "", errors.New("IA retornou resposta vazia")
	}
	return reply, nil
}

// Reply preserves explicit rules, then AI, then the configured fallback.
func (e *Engine) Reply(text string) string {
	rules := e.def
	rules.DefaultReply = ""
	if reply := Match(rules, text); reply != "" {
		return reply
	}
	ai := e.ai
	if e.ResolveAI != nil {
		var err error
		ai, err = e.ResolveAI()
		if err != nil {
			return e.def.DefaultReply
		}
	}
	if ai != nil {
		if reply, err := ai.Generate(text); err == nil {
			return reply
		}
	}
	return e.def.DefaultReply
}
