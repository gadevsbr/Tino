package bitz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ScraplingRunner drives the same UI for the credential test, probe and chatbot.
type ScraplingRunner struct{}

type ScraplingResult struct {
	OK        bool     `json:"ok"`
	Stage     string   `json:"stage"`
	Code      string   `json:"code"`
	Reference string   `json:"reference"`
	Rooms     []string `json:"rooms"`
}

func (r *ScraplingRunner) TestAccess(ctx context.Context, cfg Config, password string) error {
	_, err := r.run(ctx, "login", cfg, password, ReservationRequest{})
	return err
}

func (r *ScraplingRunner) Create(ctx context.Context, cfg Config, password string, req ReservationRequest) error {
	_, err := r.run(ctx, "create", cfg, password, req)
	return err
}

// Probe exercises every wizard step but never clicks the final save button.
func (r *ScraplingRunner) Probe(ctx context.Context, cfg Config, password string, req ReservationRequest) (ScraplingResult, error) {
	return r.run(ctx, "probe", cfg, password, req)
}

func (r *ScraplingRunner) run(parent context.Context, mode string, cfg Config, password string, req ReservationRequest) (ScraplingResult, error) {
	var result ScraplingResult
	if password == "" {
		return result, errors.New("senha Bitz vazia")
	}
	if mode != "login" && (len(req.Categories) < 1 || len(req.Categories) > 6) {
		return result, errors.New("a pré-reserva exige de 1 a 6 quartos")
	}
	browser, err := browserPath()
	if err != nil {
		return result, err
	}
	executable, args, err := scraplingCommand()
	if err != nil {
		return result, err
	}
	categories := make([]string, len(req.Categories))
	allowedRooms := make([][]int, len(req.Categories))
	for i, category := range req.Categories {
		categories[i] = physicalCategory(category)
		allowedRooms[i] = append([]int(nil), category.AllowedRooms...)
	}
	input, err := json.Marshal(map[string]any{
		"mode": mode, "url": cfg.BaseURL, "username": cfg.Username,
		"password": password, "cpf": cfg.BotCPF, "browser": browser,
		"checkin": req.CheckIn, "checkout": req.CheckOut, "categories": categories,
		"allowed_rooms": allowedRooms,
	})
	if err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(parent, 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	hideScraplingWindow(cmd)
	cmd.Stdin = bytes.NewReader(input)
	var output bytes.Buffer
	cmd.Stdout = &output
	// Never relay browser stderr: it can contain page values or credential URLs.
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return result, errors.New("Bitz: tempo limite; confira se a reserva foi criada antes de repetir")
	}
	if json.Unmarshal(output.Bytes(), &result) != nil {
		return result, errors.New("Bitz: runtime Scrapling não retornou um resultado válido")
	}
	if runErr != nil || !result.OK {
		return result, fmt.Errorf("Bitz/Scrapling: etapa %s (%s); confira o Bitz antes de repetir", result.Stage, result.Code)
	}
	expected := map[string]string{"login": "authenticated", "probe": "verified_without_save", "create": "saved"}[mode]
	if result.Code != expected || (mode == "create" && result.Reference == "") || (mode != "login" && len(result.Rooms) != len(categories)) {
		return result, errors.New("Bitz: confirmação incompleta; confira o Bitz antes de repetir")
	}
	return result, nil
}
