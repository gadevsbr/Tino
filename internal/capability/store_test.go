package capability

import (
	"path/filepath"
	"testing"
	"time"
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

func TestLoadPromotesNewlyAvailableModules(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "capabilities.json"))
	old := Defaults()
	for i := range old.Modules {
		if old.Modules[i].ID == "rooms" {
			old.Modules[i].Available = false
			old.Modules[i].Enabled = false
		}
	}
	if err := s.Save(old); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, module := range got.Modules {
		if module.ID == "rooms" && (!module.Available || !module.Enabled) {
			t.Fatalf("módulo novo não promovido: %#v", module)
		}
	}
}
func TestAIConfigurationLoadDoesNotDeadlock(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "capabilities.json"))
	done := make(chan error, 1)
	go func() { _, err := store.GetAIConfig(); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("GetAIConfig blocked")
	}
}
