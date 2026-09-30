package omnibees

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveQuoteWhenConfigured(t *testing.T) {
	link := os.Getenv("OMNIBEES_TEST_URL")
	if link == "" {
		t.Skip("live URL not configured")
	}
	search, err := ParseLink(link, 0)
	if err != nil {
		t.Fatal(err)
	}
	text, err := Fetch(context.Background(), search)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
	if !strings.Contains(text, "R$") {
		t.Fatal("no real prices in quote")
	}
}

const sampleURL = "https://book.omnibees.com/hotelresults?c=9224&q=17134&currencyId=16&NRooms=1&CheckIn=19092026&CheckOut=20092026&ad=2&ch=0&ag="

func TestParseLinkAndOccupancy(t *testing.T) {
	s, err := ParseLink(sampleURL, 5)
	if err != nil || s.Adults != 2 || s.Discount != 5 {
		t.Fatalf("search=%#v err=%v", s, err)
	}
	if _, err := ParseLink("https://evil.example/hotelresults?CheckIn=19092026&CheckOut=20092026&ad=2&ch=0", 0); err == nil {
		t.Fatal("accepted external host")
	}
	if _, err := ParseLink(strings.Replace(sampleURL, "20092026", "19092026", 1), 0); err == nil {
		t.Fatal("accepted zero nights")
	}
	if _, err := ParseLink(strings.Replace(sampleURL, "ch=0", "ch=1", 1), 0); err == nil {
		t.Fatal("accepted missing child age")
	}
}

