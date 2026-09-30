// Package commercial provides the guest-only conversation boundary. Transport
// must route only private inbound guest messages here, with canonical identities.
package commercial

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/omnibees"
)

type Mode string

const (
	Disabled Mode = "disabled"
	Test     Mode = "test"
	Public   Mode = "public"
)

type Config struct {
	Epoch         int64    `json:"epoch"`
	Mode          Mode     `json:"mode"`
	Allowlist     []string `json:"allowlist"`
	GroupPhone    string   `json:"group_phone"`
	FinalMessage1 string   `json:"final_message_1"`
	FinalMessage2 string   `json:"final_message_2"`
}
type Input struct {
	Account, Contact, MessageID, Text string
	Now                               time.Time
}
type Reply struct {
	Text             string
	Messages         []string
	Handled, Handoff bool
	GroupRequest     string
	Categories       []omnibees.Category
	SelectedCategory *omnibees.Category
}
type QuoteReply struct {
	Text         string
	Messages     []string
	Active       bool
	Categories   []omnibees.Category
	GroupRequest string
}
type QuoteFunc func(context.Context, string, string, string) (QuoteReply, error)

type Service struct {
	configLock sync.Mutex
	db         *sql.DB
	quote      QuoteFunc
	locks      [64]sync.Mutex
}
type state struct {
	ConfigEpoch      int64               `json:"config_epoch"`
	LastSeen         time.Time           `json:"last_seen"`
	Paused           bool                `json:"paused"`
	QuoteActive      bool                `json:"quote_active"`
	QuotedAt         time.Time           `json:"quoted_at"`
	Categories       []omnibees.Category `json:"categories"`
	SelectedCategory *omnibees.Category  `json:"selected_category,omitempty"`
	AwaitingChoice   bool                `json:"awaiting_choice"`
}

const defaultGroupPhone = "5573988240413"
const menu = "Como posso ajudar?\n1 — Fazer orçamento\n2 — Tratar de outros assuntos"
const greeting = "Olá! Bem-vindo ao Hotel Paraíso Tropical. Sou o assistente virtual do hotel."

// New creates only this package's tables, without changing domain migrations.
// Construct one service per running transport and reuse it for all messages.
func New(ctx context.Context, db *sql.DB, quote QuoteFunc) (*Service, error) {
	if db == nil || quote == nil {
		return nil, errors.New("banco e fluxo de orçamento são obrigatórios")
	}
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS commercial_config (
	 account TEXT PRIMARY KEY, config TEXT NOT NULL);
	 CREATE TABLE IF NOT EXISTS commercial_contacts (
	 account TEXT NOT NULL, contact TEXT NOT NULL, state TEXT NOT NULL,
	 PRIMARY KEY(account,contact));
	 CREATE TABLE IF NOT EXISTS commercial_messages (
	 account TEXT NOT NULL, contact TEXT NOT NULL, message_id TEXT NOT NULL,
	 PRIMARY KEY(account,contact,message_id));`)
	if err != nil {
		return nil, err
	}
	return &Service{db: db, quote: quote}, nil
}

func validIdentity(account, contact string) error {
	if strings.TrimSpace(account) == "" || strings.TrimSpace(contact) == "" {
		return errors.New("conta e contato obrigatórios")
	}
	return nil
}
func (s *Service) lock(account, contact string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(account + "\x00" + contact))
	m := &s.locks[h.Sum32()%uint32(len(s.locks))]
	m.Lock()
	return m.Unlock
}

// Configure is an operator-only API. Transport is responsible for authorization.
// Allowlist entries are exact canonical contact IDs, never unverified aliases.
func (s *Service) Configure(ctx context.Context, account string, config Config) error {
	s.configLock.Lock()
	defer s.configLock.Unlock()
	if strings.TrimSpace(account) == "" {
		return errors.New("conta obrigatória")
	}
	if config.Mode != Disabled && config.Mode != Test && config.Mode != Public {
		return errors.New("modo comercial inválido")
	}
	seen := map[string]bool{}
	entries := []string{}
	for _, contact := range config.Allowlist {
		contact = strings.TrimSpace(contact)
		if contact == "" {
			return errors.New("contato de teste vazio")
		}
		if !seen[contact] {
			entries = append(entries, contact)
			seen[contact] = true
		}
	}
	config.Allowlist = entries
	config.GroupPhone = onlyDigits(config.GroupPhone)
	if config.GroupPhone == "" {
		config.GroupPhone = defaultGroupPhone
	}
	if len(config.GroupPhone) < 10 || len(config.GroupPhone) > 15 {
		return errors.New("telefone do setor de grupos deve conter DDI e entre 10 e 15 dígitos")
	}
	config.FinalMessage1 = strings.TrimSpace(config.FinalMessage1)
	config.FinalMessage2 = strings.TrimSpace(config.FinalMessage2)
	if len([]rune(config.FinalMessage1)) > 2000 || len([]rune(config.FinalMessage2)) > 2000 {
		return errors.New("cada mensagem final deve ter no máximo 2000 caracteres")
	}
	previous, err := s.Configuration(ctx, account)
	if err != nil {
		return err
	}
	config.Epoch = previous.Epoch + 1
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO commercial_config(account,config) VALUES (?,?) ON CONFLICT(account) DO UPDATE SET config=excluded.config`, account, string(data))
	return err
}

