// Package commercial provides the guest-only conversation boundary. Transport
// must route only private inbound guest messages here, with canonical identities.
package commercial

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	Audio                             bool
	// TestSession is set only by the authenticated operator transport. It lets
	// an isolated synthetic contact exercise test mode without reusing the
	// configured guest's conversation state.
	TestSession bool
}
type Reply struct {
	Text             string
	Messages         []string
	Handled, Handoff bool
	GroupRequest     string
	SelectionPrompt  string
	Categories       []omnibees.Category
	SelectedCategory *omnibees.Category
	PreReservation   *PreReservation
}
type QuoteRoom struct {
	Adults     int                 `json:"adults"`
	Ages       []int               `json:"ages"`
	Categories []omnibees.Category `json:"categories"`
}
type QuotePlan struct {
	CheckIn, CheckOut string
	Rooms             []QuoteRoom
}
type PreReservation struct {
	CheckIn, CheckOut string
	Categories        []omnibees.Category
	Rooms             []QuoteRoom
}
type QuoteReply struct {
	Handoff      bool
	Text         string
	Messages     []string
	Active       bool
	Categories   []omnibees.Category
	GroupRequest string
	Plan         *QuotePlan
}
type QuoteFunc func(context.Context, string, string, string) (QuoteReply, error)

type Service struct {
	assistantMu sync.RWMutex
	assistant   AssistantFunc
	configLock  sync.Mutex
	db          *sql.DB
	quote       QuoteFunc
	locks       [64]sync.Mutex
}

func (s *Service) SetAssistant(fn AssistantFunc) {
	s.assistantMu.Lock()
	s.assistant = fn
	s.assistantMu.Unlock()
}
func (s *Service) Assistant() AssistantFunc {
	s.assistantMu.RLock()
	defer s.assistantMu.RUnlock()
	return s.assistant
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
	Plan             *QuotePlan          `json:"plan,omitempty"`
	SelectedRooms    []omnibees.Category `json:"selected_rooms,omitempty"`
}

const defaultGroupPhone = "5573988240413"
const menu = "Como posso ajudar hoje?\n1 — Fazer um orçamento\n2 — Falar com um atendente sobre outro assunto\n\nResponda com 1 ou 2."
const greeting = "Olá! Que bom receber sua mensagem. Sou o assistente virtual do Hotel Paraíso Tropical."

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
	st.Plan = nil
	st.SelectedRooms = nil
	return s.save(ctx, Input{Account: account, Contact: contact}, st)
}

