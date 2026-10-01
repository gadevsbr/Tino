//go:build bitzpackaged

package bitz

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

//go:embed runtime/TinoBitz.exe
var scraplingBinary []byte
var runtimeMutex sync.Mutex

func scraplingCommand() (string, []string, error) {
	runtimeMutex.Lock()
	defer runtimeMutex.Unlock()
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", nil, err
	}
	digest := sha256.Sum256(scraplingBinary)
	dir := filepath.Join(cache, "Tino", "bitz-runtime", fmt.Sprintf("%x", digest[:12]))
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", nil, err
	}
	target := filepath.Join(dir, "TinoBitz.exe")
	if current, err := os.ReadFile(target); err == nil && sha256.Sum256(current) == digest {
		return target, nil, nil
	}
	tmp, err := os.CreateTemp(dir, "runtime-*.exe")
	if err != nil {
		return "", nil, err
	}
	name := tmp.Name()
	defer os.Remove(name)
	_, err = tmp.Write(scraplingBinary)
	closeErr := tmp.Close()
	if err != nil {
		return "", nil, err
	}
	if closeErr != nil {
		return "", nil, closeErr
	}
	if err = os.Rename(name, target); err != nil {
		return "", nil, err
	}
	return target, nil, nil
}
