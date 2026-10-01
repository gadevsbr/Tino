package bitz

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const DefaultURL = "https://hotel.bitzsoftwares.com.br"

type Config struct {
	Enabled        bool   `json:"enabled"`
	BaseURL        string `json:"base_url"`
	Username       string `json:"username"`
	PasswordCipher string `json:"password_cipher"`
	BotCPF         string `json:"bot_cpf"`
	ApproverPhone  string `json:"approver_phone"`
	ApprovedText   string `json:"approved_text"`
}

type PublicConfig struct {
	Enabled       bool
	PasswordSet   bool
	BaseURL       string
	Username      string
	BotCPF        string
	ApproverPhone string
	ApprovedText  string
}

type Store struct{ path string }

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Load(context.Context) (Config, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{BaseURL: DefaultURL}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err = json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultURL
	}
	return cfg, nil
}

func (s *Store) Save(ctx context.Context, public PublicConfig, password string) error {
	cfg, err := s.Load(ctx)
	if err != nil {
		return err
	}
	public.BaseURL = strings.TrimRight(strings.TrimSpace(public.BaseURL), "/")
	if public.BaseURL != DefaultURL {
		return errors.New("o endereço do Bitz deve ser o oficial configurado")
	}
	if strings.TrimSpace(public.Username) == "" {
		return errors.New("usuário Bitz obrigatório")
	}
	if !validCPF(public.BotCPF) {
		return errors.New("CPF operacional inválido")
	}
	phone := digits(public.ApproverPhone)
	if len(phone) < 10 || len(phone) > 15 {
		return errors.New("WhatsApp aprovador deve conter DDI")
	}
	if strings.TrimSpace(public.ApprovedText) == "" {
		return errors.New("mensagem após aprovação obrigatória")
	}
	if password != "" {
		cfg.PasswordCipher, err = protect(password)
		if err != nil {
			return err
		}
	} else if cfg.PasswordCipher == "" {
		return errors.New("senha Bitz obrigatória")
	}
	cfg.Enabled, cfg.BaseURL, cfg.Username, cfg.BotCPF, cfg.ApproverPhone, cfg.ApprovedText = public.Enabled, public.BaseURL, strings.TrimSpace(public.Username), digits(public.BotCPF), phone, strings.TrimSpace(public.ApprovedText)
	if err = os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func (s *Store) Public(ctx context.Context) (PublicConfig, error) {
	cfg, err := s.Load(ctx)
	if err != nil {
		return PublicConfig{}, err
	}
	return PublicConfig{Enabled: cfg.Enabled, PasswordSet: cfg.PasswordCipher != "", BaseURL: cfg.BaseURL, Username: cfg.Username, BotCPF: cfg.BotCPF, ApproverPhone: cfg.ApproverPhone, ApprovedText: cfg.ApprovedText}, nil
}
func (s *Store) Credentials(ctx context.Context) (Config, string, error) {
	return s.credentials(ctx, true)
}
func (s *Store) TestCredentials(ctx context.Context) (Config, string, error) {
	return s.credentials(ctx, false)
}
func (s *Store) credentials(ctx context.Context, requireEnabled bool) (Config, string, error) {
	cfg, err := s.Load(ctx)
	if err != nil {
		return Config{}, "", err
	}
	if requireEnabled && !cfg.Enabled {
		return Config{}, "", errors.New("integração Bitz desativada")
	}
	password, err := unprotect(cfg.PasswordCipher)
	return cfg, password, err
}

func digits(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func validCPF(v string) bool {
	n := digits(v)
	if len(n) != 11 {
		return false
	}
	same := true
	for i := 1; i < len(n); i++ {
		same = same && n[i] == n[0]
	}
	if same {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(n[i]-'0') * (10 - i)
	}
	d := 11 - sum%11
	if d >= 10 {
		d = 0
	}
	if d != int(n[9]-'0') {
		return false
	}
	sum = 0
	for i := 0; i < 10; i++ {
		sum += int(n[i]-'0') * (11 - i)
	}
	d = 11 - sum%11
	if d >= 10 {
		d = 0
	}
	return d == int(n[10]-'0')
}
