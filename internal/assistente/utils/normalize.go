package utils

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// Normalize makes user commands deterministic without changing stored content.
func Normalize(value string) string {
	decomposed := norm.NFD.String(strings.ToLower(strings.TrimSpace(value)))
	var b strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(nonWord.ReplaceAllString(b.String(), " "))
}
