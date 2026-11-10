package main

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gadevsbr/tino/internal/controlpanel"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ControlPanelDTO struct {
	Enabled, TokenSet                          bool
	Endpoint, DashboardURL, InstanceID, Status string
}

type ControlPanelInput struct {
	Enabled                                   bool
	Endpoint, DashboardURL, InstanceID, Token string
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
	return ControlPanelDTO{Enabled: cfg.Enabled, TokenSet: cfg.PasswordSet, Endpoint: cfg.Endpoint, DashboardURL: cfg.DashboardURL, InstanceID: cfg.InstanceID, Status: status}, nil
}

func (a *App) ConfigureControlPanel(input ControlPanelInput) error {
	input.Endpoint, input.InstanceID = strings.TrimSpace(input.Endpoint), strings.TrimSpace(input.InstanceID)
	if input.Enabled && (!strings.HasPrefix(input.Endpoint, "https://") || !strings.HasPrefix(input.DashboardURL, "https://") || input.InstanceID == "") {
		return errors.New("informe um endereço HTTPS e um identificador para ativar o painel")
	}
	store := controlpanel.NewStore(filepath.Join(a.cfg.DataDir, "control-panel.json"))
	if err := store.Save(controlpanel.Config{Enabled: input.Enabled, Endpoint: input.Endpoint, DashboardURL: input.DashboardURL, InstanceID: input.InstanceID}, input.Token); err != nil {
		return err
	}
	return a.restartHotelRuntime()
}

func (a *App) OpenControlPanel() error {
	store := controlpanel.NewStore(filepath.Join(a.cfg.DataDir, "control-panel.json"))
	cfg, token, err := store.Load()
	if err != nil {
		return err
	}
	if !cfg.Enabled || cfg.DashboardURL == "" || token == "" {
		return errors.New("configure e ative o painel web primeiro")
	}
	wailsRuntime.BrowserOpenURL(a.ctx, strings.TrimRight(cfg.DashboardURL, "/")+"/#token="+url.QueryEscape(token))
	return nil
}
