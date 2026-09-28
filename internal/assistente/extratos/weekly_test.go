package extratos

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestWeeklyPeriodAndWorkbook(t *testing.T) {
	p, recognized, err := ParseWeeklyCommand("extrato semana 1 setembro 2026", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC))
	if err != nil || !recognized {
		t.Fatalf("parse: %v %t", err, recognized)
	}
	if _, err := p.WithDates("02/09/2026", "09/09/2026"); err == nil {
		t.Fatal("accepted eight days")
	}
	p, err = p.WithDates("02/09/2026", "08/09/2026")
	if err != nil {
		t.Fatal(err)
	}
	if !p.ContainsStay("01/09/2026", "03/09/2026") {
		t.Fatal("crossing stay excluded")
	}
	stays := []Stay{
		{ID: "1", Room: "104", Name: "TESTE UM", CheckIn: "01/09/2026", CheckOut: "03/09/2026", Package: 10000, Cash: 10000, Total: 10000},
		{ID: "2", Room: "105", Name: "TESTE DOIS", CheckIn: "09/09/2026", CheckOut: "10/09/2026", Package: 20000, Pix: 20000, Total: 20000},
	}
	data, audit, results, err := WeeklyWorkbook(p, stays)
	if err != nil {
		t.Fatal(err)
	}
	if len(audit) == 0 || len(results) != 2 || results[0].Status != "OK" || results[1].Status != "FORA DO PERÍODO" {
		t.Fatalf("results=%+v audit=%d", results, len(audit))
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) != 2 || sheets[0] != "Hospedagens" || sheets[1] != "Resumo" {
		t.Fatalf("sheets=%v", sheets)
	}
	for cell, want := range map[string]string{"A1": "QUARTO", "B1": "DATA ENTRADA", "J1": "OBSERVAÇÕES", "A2": "104", "D2": "TESTE UM"} {
		got, err := f.GetCellValue("Hospedagens", cell)
		if err != nil || got != want {
			t.Fatalf("%s=%q want %q err=%v", cell, got, want, err)
		}
	}
	if got, _ := f.GetCellValue("Hospedagens", "K1"); got != "" {
		t.Fatalf("extra column: %q", got)
	}
}
