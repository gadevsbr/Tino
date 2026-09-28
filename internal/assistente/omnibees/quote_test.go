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
	for _, want := range []string{"19/09/2026 a 20/09/2026", "Duplo", "Suíte Duplo interna", "R$ 1172,83"} {
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
	if len(categories) != 3 {
		t.Fatalf("esperava 3 categorias, recebeu %d: %#v", len(categories), categories)
	}
	want := []struct {
		key   string
		cents int64
	}{{"superluxo", 150000}, {"familia", 130000}, {"deluxe_varanda", 105000}}
	for i, item := range want {
		if categories[i].Key != item.key || categories[i].TotalCents != item.cents {
			t.Fatalf("categoria %d = %#v", i, categories[i])
		}
	}
	text := Format(s, prices)
	for _, name := range []string{"Suíte Superluxo", "Suíte Família Deluxe com vista", "Suíte Triplo Deluxe com varanda"} {
		if !strings.Contains(text, name) {
			t.Fatalf("orçamento não contém %q: %s", name, text)
		}
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

func TestCategoriesKeepsEveryRoomOfferedByOmniBees(t *testing.T) {
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
	if len(categories) != 6 {
		t.Fatalf("esperava todas as 6 categorias oferecidas, recebeu %d: %#v", len(categories), categories)
	}
	for _, sourceKey := range []string{"superluxo", "quadruploVaranda", "quadruploDeluxe", "quadruploVista", "triploVaranda", "triploDeluxe"} {
		found := false
		for _, category := range categories {
			if category.SourceKey == sourceKey {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("categoria %s foi descartada: %#v", sourceKey, categories)
		}
	}
}
