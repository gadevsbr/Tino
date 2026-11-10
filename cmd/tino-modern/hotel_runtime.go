package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/advances"
	assistapp "github.com/gadevsbr/tino/internal/assistente/app"
	"github.com/gadevsbr/tino/internal/assistente/backup"
	"github.com/gadevsbr/tino/internal/assistente/bitz"
	"github.com/gadevsbr/tino/internal/assistente/cash"
	"github.com/gadevsbr/tino/internal/assistente/catalog"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	assistconfig "github.com/gadevsbr/tino/internal/assistente/config"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	assistdb "github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/extratos"
	"github.com/gadevsbr/tino/internal/assistente/reports"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	assistwa "github.com/gadevsbr/tino/internal/assistente/whatsapp"
	"github.com/gadevsbr/tino/internal/capability"
	"github.com/gadevsbr/tino/internal/controlpanel"
	"github.com/gadevsbr/tino/internal/flow"
	"go.mau.fi/whatsmeow"
)

type hotelRuntime struct {
	db         *sql.DB
	service    *assistwa.Service
	cancel     context.CancelFunc
	dataDir    string
	zone       *time.Location
	rooms      *rooms.Repository
	cash       *cash.Repository
	advances   *advances.Repository
	extratos   *extratos.Repository
	reports    *reports.Generator
	commercial *commercial.Service
	catalog    *catalog.Repository
	backup     *backup.Manager
	bitz       *bitz.Store
	panel      *controlpanel.Store
	panelMu    sync.RWMutex
	panelState string
	version    string
}

func openHotelRuntime(parent context.Context, dataDir, version string, settings capability.Settings, client *whatsmeow.Client, markUnread func(context.Context, string) error) (*hotelRuntime, error) {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		location = time.Local
	}
	operationsDir := filepath.Join(dataDir, "operations")
	db, err := assistdb.Open(parent, filepath.Join(operationsDir, "hotel.db"))
	if err != nil {
		return nil, fmt.Errorf("abrir banco operacional: %w", err)
	}
	authorized := make(map[string]struct{}, len(settings.Operators))
	for _, number := range settings.Operators {
		authorized[number] = struct{}{}
	}
	cfg := assistconfig.Config{Authorized: authorized, Timezone: location, DataDir: operationsDir, ReportRetentionDays: 7, BackupRetentionDays: 30, LogRetentionDays: 14, DailySummaryHour: 17, BackupHour: 16}
	roomRepo := rooms.NewRepository(db, location)
	cashRepo := cash.NewRepository(db, location)
	advanceRepo := advances.NewRepository(db, location)
	application := assistapp.New(roomRepo, conversation.NewRepository(db)).EnableCash(cashRepo).EnableAdvances(advanceRepo)
	capStore := capability.NewStore(filepath.Join(dataDir, "capabilities.json"))
	assist := func(ctx context.Context, req commercial.DialogueRequest) (commercial.DialogueAnswer, error) {
		ai, err := flow.ConfiguredAI(capStore)
		if err != nil {
			return commercial.DialogueAnswer{}, err
		}
		if ai == nil {
			return commercial.DialogueAnswer{}, fmt.Errorf("IA desativada")
		}
		return ai.Converse(ctx, req)
	}
	application.EnableGuestAssistant(assist)
	service, err := assistwa.NewWithClient(parent, cfg, db, application, client)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	service.SetMarkUnread(markUnread)
	service.SetGuestAssistant(assist)
	ctx, cancel := context.WithCancel(parent)
	service.StartAttached(ctx)
	commercialService, err := commercial.New(parent, db, func(context.Context, string, string, string) (commercial.QuoteReply, error) {
		return commercial.QuoteReply{}, nil
	})
	if err != nil {
		cancel()
		service.Close()
		_ = db.Close()
		return nil, err
	}
	catalogRepo := catalog.NewRepository(db)
	if err := catalogRepo.EnsureSchema(parent); err != nil {
		cancel()
		service.Close()
		_ = db.Close()
		return nil, err
	}
	runtime := &hotelRuntime{db: db, service: service, cancel: cancel, dataDir: operationsDir, zone: location, rooms: roomRepo, cash: cashRepo, advances: advanceRepo, extratos: extratos.NewRepository(db), reports: reports.New(location), commercial: commercialService, catalog: catalogRepo, backup: backup.New(db, operationsDir, cfg.BackupRetentionDays, location), bitz: bitz.NewStore(filepath.Join(operationsDir, "bitz-config.json")), panel: controlpanel.NewStore(filepath.Join(dataDir, "control-panel.json")), version: version}
	go runtime.runControlPanel(ctx)
	return runtime, nil
}

