package authorization

import (
	"context"
	"database/sql"
	"regexp"
	"sort"
	"sync"

	"github.com/gadevsbr/tino/internal/assistente/config"
)

type Service struct {
	cfg     config.Config
	db      *sql.DB
	mu      sync.RWMutex
	dynamic map[string]struct{}
}

func New(cfg config.Config, db *sql.DB) (*Service, error) {
	s := &Service{cfg: cfg, db: db, dynamic: map[string]struct{}{}}
	rows, err := db.Query(`SELECT number FROM authorized_numbers ORDER BY number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var number string
		if err := rows.Scan(&number); err != nil {
			return nil, err
		}
		s.dynamic[number] = struct{}{}
	}
	return s, rows.Err()
}

func digits(value string) string { return regexp.MustCompile(`\D`).ReplaceAllString(value, "") }

func (s *Service) Allowed(number string) bool {
	if s.cfg.IsAuthorized(number) {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for candidate := range s.dynamic {
		if config.SamePhoneNumber(candidate, number) {
			return true
		}
	}
	return false
}

func (s *Service) Add(ctx context.Context, actor, number string) (bool, error) {
	number = digits(number)
	if s.Allowed(number) {
		return false, nil
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO authorized_numbers(number,authorized_by,created_at) VALUES (?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, number, actor); err != nil {
		return false, err
	}
	s.mu.Lock()
	s.dynamic[number] = struct{}{}
	s.mu.Unlock()
	return true, nil
}

func (s *Service) Numbers() []string {
	all := map[string]struct{}{}
	for number := range s.cfg.Authorized {
		all[number] = struct{}{}
	}
	s.mu.RLock()
	for number := range s.dynamic {
		all[number] = struct{}{}
	}
	s.mu.RUnlock()
	result := make([]string, 0, len(all))
	for number := range all {
		result = append(result, number)
	}
	sort.Strings(result)
	return result
}
