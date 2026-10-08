package capability

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Module struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
	Available   bool   `json:"available"`
}

type Role struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Modules []string `json:"modules"`
}

type AIConfig struct {
	Endpoint  string `json:"endpoint"`
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	Timeout   string `json:"timeout"`
}

type Settings struct {
	Modules       []Module `json:"modules"`
	Roles         []Role   `json:"roles"`
	Operators     []string `json:"operators"`
	WorkspaceRoot string   `json:"workspaceRoot"`
	AIConfig      AIConfig `json:"ai"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store { return &Store{path: path} }

func Defaults() Settings {
	return Settings{
		Operators: []string{},
		Modules: []Module{
			{ID: "conversations", Name: "Conversas", Description: "Histórico local e respostas manuais.", Category: "Tino", Enabled: true, Available: true},
			{ID: "notifications", Name: "Notificações", Description: "Listas consentidas com limites operacionais.", Category: "Tino", Enabled: true, Available: true},
			{ID: "audit", Name: "Auditoria de contatos", Description: "Exportação local em JSON e CSV.", Category: "Tino", Enabled: true, Available: true},
			{ID: "flow", Name: "Flow Builder", Description: "Regras automáticas de atendimento.", Category: "Tino", Enabled: false, Available: true},
			{ID: "files", Name: "Arquivos autorizados", Description: "Acesso restrito à pasta escolhida pelo administrador.", Category: "Plataforma", Enabled: false, Available: true},
			{ID: "ai", Name: "Inteligência Artificial", Description: "Integração com serviços de IA para respostas mais inteligentes.", Category: "Tino", Enabled: true, Available: true},
			{ID: "rooms", Name: "Operação de quartos", Description: "Status, ocupação, limpeza e histórico na UI e WhatsApp.", Category: "Assistente Paraíso", Enabled: true, Available: true},
			{ID: "cash", Name: "Caixa e comprovantes", Description: "Dashboard por período, comprovantes e relatórios PDF.", Category: "Assistente Paraíso", Enabled: true, Available: true},
			{ID: "statements", Name: "Extratos e conciliação", Description: "Painel de semanas, PDFs e planilhas processadas.", Category: "Assistente Paraíso", Enabled: true, Available: true},
			{ID: "advances", Name: "Vales de funcionários", Description: "Consulta mensal, filtro por funcionário e PDF.", Category: "Assistente Paraíso", Enabled: true, Available: true},
			{ID: "commercial", Name: "Atendimento comercial", Description: "Modos desativado, teste e público, OmniBees e catálogo.", Category: "Assistente Paraíso", Enabled: true, Available: true},
			{ID: "backup", Name: "Backup e saúde", Description: "Backup verificado, retenção e diagnóstico local.", Category: "Assistente Paraíso", Enabled: true, Available: true},
		},
		Roles: []Role{
			{ID: "admin", Name: "Administrador", Modules: []string{"conversations", "notifications", "audit", "flow", "files", "ai", "rooms", "cash", "statements", "advances", "commercial", "backup"}},
			{ID: "operator", Name: "Operador", Modules: []string{"conversations", "notifications", "audit", "rooms", "cash", "statements", "advances", "backup"}},
			{ID: "attendant", Name: "Atendente", Modules: []string{"conversations"}},
		},
		AIConfig: AIConfig{
			Endpoint:  "",
			Model:     "@cf/meta/llama-3.1-8b-instruct-fp8", // Free model on Cloudflare Workers AI
			MaxTokens: 500,
			Timeout:   "30s",
		},
	}
}

func (s *Store) Load() (Settings, error) {
	s.mu.Lock()
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.mu.Unlock()
		settings := Defaults()
		// Save the defaults to create the file on first run
		if err := s.Save(settings); err != nil {
			// If we can't save, we still return the defaults but log the error
			// In a real app, we might want to handle this differently
			fmt.Printf("warning: could not save default capabilities: %v\n", err)
		}
		return settings, nil
	}
	if err != nil {
		s.mu.Unlock()
		return Settings{}, err
	}
	var settings Settings
	if err := json.Unmarshal(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf}), &settings); err != nil {
		s.mu.Unlock()
		return Settings{}, err
	}
	s.mu.Unlock()
	settings = reconcile(settings)
	return settings, nil
}

func reconcile(saved Settings) Settings {
	defaults := Defaults()
	old := make(map[string]Module, len(saved.Modules))
	for _, module := range saved.Modules {
		old[module.ID] = module
	}
	for i, module := range defaults.Modules {
		if previous, ok := old[module.ID]; ok && previous.Available {
			module.Enabled = previous.Enabled
		}
		defaults.Modules[i] = module
	}
	defaults.Operators = saved.Operators
	defaults.WorkspaceRoot = saved.WorkspaceRoot
	// Only preserve AIConfig if it has been configured (non-empty endpoint)
	if saved.AIConfig.Endpoint != "" && !strings.Contains(saved.AIConfig.Endpoint, "YOUR_CLOUDFLARE") {
		defaults.AIConfig = saved.AIConfig
	}
	if len(saved.Roles) > 0 {
		defaults.Roles = saved.Roles
	}
	return defaults
}

func (s *Store) GetAIConfig() (AIConfig, error) {
	settings, err := s.Load()
	if err != nil {
		return AIConfig{}, err
	}
	return settings.AIConfig, nil
}

func (s *Store) Save(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}
