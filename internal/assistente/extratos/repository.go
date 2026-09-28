package extratos

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Week struct {
	ID              string
	Period          Period
	Status          string
	CreatedBy       string
	LastGeneratedAt string
}

type WeekFile struct {
	ID           int64
	OriginalName string
	Path         string
	SHA256       string
}

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) FindWeek(ctx context.Context, period Period) (Week, bool, error) {
	var week Week
	var start, end string
	err := r.db.QueryRowContext(ctx, `SELECT id,week_number,month_number,year_number,start_date,end_date,status,created_by,last_generated_at FROM extrato_weeks WHERE year_number=? AND month_number=? AND week_number=?`, period.Year, period.Month, period.Week).Scan(&week.ID, &week.Period.Week, &week.Period.Month, &week.Period.Year, &start, &end, &week.Status, &week.CreatedBy, &week.LastGeneratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Week{}, false, nil
	}
	if err != nil {
		return Week{}, false, err
	}
	week.Period, err = week.Period.WithDates(start, end)
	return week, err == nil, err
}

func (r *Repository) CreateWeek(ctx context.Context, week Week) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO extrato_weeks(id,week_number,month_number,year_number,start_date,end_date,status,created_by,created_at,updated_at) VALUES (?,?,?,?,?,?,'OPEN',?,strftime('%Y-%m-%dT%H:%M:%fZ','now'),strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, week.ID, week.Period.Week, week.Period.Month, week.Period.Year, week.Period.From.Format("02/01/2006"), week.Period.To.Format("02/01/2006"), week.CreatedBy)
	return err
}

func (r *Repository) AddFile(ctx context.Context, weekID, name, path, sha, actor string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `INSERT OR IGNORE INTO extrato_week_files(week_id,original_name,file_path,sha256,uploaded_by,created_at) VALUES (?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, weekID, name, path, sha, actor)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (r *Repository) Files(ctx context.Context, weekID string) ([]WeekFile, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,original_name,file_path,sha256 FROM extrato_week_files WHERE week_id=? ORDER BY id`, weekID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []WeekFile
	for rows.Next() {
		var file WeekFile
		if err := rows.Scan(&file.ID, &file.OriginalName, &file.Path, &file.SHA256); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (r *Repository) MarkGenerated(ctx context.Context, weekID string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE extrato_weeks SET last_generated_at=?,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`, now.UTC().Format(time.RFC3339), weekID)
	return err
}

func (r *Repository) ListWeeks(ctx context.Context) ([]Week, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,week_number,month_number,year_number,start_date,end_date,status,created_by,last_generated_at FROM extrato_weeks ORDER BY year_number DESC,month_number DESC,week_number DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Week
	for rows.Next() {
		var item Week
		var start, end string
		if err := rows.Scan(&item.ID, &item.Period.Week, &item.Period.Month, &item.Period.Year, &start, &end, &item.Status, &item.CreatedBy, &item.LastGeneratedAt); err != nil {
			return nil, err
		}
		item.Period, err = item.Period.WithDates(start, end)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
