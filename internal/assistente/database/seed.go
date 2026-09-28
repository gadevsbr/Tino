package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gadevsbr/tino/internal/assistente/rooms"
)

func SeedRooms(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, number := range rooms.OfficialNumbers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO rooms(number,floor,current_status,created_at,updated_at) VALUES (?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'),strftime('%Y-%m-%dT%H:%M:%fZ','now')) ON CONFLICT(number) DO NOTHING`, number, number/100, "SUJO"); err != nil {
			return fmt.Errorf("seed room %d: %w", number, err)
		}
	}
	return tx.Commit()
}
