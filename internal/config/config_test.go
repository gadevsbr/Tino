package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("data_dir: state\nprofile: corp\nbatch:\n  min_interval: 2s\n  max_interval: 5s\n  max_per_run: 8\nflow:\n  rules_file: flow.yaml\n")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Batch.MinInterval != 2*time.Second || c.Batch.MaxPerRun != 8 {
		t.Fatalf("config inesperada: %#v", c)
	}
}
func TestDesktopDataUsesPersistentUserDirectory(t *testing.T) {
	base := t.TempDir()
	userConfig := t.TempDir()
	t.Setenv("APPDATA", userConfig)
	t.Setenv("XDG_CONFIG_HOME", userConfig)
	cfg, err := LoadDesktop(base)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != filepath.Join(userConfig, "Tino", "data") || cfg.Flow.RulesFile != filepath.Join(base, "config", "flows.yaml") {
		t.Fatalf("wrong paths: %#v", cfg)
	}
}

func TestDesktopKeepsConfiguredRelativeDataWithExecutable(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "config", "config.yaml"), []byte("data_dir: state\nprofile: default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadDesktop(base)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != filepath.Join(base, "state") {
		t.Fatalf("wrong configured data path: %s", cfg.DataDir)
	}
}
