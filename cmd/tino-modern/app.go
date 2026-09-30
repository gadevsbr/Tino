package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gadevsbr/tino/internal/audit"
	"github.com/gadevsbr/tino/internal/batch"
	"github.com/gadevsbr/tino/internal/capability"
	"github.com/gadevsbr/tino/internal/chat"
	"github.com/gadevsbr/tino/internal/config"
	"github.com/gadevsbr/tino/internal/flow"
	"github.com/gadevsbr/tino/internal/session"
	"github.com/skip2/go-qrcode"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type App struct {
	ctx          context.Context
	cfg          config.Config
	mgr          *session.Manager
	chats        *chat.Store
	capabilities *capability.Store
	flowDef      flow.Definition
	version      string
	flowStarted  bool
	mu           sync.Mutex
	hotelMu      sync.Mutex
	hotel        *hotelRuntime
	eventsReady  bool
}

type StatusDTO struct {
	Authenticated bool   `json:"authenticated"`
	SessionSaved  bool   `json:"sessionSaved"`
	Text          string `json:"text"`
	Profile       string `json:"profile"`
	Version       string `json:"version"`
}

type ConversationDTO struct {
	JID, Name, LastMessage, LastAt string
	Unread                         int
}

type MessageDTO struct {
	ID, Text, Timestamp string
	FromMe              bool
}

type CSVSummary struct {
	Path, FileName, PhoneField string
	Total, Consented           int
	HasMessage, HasConsent     bool
}

type BatchRequest struct {
	Path, Message  string
	ConfirmConsent bool
}

func NewApp(cfg config.Config, mgr *session.Manager, chats *chat.Store, capabilities *capability.Store, def flow.Definition, version string) *App {
	return &App{cfg: cfg, mgr: mgr, chats: chats, capabilities: capabilities, flowDef: def, version: version}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.eventsReady = true
	a.mgr.Client.AddEventHandler(a.handleEvent)
	if err := a.restartHotelRuntime(); err != nil {
		a.emitActivity("Operação hoteleira", err.Error(), "error")
	} else {
		a.emitActivity("Operação hoteleira", "Motor operacional carregado no WhatsApp do Tino", "success")
	}
	a.emitStatus()
}

func (a *App) shutdown(context.Context) {
	a.hotelMu.Lock()
	a.hotel.Close()
	a.hotel = nil
	a.hotelMu.Unlock()
	_ = a.chats.Close()
	_ = a.mgr.Close()
}

func (a *App) GetStatus() StatusDTO {
	logged := a.mgr.Client.IsLoggedIn()
	saved := a.mgr.Client.Store.ID != nil
	text := "Não autenticada"
	if saved {
		text = "Sessão salva, desconectada"
	}
	if logged {
		text = "Conectada e autenticada"
	}
	return StatusDTO{Authenticated: logged, SessionSaved: saved, Text: text, Profile: a.cfg.Profile, Version: a.version}
}

func (a *App) Connect() error {
	a.emitActivity("Conexão", "Iniciando conexão com o WhatsApp", "info")
	err := a.mgr.ConnectWithQR(a.ctx, true, func(content string) error {
		png, err := qrcode.Encode(content, qrcode.Medium, 420)
		if err != nil {
			return err
		}
		wailsRuntime.EventsEmit(a.ctx, "pairing:qr", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(png))
		return nil
	})
	if err != nil {
		a.emitActivity("Conexão", err.Error(), "error")
		a.emitStatus()
		return err
	}
	a.emitActivity("Conexão", "Conta autenticada com sucesso", "success")
	a.emitStatus()
	return nil
}

func (a *App) ResetSession() error {
	if err := a.mgr.ResetLocalSession(a.ctx); err != nil {
		return err
	}
	a.mgr.Client.AddEventHandler(a.handleEvent)
	if err := a.restartHotelRuntime(); err != nil {
		return fmt.Errorf("reiniciar motor operacional: %w", err)
	}
	a.mu.Lock()
	a.flowStarted = false
	a.mu.Unlock()
	a.emitActivity("Sessão", "Credenciais locais removidas; histórico preservado", "info")
	a.emitStatus()
	return nil
}

