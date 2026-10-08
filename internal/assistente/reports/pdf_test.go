package reports

import (
	"bytes"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	"strings"
	"testing"
	"time"
)

func TestPDFHasOneLandscapePageAnd42Rooms(t *testing.T) {
	list := make([]rooms.Room, 42)
	for i, n := range rooms.OfficialNumbers {
		list[i] = rooms.Room{Number: n, Status: rooms.AvailableClean}
	}
	list[0] = rooms.Room{Number: 101, Status: rooms.Entry, GuestCount: 3}
	list[1] = rooms.Room{Number: 102, Status: rooms.CleanUnmade}
	data, err := New(time.UTC).Generate(list, time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatal("invalid PDF")
	}
	for _, needle := range [][]byte{[]byte("/MediaBox [0 0 841.89 595.28]"), []byte("%%EOF")} {
		if !bytes.Contains(data, needle) {
			t.Fatalf("missing %s", needle)
		}
	}
	if got := roomFormatting(list[0]); got != "ENTRADA | 3 pessoa(s)" {
		t.Fatalf("guest formatting=%q", got)
	}
}

func TestSummaryListsEntryCheckoutAndTurnoverRoomNumbers(t *testing.T) {
	list := []rooms.Room{
		{Number: 101, Status: rooms.Entry},
		{Number: 102, Status: rooms.CheckoutToday},
		{Number: 103, Status: rooms.CheckoutEntry},
		{Number: 104, Status: rooms.AvailableClean},
	}
	got := Summary(list, time.Now(), time.UTC)
	for _, want := range []string{"QUARTOS DE ENTRADA (1)\n101", "QUARTOS DE SAÍDA (1)\n102", "QUARTOS DE SAÍDA E ENTRADA (1)\n103"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q: %s", want, got)
		}
	}
	got = Summary(nil, time.Now(), time.UTC)
	for _, want := range []string{"QUARTOS DE ENTRADA (0)\nNenhum quarto.", "QUARTOS DE SAÍDA (0)\nNenhum quarto.", "QUARTOS DE SAÍDA E ENTRADA (0)\nNenhum quarto."} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q: %s", want, got)
		}
	}
}

func TestSummaryIncludesStatusAndGuestTotals(t *testing.T) {
	list := []rooms.Room{
		{Number: 101, Status: rooms.Entry, GuestCount: 3},
		{Number: 102, Status: rooms.OccupiedClean, GuestCount: 2},
		{Number: 103, Status: rooms.AvailableClean},
		{Number: 104, Status: rooms.CleanUnmade},
		{Number: 105, Status: rooms.Dirty},
		{Number: 106, Status: rooms.Dirty},
	}
	got := Summary(list, time.Date(2026, 8, 14, 9, 30, 0, 0, time.UTC), time.UTC)
	for _, want := range []string{"RESUMO DO RELATÓRIO", "Limpos, mas desforrados: 1", "Entrada: 1 quartos — 👥 3 pessoas", "Manutenção de limpeza: 1 quartos — 👥 2 pessoas", "QUARTOS SUJOS / PARA LIMPAR (2)\n105, 106", "QUARTOS LIMPOS, MAS DESFORRADOS (1)\n104", "Total: 6 quartos"} {
		if !bytes.Contains([]byte(got), []byte(want)) {
			t.Fatalf("summary missing %q: %s", want, got)
		}
	}
}
