//go:build windows

package controlpanel

import (
	"path/filepath"
	"testing"
)

func TestStoreProtectsSeparateBridgeAndDashboardTokens(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "panel.json"))
	cfg := Config{Enabled: true, Endpoint: "https://api.example", DashboardURL: "https://panel.example", InstanceID: "desk"}
	if err := store.Save(cfg, "bridge-secret", "dashboard-secret"); err != nil {
		t.Fatal(err)
	}
	loaded, bridge, err := store.Load()
	if err != nil || bridge != "bridge-secret" || !loaded.PasswordSet || !loaded.DashboardTokenSet {
		t.Fatalf("ponte não preservada: cfg=%+v token=%q err=%v", loaded, bridge, err)
	}
	dashboard, err := store.DashboardToken()
	if err != nil || dashboard != "dashboard-secret" || dashboard == bridge {
		t.Fatalf("token do painel não ficou separado: token=%q err=%v", dashboard, err)
	}
}
