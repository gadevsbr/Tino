package flow

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

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
type Engine struct {
	client *whatsmeow.Client
	def    Definition
	mu     sync.Mutex
	seen   map[string]struct{}
}

func Load(path string, client *whatsmeow.Client) (*Engine, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ler fluxo: %w", err)
	}
	var def Definition
	if err := yaml.Unmarshal(b, &def); err != nil {
		return nil, fmt.Errorf("decodificar fluxo: %w", err)
	}
	for i, r := range def.Rules {
		if r.Contains == "" || r.Reply == "" {
			return nil, fmt.Errorf("regra %d requer contains e reply", i+1)
		}
	}
	return &Engine{client: client, def: def, seen: make(map[string]struct{})}, nil
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
	reply := e.match(text)
	if reply == "" {
		return
	}
	ctx := context.Background()
	if _, err := e.client.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{Conversation: proto.String(reply)}); err != nil {
		fmt.Fprintf(os.Stderr, "erro ao responder %s: %v\n", msg.Info.Chat, err)
	}
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
	for _, r := range e.def.Rules {
		hay, needle := text, r.Contains
		if !r.CaseSensitive {
			hay, needle = strings.ToLower(hay), strings.ToLower(needle)
		}
		if strings.Contains(hay, needle) {
			return r.Reply
		}
	}
	return e.def.DefaultReply
}
