package cash

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrMovementNotFound = errors.New("cash movement not found")
var ErrCashClosed = errors.New("cash day is closed")
var ErrReceiptDuplicate = errors.New("receipt already registered")

type Movement struct {
	ID                               int64
	Kind, Method, Description, Actor string
	Cents                            int64
	CreatedAt                        time.Time
}
type Receipt struct {
	ID           int64
	BusinessDate string
	Method       string
	Cents        int64
	Description  string
	Actor        string
	MovementID   int64
	FilePath     string
	MediaType    string
	CreatedAt    string
}
type Day struct {
	Date         string
	OpeningCents int64
	ClosedAt     string
	ClosedBy     string
	Entries      []Movement
	Exits        []Movement
}
type Repository struct {
	db       *sql.DB
	location *time.Location
}

func NewRepository(db *sql.DB, location *time.Location) *Repository {
	return &Repository{db: db, location: location}
}
func (r *Repository) date(now time.Time) string { return now.In(r.location).Format("2006-01-02") }

func (r *Repository) Open(ctx context.Context, cents int64, actor string, now time.Time) error {
	return r.OpenDate(ctx, r.date(now), cents, actor)
}

func (r *Repository) OpenDate(ctx context.Context, date string, cents int64, actor string) error {
	if cents < 0 {
		return errors.New("opening balance cannot be negative")
	}
	var closed string
	err := r.db.QueryRowContext(ctx, `SELECT closed_at FROM cash_days WHERE business_date=?`, date).Scan(&closed)
	if err == nil && closed != "" {
		return ErrCashClosed
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO cash_days(business_date,opening_cents,opened_by,created_at) VALUES (?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now')) ON CONFLICT(business_date) DO UPDATE SET opening_cents=excluded.opening_cents,opened_by=excluded.opened_by`, date, cents, actor)
	return err
}

func (r *Repository) EditOpening(ctx context.Context, date string, cents int64, actor string) error {
	if cents < 0 {
		return errors.New("opening balance cannot be negative")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old int64
	if err = tx.QueryRowContext(ctx, `SELECT opening_cents FROM cash_days WHERE business_date=?`, date).Scan(&old); errors.Is(err, sql.ErrNoRows) {
		return errors.New("cash day not found")
	} else if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE cash_days SET opening_cents=?,opened_by=? WHERE business_date=?`, cents, actor, date); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO cash_opening_audit(business_date,old_amount_cents,new_amount_cents,actor,created_at) VALUES (?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, date, old, cents, actor); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) Add(ctx context.Context, kind, method string, cents int64, description, actor, messageID string, now time.Time) error {
	return r.AddDate(ctx, r.date(now), kind, method, cents, description, actor, messageID)
}

func (r *Repository) AddDate(ctx context.Context, date, kind, method string, cents int64, description, actor, messageID string) error {
	if cents <= 0 {
		return errors.New("amount must be positive")
	}
	if kind != "ENTRY" && kind != "EXIT" {
		return errors.New("invalid movement kind")
	}
	if kind == "ENTRY" && method != "DINHEIRO" && method != "CARTAO" && method != "PIX" {
		return errors.New("invalid payment method")
	}
	if kind == "EXIT" {
		method = ""
	}
	if _, err := r.db.ExecContext(ctx, `INSERT INTO cash_days(business_date,opening_cents,opened_by,created_at) VALUES (?,0,?,strftime('%Y-%m-%dT%H:%M:%fZ','now')) ON CONFLICT(business_date) DO NOTHING`, date, actor); err != nil {
		return err
	}
	var closed string
	if err := r.db.QueryRowContext(ctx, `SELECT closed_at FROM cash_days WHERE business_date=?`, date).Scan(&closed); err != nil {
		return err
	}
	if closed != "" {
		return ErrCashClosed
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO cash_movements(business_date,kind,method,amount_cents,description,actor,message_id,created_at) VALUES (?,?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, date, kind, method, cents, description, actor, messageID)
	return err
}

func (r *Repository) AddReceipt(ctx context.Context, date, method string, cents int64, description, actor, imageHash, messageID, filePath, mediaType string) error {
	if method != "PIX" && method != "CARTAO" {
		return errors.New("invalid receipt payment method")
	}
	if cents <= 0 || strings.TrimSpace(description) == "" || len(imageHash) != 64 {
		return errors.New("invalid receipt details")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO cash_days(business_date,opening_cents,opened_by,created_at) VALUES (?,0,?,strftime('%Y-%m-%dT%H:%M:%fZ','now')) ON CONFLICT(business_date) DO NOTHING`, date, actor); err != nil {
		return err
	}
	var closed string
	if err = tx.QueryRowContext(ctx, `SELECT closed_at FROM cash_days WHERE business_date=?`, date).Scan(&closed); err != nil {
		return err
	}
	if closed != "" {
		return ErrCashClosed
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO cash_movements(business_date,kind,method,amount_cents,description,actor,message_id,created_at) VALUES (?,'ENTRY',?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, date, method, cents, description, actor, messageID)
	if err != nil {
		return err
	}
	movementID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cash_receipts(image_sha256,business_date,method,amount_cents,description,actor,movement_id,file_path,media_type,created_at) VALUES (?,?,?,?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, imageHash, date, method, cents, description, actor, movementID, filePath, mediaType)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ErrReceiptDuplicate
		}
		return err
	}
	return tx.Commit()
}

func (r *Repository) ReceiptsByDate(ctx context.Context, date string) ([]Receipt, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,business_date,method,amount_cents,description,actor,movement_id,file_path,media_type,created_at FROM cash_receipts WHERE business_date=? ORDER BY id`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Receipt
	for rows.Next() {
		var item Receipt
		if err := rows.Scan(&item.ID, &item.BusinessDate, &item.Method, &item.Cents, &item.Description, &item.Actor, &item.MovementID, &item.FilePath, &item.MediaType, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) ReceiptsRange(ctx context.Context, from, to string) ([]Receipt, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,business_date,method,amount_cents,description,actor,movement_id,file_path,media_type,created_at FROM cash_receipts WHERE business_date BETWEEN ? AND ? ORDER BY business_date DESC,id DESC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Receipt
	for rows.Next() {
		var item Receipt
		if err := rows.Scan(&item.ID, &item.BusinessDate, &item.Method, &item.Cents, &item.Description, &item.Actor, &item.MovementID, &item.FilePath, &item.MediaType, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) Receipt(ctx context.Context, id int64) (Receipt, error) {
	var item Receipt
	err := r.db.QueryRowContext(ctx, `SELECT id,business_date,method,amount_cents,description,actor,movement_id,file_path,media_type,created_at FROM cash_receipts WHERE id=?`, id).Scan(&item.ID, &item.BusinessDate, &item.Method, &item.Cents, &item.Description, &item.Actor, &item.MovementID, &item.FilePath, &item.MediaType, &item.CreatedAt)
	return item, err
}

func (r *Repository) ReceiptExists(ctx context.Context, imageHash string) (bool, error) {
	var found int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cash_receipts WHERE image_sha256=?`, imageHash).Scan(&found)
	return found > 0, err
}

