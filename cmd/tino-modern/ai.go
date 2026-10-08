package main

import (
	"fmt"
	"github.com/gadevsbr/tino/internal/flow"
	"strings"
)

type AIStatusDTO struct {
	CommercialCalls uint64 `json:"commercialCalls"`
	Endpoint        string `json:"endpoint"`
	Ready           bool   `json:"ready"`
	Model           string `json:"model"`
}

func (a *App) GetAIStatus() (AIStatusDTO, error) {
	cfg, err := a.capabilities.GetAIConfig()
	if err != nil {
		return AIStatusDTO{}, err
	}
	token, _ := a.capabilities.AIToken()
	return AIStatusDTO{Endpoint: cfg.Endpoint, Ready: cfg.Endpoint != "" && token != "", Model: "Llama 3.3 70B (atendimento) / 3.1 8B (sugestões)", CommercialCalls: flow.CommercialAICalls.Load()}, nil
}

func (a *App) ConfigureAI(endpoint, token string) error {
	endpoint = strings.TrimSpace(endpoint)
	if token == "" {
		previous, err := a.capabilities.GetAIConfig()
		if err != nil {
			return err
		}
		if previous.Endpoint != endpoint {
			return fmt.Errorf("informe uma nova chave ao trocar o serviço de IA")
		}
		token, _ = a.capabilities.AIToken()
	}
	token = strings.TrimSpace(token)
	service, err := flow.NewAIService(endpoint, token)
	if err != nil {
		return err
	}
	if _, err := service.Generate("Responda apenas: conexão funcionando."); err != nil {
		return fmt.Errorf("teste de conexão: %w", err)
	}
	settings, err := a.capabilities.Load()
	if err != nil {
		return err
	}
	if err := a.capabilities.SaveAIToken(token); err != nil {
		return err
	}
	settings.AIConfig.Endpoint = endpoint
	for i := range settings.Modules {
		if settings.Modules[i].ID == "ai" {
			settings.Modules[i].Enabled = true
		}
	}
	return a.capabilities.Save(settings)
}

func (a *App) TestAI(text string) (string, error) {
	if err := a.requireCapability("ai"); err != nil {
		return "", err
	}
	service, err := flow.ConfiguredAI(a.capabilities)
	if err != nil {
		return "", err
	}
	if service == nil {
		return "", fmt.Errorf("ative a IA na Central de recursos")
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 8000 {
		return "", fmt.Errorf("informe uma mensagem de até 8000 bytes")
	}
	return service.Generate(text)
}

func (a *App) SuggestAIReply(jid string) (string, error) {
	if strings.HasSuffix(jid, "@g.us") {
		return "", fmt.Errorf("sugestões de IA disponíveis em conversas individuais")
	}
	messages, err := a.GetMessages(jid)
	if err != nil {
		return "", err
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if !messages[i].FromMe && strings.TrimSpace(messages[i].Text) != "" {
			return a.TestAI(messages[i].Text)
		}
	}
	return "", fmt.Errorf("nenhuma mensagem recebida para responder")
}
