package database

import (
	"context"
	"database/sql"
	"fmt"
)

var migrations = []string{
	`CREATE TABLE rooms (
 id INTEGER PRIMARY KEY, number INTEGER NOT NULL UNIQUE, floor INTEGER NOT NULL,
 current_status TEXT NOT NULL CHECK(current_status IN ('DISPONIVEL_LIMPO','OCUPADO_LIMPEZA','SAIDA_HOJE','INTERDITADO_MANUTENCAO','SUJO')),
 observation TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE room_history (
 id INTEGER PRIMARY KEY, room_id INTEGER NOT NULL REFERENCES rooms(id), old_status TEXT NOT NULL,
 new_status TEXT NOT NULL, changed_by TEXT NOT NULL, source TEXT NOT NULL, message_id TEXT,
 created_at TEXT NOT NULL
);
CREATE INDEX idx_room_history_room_created ON room_history(room_id, created_at DESC);
CREATE TABLE conversation_sessions (
 id INTEGER PRIMARY KEY, user_number TEXT NOT NULL UNIQUE, session_type TEXT NOT NULL, status TEXT NOT NULL,
 current_index INTEGER NOT NULL DEFAULT 0, current_room INTEGER, selected_rooms_json TEXT NOT NULL DEFAULT '[]',
 started_at TEXT NOT NULL, last_interaction_at TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE processed_messages (message_id TEXT PRIMARY KEY, sender TEXT NOT NULL, processed_at TEXT NOT NULL);
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE reports (id INTEGER PRIMARY KEY, path TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE backups (id INTEGER PRIMARY KEY, path TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE audit_log (id INTEGER PRIMARY KEY, event TEXT NOT NULL, actor TEXT, details TEXT, created_at TEXT NOT NULL);`,
	`ALTER TABLE rooms ADD COLUMN operational_status TEXT NOT NULL DEFAULT 'LIMPAR';
UPDATE rooms SET operational_status=CASE current_status WHEN 'DISPONIVEL_LIMPO' THEN 'DISPONIVEL' WHEN 'OCUPADO_LIMPEZA' THEN 'MANUTENCAO_LIMPEZA' WHEN 'SAIDA_HOJE' THEN 'SAIDA' WHEN 'INTERDITADO_MANUTENCAO' THEN 'INTERDITADO' ELSE 'LIMPAR' END;`,
	`ALTER TABLE rooms ADD COLUMN cleaning_completed_date TEXT NOT NULL DEFAULT '';
ALTER TABLE rooms ADD COLUMN cleaning_completed_by TEXT NOT NULL DEFAULT '';`,
	`ALTER TABLE rooms ADD COLUMN guest_count INTEGER NOT NULL DEFAULT 0 CHECK(guest_count >= 0);`,
	`CREATE TABLE cash_days (business_date TEXT PRIMARY KEY, opening_cents INTEGER NOT NULL CHECK(opening_cents >= 0), opened_by TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE cash_movements (id INTEGER PRIMARY KEY, business_date TEXT NOT NULL REFERENCES cash_days(business_date), kind TEXT NOT NULL CHECK(kind IN ('ENTRY','EXIT')), method TEXT NOT NULL, amount_cents INTEGER NOT NULL CHECK(amount_cents > 0), description TEXT NOT NULL, actor TEXT NOT NULL, message_id TEXT, created_at TEXT NOT NULL);
CREATE UNIQUE INDEX idx_cash_message ON cash_movements(message_id) WHERE message_id IS NOT NULL;
CREATE INDEX idx_cash_date_id ON cash_movements(business_date,id);`,
	`CREATE TABLE cash_movement_audit (
 id INTEGER PRIMARY KEY, movement_id INTEGER NOT NULL, action TEXT NOT NULL CHECK(action IN ('EDIT','DELETE')),
 old_kind TEXT NOT NULL, old_method TEXT NOT NULL, old_amount_cents INTEGER NOT NULL, old_description TEXT NOT NULL,
 new_kind TEXT NOT NULL, new_method TEXT NOT NULL, new_amount_cents INTEGER NOT NULL, new_description TEXT NOT NULL,
 actor TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE INDEX idx_cash_audit_movement ON cash_movement_audit(movement_id,id);`,
	`ALTER TABLE cash_days ADD COLUMN closed_at TEXT NOT NULL DEFAULT '';
ALTER TABLE cash_days ADD COLUMN closed_by TEXT NOT NULL DEFAULT '';
CREATE TABLE cash_day_audit (
 id INTEGER PRIMARY KEY, business_date TEXT NOT NULL, action TEXT NOT NULL CHECK(action IN ('CLOSE','REOPEN')),
 actor TEXT NOT NULL, details TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE cash_opening_audit (
 id INTEGER PRIMARY KEY, business_date TEXT NOT NULL, old_amount_cents INTEGER NOT NULL,
 new_amount_cents INTEGER NOT NULL, actor TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE INDEX idx_cash_day_audit_date ON cash_day_audit(business_date,id);`,
	`ALTER TABLE conversation_sessions ADD COLUMN payload_json TEXT NOT NULL DEFAULT '{}';
CREATE TABLE employee_advances (
 id INTEGER PRIMARY KEY,
 employee_name TEXT NOT NULL,
 amount_cents INTEGER NOT NULL CHECK(amount_cents > 0),
 note TEXT NOT NULL,
 deduct_cash INTEGER NOT NULL CHECK(deduct_cash IN (0,1)),
 cash_movement_id INTEGER REFERENCES cash_movements(id) ON DELETE SET NULL,
 business_date TEXT NOT NULL,
 month_key TEXT NOT NULL,
 actor TEXT NOT NULL,
 message_id TEXT NOT NULL UNIQUE,
 created_at TEXT NOT NULL
);
CREATE INDEX idx_employee_advances_month_name ON employee_advances(month_key,employee_name,id);
CREATE TABLE authorized_numbers (
 number TEXT PRIMARY KEY,
 authorized_by TEXT NOT NULL,
 created_at TEXT NOT NULL
);`,
	`CREATE TABLE IF NOT EXISTS cash_opening_audit (
 id INTEGER PRIMARY KEY, business_date TEXT NOT NULL, old_amount_cents INTEGER NOT NULL,
 new_amount_cents INTEGER NOT NULL, actor TEXT NOT NULL, created_at TEXT NOT NULL
);`,
	`CREATE TABLE cash_receipts (
 id INTEGER PRIMARY KEY, image_sha256 TEXT NOT NULL UNIQUE, business_date TEXT NOT NULL,
 method TEXT NOT NULL CHECK(method IN ('PIX','CARTAO')), amount_cents INTEGER NOT NULL CHECK(amount_cents > 0),
 description TEXT NOT NULL, actor TEXT NOT NULL, movement_id INTEGER NOT NULL UNIQUE REFERENCES cash_movements(id),
 created_at TEXT NOT NULL
);`,
	`ALTER TABLE cash_receipts ADD COLUMN file_path TEXT NOT NULL DEFAULT '';
ALTER TABLE cash_receipts ADD COLUMN media_type TEXT NOT NULL DEFAULT '';`,
	`CREATE TABLE extrato_weeks (
 id TEXT PRIMARY KEY, week_number INTEGER NOT NULL CHECK(week_number BETWEEN 1 AND 6),
 month_number INTEGER NOT NULL CHECK(month_number BETWEEN 1 AND 12), year_number INTEGER NOT NULL,
 start_date TEXT NOT NULL, end_date TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'OPEN' CHECK(status IN ('OPEN','CLOSED')),
 created_by TEXT NOT NULL, last_generated_at TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 UNIQUE(year_number,month_number,week_number)
);
CREATE TABLE extrato_week_files (
 id INTEGER PRIMARY KEY, week_id TEXT NOT NULL REFERENCES extrato_weeks(id) ON DELETE CASCADE,
 original_name TEXT NOT NULL, file_path TEXT NOT NULL, sha256 TEXT NOT NULL, uploaded_by TEXT NOT NULL, created_at TEXT NOT NULL,
 UNIQUE(week_id,sha256)
);
CREATE INDEX idx_extrato_week_files_week ON extrato_week_files(week_id,id);`,
	`ALTER TABLE rooms ADD COLUMN bitz_category TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_rooms_bitz_category_status ON rooms(bitz_category,operational_status,number);`,
	`CREATE TABLE payable_bills (
 id INTEGER PRIMARY KEY, description TEXT NOT NULL, amount_cents INTEGER NOT NULL CHECK(amount_cents>0),
 due_date TEXT NOT NULL, status TEXT NOT NULL CHECK(status IN ('OPEN','PAID','CANCELLED')),
 created_at TEXT NOT NULL, paid_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE bill_reminders (
 bill_id INTEGER NOT NULL REFERENCES payable_bills(id), day TEXT NOT NULL,
 status TEXT NOT NULL, lease_until INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(bill_id,day)
);`,
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}
	for i, migration := range migrations {
		version := i + 1
		var exists int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`, version).Scan(&exists); err != nil {
			return err
		}
		if exists == 1 {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, migration); err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, version)
		}
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", version, err)
		}
	}
	return nil
}
