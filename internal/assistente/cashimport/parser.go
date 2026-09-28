package cashimport

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

type Movement struct {
	Method, Description string
	Cents               int64
}
type Document struct {
	FileName, SHA256, Date   string
	OpeningCents, FinalCents int64
	Entries, Exits           []Movement
	Closed, Review           bool
	Warnings                 []string
}

var dateRE = regexp.MustCompile(`\b(\d{2})/(\d{2})/(\d{4})\b`)
var moneyRE = regexp.MustCompile(`(?i)R\$\s*([0-9][0-9.,]*)`)

type piece struct {
	x, y, w, size float64
	text          string
}
type row []piece

func Parse(filename string, data []byte) (Document, error) {
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return Document{}, errors.New("arquivo não é PDF")
	}
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Document{}, err
	}
	if r.NumPage() != 1 {
		return Document{}, errors.New("relatório de caixa deve ter uma página")
	}
	content := r.Page(1).Content()
	if len(content.Text) == 0 {
		return Document{}, errors.New("PDF sem texto selecionável")
	}
	items := make([]piece, 0, len(content.Text))
	for _, t := range content.Text {
		if s := strings.TrimSpace(t.S); s != "" {
			items = append(items, piece{x: t.X, y: t.Y, w: t.W, size: t.FontSize, text: s})
		}
	}
	rows := groupRows(items)
	plainText, err := r.Page(1).GetPlainText(nil)
	if err != nil {
		return Document{}, fmt.Errorf("extrair texto do PDF: %w", err)
	}
	plain := normalize(plainText)
	match := dateRE.FindStringSubmatch(strings.ReplaceAll(filename, "-", "/"))
	if len(match) == 0 {
		match = dateRE.FindStringSubmatch(plain)
	}
	if len(match) == 0 {
		return Document{}, errors.New("data do caixa não encontrada")
	}
	date, err := time.Parse("02/01/2006", match[0])
	if err != nil {
		return Document{}, err
	}
	doc := Document{FileName: filename, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Date: date.Format("2006-01-02"), Closed: strings.Contains(plain, "FECHADO")}
	doc.OpeningCents = findLabeledMoneyText(plainText, "CAIXA INICIAL")
	doc.FinalCents = findLabeledMoneyText(plainText, "CAIXA FINAL")
	entryHeader, exitHeader, entryEnd := -1, -1, -1
	for i, r := range rows {
		u := strings.TrimSpace(normalize(rowText(r)))
		compact := strings.ReplaceAll(u, " ", "")
		switch {
		case entryHeader < 0 && strings.Contains(compact, "ENTRADAS"):
			entryHeader = i
		case exitHeader < 0 && strings.Contains(compact, "SAIDAS"):
			exitHeader = i
		case strings.Contains(u, "TOTAL ENTRADAS") || strings.Contains(u, "DINHEIRO:") && strings.Contains(u, "PIX:"):
			if entryEnd < 0 {
				entryEnd = i
			}
		}
	}
	if entryHeader < 0 || exitHeader < 0 || exitHeader <= entryHeader {
		return Document{}, errors.New("seções de entradas e saídas não encontradas")
	}
	if entryEnd < entryHeader || entryEnd > exitHeader {
		entryEnd = exitHeader
	}
	modern := strings.Contains(plain, "CAIXA -")
	doc.Entries = parseEntries(rows[entryHeader+1:entryEnd], modern)
	doc.Exits = parseExits(rows[exitHeader+1:], modern)
	if doc.OpeningCents == 0 {
		doc.Warnings = append(doc.Warnings, "saldo inicial zerado ou não identificado")
	}
	var in, out int64
	for _, m := range doc.Entries {
		in += m.Cents
	}
	for _, m := range doc.Exits {
		out += m.Cents
	}
	calculated := doc.OpeningCents + in - out
	if calculated != doc.FinalCents {
		doc.Review = true
		doc.Warnings = append(doc.Warnings, fmt.Sprintf("diferença: calculado R$ %.2f, PDF R$ %.2f", float64(calculated)/100, float64(doc.FinalCents)/100))
	}
	if len(doc.Entries) == 0 && len(doc.Exits) == 0 {
		doc.Warnings = append(doc.Warnings, "dia sem movimentos")
	}
	return doc, nil
}