func (r *Repository) Today(ctx context.Context, now time.Time) (Day, error) {
	return r.ByDate(ctx, r.date(now))
}

func (r *Repository) ByDate(ctx context.Context, date string) (Day, error) {
	day := Day{Date: date}
	err := r.db.QueryRowContext(ctx, `SELECT opening_cents,closed_at,closed_by FROM cash_days WHERE business_date=?`, day.Date).Scan(&day.OpeningCents, &day.ClosedAt, &day.ClosedBy)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	if err != nil {
		return day, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,kind,method,amount_cents,description,actor,created_at FROM cash_movements WHERE business_date=? ORDER BY id`, day.Date)
	if err != nil {
		return day, err
	}
	defer rows.Close()
	for rows.Next() {
		var m Movement
		var raw string
		if err := rows.Scan(&m.ID, &m.Kind, &m.Method, &m.Cents, &m.Description, &m.Actor, &raw); err != nil {
			return day, err
		}
		m.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", raw)
		if m.Kind == "ENTRY" {
			day.Entries = append(day.Entries, m)
		} else {
			day.Exits = append(day.Exits, m)
		}
	}
	return day, rows.Err()
}

func (r *Repository) CloseToday(ctx context.Context, actor string, now time.Time) (Day, error) {
	date := r.date(now)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Day{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE cash_days SET closed_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'),closed_by=? WHERE business_date=? AND closed_at=''`, actor, date)
	if err != nil {
		return Day{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Day{}, ErrCashClosed
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO cash_day_audit(business_date,action,actor,details,created_at) VALUES (?,'CLOSE',?,'',strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, date, actor); err != nil {
		return Day{}, err
	}
	if err = tx.Commit(); err != nil {
		return Day{}, err
	}
	return r.ByDate(ctx, date)
}

func (r *Repository) ReopenToday(ctx context.Context, actor string, now time.Time) error {
	date := r.date(now)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE cash_days SET closed_at='',closed_by='' WHERE business_date=? AND closed_at<>''`, date)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("cash day is not closed")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO cash_day_audit(business_date,action,actor,details,created_at) VALUES (?,'REOPEN',?,'',strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, date, actor); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) Range(ctx context.Context, from, to string) ([]Day, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT business_date FROM cash_days WHERE business_date BETWEEN ? AND ? ORDER BY business_date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// ByDate performs another query. Release the cursor first: the database
	// intentionally has only one open connection.
	if err := rows.Close(); err != nil {
		return nil, err
	}
	days := make([]Day, 0, len(dates))
	for _, date := range dates {
		d, err := r.ByDate(ctx, date)
		if err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	return days, nil
}

func (r *Repository) MovementToday(ctx context.Context, id int64, now time.Time) (Movement, error) {
	var m Movement
	var raw string
	err := r.db.QueryRowContext(ctx, `SELECT id,kind,method,amount_cents,description,actor,created_at FROM cash_movements WHERE id=? AND business_date=?`, id, r.date(now)).Scan(&m.ID, &m.Kind, &m.Method, &m.Cents, &m.Description, &m.Actor, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrMovementNotFound
	}
	m.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", raw)
	return m, err
}

func (r *Repository) EditToday(ctx context.Context, id int64, kind, method string, cents int64, description, actor string, now time.Time) error {
	if day, err := r.Today(ctx, now); err != nil {
		return err
	} else if day.ClosedAt != "" {
		return ErrCashClosed
	}
	if cents <= 0 || description == "" || (kind != "ENTRY" && kind != "EXIT") {
		return errors.New("invalid cash movement")
	}
	if kind == "ENTRY" && method != "DINHEIRO" && method != "CARTAO" && method != "PIX" {
		return errors.New("invalid payment method")
	}
	if kind == "EXIT" {
		method = ""
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old Movement
	err = tx.QueryRowContext(ctx, `SELECT kind,method,amount_cents,description FROM cash_movements WHERE id=? AND business_date=?`, id, r.date(now)).Scan(&old.Kind, &old.Method, &old.Cents, &old.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMovementNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO cash_movement_audit(movement_id,action,old_kind,old_method,old_amount_cents,old_description,new_kind,new_method,new_amount_cents,new_description,actor,created_at) VALUES (?,'EDIT',?,?,?,?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, id, old.Kind, old.Method, old.Cents, old.Description, kind, method, cents, description, actor); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE cash_movements SET kind=?,method=?,amount_cents=?,description=?,actor=? WHERE id=?`, kind, method, cents, description, actor, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) DeleteToday(ctx context.Context, id int64, actor string, now time.Time) error {
	if day, err := r.Today(ctx, now); err != nil {
		return err
	} else if day.ClosedAt != "" {
		return ErrCashClosed
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old Movement
	err = tx.QueryRowContext(ctx, `SELECT kind,method,amount_cents,description FROM cash_movements WHERE id=? AND business_date=?`, id, r.date(now)).Scan(&old.Kind, &old.Method, &old.Cents, &old.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMovementNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO cash_movement_audit(movement_id,action,old_kind,old_method,old_amount_cents,old_description,new_kind,new_method,new_amount_cents,new_description,actor,created_at) VALUES (?,'DELETE',?,?,?,?, '', '',0,'',?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, id, old.Kind, old.Method, old.Cents, old.Description, actor); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM cash_movements WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (d Day) EntryTotal() int64 {
	var n int64
	for _, m := range d.Entries {
		n += m.Cents
	}
	return n
}
func (d Day) MethodTotal(method string) int64 {
	var n int64
	for _, m := range d.Entries {
		if m.Method == method {
			n += m.Cents
		}
	}
	return n
}
func (d Day) ExitTotal() int64 {
	var n int64
	for _, m := range d.Exits {
		n += m.Cents
	}
	return n
}
func (d Day) FinalCents() int64 { return d.OpeningCents + d.EntryTotal() - d.ExitTotal() }
func (d Day) FinalMethodTotal(method string) int64 {
	if method == "DINHEIRO" {
		return d.OpeningCents + d.MethodTotal(method) - d.ExitTotal()
	}
	return d.MethodTotal(method)
}
func Money(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%sR$ %d,%02d", sign, cents/100, cents%100)
}
