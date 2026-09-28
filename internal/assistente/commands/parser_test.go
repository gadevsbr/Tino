package commands

import (
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	"testing"
)

func TestParse(t *testing.T) {
	if c := Parse("comprovantes"); c.Kind != ReceiptList || c.Date != "" {
		t.Fatalf("receipt list=%#v", c)
	}
	if c := Parse("comprovantes 21/09/2026"); c.Kind != ReceiptList || c.Date != "2026-09-21" {
		t.Fatalf("dated receipt list=%#v", c)
	}
	if c := Parse("comprovante 42"); c.Kind != ReceiptGet || c.ReceiptID != 42 {
		t.Fatalf("receipt get=%#v", c)
	}
	if c := Parse("orçamento"); c.Kind != StartQuote {
		t.Fatalf("guided quote=%#v", c)
	}
	if c := Parse("https://book.omnibees.com/hotelresults?CheckIn=19092026&CheckOut=20092026&ad=2&ch=0 5%"); c.Kind != Quote || c.Discount != 5 {
		t.Fatalf("quote=%#v", c)
	}
	c := Parse("101 está sujo")
	if c.Kind != RoomUpdate || c.Room != 101 || c.Status != rooms.Dirty {
		t.Fatalf("unexpected: %#v", c)
	}
	if c := Parse("atualizar status"); c.Kind != StartAll {
		t.Fatalf("unexpected: %#v", c)
	}
	if c := Parse("ok 101"); c.Kind != CompleteCleaning || c.Room != 101 {
		t.Fatalf("unexpected cleaning completion: %#v", c)
	}
	if c := Parse("101 amarelo 3 pessoas"); c.Kind != RoomUpdate || c.Status != rooms.Entry || c.Guests != 3 {
		t.Fatalf("unexpected guests: %#v", c)
	}
	if c := Parse("101 limpo mas desforrado"); c.Kind != RoomUpdate || c.Status != rooms.CleanUnmade {
		t.Fatalf("unexpected clean unmade: %#v", c)
	}
	if c := Parse("desforrados 101 102"); c.Kind != BatchUpdate || c.Status != rooms.CleanUnmade || len(c.Rooms) != 2 {
		t.Fatalf("unexpected clean unmade batch: %#v", c)
	}
	if c := Parse("entrada dinheiro 2100 hospedagem qto 104"); c.Kind != CashEntry || c.Cents != 210000 || c.Method != "DINHEIRO" || c.Text != "hospedagem qto 104" {
		t.Fatalf("cash entry=%#v", c)
	}
	if c := Parse("saida 50,25 material"); c.Kind != CashExit || c.Cents != 5025 {
		t.Fatalf("cash exit=%#v", c)
	}
	if c := Parse("abrir caixa 191,85"); c.Kind != OpenCash || c.Cents != 19185 {
		t.Fatalf("open=%#v", c)
	}
	if c := Parse("abrir caixa 1.774,25"); c.Kind != OpenCash || c.Cents != 177425 {
		t.Fatalf("br money=%#v", c)
	}
	if c := Parse("abrir caixa 1.774.25"); c.Kind != Unknown {
		t.Fatalf("ambiguous money=%#v", c)
	}
	if c := Parse("editar abertura caixa 20/09/2026 1.774,25"); c.Kind != CashOpeningEdit || c.Date != "2026-09-20" || c.Cents != 177425 {
		t.Fatalf("opening edit=%#v", c)
	}
	if c := Parse("relatorio caixa em pdf"); c.Kind != CashReport {
		t.Fatalf("report=%#v", c)
	}
	if c := Parse("relatorio caixa em pdf 18/09/2026"); c.Kind != CashReport || c.Date != "2026-09-18" || c.EndDate != "" {
		t.Fatalf("dated report=%#v", c)
	}
	if c := Parse("relatório caixa em pdf 18/09/2026"); c.Kind != CashReport || c.Date != "2026-09-18" {
		t.Fatalf("accented dated report=%#v", c)
	}
	if c := Parse("relatorio caixa em pdf 01/09/2026 a 30/09/2026"); c.Kind != CashReport || c.Date != "2026-09-01" || c.EndDate != "2026-09-30" {
		t.Fatalf("period report=%#v", c)
	}
	if c := Parse("relatorio caixa em pdf 01/09/2026 a 31/09/2026"); c.Kind != Unknown {
		t.Fatalf("invalid date=%#v", c)
	}
	if c := Parse("movimentos caixa"); c.Kind != CashMovements {
		t.Fatalf("movements=%#v", c)
	}
	if c := Parse("editar movimento 15 entrada pix 250 hospedagem qto 104"); c.Kind != CashEdit || c.MovementID != 15 || c.Method != "PIX" || c.Cents != 25000 || c.Text != "hospedagem qto 104" {
		t.Fatalf("edit entry=%#v", c)
	}
	if c := Parse("editar movimento 16 saida 80 compra de material"); c.Kind != CashEdit || c.MovementID != 16 || c.Method != "" || c.Cents != 8000 {
		t.Fatalf("edit exit=%#v", c)
	}
	if c := Parse("excluir movimento 15"); c.Kind != CashDelete || c.MovementID != 15 {
		t.Fatalf("delete=%#v", c)
	}
	for input, kind := range map[string]Kind{"fechar caixa": CashClose, "reabrir caixa": CashReopen, "caixa semana": CashHistoryWeek, "caixa mes": CashHistoryMonth, "backup agora": BackupNow, "status backup": BackupStatus, "diagnostico": Health} {
		if c := Parse(input); c.Kind != kind {
			t.Fatalf("%s=%#v", input, c)
		}
	}
	for input, kind := range map[string]Kind{"vale": StartAdvance, "vales mes": AdvanceReport, "relatorio vales em pdf": AdvancePDF, "pdf vales": AdvancePDF, "enviar relatorios": SendReports, "autorizar numero": AuthorizeNumber} {
		if c := Parse(input); c.Kind != kind {
			t.Fatalf("%s=%#v", input, c)
		}
	}
	if c := Parse("autorizar numero 5573988888888"); c.Kind != AuthorizeNumber || c.Text != "5573988888888" {
		t.Fatalf("authorize=%#v", c)
	}
	if c := Parse("caixa 15/08/2026"); c.Kind != CashHistoryDate || c.Date != "2026-08-15" {
		t.Fatalf("date=%#v", c)
	}
}
