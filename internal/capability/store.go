package capability

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

type Settings struct {
	Modules       []Module `json:"modules"`
	Roles         []Role   `json:"roles"`
	Operators     []string `json:"operators"`
	WorkspaceRoot string   `json:"workspaceRoot"`
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
			{ID: "rooms", Name: "Operação de quartos", Description: "Status, ocupação, limpeza e histórico na UI e WhatsApp.", Category: "Assistente Paraíso", Enabled: true, Available: true},
			{ID: "cash", Name: "Caixa e comprovantes", Description: "Dashboard por período, comprovantes e relatórios PDF.", Category: "Assistente Paraíso", Enabled: true, Available: true},
			{ID: "statements", Name: "Extratos e conciliação", Description: "Painel de semanas, PDFs e planilhas processadas.", Category: "Assistente Paraíso", Enabled: true, Available: true},
			{ID: "advances", Name: "Vales de funcionários", Description: "Consulta mensal, filtro por funcionário e PDF.", Category: "Assistente Paraíso", Enabled: true, Available: true},
			{ID: "commercial", Name: "Atendimento comercial", Description: "Modos desativado, teste e público, OmniBees e catálogo.", Category: "Assistente Paraíso", Enabled: true, Available: true},
			{ID: "backup", Name: "Backup e saúde", Description: "Backup verificado, retenção e diagnóstico local.", Category: "Assistente Paraíso", Enabled: true, Available: true},
		},
		Roles: []Role{
			{ID: "admin", Name: "Administrador", Modules: []string{"conversations", "notifications", "audit", "flow", "files", "rooms", "cash", "statements", "advances", "commercial", "backup"}},
			{ID: "operator", Name: "Operador", Modules: []string{"conversations", "notifications", "audit", "rooms", "cash", "statements", "advances", "backup"}},
			{ID: "attendant", Name: "Atendente", Modules: []string{"conversations"}},
		},
	}
}

func (s *Store) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	var settings Settings
	if err := json.Unmarshal(b, &settings); err != nil {
		return Settings{}, err
	}
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
	if len(saved.Roles) > 0 {
		defaults.Roles = saved.Roles
	}
	return defaults
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
