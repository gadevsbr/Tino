//go:build !windows

package controlpanel

import "errors"

func protect(string) (string, error)   { return "", errors.New("proteção do token requer Windows") }
func unprotect(string) (string, error) { return "", errors.New("proteção do token requer Windows") }
