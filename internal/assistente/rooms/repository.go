package rooms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("room not found")
var ErrNotCleaning = errors.New("room is not awaiting cleaning maintenance")
var ErrCleaningCompletedToday = errors.New("cleaning maintenance already completed today")

type Repository struct {
	db       *sql.DB
	location *time.Location
}

func NewRepository(db *sql.DB, locations ...*time.Location) *Repository {
	location := time.Local
	if len(locations) > 0 && locations[0] != nil {
		location = locations[0]
	}
	return &Repository{db: db, location: location}
}

func (r *Repository) Get(ctx context.Context, number int) (Room, error) {
	var room Room
	var status string
	var created, updated string
	err := r.db.QueryRowContext(ctx, `SELECT id,number,floor,operational_status,observation,guest_count,created_at,updated_at FROM rooms WHERE number=?`, number).Scan(&room.ID, &room.Number, &room.Floor, &status, &room.Observation, &room.GuestCount, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Room{}, ErrNotFound
	}
	if err != nil {
		return Room{}, err
	}
	room.Status = Status(status)
	room.CreatedAt, _ = parseTime(created)
	room.UpdatedAt, _ = parseTime(updated)
	return room, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, number int, status Status, actor, source, messageID string) (bool, Status, error) {
	return r.UpdateStatusWithGuests(ctx, number, status, 0, actor, source, messageID)
}

func (r *Repository) UpdateStatusWithGuests(ctx context.Context, number int, status Status, guests int, actor, source, messageID string) (bool, Status, error) {
	if !status.Valid() {
		return false, "", fmt.Errorf("invalid status %q", status)
	}
	if guests < 0 || ((status == Entry || status == OccupiedClean) && guests < 1) {
		return false, "", fmt.Errorf("guest count required for %s", status)
	}
	if status != Entry && status != OccupiedClean {
		guests = 0
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, "", err
	}
	defer tx.Rollback()
	var id int64
	var old Status
	var completedDate string
	if err := tx.QueryRowContext(ctx, `SELECT id,operational_status,cleaning_completed_date FROM rooms WHERE number=?`, number).Scan(&id, &old, &completedDate); errors.Is(err, sql.ErrNoRows) {
		return false, "", ErrNotFound
	} else if err != nil {
		return false, "", err
	}
	if status == OccupiedClean && completedDate == time.Now().In(r.location).Format("2006-01-02") {
		return false, old, ErrCleaningCompletedToday
	}
	var oldGuests int
	if err := tx.QueryRowContext(ctx, `SELECT guest_count FROM rooms WHERE id=?`, id).Scan(&oldGuests); err != nil {
		return false, "", err
	}
	if old == status && oldGuests == guests {
		if status == OccupiedClean && completedDate != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE rooms SET cleaning_completed_date='',cleaning_completed_by='',updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`, id); err != nil {
				return false, "", err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO room_history(room_id,old_status,new_status,changed_by,source,message_id,created_at) VALUES (?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, id, old, status, actor, source, messageID); err != nil {
				return false, "", err
			}
			return true, old, tx.Commit()
		}
		return false, old, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE rooms SET operational_status=?,guest_count=?,cleaning_completed_date='',cleaning_completed_by='',updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`, status, guests, id); err != nil {
		return false, "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO room_history(room_id,old_status,new_status,changed_by,source,message_id,created_at) VALUES (?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, id, old, status, actor, source, messageID); err != nil {
		return false, "", err
	}
	return true, old, tx.Commit()
}

