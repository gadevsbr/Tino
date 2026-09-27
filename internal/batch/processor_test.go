package batch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCSVRequiresConsentAndNormalizesPhone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contacts.csv")
	if err := os.WriteFile(path, []byte("phone,name,message,consent\n+55 (11) 99999-9999,Ana,Oi,sim\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := LoadCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Phone != "5511999999999" || !items[0].Consented {
		t.Fatalf("item inesperado: %#v", items)
	}
}

func TestLoadCSVRejectsMissingConsentColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.csv")
	if err := os.WriteFile(path, []byte("phone,message\n5511,Oi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCSV(path); err == nil {
		t.Fatal("esperava erro")
	}
}