func onlyDigits(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func (s *Service) Configuration(ctx context.Context, account string) (Config, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT config FROM commercial_config WHERE account=?`, account).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Config{Mode: Disabled, GroupPhone: defaultGroupPhone}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var config Config
	err = json.Unmarshal([]byte(raw), &config)
	if config.GroupPhone == "" {
		config.GroupPhone = defaultGroupPhone
	}
	return config, err
}
func (s *Service) load(ctx context.Context, account, contact string) (state, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT state FROM commercial_contacts WHERE account=? AND contact=?`, account, contact).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return state{}, nil
	}
	if err != nil {
		return state{}, err
	}
	var st state
	err = json.Unmarshal([]byte(raw), &st)
	return st, err
}
func (s *Service) save(ctx context.Context, in Input, st state) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO commercial_contacts(account,contact,state) VALUES (?,?,?) ON CONFLICT(account,contact) DO UPDATE SET state=excluded.state`, in.Account, in.Contact, string(data))
	if err != nil {
		return err
	}
	if in.MessageID != "" {
		_, err = tx.ExecContext(ctx, `INSERT INTO commercial_messages(account,contact,message_id) VALUES (?,?,?)`, in.Account, in.Contact, in.MessageID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Resume explicitly releases human ownership; elapsed time never does so.
func (s *Service) Resume(ctx context.Context, account, contact string) error {
	if err := validIdentity(account, contact); err != nil {
		return err
	}
	unlock := s.lock(account, contact)
	defer unlock()
	st, err := s.load(ctx, account, contact)
	if err != nil {
		return err
	}
	st.Paused = false
	if st.QuoteActive {
		if _, err = s.quote(ctx, account, contact, "cancelar"); err != nil {
			return err
		}
	}
	st.QuoteActive = false
	st.Categories = nil
	st.SelectedCategory = nil
	st.AwaitingChoice = false
	st.QuotedAt = time.Time{}
	return s.save(ctx, Input{Account: account, Contact: contact}, st)
}

func (s *Service) Handle(ctx context.Context, in Input) (Reply, error) {
	if err := validIdentity(in.Account, in.Contact); err != nil {
		return Reply{}, err
	}
	if in.MessageID == "" {
		return Reply{}, errors.New("ID de mensagem obrigatório")
	}
	if in.Now.IsZero() {
		in.Now = time.Now()
	}
	unlock := s.lock(in.Account, in.Contact)
	defer unlock()
	config, err := s.Configuration(ctx, in.Account)
	if err != nil {
		return Reply{}, err
	}
	allowed := config.Mode == Public
	if config.Mode == Test {
		for _, contact := range config.Allowlist {
			if contact == in.Contact {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return Reply{}, nil
	}
	var exists int
	err = s.db.QueryRowContext(ctx, `SELECT 1 FROM commercial_messages WHERE account=? AND contact=? AND message_id=?`, in.Account, in.Contact, in.MessageID).Scan(&exists)
	if err == nil {
		return Reply{Handled: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Reply{}, err
	}
	st, err := s.load(ctx, in.Account, in.Contact)
	if err != nil {
		return Reply{}, err
	}
	idle := st.LastSeen.IsZero() || in.Now.Sub(st.LastSeen) >= 24*time.Hour
	if in.Now.After(st.LastSeen) {
		st.LastSeen = in.Now
	}
	text := strings.ToLower(strings.TrimSpace(in.Text))
	r := Reply{Handled: true}
	finish := func() (Reply, error) {
		if err := s.save(ctx, in, st); err != nil {
			return Reply{}, err
		}
		return r, nil
	}
	if st.Paused {
		if text == "retomar atendimento" {
			st.Paused = false
			st.QuoteActive = false
			st.Categories = nil
			st.SelectedCategory = nil
			r.Text = "Atendimento automático retomado.\n\n" + menu
		}
		return finish()
	}
	if idle || st.ConfigEpoch != config.Epoch {
		if st.QuoteActive {
			if _, err = s.quote(ctx, in.Account, in.Contact, "cancelar"); err != nil {
				return Reply{}, err
			}
		}
		st.QuoteActive = false
		st.Categories = nil
		st.SelectedCategory = nil
		st.AwaitingChoice = true
	}
	st.ConfigEpoch = config.Epoch
	if idle {
		r.Text = greeting + "\n\n" + menu
		return finish()
	}
	budgetChoice := text == "orçamento" || text == "orcamento" || text == "1"
	if st.AwaitingChoice && !budgetChoice {
		st.AwaitingChoice = false
		st.Paused = true
		r.Handoff = true
		r.Text = "Certo. Vou deixar sua conversa para um atendente humano responder."
	} else if text == "atendimento humano" || text == "atendente" || text == "humano" || text == "falar com atendente" || text == "falar com um atendente" || text == "outros assuntos" || (!st.QuoteActive && len(st.Categories) == 0 && text == "2") {
		if st.QuoteActive {
			if _, err = s.quote(ctx, in.Account, in.Contact, "cancelar"); err != nil {
				return Reply{}, err
			}
		}
		st.Paused = true
		st.QuoteActive = false
		r.Handoff = true
		r.Text = "Você solicitou atendimento humano. O assistente automático ficará pausado. Para voltar ao assistente, envie retomar atendimento."
	} else if budgetChoice {
		st.AwaitingChoice = false
		q, e := s.quote(ctx, in.Account, in.Contact, "orçamento")
		if e != nil {
			return Reply{}, e
		}
		st.QuoteActive = q.Active
		st.Categories = q.Categories
		r.Text = q.Text
		r.Messages = append(r.Messages, q.Messages...)
		r.GroupRequest = q.GroupRequest
		r.Categories = q.Categories
		if len(q.Categories) > 0 {
			st.QuotedAt = in.Now
		}
	} else if text == "cancelar" || text == "menu" {
		if st.QuoteActive {
			if _, e := s.quote(ctx, in.Account, in.Contact, "cancelar"); e != nil {
				return Reply{}, e
			}
		}
		st.QuoteActive = false
		st.Categories = nil
		r.Text = menu
	} else if st.QuoteActive {
		q, e := s.quote(ctx, in.Account, in.Contact, in.Text)
		if e != nil {
			return Reply{}, e
		}
		st.QuoteActive = q.Active
		st.Categories = q.Categories
		r.Text = q.Text
		r.Messages = append(r.Messages, q.Messages...)
		r.GroupRequest = q.GroupRequest
		r.Categories = q.Categories
		if len(q.Categories) > 0 {
			st.QuotedAt = in.Now
			if strings.TrimSpace(config.FinalMessage1) != "" {
				r.Messages = append(r.Messages, config.FinalMessage1)
			}
			if strings.TrimSpace(config.FinalMessage2) != "" {
				r.Messages = append(r.Messages, config.FinalMessage2)
			}
		}
	} else if len(st.Categories) > 0 {
		if in.Now.Sub(st.QuotedAt) >= 24*time.Hour {
			st.Categories = nil
			r.Text = "Este orçamento expirou. Envie orçamento para consultar valores atualizados."
		} else {
			for i, c := range st.Categories {
				if text == strconv.Itoa(i+1) || text == strings.ToLower(c.Key) || text == strings.ToLower(c.Name) {
					selected := c
					r.SelectedCategory = &selected
					st.SelectedCategory = &selected
					r.Handoff = true
					st.Paused = true
					r.Text = "Você escolheu " + c.Name + ". Solicitação de atendimento humano registrada para a equipe confirmar disponibilidade e reserva. Nenhuma reserva foi confirmada. Para voltar ao assistente, envie retomar atendimento."
					break
				}
			}
			if r.SelectedCategory == nil {
				r.Text = "Escolha um dos números das categorias do orçamento ou envie atendente."
			}
		}
	} else {
		r.Text = menu
	}
	return finish()
}