func (r *Repository) CompleteCleaning(ctx context.Context, number int, actor, messageID string, now time.Time) error {
	today := now.In(r.location).Format("2006-01-02")
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	var status Status
	var completed string
	err = tx.QueryRowContext(ctx, `SELECT id,operational_status,cleaning_completed_date FROM rooms WHERE number=?`, number).Scan(&id, &status, &completed)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != OccupiedClean {
		return ErrNotCleaning
	}
	if completed == today {
		return ErrCleaningCompletedToday
	}
	if _, err = tx.ExecContext(ctx, `UPDATE rooms SET cleaning_completed_date=?,cleaning_completed_by=?,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`, today, actor, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_log(event,actor,details,created_at) VALUES ('CLEANING_COMPLETED',?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, actor, fmt.Sprintf("room=%d message_id=%s", number, messageID)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) CleaningCompletedToday(ctx context.Context, number int, now time.Time) (bool, error) {
	var date string
	err := r.db.QueryRowContext(ctx, `SELECT cleaning_completed_date FROM rooms WHERE number=?`, number).Scan(&date)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	return date == now.In(r.location).Format("2006-01-02"), err
}

func (r *Repository) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rooms`).Scan(&n)
	return n, err
}

func (r *Repository) ListAll(ctx context.Context) ([]Room, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,number,floor,operational_status,observation,guest_count,created_at,updated_at FROM rooms ORDER BY number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Room
	for rows.Next() {
		var room Room
		var status, created, updated string
		if err := rows.Scan(&room.ID, &room.Number, &room.Floor, &status, &room.Observation, &room.GuestCount, &created, &updated); err != nil {
			return nil, err
		}
		room.Status = Status(status)
		room.CreatedAt, _ = parseTime(created)
		room.UpdatedAt, _ = parseTime(updated)
		result = append(result, room)
	}
	return result, rows.Err()
}

func (r *Repository) ListByStatus(ctx context.Context, status Status) ([]Room, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,number,floor,operational_status,observation,guest_count,created_at,updated_at FROM rooms WHERE operational_status=? ORDER BY number`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Room
	for rows.Next() {
		var room Room
		var raw, created, updated string
		if err := rows.Scan(&room.ID, &room.Number, &room.Floor, &raw, &room.Observation, &room.GuestCount, &created, &updated); err != nil {
			return nil, err
		}
		room.Status = Status(raw)
		room.CreatedAt, _ = parseTime(created)
		room.UpdatedAt, _ = parseTime(updated)
		result = append(result, room)
	}
	return result, rows.Err()
}

func (r *Repository) Counts(ctx context.Context) (map[Status]int, error) {
	result := map[Status]int{}
	rows, err := r.db.QueryContext(ctx, `SELECT operational_status,COUNT(*) FROM rooms GROUP BY operational_status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status Status
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}
	return result, rows.Err()
}

func (r *Repository) GuestTotals(ctx context.Context) (map[Status]int, error) {
	result := map[Status]int{}
	rows, err := r.db.QueryContext(ctx, `SELECT operational_status,COALESCE(SUM(guest_count),0) FROM rooms WHERE operational_status IN (?,?) GROUP BY operational_status`, Entry, OccupiedClean)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status Status
		var total int
		if err := rows.Scan(&status, &total); err != nil {
			return nil, err
		}
		result[status] = total
	}
	return result, rows.Err()
}

func (r *Repository) SetObservation(ctx context.Context, number int, text string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE rooms SET observation=?,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE number=?`, text, number)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

type HistoryEntry struct {
	Old       Status
	New       Status
	CreatedAt time.Time
}

type StatusChange struct {
	Number     int
	Old, New   Status
	GuestCount int
}

func (r *Repository) ChangesByMessage(ctx context.Context, messageID string) ([]StatusChange, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT r.number,h.old_status,h.new_status,r.guest_count FROM room_history h JOIN rooms r ON r.id=h.room_id WHERE h.message_id=? ORDER BY r.number`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []StatusChange
	for rows.Next() {
		var change StatusChange
		if err := rows.Scan(&change.Number, &change.Old, &change.New, &change.GuestCount); err != nil {
			return nil, err
		}
		result = append(result, change)
	}
	return result, rows.Err()
}

func (r *Repository) History(ctx context.Context, number, limit int) ([]HistoryEntry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT h.old_status,h.new_status,h.created_at FROM room_history h JOIN rooms r ON r.id=h.room_id WHERE r.number=? ORDER BY h.created_at DESC LIMIT ?`, number, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []HistoryEntry
	for rows.Next() {
		var h HistoryEntry
		var raw string
		if err := rows.Scan(&h.Old, &h.New, &raw); err != nil {
			return nil, err
		}
		h.CreatedAt, _ = parseTime(raw)
		result = append(result, h)
	}
	return result, rows.Err()
}

func (r *Repository) BatchUpdate(ctx context.Context, numbers []int, status Status, actor, source, messageID string) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	changed := 0
	for _, number := range numbers {
		var id int64
		var old Status
		var completedDate string
		if err := tx.QueryRowContext(ctx, `SELECT id,operational_status,cleaning_completed_date FROM rooms WHERE number=?`, number).Scan(&id, &old, &completedDate); errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("%w: %d", ErrNotFound, number)
		} else if err != nil {
			return 0, err
		}
		if status == OccupiedClean && completedDate == time.Now().In(r.location).Format("2006-01-02") {
			return 0, fmt.Errorf("%w: %d", ErrCleaningCompletedToday, number)
		}
		if old == status {
			if status == OccupiedClean && completedDate != "" {
				if _, err := tx.ExecContext(ctx, `UPDATE rooms SET cleaning_completed_date='',cleaning_completed_by='',updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`, id); err != nil {
					return 0, err
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO room_history(room_id,old_status,new_status,changed_by,source,message_id,created_at) VALUES (?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, id, old, status, actor, source, messageID); err != nil {
					return 0, err
				}
				changed++
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE rooms SET operational_status=?,cleaning_completed_date='',cleaning_completed_by='',updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`, status, id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO room_history(room_id,old_status,new_status,changed_by,source,message_id,created_at) VALUES (?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, id, old, status, actor, source, messageID); err != nil {
			return 0, err
		}
		changed++
	}
	return changed, tx.Commit()
}

func parseTime(v string) (t time.Time, err error) { return time.Parse("2006-01-02T15:04:05.000Z", v) }