func (a *App) ListChats(search string) ([]ConversationDTO, error) {
	items, err := a.chats.Conversations(a.ctx, search)
	if err != nil {
		return nil, err
	}
	out := make([]ConversationDTO, 0, len(items))
	for _, c := range items {
		name := c.Name
		if jid, parseErr := types.ParseJID(c.JID); parseErr == nil && jid.Server != types.GroupServer {
			if contact, contactErr := a.mgr.Client.Store.Contacts.GetContact(a.ctx, jid); contactErr == nil && contact.Found {
				for _, candidate := range []string{contact.BusinessName, contact.FullName, contact.PushName, contact.FirstName} {
					if strings.TrimSpace(candidate) != "" {
						name = strings.TrimSpace(candidate)
						break
					}
				}
			}
		}
		if name == "" {
			name = c.JID
		}
		out = append(out, ConversationDTO{JID: c.JID, Name: name, LastMessage: c.LastMessage, LastAt: c.LastAt.Format(time.RFC3339), Unread: c.Unread})
	}
	return out, nil
}

func (a *App) GetMessages(jid string) ([]MessageDTO, error) {
	if err := a.chats.MarkRead(a.ctx, jid); err != nil {
		return nil, err
	}
	items, err := a.chats.Messages(a.ctx, jid, 300)
	if err != nil {
		return nil, err
	}
	out := make([]MessageDTO, 0, len(items))
	for _, m := range items {
		out = append(out, MessageDTO{ID: m.ID, Text: m.Text, Timestamp: m.Timestamp.Format(time.RFC3339), FromMe: m.FromMe})
	}
	return out, nil
}

func (a *App) SendMessage(jidRaw, text string) error {
	if err := a.ensureConnected(); err != nil {
		return err
	}
	jid, err := types.ParseJID(jidRaw)
	if err != nil {
		return err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("a mensagem está vazia")
	}
	resp, err := a.mgr.Client.SendMessage(a.ctx, jid, &waE2E.Message{Conversation: proto.String(text)})
	if err != nil {
		return err
	}
	evt := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: jid, IsFromMe: true, IsGroup: jid.Server == types.GroupServer}, ID: resp.ID, Timestamp: time.Now()}, Message: &waE2E.Message{Conversation: proto.String(text)}}
	if err := a.chats.SaveEvent(a.ctx, evt, false); err != nil {
		return err
	}
	wailsRuntime.EventsEmit(a.ctx, "chats:changed", map[string]string{"jid": jid.String()})
	return nil
}

func (a *App) ChooseCSV() (CSVSummary, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{Title: "Selecionar lista CSV", Filters: []wailsRuntime.FileFilter{{DisplayName: "Arquivos CSV", Pattern: "*.csv"}}})
	if err != nil || path == "" {
		return CSVSummary{}, err
	}
	parsed, err := batch.LoadCSVFlexible(path)
	if err != nil {
		return CSVSummary{}, err
	}
	approved := 0
	for _, item := range parsed.Items {
		if item.Consented {
			approved++
		}
	}
	return CSVSummary{Path: path, FileName: filepath.Base(path), PhoneField: parsed.PhoneField, Total: len(parsed.Items), Consented: approved, HasMessage: parsed.HasMessage, HasConsent: parsed.HasConsent}, nil
}

