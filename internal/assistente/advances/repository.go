package advances

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/cash"
)

type Advance struct {
	ID             int64
	Employee       string
	Cents          int64
	Note           string
	DeductCash     bool
	CashMovementID sql.NullInt64
	Date           string
	Actor          string
	CreatedAt      time.Time
}

type Repository struct {
	db       *sql.DB
	location *time.Location
}

func NewRepository(db *sql.DB, location *time.Location) *Repository {
	return &Repository{db: db, location: location}
}

func (r *Repository) Create(ctx context.Context, employee string, cents int64, note string, deductCash bool, actor, messageID string, now time.Time) (Advance, error) {
	employee = strings.TrimSpace(employee)
	note = strings.TrimSpace(note)
	if employee == "" || cents <= 0 {
		return Advance{}, errors.New("employee and positive amount are required")
	}
	local := now.In(r.location)
	date := local.Format("2006-01-02")
	month := local.Format("2006-01")
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Advance{}, err
	}
	defer tx.Rollback()

	var movementID sql.NullInt64
	if deductCash {
		if _, err = tx.ExecContext(ctx, `INSERT INTO cash_days(business_date,opening_cents,opened_by,created_at) VALUES (?,0,?,strftime('%Y-%m-%dT%H:%M:%fZ','now')) ON CONFLICT(business_date) DO NOTHING`, date, actor); err != nil {
			return Advance{}, err
		}
		var closed string
		if err = tx.QueryRowContext(ctx, `SELECT closed_at FROM cash_days WHERE business_date=?`, date).Scan(&closed); err != nil {
			return Advance{}, err
		}
		if closed != "" {
			return Advance{}, cash.ErrCashClosed
		}
		description := "VALE - " + employee
		if note != "" {
			description += " - " + note
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO cash_movements(business_date,kind,method,amount_cents,description,actor,message_id,created_at) VALUES (?,'EXIT','',?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, date, cents, description, actor, "vale:"+messageID)
		if err != nil {
			return Advance{}, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return Advance{}, err
		}
		movementID = sql.NullInt64{Int64: id, Valid: true}
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO employee_advances(employee_name,amount_cents,note,deduct_cash,cash_movement_id,business_date,month_key,actor,message_id,created_at) VALUES (?,?,?,?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, employee, cents, note, deductCash, movementID, date, month, actor, messageID)
	if err != nil {
		return Advance{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Advance{}, err
	}
	if err = tx.Commit(); err != nil {
		return Advance{}, err
	}
	return Advance{ID: id, Employee: employee, Cents: cents, Note: note, DeductCash: deductCash, CashMovementID: movementID, Date: date, Actor: actor}, nil
}

func (r *Repository) Month(ctx context.Context, employee string, now time.Time) ([]Advance, error) {
	month := now.In(r.location).Format("2006-01")
	query := `SELECT id,employee_name,amount_cents,note,deduct_cash,cash_movement_id,business_date,actor,created_at FROM employee_advances WHERE month_key=?`
	args := []any{month}
	if strings.TrimSpace(employee) != "" {
		query += ` AND lower(employee_name)=lower(?)`
		args = append(args, strings.TrimSpace(employee))
	}
	query += ` ORDER BY employee_name,id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Advance
	for rows.Next() {
		var a Advance
		var created string
		if err := rows.Scan(&a.ID, &a.Employee, &a.Cents, &a.Note, &a.DeductCash, &a.CashMovementID, &a.Date, &a.Actor, &created); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", created)
		result = append(result, a)
	}
	return result, rows.Err()
}

func Report(items []Advance, month string) string {
	if len(items) == 0 {
		return "📋 Nenhum vale encontrado em " + month + "."
	}
	var total int64
	lines := make([]string, 0, len(items))
	for _, item := range items {
		total += item.Cents
		cashText := "não descontado do caixa"
		if item.DeductCash {
			cashText = "descontado do caixa em dinheiro"
		}
		line := fmt.Sprintf("• %s — %s — %s (%s)", item.Date, item.Employee, cash.Money(item.Cents), cashText)
		if item.Note != "" {
			line += " — " + item.Note
		}
		lines = append(lines, line)
	}
	return fmt.Sprintf("💵 VALES — %s\n\n%s\n\nTotal: %s", month, strings.Join(lines, "\n"), cash.Money(total))
}