func TestNewSearchBuildsValidatedHotelLink(t *testing.T) {
	s, err := NewSearch(mustDate(t, "19/09/2026"), mustDate(t, "20/09/2026"), 2, []int{7, 10})
	if err != nil || s.URL.Query().Get("ag") != "7;10" || s.Children != 2 {
		t.Fatalf("search=%#v err=%v", s, err)
	}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	d, err := time.Parse("02/01/2006", value)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestExtractAndFormat(t *testing.T) {
	page := `<html><div data-room-name="Quarto Duplo"><div><span class="price-total-bold">R$ 1.234,56</span></div></div></html>`
	prices, err := ExtractPrices(page)
	if err != nil || prices[normalize("Quarto Duplo")] != 123456 {
		t.Fatalf("prices=%v err=%v", prices, err)
	}
	s, _ := ParseLink(sampleURL, 5)
	text := Format(s, prices)
	for _, want := range []string{"19/09/2026 a 20/09/2026", "Duplo", "Suíte Duplo interna", "R$ 1.172,83"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if _, err := ExtractPrices(`<html>sem preços</html>`); err != ErrNoPrices {
		t.Fatalf("err=%v", err)
	}
}

func TestExtractAndFormatAllAvailableCategoryNameVariants(t *testing.T) {
	page := `<html>
	<div data-room-name="Suíte Super Luxo com Varanda e Vista para o Mar"><span class="price-total-bold">R$ 1.500,00</span></div>
	<div data-room-name="Suíte Família Deluxe Vista Mar"><span class="price-total-bold">R$ 1.300,00</span></div>
	<div data-room-name="Quarto Triplo Deluxe Varanda"><span class="price-total-bold">R$ 1.100,00</span></div>
	<div data-room-name="Quarto Triplo Deluxe Varanda"><span class="price-total-bold">R$ 1.050,00</span></div>
	</html>`
	prices, err := ExtractPrices(page)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := ParseLink(sampleURL, 0)
	categories := Categories(s, prices)
	if len(categories) != 1 || categories[0].Key != "superluxo" || categories[0].TotalCents != 150000 {
		t.Fatalf("duplo incorreto: %#v", categories)
	}
	triplo := Categories(Search{Adults: 3}, prices)
	if len(triplo) != 1 || triplo[0].SourceKey != "triploDeluxe" || triplo[0].TotalCents != 105000 {
		t.Fatalf("triplo incorreto: %#v", triplo)
	}
	familia := Categories(Search{Adults: 5}, prices)
	if len(familia) != 1 || familia[0].SourceKey != "familia" || familia[0].TotalCents != 130000 {
		t.Fatalf("família incorreta: %#v", familia)
	}
	text := Format(s, prices)
	if !strings.Contains(text, "Suíte Superluxo") || strings.Contains(text, "Triplo") || strings.Contains(text, "Família") {
		t.Fatalf("orçamento duplo misturou capacidades: %s", text)
	}
}

func TestCourtesyUsesPhysicalOccupancy(t *testing.T) {
	s, err := ParseLink(strings.Replace(sampleURL, "ch=0&ag=", "ch=2&ag=7%3B10", 1), 0)
	if err != nil {
		t.Fatal(err)
	}
	text := Format(s, map[string]int64{normalize(roomNames["quadruploVista"]): 90000})
	if !strings.Contains(text, "Triplo + 01 cortesia infantil") || !strings.Contains(text, "Suíte Quádruplo Deluxe com varanda e vista mar") {
		t.Fatal(text)
	}
}

func TestCategoriesKeepsOnlyExactCapacityOfferedByOmniBees(t *testing.T) {
	s, err := ParseLink(sampleURL, 0)
	if err != nil {
		t.Fatal(err)
	}
	prices := map[string]int64{
		normalize(roomNames["superluxo"]):        223554,
		normalize(roomNames["quadruploVaranda"]): 130406,
		normalize(roomNames["quadruploDeluxe"]):  158351,
		normalize(roomNames["quadruploVista"]):   180708,
		normalize(roomNames["triploVaranda"]):    124818,
		normalize(roomNames["triploDeluxe"]):     152762,
	}
	categories := Categories(s, prices)
	if len(categories) != 1 || categories[0].SourceKey != "superluxo" {
		t.Fatalf("orçamento para duas pessoas deve conter somente opções duplas: %#v", categories)
	}
}

func TestFormatBRLUsesBrazilianThousandsSeparator(t *testing.T) {
	for _, tc := range []struct {
		cents int64
		want  string
	}{
		{0, "R$ 0,00"},
		{99999, "R$ 999,99"},
		{250420, "R$ 2.504,20"},
		{123456789, "R$ 1.234.567,89"},
	} {
		if got := formatBRL(tc.cents); got != tc.want {
			t.Fatalf("formatBRL(%d)=%q, want %q", tc.cents, got, tc.want)
		}
	}
}

func TestCategoriesAcceptsOmniBeesLabelVariantsWithoutMixingCapacity(t *testing.T) {
	prices := map[string]int64{
		normalize("Suíte Duplo Deluxe com Varanda"):              250420,
		normalize("Apartamento Duplo Standard Interno"):          180000,
		normalize("Suíte Triplo Super Deluxe com Varanda"):       310000,
		normalize("Suíte Quádruplo Deluxe com Vista para o Mar"): 400000,
	}
	duplos := Categories(Search{Adults: 2}, prices)
	if len(duplos) != 2 || duplos[0].SourceKey != "duploDeluxe" || duplos[0].TotalCents != 250420 || duplos[1].SourceKey != "duplo" {
		t.Fatalf("duplos=%#v", duplos)
	}
	triplos := Categories(Search{Adults: 3}, prices)
	if len(triplos) != 1 || triplos[0].SourceKey != "triploDeluxe" {
		t.Fatalf("triplos=%#v", triplos)
	}
	quadruplos := Categories(Search{Adults: 4}, prices)
	if len(quadruplos) != 1 || quadruplos[0].SourceKey != "quadruploVista" {
		t.Fatalf("quadruplos=%#v", quadruplos)
	}
}

func TestCategoriesMatchesFiveObservedOmniBeesOccupancies(t *testing.T) {
	// Snapshot of the room labels observed in the five live searches supplied by
	// the operator. OmniBees may advertise larger rooms in the HTML, so each
	// quote must retain every available option of the requested capacity only.
	prices := map[string]int64{
		normalize("Quarto Duplo"):                                    132908,
		normalize("Quarto Duplo Superluxo com Varanda e Vista Mar"):  257240,
		normalize("Quarto Triplo"):                                   174116,
		normalize("Quarto Triplo com Varanda"):                       179476,
		normalize("Quarto Triplo Deluxe com Varanda"):                219656,
		normalize("Quarto Quadruplo com Varanda"):                    187512,
		normalize("Quarto Quadruplo Deluxe com Varanda"):             227692,
		normalize("Quarto Quadruplo Deluxe com Varanda e Vista Mar"): 259836,
		normalize("Quarto Familia Deluxe com Vista Mar"):             308000,
	}
	cases := []struct {
		name       string
		adults     int
		children   int
		ages       []int
		wantSource []string
	}{
		{"dois adultos", 2, 0, nil, []string{"superluxo", "duplo"}},
		{"tres adultos", 3, 0, nil, []string{"triploDeluxe", "triploVaranda", "triplo"}},
		{"dois adultos e duas criancas", 2, 2, []int{5, 10}, []string{"quadruploVista", "quadruploDeluxe", "quadruploVaranda"}},
		{"dois adultos, uma cortesia e uma crianca", 2, 2, []int{1, 10}, []string{"quadruploVista", "quadruploDeluxe", "quadruploVaranda"}},
		{"tres adultos e duas criancas", 3, 2, []int{5, 10}, []string{"familia"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Categories(Search{Adults: tc.adults, Children: tc.children, Ages: tc.ages}, prices)
			if len(got) != len(tc.wantSource) {
				t.Fatalf("categorias=%#v, esperava %v", got, tc.wantSource)
			}
			for i, want := range tc.wantSource {
				if got[i].SourceKey != want {
					t.Fatalf("categoria[%d]=%q, esperava %q; todas=%#v", i, got[i].SourceKey, want, got)
				}
			}
		})
	}
}
