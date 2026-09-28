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
			{ID: "rooms", Name: "Operação de quartos", Description: "Status, ocupação, limpeza e histórico do Assistente Paraíso.", Category: "Assistente Paraíso", Enabled: false, Available: false},
			{ID: "cash", Name: "Caixa e comprovantes", Description: "Caixa diário, OCR, auditoria e relatórios PDF.", Category: "Assistente Paraíso", Enabled: false, Available: false},
			{ID: "statements", Name: "Extratos e conciliação", Description: "Processamento de PDFs e planilhas com revisão.", Category: "Assistente Paraíso", Enabled: false, Available: false},
			{ID: "advances", Name: "Vales de funcionários", Description: "Cadastro, histórico mensal e PDF.", Category: "Assistente Paraíso", Enabled: false, Available: false},
			{ID: "commercial", Name: "Atendimento comercial", Description: "Orçamentos OmniBees, catálogo e handoff humano.", Category: "Assistente Paraíso", Enabled: false, Available: false},
			{ID: "backup", Name: "Backup e saúde", Description: "Backup íntegro, retenção e diagnóstico.", Category: "Assistente Paraíso", Enabled: false, Available: false},
		},
		Roles: []Role{
			{ID: "admin", Name: "Administrador", Modules: []string{"conversations", "notifications", "audit", "flow", "files"}},
			{ID: "operator", Name: "Operador", Modules: []string{"conversations", "notifications", "audit"}},
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
	return settings, nil
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
