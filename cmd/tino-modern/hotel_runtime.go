package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/advances"
	assistapp "github.com/gadevsbr/tino/internal/assistente/app"
	"github.com/gadevsbr/tino/internal/assistente/backup"
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
}

func openHotelRuntime(parent context.Context, dataDir string, settings capability.Settings, client *whatsmeow.Client) (*hotelRuntime, error) {
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
	service, err := assistwa.NewWithClient(parent, cfg, db, application, client)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
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
	return &hotelRuntime{db: db, service: service, cancel: cancel, dataDir: operationsDir, zone: location, rooms: roomRepo, cash: cashRepo, advances: advanceRepo, extratos: extratos.NewRepository(db), reports: reports.New(location), commercial: commercialService, catalog: catalogRepo, backup: backup.New(db, operationsDir, cfg.BackupRetentionDays, location)}, nil
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
