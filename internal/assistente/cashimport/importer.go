package cashimport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Preview struct {
	Path string
	Document
	EntryCents, ExitCents int64
	Status                string
}

type Result struct {
	Imported, Skipped int
	Dates             []string
}

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS cash_pdf_imports (
	 sha256 TEXT PRIMARY KEY, business_date TEXT NOT NULL UNIQUE, original_name TEXT NOT NULL,
	 stored_path TEXT NOT NULL, opening_cents INTEGER NOT NULL, final_cents INTEGER NOT NULL,
	 imported_at TEXT NOT NULL, imported_by TEXT NOT NULL
	);`)
	return err
}

func PreviewFiles(ctx context.Context, db *sql.DB, paths []string) ([]Preview, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	result := make([]Preview, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		p := Preview{Path: path, Status: "READY"}
		data, err := os.ReadFile(path)
		if err != nil {
			p.Status = "ERROR"
			p.Warnings = []string{err.Error()}
			result = append(result, p)
			continue
		}
		doc, err := Parse(filepath.Base(path), data)
		if err != nil {
			p.Status = "ERROR"
			p.Warnings = []string{err.Error()}
			result = append(result, p)
			continue
		}
		p.Document = doc
		for _, m := range doc.Entries {
			p.EntryCents += m.Cents
		}
		for _, m := range doc.Exits {
			p.ExitCents += m.Cents
		}
		if doc.Review {
			p.Status = "REVIEW"
		}
		if seen[doc.Date] {
			p.Status = "DUPLICATE"
			p.Warnings = append(p.Warnings, "data repetida na seleção")
		}
		seen[doc.Date] = true
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cash_pdf_imports WHERE sha256=? OR business_date=?`, doc.SHA256, doc.Date).Scan(&count); err != nil {
			return nil, err
		}
		if count > 0 {
			p.Status = "IMPORTED"
			p.Warnings = append(p.Warnings, "PDF ou data já importado")
		}
		if p.Status == "READY" {
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cash_days WHERE business_date=?`, doc.Date).Scan(&count); err != nil {
				return nil, err
			}
			if count > 0 {
				p.Status = "EXISTING"
				p.Warnings = append(p.Warnings, "já existe caixa nesta data")
			}
		}
		result = append(result, p)
	}
	return result, nil
}

func ImportReady(ctx context.Context, db *sql.DB, dataDir string, paths []string) (Result, error) {
	previews, err := PreviewFiles(ctx, db, paths)
	if err != nil {
		return Result{}, err
	}
	ready := make([]Preview, 0, len(previews))
	for _, p := range previews {
		if p.Status == "READY" {
			ready = append(ready, p)
		}
	}
	result := Result{Skipped: len(previews) - len(ready)}
	if len(ready) == 0 {
		return result, errors.New("nenhum PDF conciliado e inédito para importar")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	root := filepath.Join(dataDir, "caixa-importados")
	for _, p := range ready {
		folder := filepath.Join(root, p.Date[:7])
		if err := os.MkdirAll(folder, 0o700); err != nil {
			return result, err
		}
		stored := filepath.Join(folder, p.SHA256[:12]+"-"+filepath.Base(p.FileName))
		raw, err := os.ReadFile(p.Path)
		if err != nil {
			return result, err
		}
		if err = os.WriteFile(stored, raw, 0o600); err != nil {
			return result, err
		}
		rel, err := filepath.Rel(dataDir, stored)
		if err != nil {
			return result, err
		}
		closedAt, closedBy := "", ""
		if p.Closed {
			closedAt = time.Now().UTC().Format(time.RFC3339Nano)
			closedBy = "importador-pdf"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO cash_days(business_date,opening_cents,opened_by,created_at,closed_at,closed_by) VALUES(?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'),?,?)`, p.Date, p.OpeningCents, "importador-pdf", closedAt, closedBy)
		if err != nil {
			return result, fmt.Errorf("importar %s: %w", p.FileName, err)
		}
		index := 0
		for _, kind := range []string{"ENTRY", "EXIT"} {
			items := p.Entries
			if kind == "EXIT" {
				items = p.Exits
			}
			for _, m := range items {
				index++
				method := strings.TrimSpace(m.Method)
				if kind == "EXIT" {
					method = "DINHEIRO"
				}
				_, err = tx.ExecContext(ctx, `INSERT INTO cash_movements(business_date,kind,method,amount_cents,description,actor,message_id,created_at) VALUES(?,?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, p.Date, kind, method, m.Cents, m.Description, "importador-pdf", fmt.Sprintf("pdf:%s:%d", p.SHA256, index))
				if err != nil {
					return result, err
				}
			}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO cash_pdf_imports(sha256,business_date,original_name,stored_path,opening_cents,final_cents,imported_at,imported_by) VALUES(?,?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'),?)`, p.SHA256, p.Date, p.FileName, filepath.ToSlash(rel), p.OpeningCents, p.FinalCents, "interface")
		if err != nil {
			return result, err
		}
		result.Dates = append(result.Dates, p.Date)
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	result.Imported = len(ready)
	return result, nil
}
