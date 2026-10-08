package flow

import (
	"fmt"
	"github.com/gadevsbr/tino/internal/capability"
	"net/http"
	"net/url"
	"time"
)

func ConfiguredAI(store *capability.Store) (*AIService, error) {
	settings, err := store.Load()
	if err != nil {
		return nil, err
	}
	enabled := false
	for _, m := range settings.Modules {
		if m.ID == "ai" {
			enabled = m.Enabled
		}
	}
	if !enabled {
		return nil, nil
	}
	cfg := settings.AIConfig
	if cfg.Endpoint == "" {
		return nil, nil
	}
	token, err := store.AIToken()
	if err != nil {
		return nil, fmt.Errorf("ative a IA na Central de recursos")
	}
	return NewAIService(cfg.Endpoint, token)
}

func NewAIService(endpoint, token string) (*AIService, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || token == "" {
		return nil, fmt.Errorf("informe o endereço HTTPS do Worker e a chave de acesso")
	}
	return &AIService{Endpoint: endpoint, Token: token, Model: "@cf/meta/llama-3.1-8b-instruct-fp8", MaxTokens: 500, Timeout: 30 * time.Second, Client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