// Reset removes an isolated conversation state so its next message starts at
// the greeting. Transport uses this when an operator starts or exits a test.
func (s *Service) Reset(ctx context.Context, account, contact string) error {
	if err := validIdentity(account, contact); err != nil {
		return err
	}
	unlock := s.lock(account, contact)
	defer unlock()
	st, err := s.load(ctx, account, contact)
	if err != nil {
		return err
	}
	if st.QuoteActive {
		if _, err = s.quote(ctx, account, contact, "cancelar"); err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM commercial_contacts WHERE account=? AND contact=?`, account, contact)
	return err
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
	allowed := config.Mode == Public || (config.Mode == Test && in.TestSession)
	if config.Mode == Test && !in.TestSession {
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
	guestMessage := in.Text
	assist := s.Assistant()
	if !idle && !st.Paused && !in.Audio && assist != nil {
		if st.AwaitingChoice {
			if text != "1" && text != "2" && text != "sair" && text != "cancelar" {
				in.Text = NormalizeDialogue(ctx, assist, DialogueRequest{Step: "TRIAGE", Message: in.Text, Options: []string{"orçamento", "atendente"}})
			}
		} else if !st.QuoteActive && (st.Plan != nil || len(st.Categories) > 0) {
			categories := st.Categories
			if st.Plan != nil && len(st.SelectedRooms) < len(st.Plan.Rooms) {
				categories = st.Plan.Rooms[len(st.SelectedRooms)].Categories
			}
			if _, err := strconv.Atoi(text); err != nil && text != "sair" && text != "cancelar" && text != "atendente" {
				options := make([]string, len(categories))
				for i, c := range categories {
					options[i] = fmt.Sprintf("%d — %s", i+1, c.Name)
				}
				// Category values are canonical indexes, while labels are context.
				answer, err := assist(ctx, DialogueRequest{Task: "interpret", Step: "CATEGORY", Message: in.Text, Options: options, Today: HotelToday()})
				if err == nil && answer.Understood {
					index, e := strconv.Atoi(answer.Value)
					if e == nil && index >= 1 && index <= len(categories) {
						in.Text = answer.Value
					} else if answer.Value == "atendente" || answer.Value == "sair" {
						in.Text = answer.Value
					}
				}
			}
		}
		text = strings.ToLower(strings.TrimSpace(in.Text))
	}
	r := Reply{Handled: true}
	finish := func() (Reply, error) {
		if err := s.save(ctx, in, st); err != nil {
			return Reply{}, err
		}
		r.Text = HumanizeDialogue(ctx, assist, guestMessage, r.Text)
		if r.SelectionPrompt != "" {
			r.SelectionPrompt = HumanizeDialogue(ctx, assist, guestMessage, r.SelectionPrompt)
		}
		return r, nil
	}
	if in.Audio {
		if st.QuoteActive {
			if _, err = s.quote(ctx, in.Account, in.Contact, "cancelar"); err != nil {
				return Reply{}, err
			}
		}
		st.Paused = true
		st.QuoteActive = false
		st.AwaitingChoice = false
		st.Categories = nil
		st.SelectedCategory = nil
		st.Plan = nil
		st.SelectedRooms = nil
		r.Handoff = true
		r.Text = "Recebi seu áudio. Para que sua mensagem seja atendida com atenção, encaminhei a conversa diretamente para nossa equipe. Um atendente responderá por aqui assim que possível."
		return finish()
	}
	if text == "sair" {
		if st.QuoteActive {
			if _, err = s.quote(ctx, in.Account, in.Contact, "cancelar"); err != nil {
				return Reply{}, err
			}
		}
		st = state{ConfigEpoch: config.Epoch}
		r.Text = "Atendimento encerrado. Quando precisar, é só enviar uma nova mensagem."
		return finish()
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
		st.Plan = nil
		st.SelectedRooms = nil
	}
	st.ConfigEpoch = config.Epoch
	if idle {
		r.Text = greeting + "\n\n" + menu
		return finish()
	}
	explicitBudget := text == "orçamento" || text == "orcamento"
	menuBudget := text == "1"
	budgetChoice := explicitBudget || menuBudget
	if st.AwaitingChoice && !budgetChoice {
		st.AwaitingChoice = false
		st.Paused = true
		r.Handoff = true
		r.Text = "Certo. Encaminhei sua conversa para nossa equipe. Um atendente responderá por aqui assim que possível."
	} else if text == "atendimento humano" || text == "atendente" || text == "humano" || text == "falar com atendente" || text == "falar com um atendente" || text == "outros assuntos" || (!st.QuoteActive && len(st.Categories) == 0 && text == "2") {
		if st.QuoteActive {
			if _, err = s.quote(ctx, in.Account, in.Contact, "cancelar"); err != nil {
				return Reply{}, err
			}
		}
		st.Paused = true
		st.QuoteActive = false
		r.Handoff = true
		r.Text = "Tudo certo. Encaminhei sua conversa para nossa equipe e pausei o atendimento automático. Um atendente responderá por aqui assim que possível."
	} else if explicitBudget || (menuBudget && !st.QuoteActive && len(st.Categories) == 0) {
		st.AwaitingChoice = false
		q, e := s.quote(ctx, in.Account, in.Contact, "orçamento")
		if e != nil {
			return Reply{}, e
		}
		st.QuoteActive = q.Active
		if q.Handoff {
			st.Paused = true
			r.Handoff = true
		}
		st.Categories = q.Categories
		st.Plan = q.Plan
		r.Text = q.Text
		r.Messages = append(r.Messages, q.Messages...)
		r.GroupRequest = q.GroupRequest
		r.Categories = q.Categories
		if q.Plan != nil && len(q.Plan.Rooms) > 0 {
			st.QuotedAt = in.Now
			r.SelectionPrompt = roomCategorySelectionPrompt(1, len(q.Plan.Rooms), q.Plan.Rooms[0].Categories)
		} else if len(q.Categories) > 0 {
			st.QuotedAt = in.Now
			r.SelectionPrompt = categorySelectionPrompt(q.Categories)
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
		if q.Handoff {
			st.Paused = true
			r.Handoff = true
		}
		st.Categories = q.Categories
		st.Plan = q.Plan
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
			if q.Plan != nil && len(q.Plan.Rooms) > 0 {
				r.SelectionPrompt = roomCategorySelectionPrompt(1, len(q.Plan.Rooms), q.Plan.Rooms[0].Categories)
			} else {
				r.SelectionPrompt = categorySelectionPrompt(q.Categories)
			}
		}
	} else if st.Plan != nil && len(st.Plan.Rooms) > 0 {
		if in.Now.Sub(st.QuotedAt) >= 24*time.Hour {
			st.Categories, st.Plan, st.SelectedRooms = nil, nil, nil
			r.Text = "Este orçamento expirou. Envie orçamento para consultar valores atualizados."
		} else {
			roomIndex := len(st.SelectedRooms)
			if roomIndex >= len(st.Plan.Rooms) {
				st.Plan, st.SelectedRooms = nil, nil
				r.Text = menu
			} else {
				roomCategories := st.Plan.Rooms[roomIndex].Categories
				for i, c := range roomCategories {
					if text == strconv.Itoa(i+1) || text == strings.ToLower(c.Key) || text == strings.ToLower(c.Name) {
						st.SelectedRooms = append(st.SelectedRooms, c)
						break
					}
				}
				if len(st.SelectedRooms) == roomIndex {
					r.Text = roomCategorySelectionPrompt(roomIndex+1, len(st.Plan.Rooms), roomCategories)
				} else if len(st.SelectedRooms) < len(st.Plan.Rooms) {
					next := len(st.SelectedRooms)
					r.Text = "Categoria do quarto confirmada.\n\n" + roomCategorySelectionPrompt(next+1, len(st.Plan.Rooms), st.Plan.Rooms[next].Categories)
				} else {
					r.PreReservation = &PreReservation{CheckIn: st.Plan.CheckIn, CheckOut: st.Plan.CheckOut, Categories: append([]omnibees.Category(nil), st.SelectedRooms...), Rooms: append([]QuoteRoom(nil), st.Plan.Rooms...)}
					r.Text = "Tudo certo com as categorias. Agora vou criar sua pré-reserva e avisar nossa equipe para concluir o atendimento."
					st.Paused = true
					st.Plan, st.Categories, st.SelectedRooms = nil, nil, nil
				}
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
					r.Text = "Você escolheu *" + c.Name + "*, disponível para o período consultado. Vou encaminhar a conversa para um *atendente* concluir sua reserva."
					break
				}
			}
			if r.SelectedCategory == nil {
				r.Text = categorySelectionPrompt(st.Categories)
			}
		}
	} else {
		r.Text = menu
	}
	return finish()
}

func roomCategorySelectionPrompt(current, total int, categories []omnibees.Category) string {
	return fmt.Sprintf("*Quarto %d de %d*\n%s", current, total, categorySelectionPrompt(categories))
}

func categorySelectionPrompt(categories []omnibees.Category) string {
	var b strings.Builder
	b.WriteString("Escolha uma categoria pelo número:\n")
	for i, category := range categories {
		fmt.Fprintf(&b, "%d — %s\n", i+1, category.Name)
	}
	b.WriteString("\nOu envie *atendente* para falar com nossa equipe. Para encerrar, envie *sair*.")
	return b.String()
}