func (r *hotelRuntime) setPanelState(value string) {
	r.panelMu.Lock()
	r.panelState = value
	r.panelMu.Unlock()
}

func (r *hotelRuntime) controlPanelState() string {
	r.panelMu.RLock()
	defer r.panelMu.RUnlock()
	return r.panelState
}

func (r *hotelRuntime) runControlPanel(ctx context.Context) {
	for {
		cfg, token, err := r.panel.Load()
		if err != nil {
			r.setPanelState("Erro de configuração: " + err.Error())
		} else if !cfg.Enabled {
			r.setPanelState("Desativado")
		} else {
			contacts, loadErr := r.commercial.Pipeline(ctx)
			if loadErr != nil {
				r.setPanelState("Falha ao ler o funil: " + loadErr.Error())
			} else {
				snapshot := controlpanel.Snapshot{Version: r.version, Contacts: make([]controlpanel.Contact, 0, len(contacts)), Failures: r.panelFailures(ctx)}
				for _, item := range contacts {
					snapshot.Contacts = append(snapshot.Contacts, controlpanel.Contact{Account: item.Account, Contact: item.Contact, Stage: item.Stage, LastSeen: item.LastSeen, CheckIn: item.CheckIn, CheckOut: item.CheckOut, Paused: item.Paused, QuoteActive: item.QuoteActive, AwaitingChoice: item.AwaitingChoice, Adults: item.Adults, Children: item.Children, Rooms: item.Rooms, Categories: item.Categories, SelectedCategories: item.SelectedCategories, PreReservationCode: item.PreReservationCode, PreReservationStatus: item.PreReservationStatus})
				}
				client := controlpanel.Client{Endpoint: cfg.Endpoint, Token: token, InstanceID: cfg.InstanceID}
				commands, syncErr := client.Sync(ctx, snapshot)
				if syncErr != nil {
					r.setPanelState("Sem conexão: " + syncErr.Error())
				} else {
					for _, command := range commands {
						runErr := r.executePanelCommand(ctx, command)
						if ackErr := client.Ack(ctx, command.ID, runErr); ackErr != nil {
							r.setPanelState("Falha ao confirmar comando: " + ackErr.Error())
							break
						}
					}
					if len(commands) == 0 || !strings.HasPrefix(r.controlPanelState(), "Falha") {
						r.setPanelState("Sincronizado")
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (r *hotelRuntime) executePanelCommand(ctx context.Context, command controlpanel.Command) error {
	switch command.Action {
	case "pause", "takeover":
		return r.commercial.Pause(ctx, command.Account, command.Contact)
	case "resume":
		return r.commercial.Resume(ctx, command.Account, command.Contact)
	case "restart_quote":
		return r.commercial.Reset(ctx, command.Account, command.Contact)
	default:
		return fmt.Errorf("comando do painel não permitido: %s", command.Action)
	}
}

func (r *hotelRuntime) panelFailures(ctx context.Context) []controlpanel.Failure {
	failures, err := r.commercial.Failures(ctx, 100)
	if err != nil {
		return nil
	}
	items := make([]controlpanel.Failure, 0, len(failures))
	for _, failure := range failures {
		items = append(items, controlpanel.Failure{ID: failure.ID, Source: failure.Source, Contact: failure.Contact, Message: failure.Message, CreatedAt: failure.At})
	}
	return items
}

func (r *hotelRuntime) Close() {
	if r == nil {
		return
	}
	if r.cancel != nil {
		r.cancel()
	}
	if r.service != nil {
		r.service.Close()
	}
	if r.db != nil {
		_ = r.db.Close()
	}
}
