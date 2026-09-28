package utils

import "testing"

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{"  SAÍDA,  HOJE!\n": "saida hoje", "Manutenção": "manutencao", "VERDE": "verde"} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q)=%q want %q", in, got, want)
		}
	}
}