func groupRows(items []piece) []row {
	sort.Slice(items, func(i, j int) bool {
		if absf(items[i].y-items[j].y) < 1 {
			return items[i].x < items[j].x
		}
		return items[i].y > items[j].y
	})
	var rows []row
	for _, it := range items {
		placed := false
		for i := range rows {
			if absf(rows[i][0].y-it.y) < 1 {
				rows[i] = append(rows[i], it)
				placed = true
				break
			}
		}
		if !placed {
			rows = append(rows, row{it})
		}
	}
	for i := range rows {
		sort.Slice(rows[i], func(a, b int) bool { return rows[i][a].x < rows[i][b].x })
	}
	return rows
}
func joinRows(rows []row) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(rowText(r))
		b.WriteByte('\n')
	}
	return b.String()
}
func rowText(r row) string {
	var b strings.Builder
	for i, p := range r {
		if i > 0 {
			previous := r[i-1]
			gap := p.x - (previous.x + previous.w)
			if gap > maxf(1, previous.size*0.3) {
				b.WriteByte(' ')
			}
		}
		b.WriteString(p.text)
	}
	return b.String()
}
func findLabeledMoneyText(text, label string) int64 {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(normalize(line), label) {
			all := moneyRE.FindAllStringSubmatch(line, -1)
			if len(all) > 0 {
				return parseMoney(all[len(all)-1][1])
			}
		}
	}
	return 0
}
func parseEntries(rows []row, modern bool) []Movement {
	var out []Movement
	first, second := 240.0, 430.0
	if modern {
		first, second = 200, 376
	}
	for _, r := range rows {
		cells := []row{{}, {}, {}}
		for _, p := range r {
			i := 0
			if p.x >= first {
				i = 1
			}
			if p.x >= second {
				i = 2
			}
			cells[i] = append(cells[i], p)
		}
		for i, c := range cells {
			if m, ok := movement(rowText(c)); ok {
				m.Method = []string{"DINHEIRO", "CARTAO", "PIX"}[i]
				out = append(out, m)
			}
		}
	}
	return out
}
func parseExits(rows []row, modern bool) []Movement {
	var out []Movement
	middle := 250.0
	if modern {
		middle = 297.5
	}
	for _, r := range rows {
		u := strings.ReplaceAll(normalize(rowText(r)), " ", "")
		if strings.Contains(u, "CAIXAFINAL") || strings.Contains(u, "GERADOEM") || strings.Contains(u, "DINHEIRO:") {
			continue
		}
		halves := []row{{}, {}}
		for _, p := range r {
			i := 0
			if p.x >= middle {
				i = 1
			}
			halves[i] = append(halves[i], p)
		}
		for _, c := range halves {
			if m, ok := movement(rowText(c)); ok {
				m.Method = ""
				out = append(out, m)
			}
		}
	}
	return out
}
func movement(text string) (Movement, bool) {
	text = strings.TrimSpace(text)
	m := moneyRE.FindStringSubmatch(text)
	if len(m) == 0 {
		return Movement{}, false
	}
	value := parseMoney(m[1])
	if value <= 0 {
		return Movement{}, false
	}
	desc := strings.TrimSpace(moneyRE.ReplaceAllString(text, ""))
	desc = strings.Trim(desc, " -/|")
	desc = strings.Join(strings.Fields(desc), " ")
	if desc == "" {
		desc = "Movimento importado"
	}
	return Movement{Description: desc, Cents: value}, true
}
func parseMoney(v string) int64 {
	v = strings.TrimSpace(v)
	last := strings.LastIndexAny(v, ".,")
	if last < 0 {
		n, _ := strconv.ParseInt(digits(v), 10, 64)
		return n * 100
	}
	whole := digits(v[:last])
	fraction := digits(v[last+1:])
	if strings.Count(v, ".")+strings.Count(v, ",") == 1 && len(fraction) == 3 {
		n, _ := strconv.ParseInt(digits(v), 10, 64)
		return n * 100
	}
	if len(fraction) != 2 {
		all := digits(v)
		if len(all) < 3 {
			return 0
		}
		whole = all[:len(all)-2]
		fraction = all[len(all)-2:]
	}
	w, _ := strconv.ParseInt(whole, 10, 64)
	f, _ := strconv.ParseInt(fraction, 10, 64)
	return w*100 + f
}
func digits(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func normalize(v string) string {
	v = strings.ToUpper(v)
	return strings.NewReplacer("Á", "A", "À", "A", "Â", "A", "Ã", "A", "É", "E", "Ê", "E", "Í", "I", "Ó", "O", "Ô", "O", "Õ", "O", "Ú", "U", "Ç", "C").Replace(v)
}
