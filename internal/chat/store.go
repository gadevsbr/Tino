package chat

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types/events"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

type Conversation struct {
	JID, Name, LastMessage string
	LastAt                 time.Time
	Unread                 int
}

type Message struct {
	ID, ChatJID, SenderJID, Text string
	Timestamp                    time.Time
	FromMe                       bool
}

func Open(path string) (*Store, error) {
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS chat_messages (
		id TEXT PRIMARY KEY, chat_jid TEXT NOT NULL, sender_jid TEXT NOT NULL,
		body TEXT NOT NULL, timestamp INTEGER NOT NULL, from_me INTEGER NOT NULL);
		CREATE INDEX IF NOT EXISTS idx_chat_messages_chat_time ON chat_messages(chat_jid, timestamp);
		CREATE TABLE IF NOT EXISTS chats (
		jid TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', last_message TEXT NOT NULL DEFAULT '',
		last_at INTEGER NOT NULL DEFAULT 0, unread INTEGER NOT NULL DEFAULT 0);`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("criar histórico local: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) SaveEvent(ctx context.Context, evt *events.Message, unread bool) error {
	if evt == nil || evt.Info.ID == "" {
		return nil
	}
	text := ExtractText(evt)
	if text == "" {
		text = "[mídia ou mensagem não textual]"
	}
	name := strings.TrimSpace(evt.Info.PushName)
	if name == "" {
		name = evt.Info.Chat.User
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO chat_messages(id,chat_jid,sender_jid,body,timestamp,from_me) VALUES(?,?,?,?,?,?)`, string(evt.Info.ID), evt.Info.Chat.String(), evt.Info.Sender.String(), text, evt.Info.Timestamp.Unix(), evt.Info.IsFromMe)
	if err != nil {
		return err
	}
	inserted, _ := res.RowsAffected()
	inc := 0
	if unread && !evt.Info.IsFromMe && inserted > 0 {
		inc = 1
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO chats(jid,name,last_message,last_at,unread) VALUES(?,?,?,?,?)
		ON CONFLICT(jid) DO UPDATE SET name=CASE WHEN excluded.name<>'' THEN excluded.name ELSE chats.name END,
		last_message=CASE WHEN excluded.last_at>=chats.last_at THEN excluded.last_message ELSE chats.last_message END,
		last_at=MAX(chats.last_at,excluded.last_at), unread=chats.unread+?`, evt.Info.Chat.String(), name, text, evt.Info.Timestamp.Unix(), inc, inc)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Conversations(ctx context.Context, search string) ([]Conversation, error) {
	pattern := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
	rows, err := s.db.QueryContext(ctx, `SELECT jid,name,last_message,last_at,unread FROM chats WHERE lower(name) LIKE ? OR lower(jid) LIKE ? OR lower(last_message) LIKE ? ORDER BY last_at DESC`, pattern, pattern, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var c Conversation
		var ts int64
		if err := rows.Scan(&c.JID, &c.Name, &c.LastMessage, &ts, &c.Unread); err != nil {
			return nil, err
		}
		c.LastAt = time.Unix(ts, 0)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Messages(ctx context.Context, jid string, limit int) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,chat_jid,sender_jid,body,timestamp,from_me FROM (SELECT id,chat_jid,sender_jid,body,timestamp,from_me FROM chat_messages WHERE chat_jid=? ORDER BY timestamp DESC LIMIT ?) ORDER BY timestamp`, jid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var ts int64
		if err := rows.Scan(&m.ID, &m.ChatJID, &m.SenderJID, &m.Text, &ts, &m.FromMe); err != nil {
			return nil, err
		}
		m.Timestamp = time.Unix(ts, 0)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) MarkRead(ctx context.Context, jid string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET unread=0 WHERE jid=?`, jid)
	return err
}

func (s *Store) UpdateName(ctx context.Context, jid, name string) error {
	name = strings.TrimSpace(name)
	if jid == "" || name == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET name=? WHERE jid=?`, name, jid)
	return err
}

func ExtractText(evt *events.Message) string {
	if evt == nil || evt.Message == nil {
		return ""
	}
	if v := evt.Message.GetConversation(); v != "" {
		return strings.TrimSpace(v)
	}
	if v := evt.Message.GetExtendedTextMessage().GetText(); v != "" {
		return strings.TrimSpace(v)
	}
	if v := evt.Message.GetImageMessage().GetCaption(); v != "" {
		return strings.TrimSpace(v)
	}
	if v := evt.Message.GetVideoMessage().GetCaption(); v != "" {
		return strings.TrimSpace(v)
	}
	if v := evt.Message.GetDocumentMessage().GetCaption(); v != "" {
		return strings.TrimSpace(v)
	}
	return ""
}
