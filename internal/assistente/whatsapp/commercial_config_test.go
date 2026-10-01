package whatsapp

import (
	"testing"

	"github.com/gadevsbr/tino/internal/assistente/commercial"
)

func TestCommercialModeCommandDoesNotReplaceUIFields(t *testing.T) {
	current := commercial.Config{Mode: commercial.Public, GroupPhone: "5573988240413", FinalMessage1: "mensagem um", FinalMessage2: "mensagem dois"}
	current = commercialModeConfig(current, "comercial teste 5573999999999")
	if current.Mode != commercial.Test || current.GroupPhone != "5573988240413" || current.FinalMessage1 != "mensagem um" || current.FinalMessage2 != "mensagem dois" {
		t.Fatal(current)
	}
}
