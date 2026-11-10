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
