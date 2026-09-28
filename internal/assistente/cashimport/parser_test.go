package cashimport

import "testing"

func TestParseMoneyLegacyFormats(t *testing.T){
	tests:=map[string]int64{"2.206.80":220680,"2.250":225000,"400.00":40000,"1.050,00":105000,"128,80":12880,"410":41000}
	for raw,want:=range tests{if got:=parseMoney(raw);got!=want{t.Errorf("parseMoney(%q)=%d, want %d",raw,got,want)}}
}
