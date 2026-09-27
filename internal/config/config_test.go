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
