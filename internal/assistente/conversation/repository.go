package conversation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const Active = "ACTIVE"

type Session struct {
	User         string
	Type         string
	Status       string
	CurrentIndex int
	CurrentRoom  int
	Selected     []int
	Payload      string
	StartedAt    time.Time
}
type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Save(ctx context.Context, s Session) error {
	selected, _ := json.Marshal(s.Selected)
	_, err := r.db.ExecContext(ctx, `INSERT INTO conversation_sessions(user_number,session_type,status,current_index,current_room,selected_rooms_json,payload_json,started_at,last_interaction_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'),strftime('%Y-%m-%dT%H:%M:%fZ','now'),strftime('%Y-%m-%dT%H:%M:%fZ','now'),strftime('%Y-%m-%dT%H:%M:%fZ','now')) ON CONFLICT(user_number) DO UPDATE SET session_type=excluded.session_type,status=excluded.status,current_index=excluded.current_index,current_room=excluded.current_room,selected_rooms_json=excluded.selected_rooms_json,payload_json=excluded.payload_json,last_interaction_at=excluded.last_interaction_at,updated_at=excluded.updated_at`, s.User, s.Type, s.Status, s.CurrentIndex, s.CurrentRoom, string(selected), s.Payload)
	return err
}
func (r *Repository) Get(ctx context.Context, user string) (Session, error) {
	var s Session
	var selected, started string
	err := r.db.QueryRowContext(ctx, `SELECT user_number,session_type,status,current_index,current_room,selected_rooms_json,payload_json,started_at FROM conversation_sessions WHERE user_number=?`, user).Scan(&s.User, &s.Type, &s.Status, &s.CurrentIndex, &s.CurrentRoom, &selected, &s.Payload, &started)
	if err != nil {
		return s, err
	}
	_ = json.Unmarshal([]byte(selected), &s.Selected)
	s.StartedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", started)
	return s, nil
}
func (r *Repository) Active(ctx context.Context, user string) (Session, bool, error) {
	s, e := r.Get(ctx, user)
	if errors.Is(e, sql.ErrNoRows) {
		return s, false, nil
	}
	return s, e == nil && s.Status == Active, e
}
