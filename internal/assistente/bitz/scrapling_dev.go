//go:build !bitzpackaged

package bitz

import (
	"errors"
	"os"
	"path/filepath"
)

func scraplingCommand() (string, []string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", nil, err
	}
	for {
		python := filepath.Join(root, "data", "bitz-runtime", "Scripts", "python.exe")
		script := filepath.Join(root, "internal", "assistente", "bitz", "scrapling", "runner.py")
		if _, err := os.Stat(python); err == nil {
			if _, err := os.Stat(script); err == nil {
				return python, []string{script}, nil
			}
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}
	return "", nil, errors.New("runtime Scrapling ausente; execute scripts/build-bitz-runtime.ps1")
}
