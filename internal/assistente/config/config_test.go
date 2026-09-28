package config

import "testing"

func TestBrazilianNinthDigitAuthorization(t *testing.T) {
	cfg := Config{Authorized: map[string]struct{}{"5511987654321": {}}}
	if !cfg.IsAuthorized("551187654321") {
		t.Fatal("legacy Brazilian number should match configured ninth-digit form")
	}
	if cfg.IsAuthorized("551187654322") {
		t.Fatal("different suffix must not be authorized")
	}
	if cfg.IsAuthorized("5521987654321") {
		t.Fatal("different area code must not be authorized")
	}
}
