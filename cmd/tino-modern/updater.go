package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	releaseAPI      = "https://github.com/gadevsbr/Tino/releases/latest"
	maxUpdateBytes  = 250 << 20
	updateAssetName = "Tino.exe"
	updateUserAgent = "Tino-Updater"
)

func downloadUpdate(ctx context.Context, client *http.Client, apiURL, currentVersion, dir string) (string, string, error) {
	if normalizeVersion(currentVersion) == "" {
		return "", "", nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", updateUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub respondeu HTTP %d", resp.StatusCode)
	}
	prefix := "/gadevsbr/Tino/releases/tag/"
	if resp.Request == nil || !strings.EqualFold(resp.Request.URL.Hostname(), "github.com") || !strings.HasPrefix(resp.Request.URL.Path, prefix) {
		return "", "", errors.New("GitHub não informou a release estável")
	}
	tag := strings.TrimPrefix(resp.Request.URL.Path, prefix)
	if normalizeVersion(tag) == "" || compareVersions(tag, currentVersion) <= 0 {
		return "", "", nil
	}
	downloadURL := "https://github.com/gadevsbr/Tino/releases/download/" + tag + "/" + updateAssetName
	checksumURL := downloadURL + ".sha256"
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	req.Header.Set("User-Agent", updateUserAgent)
	resp, err = client.Do(req)
	if err != nil {
		return "", "", err
	}
	checksum, readErr := io.ReadAll(io.LimitReader(resp.Body, 128))
	resp.Body.Close()
	want := strings.ToLower(strings.TrimSpace(string(checksum)))
	if readErr != nil || resp.StatusCode != http.StatusOK || len(want) != 64 {
		return "", "", errors.New("release sem SHA-256 válido")
	}
	if _, err = hex.DecodeString(want); err != nil {
		return "", "", errors.New("SHA-256 da release inválido")
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	target := filepath.Join(dir, "Tino-"+normalizeVersion(tag)+".exe")
	file, err := os.OpenFile(target+".download", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", "", err
	}
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	req.Header.Set("User-Agent", updateUserAgent)
	resp, err = client.Do(req)
	if err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", "", err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, maxUpdateBytes+1))
	closeErr := file.Close()
	resp.Body.Close()
	if copyErr != nil || closeErr != nil || resp.StatusCode != http.StatusOK || written <= 0 || written > maxUpdateBytes || (resp.ContentLength > 0 && written != resp.ContentLength) || hex.EncodeToString(hash.Sum(nil)) != want {
		os.Remove(file.Name())
		return "", "", errors.New("download da atualização não passou na verificação")
	}
	_ = os.Remove(target)
	if err = os.Rename(file.Name(), target); err != nil {
		os.Remove(file.Name())
		return "", "", err
	}
	return target, tag, nil
}

func normalizeVersion(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return ""
	}
	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return ""
		}
	}
	return value
}

func compareVersions(a, b string) int {
	a, b = normalizeVersion(a), normalizeVersion(b)
	if a == "" || b == "" {
		return 0
	}
	ap, bp := strings.Split(a, "."), strings.Split(b, ".")
	for i := range ap {
		av, _ := strconv.Atoi(ap[i])
		bv, _ := strconv.Atoi(bp[i])
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func launchUpdateHelper(downloaded, target string) error {
	current, err := os.Executable()
	if err != nil {
		return err
	}
	helper := filepath.Join(filepath.Dir(downloaded), fmt.Sprintf("TinoUpdater-%d.exe", time.Now().UnixNano()))
	if err = copyFile(current, helper); err != nil {
		return err
	}
	cmd := exec.Command(helper, "--apply-update", strconv.Itoa(os.Getpid()), target, downloaded)
	return cmd.Start()
}

func applyUpdate(args []string) error {
	if len(args) != 4 {
		return errors.New("argumentos de atualização inválidos")
	}
	pid, err := strconv.Atoi(args[1])
	if err != nil || pid <= 0 {
		return errors.New("processo de atualização inválido")
	}
	target, downloaded := args[2], args[3]
	if filepath.Ext(target) != ".exe" || filepath.Ext(downloaded) != ".exe" {
		return errors.New("executável de atualização inválido")
	}
	backup := target + ".old"
	for deadline := time.Now().Add(90 * time.Second); ; time.Sleep(500 * time.Millisecond) {
		_ = os.Remove(backup)
		err = os.Rename(target, backup)
		if err == nil || time.Now().After(deadline) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("aguardar fechamento do Tino: %w", err)
	}
	if err = copyFile(downloaded, target); err != nil {
		_ = os.Remove(target)
		_ = os.Rename(backup, target)
		return err
	}
	if err = exec.Command(target).Start(); err != nil {
		return err
	}
	_ = os.Remove(backup)
	_ = os.Remove(downloaded)
	return nil
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
