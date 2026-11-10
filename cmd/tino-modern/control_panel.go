package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/gadevsbr/tino/internal/controlpanel"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	controlPanelEndpoint  = "https://tino-control-api.gadevs2020.workers.dev"
	controlPanelDashboard = "https://tino-control.pages.dev"
	controlPanelInstance  = "hotel-paraiso-recepcao"
)

type ControlPanelDTO struct {
	Enabled          bool
	Username, Status string
}

func (a *App) GetControlPanel() (ControlPanelDTO, error) {
	store := controlpanel.NewStore(filepath.Join(a.cfg.DataDir, "control-panel.json"))
	cfg, _, err := store.Load()
	if err != nil {
		return ControlPanelDTO{}, err
	}
	status := "Desativado"
	a.hotelMu.Lock()
	if a.hotel != nil {
		status = a.hotel.controlPanelState()
	}
	a.hotelMu.Unlock()
	return ControlPanelDTO{Enabled: cfg.Enabled, Username: cfg.Username, Status: status}, nil
}

func (a *App) LoginControlPanel(username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return errors.New("informe usuário e senha")
	}
	token, err := (controlpanel.Client{Endpoint: controlPanelEndpoint, InstanceID: controlPanelInstance}).Login(context.Background(), username, password)
	if err != nil {
		return err
	}
	store := controlpanel.NewStore(filepath.Join(a.cfg.DataDir, "control-panel.json"))
	if err := store.Save(controlpanel.Config{Enabled: true, Endpoint: controlPanelEndpoint, DashboardURL: controlPanelDashboard, InstanceID: controlPanelInstance, Username: username}, token, ""); err != nil {
		return err
	}
	return a.restartHotelRuntime()
}

func (a *App) DisconnectControlPanel() error {
	store := controlpanel.NewStore(filepath.Join(a.cfg.DataDir, "control-panel.json"))
	cfg, _, err := store.Load()
	if err != nil {
		return err
	}
	cfg.Enabled = false
	if err := store.Save(cfg, "", ""); err != nil {
		return err
	}
	return a.restartHotelRuntime()
}

func (a *App) OpenControlPanel() error {
	wailsRuntime.BrowserOpenURL(a.ctx, controlPanelDashboard)
	return nil
}
