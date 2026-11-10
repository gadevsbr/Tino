package bitz

import (
	"strings"
	"testing"
)

func TestScraplingErrorExplainsMissingOperationalCPF(t *testing.T) {
	err := scraplingError(ScraplingResult{Stage: "identity", Code: "cpf_not_resolved"})
	if !strings.Contains(err.Error(), "CPF operacional não localizado") {
		t.Fatal(err)
	}
}

func TestScraplingErrorExplainsUnavailableRooms(t *testing.T) {
	for _, code := range []string{"no_rooms_available", "category_unavailable"} {
		err := scraplingError(ScraplingResult{Stage: "rooms_scan", Code: code})
		if !strings.Contains(err.Error(), "nenhuma UH compatível") {
			t.Fatalf("code=%s err=%v", code, err)
		}
	}
}
