package omnibees

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NewSearch uses the same hotel and URL validation as operator-supplied links.
func NewSearch(checkIn, checkOut time.Time, adults int, ages []int) (Search, error) {
	q := url.Values{
		"c": {"9224"}, "q": {"17134"}, "currencyId": {"16"},
		"lang": {"pt-BR"}, "NRooms": {"1"}, "version": {"4"},
		"CheckIn": {checkIn.Format("02012006")}, "CheckOut": {checkOut.Format("02012006")},
		"ad": {strconv.Itoa(adults)}, "ch": {strconv.Itoa(len(ages))},
	}
	parts := make([]string, len(ages))
	for i, age := range ages {
		parts[i] = strconv.Itoa(age)
	}
	q.Set("ag", strings.Join(parts, ";"))
	return ParseLink((&url.URL{Scheme: "https", Host: "book.omnibees.com", Path: "/hotelresults", RawQuery: q.Encode()}).String(), 0)
}
