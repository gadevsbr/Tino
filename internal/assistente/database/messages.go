package database

import (
	"context"
	"database/sql"
	"errors"
)

func ClaimMessage(ctx context.Context, db *sql.DB, id, sender string) (bool, error) {
	_, err := db.ExecContext(ctx, `INSERT INTO processed_messages(message_id,sender,processed_at) VALUES (?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, id, sender)
	if err == nil {
		return true, nil
	}
	var exists int
	if scanErr := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM processed_messages WHERE message_id=?`, id).Scan(&exists); scanErr != nil {
		return false, errors.Join(err, scanErr)
	}
	if exists == 1 {
		return false, nil
	}
	return false, err
}
