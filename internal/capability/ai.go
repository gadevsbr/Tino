package capability

import (
	"os"
	"strings"
)

func (s *Store) AIToken() (string, error) {
	b, err := os.ReadFile(s.path + ".ai-secret")
	if err != nil {
		return "", err
	}
	return unprotect(strings.TrimSpace(string(b)))
}

func (s *Store) SaveAIToken(token string) error {
	cipher, err := protect(token)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path+".ai-secret", []byte(cipher), 0600)
}
