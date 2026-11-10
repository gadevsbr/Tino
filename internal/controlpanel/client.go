package controlpanel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Contact struct {
	Account, Contact, Stage, LastSeen, CheckIn, CheckOut string
	Paused, QuoteActive, AwaitingChoice                  bool
	Adults, Children, Rooms                              int
	Categories, SelectedCategories                       []string
	PreReservationCode, PreReservationStatus             string
}

type Failure struct {
	ID                                  int64 `json:"id"`
	Source, Contact, Message, CreatedAt string
}

type Snapshot struct {
	InstanceID string    `json:"instanceId"`
	Version    string    `json:"version"`
	OnlineAt   string    `json:"onlineAt"`
	Contacts   []Contact `json:"contacts"`
	Failures   []Failure `json:"failures"`
}

type Command struct {
	ID, Account, Contact, Action string
}

type Config struct {
	Enabled, PasswordSet               bool
	Endpoint, DashboardURL, InstanceID string
}

type Client struct {
	Endpoint, Token, InstanceID string
	HTTP                        *http.Client
}

func (c Client) validate() error {
	if !strings.HasPrefix(c.Endpoint, "https://") && !strings.HasPrefix(c.Endpoint, "http://127.0.0.1") && !strings.HasPrefix(c.Endpoint, "http://localhost") {
		return errors.New("o painel exige HTTPS")
	}
	if strings.TrimSpace(c.Token) == "" || strings.TrimSpace(c.InstanceID) == "" {
		return errors.New("token e identificador da instalação são obrigatórios")
	}
	return nil
}

func (c Client) request(ctx context.Context, method, path string, body any, out any) error {
	if err := c.validate(); err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.Endpoint, "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("painel retornou HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(out)
	}
	return nil
}

func (c Client) Sync(ctx context.Context, snapshot Snapshot) ([]Command, error) {
	snapshot.InstanceID = c.InstanceID
	snapshot.OnlineAt = time.Now().UTC().Format(time.RFC3339)
	var result struct {
		Commands []Command `json:"commands"`
	}
	if err := c.request(ctx, http.MethodPost, "/api/bridge/sync", snapshot, &result); err != nil {
		return nil, err
	}
	return result.Commands, nil
}

func (c Client) Ack(ctx context.Context, id string, runErr error) error {
	status, message := "executed", ""
	if runErr != nil {
		status, message = "failed", runErr.Error()
		if len(message) > 500 {
			message = message[:500]
		}
	}
	return c.request(ctx, http.MethodPost, "/api/bridge/commands/"+id+"/ack", map[string]string{"instanceId": c.InstanceID, "status": status, "message": message}, nil)
}
