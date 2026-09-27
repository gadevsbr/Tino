package flow

import "testing"

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
