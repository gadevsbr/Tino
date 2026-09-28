package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Manager struct {
	db        *sql.DB
	dataDir   string
	dir       string
	retention int
	location  *time.Location
}

type Status struct{ Path, CreatedAt string }

func New(db *sql.DB, dataDir string, retention int, location *time.Location) *Manager {
	return &Manager{db: db, dataDir: dataDir, dir: filepath.Join(dataDir, "backups"), retention: retention, location: location}
}

func (m *Manager) Run(ctx context.Context, now time.Time) (string, error) {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return "", err
	}
	baseName := "hotel-" + now.In(m.location).Format("2006-01-02-150405.000")
	dbPath := filepath.Join(m.dir, baseName+".db")
	path := filepath.Join(m.dir, baseName+".zip")
	query := "VACUUM INTO '" + strings.ReplaceAll(dbPath, "'", "''") + "'"
	if _, err := m.db.ExecContext(ctx, query); err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	if err := Verify(dbPath); err != nil {
		_ = os.Remove(dbPath)
		return "", err
	}
	if err := createArchive(path, dbPath, map[string]string{
		"comprovantes":      filepath.Join(m.dataDir, "comprovantes"),
		"extratos-semanais": filepath.Join(m.dataDir, "extratos-semanais"),
	}); err != nil {
		_ = os.Remove(dbPath)
		_ = os.Remove(path)
		return "", err
	}
	_ = os.Remove(dbPath)
	if err := Verify(path); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", err
	}
	if _, err := m.db.ExecContext(ctx, `INSERT INTO backups(path,created_at) VALUES (?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, path); err != nil {
		return "", err
	}
	_ = m.prune(now)
	return path, nil
}

func Verify(path string) error {
	if strings.EqualFold(filepath.Ext(path), ".zip") {
		reader, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		defer reader.Close()
		for _, file := range reader.File {
			if file.Name != "hotel.db" {
				continue
			}
			source, err := file.Open()
			if err != nil {
				return err
			}
			defer source.Close()
			temp, err := os.CreateTemp("", "hotel-backup-*.db")
			if err != nil {
				return err
			}
			tempPath := temp.Name()
			defer os.Remove(tempPath)
			if _, err = io.Copy(temp, source); err != nil {
				temp.Close()
				return err
			}
			if err = temp.Close(); err != nil {
				return err
			}
			return Verify(tempPath)
		}
		return errors.New("backup archive does not contain hotel.db")
	}
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integrity check: %s", result)
	}
	return nil
}

func createArchive(path, dbPath string, directories map[string]string) error {
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(output)
	closeWithError := func(current error) error {
		zipErr := archive.Close()
		fileErr := output.Close()
		if current != nil {
			return current
		}
		if zipErr != nil {
			return zipErr
		}
		return fileErr
	}
	if err := addArchiveFile(archive, dbPath, "hotel.db"); err != nil {
		return closeWithError(err)
	}
	for archiveRoot, sourceRoot := range directories {
		err := filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(sourceRoot, sourcePath)
			if err != nil {
				return err
			}
			return addArchiveFile(archive, sourcePath, filepath.ToSlash(filepath.Join(archiveRoot, relative)))
		})
		if err != nil && !os.IsNotExist(err) {
			return closeWithError(err)
		}
	}
	return closeWithError(nil)
}

func addArchiveFile(archive *zip.Writer, sourcePath, archiveName string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = archiveName
	header.Method = zip.Deflate
	destination, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(destination, source)
	return err
}

func (m *Manager) Latest(ctx context.Context) (Status, error) {
	var s Status
	err := m.db.QueryRowContext(ctx, `SELECT path,created_at FROM backups ORDER BY id DESC LIMIT 1`).Scan(&s.Path, &s.CreatedAt)
	return s, err
}

func (m *Manager) prune(now time.Time) error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return err
	}
	cutoff := now.AddDate(0, 0, -m.retention)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(m.dir, e.Name()))
		}
	}
	return nil
}
