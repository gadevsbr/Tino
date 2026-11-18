package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/gadevsbr/tino/internal/controlpanel"
)

func (r *hotelRuntime) panelHealth(ctx context.Context, contacts []controlpanel.Contact) controlpanel.Health {
	now := time.Now().UTC()
	health := controlpanel.Health{}
	switch {
	case r.client != nil && r.client.IsLoggedIn():
		health.WhatsApp = controlpanel.Signal{State: "ok", Label: "Conectado", Detail: "Sessão autenticada e pronta para operar"}
	case r.client != nil && r.client.IsConnected():
		health.WhatsApp = controlpanel.Signal{State: "warning", Label: "Sem autenticação", Detail: "Conexão aberta, mas a sessão não está autenticada"}
	default:
		health.WhatsApp = controlpanel.Signal{State: "error", Label: "Desconectado", Detail: "O Tino não consegue enviar ou receber mensagens"}
	}

	quoteAt := r.latestQuoteAt(ctx)
	omniFailureAt := r.latestFailureAt(ctx, "omnibees")
	health.OmniBees = activitySignal("OmniBees", quoteAt, omniFailureAt)
	health.Bitz = r.bitzSignal(ctx, now)

	for _, item := range contacts {
		lastSeen, err := time.Parse(time.RFC3339, item.LastSeen)
		if err == nil && now.Sub(lastSeen) > time.Hour && item.Stage != "new" && item.Stage != "approved" && item.Stage != "human" {
			health.StalledContacts++
		}
	}
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bitz_prereservations WHERE status IN ('awaiting_approval','manual_review')`).Scan(&health.AwaitingApproval)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM commercial_payment_proofs`).Scan(&health.PendingProofs)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM backups`).Scan(&health.BackupCount)
	if latest, err := r.backup.Latest(ctx); err == nil {
		health.LastBackup = latest.CreatedAt
	}
	cutoff := now.Add(-24 * time.Hour).Format(time.RFC3339Nano)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM integration_failures WHERE created_at>=?`, cutoff).Scan(&health.ActionErrors)
	var failedJobs int
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bitz_prereservations WHERE status IN ('failed','manual_review')`).Scan(&failedJobs)
	health.ActionErrors += failedJobs
	return health
}

func activitySignal(name string, successAt, failureAt time.Time) controlpanel.Signal {
	if successAt.IsZero() && failureAt.IsZero() {
		return controlpanel.Signal{State: "unknown", Label: "Sem atividade recente", Detail: name + " ainda não foi exercitado nesta instalação"}
	}
	if failureAt.After(successAt) {
		return controlpanel.Signal{State: "error", Label: "Falha recente", Detail: "A última operação terminou com erro", CheckedAt: failureAt.Format(time.RFC3339)}
	}
	return controlpanel.Signal{State: "ok", Label: "Respondendo", Detail: "A última operação foi concluída", CheckedAt: successAt.Format(time.RFC3339)}
}

func (r *hotelRuntime) latestQuoteAt(ctx context.Context) time.Time {
	rows, err := r.db.QueryContext(ctx, `SELECT state FROM commercial_contacts`)
	if err != nil {
		return time.Time{}
	}
	defer rows.Close()
	var latest time.Time
	for rows.Next() {
		var raw string
		var state struct {
			QuotedAt time.Time `json:"quoted_at"`
		}
		if rows.Scan(&raw) == nil && json.Unmarshal([]byte(raw), &state) == nil && state.QuotedAt.After(latest) {
			latest = state.QuotedAt
		}
	}
	return latest
}

func (r *hotelRuntime) latestFailureAt(ctx context.Context, source string) time.Time {
	var raw string
	if err := r.db.QueryRowContext(ctx, `SELECT created_at FROM integration_failures WHERE source=? ORDER BY id DESC LIMIT 1`, source).Scan(&raw); err != nil {
		return time.Time{}
	}
	value, _ := time.Parse(time.RFC3339Nano, raw)
	return value
}

func (r *hotelRuntime) bitzSignal(ctx context.Context, now time.Time) controlpanel.Signal {
	cfg, err := r.bitz.Load(ctx)
	if err != nil {
		return controlpanel.Signal{State: "error", Label: "Configuração inválida", Detail: err.Error()}
	}
	if !cfg.Enabled {
		return controlpanel.Signal{State: "warning", Label: "Desativado", Detail: "A integração Bitz está desligada no Tino"}
	}
	var status, updated string
	err = r.db.QueryRowContext(ctx, `SELECT status,updated_at FROM bitz_prereservations ORDER BY updated_at DESC LIMIT 1`).Scan(&status, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return controlpanel.Signal{State: "unknown", Label: "Sem atividade recente", Detail: "Configurado, mas ainda sem pré-reserva registrada"}
	}
	if err != nil {
		return controlpanel.Signal{State: "error", Label: "Consulta indisponível", Detail: err.Error()}
	}
	checked, _ := time.Parse(time.RFC3339Nano, updated)
	signal := controlpanel.Signal{CheckedAt: checked.Format(time.RFC3339)}
	switch status {
	case "approved", "awaiting_approval":
		signal.State, signal.Label, signal.Detail = "ok", "Funcionando", "A última pré-reserva avançou normalmente"
	case "running":
		if !checked.IsZero() && now.Sub(checked) > 10*time.Minute {
			signal.State, signal.Label, signal.Detail = "error", "Processamento travado", "Pré-reserva em processamento há mais de 10 minutos"
		} else {
			signal.State, signal.Label, signal.Detail = "warning", "Processando", "Existe uma pré-reserva em andamento"
		}
	case "manual_review":
		signal.State, signal.Label, signal.Detail = "error", "Revisão manual", "A última pré-reserva exige conclusão por um operador"
	default:
		signal.State, signal.Label, signal.Detail = "error", "Falha recente", "A última pré-reserva não foi concluída"
	}
	return signal
}
