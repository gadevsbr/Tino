package controlpanel

import (
	"bytes"
	"context"
	"crypto/sha256"
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
	InstanceID   string    `json:"instanceId"`
	Version      string    `json:"version"`
	OnlineAt     string    `json:"onlineAt"`
	SnapshotHash string    `json:"snapshotHash"`
	Contacts     []Contact `json:"contacts"`
	Failures     []Failure `json:"failures"`
}

type Command struct {
	ID, Account, Contact, Action string
}

type Config struct {
	Enabled, PasswordSet, DashboardTokenSet      bool
	Endpoint, DashboardURL, InstanceID, Username string
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

func (c Client) Login(ctx context.Context, username, password string) (string, error) {
	if !strings.HasPrefix(c.Endpoint, "https://") && !strings.HasPrefix(c.Endpoint, "http://127.0.0.1") && !strings.HasPrefix(c.Endpoint, "http://localhost") {
		return "", errors.New("o painel exige HTTPS")
	}
	data, err := json.Marshal(map[string]string{"username": strings.TrimSpace(username), "password": password, "purpose": "bridge"})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.Endpoint, "/")+"/api/auth/login", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Token string `json:"token"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&result); err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK || result.Token == "" {
		if result.Error == "" {
			result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return "", errors.New(result.Error)
	}
	return result.Token, nil
}

func (c Client) Sync(ctx context.Context, snapshot Snapshot) ([]Command, error) {
	snapshot.InstanceID = c.InstanceID
	snapshot.OnlineAt = time.Now().UTC().Format(time.RFC3339)
	payload, err := json.Marshal(struct {
		Version  string
		Contacts []Contact
		Failures []Failure
	}{snapshot.Version, snapshot.Contacts, snapshot.Failures})
	if err != nil {
		return nil, err
	}
	snapshot.SnapshotHash = fmt.Sprintf("%x", sha256.Sum256(payload))
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
