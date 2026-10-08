package flow

import (
	"github.com/gadevsbr/tino/internal/assistente/queue"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAIReplyRulesFallbackAndAuthentication(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"result":{"response":"Resposta de teste"}}`))
	}))
	defer server.Close()
	engine := Engine{def: Definition{Rules: []Rule{{Name: "Regra", Contains: "horario", Reply: "Horário confirmado"}}, DefaultReply: "Fallback"}, ai: &AIService{Endpoint: server.URL, Token: "test-key", Client: server.Client(), Timeout: time.Second}}
	if got := engine.Reply("horario"); got != "Horário confirmado" || calls != 0 {
		t.Fatal("rule precedence", got, calls)
	}
	if got := engine.Reply("Outra pergunta"); got != "Resposta de teste" || calls != 1 {
		t.Fatal("AI fallback skipped", got, calls)
	}
	engine.ai.Endpoint = server.URL + "/broken"
	engine.ai.Client = &http.Client{Transport: errorTransport{}}
	if got := engine.Reply("Outra pergunta"); got != "Fallback" {
		t.Fatal("default fallback", got)
	}
}

type errorTransport struct{}

func (errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, http.ErrHandlerTimeout
}

func TestSlowAIContextDoesNotBlockWhatsAppDispatcher(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	engine := Engine{seen: map[string]struct{}{}, queue: queue.New(), Allow: func(*events.Message) bool { close(entered); <-release; return false }}
	returned := make(chan struct{})
	go func() {
		engine.Handle(&events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("5511999999999", types.DefaultUserServer)}, ID: "slow-context"}, Message: &waE2E.Message{Conversation: proto.String("teste")}})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("event dispatcher blocked")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("queued task not started")
	}
}

func TestAIRejectsEmptyAndOversizedResponses(t *testing.T) {
	for _, body := range []string{`{"success":true,"result":{"response":" "}}`, `{"success":false}`, `invalid`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer server.Close()
			ai := AIService{Endpoint: server.URL, Client: server.Client(), Timeout: time.Second}
			if _, err := ai.Generate("teste"); err == nil {
				t.Fatal("invalid result accepted")
			}
		})
	}
}
