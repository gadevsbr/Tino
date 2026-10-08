//go:build !windows

package capability

import "errors"

func protect(string) (string, error) {
	return "", errors.New("proteção de credenciais Bitz requer Windows")
}
func unprotect(string) (string, error) {
	return "", errors.New("proteção de credenciais Bitz requer Windows")
}