func (a *App) RunBatch(req BatchRequest) error {
	if err := a.requireCapability("notifications"); err != nil {
		return err
	}
	if err := a.ensureConnected(); err != nil {
		return err
	}
	parsed, err := batch.LoadCSVFlexible(req.Path)
	if err != nil {
		return err
	}
	for i := range parsed.Items {
		if strings.TrimSpace(parsed.Items[i].Message) == "" {
			parsed.Items[i].Message = strings.TrimSpace(req.Message)
		}
		if !parsed.HasConsent && req.ConfirmConsent {
			parsed.Items[i].Consented = true
		}
	}
	if !parsed.HasConsent && !req.ConfirmConsent {
		return errors.New("confirme o consentimento antes do envio")
	}
	p := batch.Processor{Client: a.mgr.Client, MinInterval: a.cfg.Batch.MinInterval, MaxInterval: a.cfg.Batch.MaxInterval, MaxPerRun: a.cfg.Batch.MaxPerRun}
	return p.Run(a.ctx, parsed.Items, func(result batch.Result) { wailsRuntime.EventsEmit(a.ctx, "batch:result", result) })
}

func (a *App) ExportAudit() (string, error) {
	if err := a.requireCapability("audit"); err != nil {
		return "", err
	}
	if err := a.ensureConnected(); err != nil {
		return "", err
	}
	snapshot, err := audit.Collect(a.ctx, a.mgr.Client)
	if err != nil {
		return "", err
	}
	dir := filepath.Join("exports", a.cfg.Profile)
	if err := audit.Write(snapshot, dir); err != nil {
		return "", err
	}
	a.emitActivity("Auditoria", fmt.Sprintf("%d contatos e %d grupos exportados", len(snapshot.Contacts), len(snapshot.Groups)), "success")
	return dir, nil
}

func (a *App) GetFlow() flow.Definition                         { return a.flowDef }
func (a *App) TestFlow(text string, def flow.Definition) string { return flow.Match(def, text) }
func (a *App) SaveFlow(def flow.Definition) error {
	if err := flow.SaveDefinition(a.cfg.Flow.RulesFile, def); err != nil {
		return err
	}
	a.flowDef = def
	return nil
}
func (a *App) StartFlow(def flow.Definition) error {
	if err := a.requireCapability("flow"); err != nil {
		return err
	}
	if err := a.ensureConnected(); err != nil {
		return err
	}
	if err := a.SaveFlow(def); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.flowStarted {
		return nil
	}
	engine, err := flow.New(a.mgr.Client, def)
	if err != nil {
		return err
	}
	a.mgr.Client.AddEventHandler(engine.Handle)
	a.flowStarted = true
	a.emitActivity("Flow Builder", "Atendimento automático ativado", "success")
	return nil
}

func (a *App) GetCapabilities() (capability.Settings, error) { return a.capabilities.Load() }
func (a *App) SaveCapabilities(settings capability.Settings) error {
	seenOperators := map[string]bool{}
	for i, raw := range settings.Operators {
		number := digitsOnly(raw)
		if len(number) < 10 || len(number) > 15 {
			return fmt.Errorf("operador %q precisa ter DDI e entre 10 e 15 dígitos", raw)
		}
		if seenOperators[number] {
			return fmt.Errorf("operador duplicado: %s", number)
		}
		seenOperators[number] = true
		settings.Operators[i] = number
	}
	known := map[string]bool{}
	for _, module := range settings.Modules {
		if !module.Available && module.Enabled {
			return fmt.Errorf("o recurso %q ainda não está disponível para ativação", module.Name)
		}
		known[module.ID] = module.Available
	}
	for _, role := range settings.Roles {
		for _, moduleID := range role.Modules {
			if !known[moduleID] {
				return fmt.Errorf("o perfil %q referencia um recurso indisponível", role.Name)
			}
		}
	}
	if err := a.capabilities.Save(settings); err != nil {
		return err
	}
	if a.ctx != nil {
		if err := a.restartHotelRuntime(); err != nil {
			return fmt.Errorf("configuração salva, mas o motor operacional não reiniciou: %w", err)
		}
	}
	return nil
}

