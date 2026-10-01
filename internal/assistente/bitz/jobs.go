package bitz

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	JobRunning          = "running"
	JobAwaitingApproval = "awaiting_approval"
	JobApproved         = "approved"
	JobFailed           = "failed"
)

type Job struct {
	ID, Account, Contact, ChatJID, Code, Status, Error string
	CreatedAt, UpdatedAt                               time.Time
}

type Jobs struct{ db *sql.DB }

func NewJobs(ctx context.Context, db *sql.DB) (*Jobs, error) {
	if db == nil {
		return nil, errors.New("banco obrigatório")
	}
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS bitz_prereservations (
id TEXT PRIMARY KEY, account TEXT NOT NULL, contact TEXT NOT NULL, chat_jid TEXT NOT NULL,
code TEXT NOT NULL UNIQUE, status TEXT NOT NULL, error TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL, updated_at TEXT NOT NULL);`)
	if err != nil {
		return nil, err
	}
	return &Jobs{db: db}, nil
}

func newCode() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(b[:])), nil
}

func (j *Jobs) Claim(ctx context.Context, id, account, contact, chatJID string) (Job, bool, error) {
	if strings.TrimSpace(id) == "" || account == "" || contact == "" || chatJID == "" {
		return Job{}, false, errors.New("identidade da pré-reserva incompleta")
	}
	code, err := newCode()
	if err != nil {
		return Job{}, false, err
	}
	now := time.Now().UTC()
	job := Job{ID: id, Account: account, Contact: contact, ChatJID: chatJID, Code: code, Status: JobRunning, CreatedAt: now, UpdatedAt: now}
	res, err := j.db.ExecContext(ctx, `INSERT OR IGNORE INTO bitz_prereservations(id,account,contact,chat_jid,code,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, id, account, contact, chatJID, code, job.Status, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return Job{}, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Job{}, false, err
	}
	if n == 0 {
		existing, e := j.ByID(ctx, id)
		return existing, false, e
	}
	return job, true, nil
}
func (j *Jobs) set(ctx context.Context, id, status, message string) error {
	_, err := j.db.ExecContext(ctx, `UPDATE bitz_prereservations SET status=?,error=?,updated_at=? WHERE id=?`, status, message, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
func (j *Jobs) AwaitingApproval(ctx context.Context, id string) error {
	return j.set(ctx, id, JobAwaitingApproval, "")
}
func (j *Jobs) Fail(ctx context.Context, id string, cause error) error {
	msg := ""
	if cause != nil {
		msg = cause.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
	}
	return j.set(ctx, id, JobFailed, msg)
}
func (j *Jobs) Approve(ctx context.Context, code string) (Job, error) {
	job, err := j.ByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return Job{}, err
	}
	if job.Status != JobAwaitingApproval {
		return Job{}, fmt.Errorf("pré-reserva %s não aguarda aprovação", code)
	}
	if err = j.set(ctx, job.ID, JobApproved, ""); err != nil {
		return Job{}, err
	}
	job.Status = JobApproved
	return job, nil
}
func (j *Jobs) ByID(ctx context.Context, id string) (Job, error) {
	return j.query(ctx, `SELECT id,account,contact,chat_jid,code,status,error,created_at,updated_at FROM bitz_prereservations WHERE id=?`, id)
}
func (j *Jobs) ByCode(ctx context.Context, code string) (Job, error) {
	return j.query(ctx, `SELECT id,account,contact,chat_jid,code,status,error,created_at,updated_at FROM bitz_prereservations WHERE code=?`, code)
}
func (j *Jobs) query(ctx context.Context, q, arg string) (Job, error) {
	var job Job
	var created, updated string
	err := j.db.QueryRowContext(ctx, q, arg).Scan(&job.ID, &job.Account, &job.Contact, &job.ChatJID, &job.Code, &job.Status, &job.Error, &created, &updated)
	if err != nil {
		return Job{}, err
	}
	job.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	job.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return job, nil
}
