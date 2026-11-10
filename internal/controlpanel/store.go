package controlpanel

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type storedConfig struct {
	Enabled              bool   `json:"enabled"`
	Endpoint             string `json:"endpoint"`
	DashboardURL         string `json:"dashboard_url"`
	InstanceID           string `json:"instance_id"`
	TokenCipher          string `json:"token_cipher,omitempty"`
	DashboardTokenCipher string `json:"dashboard_token_cipher,omitempty"`
}

type Store struct{ path string }

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Load() (Config, string, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, "", nil
	}
	if err != nil {
		return Config{}, "", err
	}
	var stored storedConfig
	if err := json.Unmarshal(data, &stored); err != nil {
		return Config{}, "", err
	}
	var token string
	if stored.TokenCipher != "" {
		token, err = unprotect(stored.TokenCipher)
		if err != nil {
			return Config{}, "", err
		}
	}
	return Config{Enabled: stored.Enabled, Endpoint: stored.Endpoint, DashboardURL: stored.DashboardURL, InstanceID: stored.InstanceID, PasswordSet: token != "", DashboardTokenSet: stored.DashboardTokenCipher != ""}, token, nil
}

func (s *Store) DashboardToken() (string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return "", err
	}
	var stored storedConfig
	if err := json.Unmarshal(data, &stored); err != nil {
		return "", err
	}
	if stored.DashboardTokenCipher == "" {
		return "", errors.New("token do painel não configurado")
	}
	return unprotect(stored.DashboardTokenCipher)
}

func (s *Store) Save(cfg Config, token, dashboardToken string) error {
	cfg.Endpoint, cfg.DashboardURL, cfg.InstanceID, token, dashboardToken = strings.TrimSpace(cfg.Endpoint), strings.TrimSpace(cfg.DashboardURL), strings.TrimSpace(cfg.InstanceID), strings.TrimSpace(token), strings.TrimSpace(dashboardToken)
	previous, oldToken, err := s.Load()
	if err != nil {
		return err
	}
	if token == "" {
		token = oldToken
	}
	oldDashboardToken, _ := s.DashboardToken()
	if dashboardToken == "" {
		dashboardToken = oldDashboardToken
	}
	if cfg.Enabled && (cfg.Endpoint == "" || cfg.InstanceID == "" || token == "" || dashboardToken == "") {
		return errors.New("endereço, instalação e os dois tokens são obrigatórios para ativar o painel")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = previous.Endpoint
	}
	if cfg.DashboardURL == "" {
		cfg.DashboardURL = previous.DashboardURL
	}
	if cfg.InstanceID == "" {
		cfg.InstanceID = previous.InstanceID
	}
	cipher := ""
	if token != "" {
		cipher, err = protect(token)
		if err != nil {
			return err
		}
	}
	dashboardCipher := ""
	if dashboardToken != "" {
		dashboardCipher, err = protect(dashboardToken)
		if err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(storedConfig{Enabled: cfg.Enabled, Endpoint: cfg.Endpoint, DashboardURL: cfg.DashboardURL, InstanceID: cfg.InstanceID, TokenCipher: cipher, DashboardTokenCipher: dashboardCipher}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
