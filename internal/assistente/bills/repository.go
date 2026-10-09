package bills

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Bill struct {
	ID          int64
	Description string
	AmountCents int64
	DueDate     string
	Status      string
	CreatedAt   string
	PaidAt      string
}
type Settings struct {
	Enabled    bool
	Target     string
	DaysBefore int
	Hour       int
	Minute     int
}
type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Add(ctx context.Context, description string, cents int64, due string) (Bill, error) {
	description = strings.TrimSpace(description)
	date, err := time.Parse("2006-01-02", due)
	if description == "" || len([]rune(description)) > 250 || cents <= 0 || cents > 999999999999 || err != nil || date.Format("2006-01-02") != due {
		return Bill{}, errors.New("informe descrição, valor positivo e vencimento válido")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := r.db.ExecContext(ctx, `INSERT INTO payable_bills(description,amount_cents,due_date,status,created_at) VALUES(?,?,?,'OPEN',?)`, description, cents, due, now)
	if err != nil {
		return Bill{}, err
	}
	id, err := result.LastInsertId()
	return Bill{ID: id, Description: description, AmountCents: cents, DueDate: due, Status: "OPEN", CreatedAt: now}, err
}
func (r *Repository) List(ctx context.Context) ([]Bill, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,description,amount_cents,due_date,status,created_at,paid_at FROM payable_bills ORDER BY CASE status WHEN 'OPEN' THEN 0 ELSE 1 END,due_date,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Bill{}
	for rows.Next() {
		var b Bill
		if err := rows.Scan(&b.ID, &b.Description, &b.AmountCents, &b.DueDate, &b.Status, &b.CreatedAt, &b.PaidAt); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}
func (r *Repository) SetStatus(ctx context.Context, id int64, status string) error {
	if status != "PAID" && status != "CANCELLED" {
		return errors.New("situação inválida")
	}
	paid := ""
	if status == "PAID" {
		paid = time.Now().UTC().Format(time.RFC3339)
	}
	res, err := r.db.ExecContext(ctx, `UPDATE payable_bills SET status=?,paid_at=? WHERE id=? AND status='OPEN'`, status, paid, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("boleto não encontrado ou já encerrado")
	}
	return nil
}
func (r *Repository) Settings(ctx context.Context) (Settings, error) {
	s := Settings{DaysBefore: 3, Hour: 9}
	var raw string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='bills:config'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal([]byte(raw), &s)
}
func (r *Repository) Configure(ctx context.Context, s Settings) error {
	s.Target = strings.TrimSpace(s.Target)
	if s.DaysBefore < 0 || s.DaysBefore > 90 || s.Hour < 0 || s.Hour > 23 || s.Minute < 0 || s.Minute > 59 || (s.Enabled && !regexp.MustCompile(`^[1-9][0-9]{9,14}$`).MatchString(s.Target)) {
		return errors.New("use telefone com DDI, antecedência de 0 a 90 dias e horário válido")
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES('bills:config',?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, string(raw), time.Now().UTC().Format(time.RFC3339))
	return err
}

// Remind sends at most one acknowledged notification per bill and local day.
// A lease prevents parallel dispatch; failures release it for retry.
func (r *Repository) Remind(ctx context.Context, now time.Time, zone *time.Location, send func(context.Context, string, string) error) error {
	cfg, err := r.Settings(ctx)
	if err != nil || !cfg.Enabled {
		return err
	}
	local := now.In(zone)
	if local.Hour()*60+local.Minute() < cfg.Hour*60+cfg.Minute {
		return nil
	}
	list, err := r.List(ctx)
	if err != nil {
		return err
	}
	day := local.Format("2006-01-02")
	today, err := time.Parse("2006-01-02", day)
	if err != nil {
		return err
	}
	for _, b := range list {
		if b.Status != "OPEN" {
			continue
		}
		due, err := time.Parse("2006-01-02", b.DueDate)
		if err != nil {
			return err
		}
		days := int(due.Sub(today).Hours() / 24)
		if days > cfg.DaysBefore {
			continue
		}
		lease := now.UTC().Unix()
		res, err := r.db.ExecContext(ctx, `INSERT INTO bill_reminders(bill_id,day,status,lease_until) SELECT ?,?,'SENDING',? WHERE EXISTS(SELECT 1 FROM payable_bills WHERE id=? AND status='OPEN') ON CONFLICT(bill_id,day) DO UPDATE SET status='SENDING',lease_until=excluded.lease_until WHERE bill_reminders.status!='SENT' AND bill_reminders.lease_until<?`, b.ID, day, lease+300, b.ID, lease)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		label := fmt.Sprintf("Vence em %d dia(s)", days)
		if days == 0 {
			label = "Vence hoje"
		}
		if days < 0 {
			label = fmt.Sprintf("Vencido há %d dia(s)", -days)
		}
		text := fmt.Sprintf("🔔 BOLETO A PAGAR #%d\n\n%s\nValor: R$ %s\nVencimento: %s\n%s\n\nMarque como pago no Tino ou envie boleto pago %d no chat administrativo para encerrar os lembretes.", b.ID, b.Description, Money(b.AmountCents), due.Format("02/01/2006"), label, b.ID)
		if err := send(ctx, cfg.Target, text); err != nil {
			_, _ = r.db.ExecContext(ctx, `UPDATE bill_reminders SET status='RETRY',lease_until=0 WHERE bill_id=? AND day=? AND status='SENDING'`, b.ID, day)
			return err
		}
		if _, err := r.db.ExecContext(ctx, `UPDATE bill_reminders SET status='SENT',lease_until=0 WHERE bill_id=? AND day=?`, b.ID, day); err != nil {
			return err
		}
	}
	return nil
}
