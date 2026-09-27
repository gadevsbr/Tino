package flow

import (
	"path/filepath"
	"testing"
)

func TestMatchFirstRuleAndDefault(t *testing.T) {
	e := Engine{def: Definition{Rules: []Rule{{Contains: "HORÁRIO", Reply: "aberto"}}, DefaultReply: "menu"}}
	if got := e.match("Qual o horário?"); got != "aberto" {
		t.Fatalf("got %q", got)
	}
	if got := e.match("outra pergunta"); got != "menu" {
		t.Fatalf("got %q", got)
	}
}

func TestMatchCaseSensitive(t *testing.T) {
	e := Engine{def: Definition{Rules: []Rule{{Contains: "VIP", Reply: "ok", CaseSensitive: true}}}}
	if got := e.match("vip"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestSaveLoadAndMatchDefinition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flows.yaml")
	want := Definition{Rules: []Rule{{Name: "Preço", Contains: "valor", Reply: "Veja a tabela."}}, DefaultReply: "Posso ajudar?"}
	if err := SaveDefinition(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rules) != 1 || got.Rules[0].Name != "Preço" {
		t.Fatalf("fluxo inesperado: %#v", got)
	}
	if reply := Match(got, "Qual o VALOR?"); reply != "Veja a tabela." {
		t.Fatalf("resposta: %q", reply)
	}
	if reply := Match(got, "Olá"); reply != "Posso ajudar?" {
		t.Fatalf("fallback: %q", reply)
	}
}

func TestValidateRequiresFriendlyRuleFields(t *testing.T) {
	err := Validate(Definition{Rules: []Rule{{Name: "", Contains: "oi", Reply: "olá"}}})
	if err == nil {
		t.Fatal("esperava erro de validação")
	}
}
