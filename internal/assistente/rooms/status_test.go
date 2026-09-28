package rooms

import "testing"

func TestAliases(t *testing.T) {
	cases := map[string]Status{"disponível e limpo": AvailableClean, "limpo mas desforrado": CleanUnmade, "verde e roxo": CleanUnmade, "ocupado com manutenção de limpeza": OccupiedClean, "check-out hoje": CheckoutToday, "em manutenção": Maintenance, "roxo": Dirty}
	for in, want := range cases {
		got, ok := ParseStatus(in)
		if !ok || got != want {
			t.Errorf("%q => %q,%v want %q", in, got, ok, want)
		}
	}
}
