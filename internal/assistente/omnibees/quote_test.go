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
	for _, want := range []string{"19/09/2026 a 20/09/2026", "Duplo", "Suíte interna", "R$ 1172,83"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if _, err := ExtractPrices(`<html>sem preços</html>`); err != ErrNoPrices {
		t.Fatalf("err=%v", err)
	}
}

func TestCourtesyUsesPhysicalOccupancy(t *testing.T) {
	s, err := ParseLink(strings.Replace(sampleURL, "ch=0&ag=", "ch=2&ag=7%3B10", 1), 0)
	if err != nil {
		t.Fatal(err)
	}
	text := Format(s, map[string]int64{normalize(roomNames["quadruploVista"]): 90000})
	if !strings.Contains(text, "Triplo + 01 cortesia infantil") || !strings.Contains(text, "Suíte Deluxe com vista para o mar") {
		t.Fatal(text)
	}
}
