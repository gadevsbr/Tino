package capability

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "capabilities.json"))
	cfg := Defaults()
	cfg.WorkspaceRoot = `C:\Dados`
	cfg.Operators = []string{"5573999999999"}
	cfg.Modules[0].Enabled = false
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.WorkspaceRoot != cfg.WorkspaceRoot || got.Modules[0].Enabled || len(got.Operators) != 1 {
		t.Fatalf("configuração não persistida: %#v", got)
	}
}