func (a *App) restartHotelRuntime() error {
	a.hotelMu.Lock()
	defer a.hotelMu.Unlock()
	if a.hotel != nil {
		a.hotel.Close()
		a.hotel = nil
	}
	settings, err := a.capabilities.Load()
	if err != nil {
		return err
	}
	runtime, err := openHotelRuntime(a.ctx, a.cfg.DataDir, settings, a.mgr.Client, func(ctx context.Context, jid string) error {
		if err := a.chats.MarkUnread(ctx, jid); err != nil {
			return err
		}
		wailsRuntime.EventsEmit(a.ctx, "chats:changed", map[string]string{"jid": jid})
		return nil
	})
	if err != nil {
		return err
	}
	a.hotel = runtime
	return nil
}

func digitsOnly(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func (a *App) ChooseWorkspace() (string, error) {
	path, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{Title: "Escolha a pasta autorizada"})
	if err != nil || path == "" {
		return path, err
	}
	settings, err := a.capabilities.Load()
	if err != nil {
		return "", err
	}
	settings.WorkspaceRoot = filepath.Clean(path)
	if err := a.capabilities.Save(settings); err != nil {
		return "", err
	}
	return settings.WorkspaceRoot, nil
}

func (a *App) ensureConnected() error {
	if a.mgr.Client.IsLoggedIn() {
		return nil
	}
	return a.mgr.ConnectWithQR(a.ctx, false, nil)
}

func (a *App) requireCapability(id string) error {
	settings, err := a.capabilities.Load()
	if err != nil {
		return fmt.Errorf("carregar configuração de recursos: %w", err)
	}
	for _, module := range settings.Modules {
		if module.ID == id {
			if module.Available && module.Enabled {
				return nil
			}
			return fmt.Errorf("o recurso %q está desativado na Central de recursos", module.Name)
		}
	}
	return fmt.Errorf("recurso desconhecido: %s", id)
}

func (a *App) emitStatus() {
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "session:status", a.GetStatus())
	}
}
func (a *App) emitActivity(title, message, level string) {
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "activity", map[string]string{"title": title, "message": message, "level": level, "at": time.Now().Format(time.RFC3339)})
	}
}

func (a *App) handleEvent(raw any) {
	switch evt := raw.(type) {
	case *events.Message:
		if err := a.chats.SaveEvent(context.Background(), evt, true); err == nil {
			wailsRuntime.EventsEmit(a.ctx, "chats:changed", map[string]string{"jid": evt.Info.Chat.String()})
		}
	case *events.ChatPresence:
		wailsRuntime.EventsEmit(a.ctx, "chats:presence", map[string]string{"jid": evt.Chat.String(), "state": string(evt.State), "media": string(evt.Media)})
	case *events.HistorySync:
		go a.importHistory(evt)
	case *events.Connected:
		a.emitStatus()
	case *events.Disconnected:
		a.emitStatus()
	case *events.LoggedOut:
		a.emitActivity("Sessão", "O WhatsApp desconectou este dispositivo", "error")
		a.emitStatus()
	case *events.ConnectFailure:
		a.emitActivity("Sessão", "Falha ao autenticar a sessão", "error")
		a.emitStatus()
	}
}

func (a *App) importHistory(evt *events.HistorySync) {
	if evt == nil || evt.Data == nil {
		return
	}
	count := 0
	for _, conv := range evt.Data.GetConversations() {
		jid, err := types.ParseJID(conv.GetID())
		if err != nil {
			continue
		}
		name := strings.TrimSpace(conv.GetDisplayName())
		if name == "" {
			name = strings.TrimSpace(conv.GetName())
		}
		for _, item := range conv.GetMessages() {
			parsed, err := a.mgr.Client.ParseWebMessage(jid, item.GetMessage())
			if err != nil {
				continue
			}
			if a.chats.SaveEvent(context.Background(), parsed, false) == nil {
				count++
			}
		}
		_ = a.chats.UpdateName(context.Background(), jid.String(), name)
	}
	a.emitActivity("Sincronização", fmt.Sprintf("%d mensagens processadas", count), "success")
	wailsRuntime.EventsEmit(a.ctx, "chats:changed")
}
