package omnibees

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

var ErrNoPrices = errors.New("nenhum preço total confirmado na OmniBees")

var roomNames = map[string]string{
	"superluxo":        "Quarto Duplo Superluxo com Varanda e Vista Mar",
	"familia":          "Quarto Família Deluxe com Vista Mar",
	"triploDeluxe":     "Quarto Triplo Deluxe com Varanda",
	"quadruploDeluxe":  "Quarto Quadruplo Deluxe com Varanda",
	"quadruploVista":   "Quarto Quadruplo Deluxe com Varanda e Vista Mar",
	"triploVaranda":    "Quarto Triplo com Varanda",
	"triplo":           "Quarto Triplo",
	"quadruploVaranda": "Quarto Quádruplo Com Varanda",
	"duplo":            "Quarto Duplo",
}

type Search struct {
	URL      *url.URL
	CheckIn  time.Time
	CheckOut time.Time
	Adults   int
	Children int
	Ages     []int
	Discount int
}

func ParseLink(raw string, discount int) (Search, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Hostname(), "book.omnibees.com") || u.User != nil || u.Port() != "" || u.Path != "/hotelresults" {
		return Search{}, errors.New("envie um link HTTPS de resultados da OmniBees")
	}
	q := u.Query()
	if q.Get("q") != "17134" || q.Get("c") != "9224" || q.Get("currencyId") != "16" || q.Get("NRooms") != "1" {
		return Search{}, errors.New("o link precisa ser da busca do Hotel Paraíso Tropical, em reais e para um quarto")
	}
	checkIn, err := time.Parse("02012006", q.Get("CheckIn"))
	if err != nil || checkIn.Format("02012006") != q.Get("CheckIn") {
		return Search{}, errors.New("CheckIn inválido no link")
	}
	checkOut, err := time.Parse("02012006", q.Get("CheckOut"))
	if err != nil || checkOut.Format("02012006") != q.Get("CheckOut") || !checkOut.After(checkIn) {
		return Search{}, errors.New("CheckOut inválido no link")
	}
	adults, err := strconv.Atoi(q.Get("ad"))
	if err != nil || adults < 1 || adults > 5 {
		return Search{}, errors.New("quantidade de adultos inválida no link")
	}
	children, err := strconv.Atoi(q.Get("ch"))
	if err != nil || children < 0 || children > 4 || adults+children > 5 {
		return Search{}, errors.New("quantidade de crianças inválida no link")
	}
	ages := []int{}
	if strings.TrimSpace(q.Get("ag")) != "" {
		for _, item := range regexp.MustCompile(`[;,\s]+`).Split(strings.TrimSpace(q.Get("ag")), -1) {
			if item == "" {
				continue
			}
			age, err := strconv.Atoi(item)
			if err != nil || age < 0 || age > 17 {
				return Search{}, errors.New("idade infantil inválida no link")
			}
			ages = append(ages, age)
		}
	}
	if len(ages) != children {
		return Search{}, errors.New("o link não informa a idade de todas as crianças")
	}
	if discount < 0 || discount > 8 {
		return Search{}, errors.New("desconto deve ser de 0% a 8%")
	}
	return Search{URL: u, CheckIn: checkIn, CheckOut: checkOut, Adults: adults, Children: children, Ages: ages, Discount: discount}, nil
}

func Fetch(ctx context.Context, search Search) (string, error) {
	result, err := FetchResult(ctx, search)
	return result.Text, err
}

// FetchResult keeps category identities alongside the same verified totals used
// by the operator quote. Callers must never reconstruct categories from prose.
func FetchResult(ctx context.Context, search Search) (Result, error) {
	prices, err := fetchPrices(ctx, search)
	if err != nil {
		return Result{}, err
	}
	return Result{Text: Format(search, prices), Categories: Categories(search, prices)}, nil
}

func fetchPrices(ctx context.Context, search Search) (map[string]int64, error) {
	if search.URL == nil {
		return nil, errors.New("URL de orçamento ausente")
	}
	validated, err := ParseLink(search.URL.String(), search.Discount)
	if err != nil {
		return nil, err
	}
	if validated.Adults != search.Adults || validated.Children != search.Children || !validated.CheckIn.Equal(search.CheckIn) || !validated.CheckOut.Equal(search.CheckOut) || fmt.Sprint(validated.Ages) != fmt.Sprint(search.Ages) {
		return nil, errors.New("dados do orçamento divergem da URL")
	}
	client := &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || req.URL.Scheme != "https" || !strings.EqualFold(req.URL.Hostname(), "book.omnibees.com") {
			return errors.New("redirecionamento fora da OmniBees")
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, search.URL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AssistenteParaiso/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OmniBees respondeu HTTP %d", resp.StatusCode)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "html") {
		return nil, errors.New("resposta da OmniBees não é HTML")
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, (5<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(page) > 5<<20 {
		return nil, errors.New("resposta da OmniBees maior que o limite de segurança")
	}
	prices, err := ExtractPrices(string(page))
	if err != nil {
		return nil, err
	}
	return prices, nil
}

func ExtractPrices(page string) (map[string]int64, error) {
	root, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil, err
	}
	prices := map[string]int64{}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			name := attr(n, "data-room-name")
			if name != "" && !hidden(n) {
				if total := findClass(n, "price-total-bold"); total != nil {
					if cents, ok := parseBRL(textContent(total)); ok {
						key := normalize(name)
						if current, exists := prices[key]; !exists || cents < current {
							prices[key] = cents
						}
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	if len(prices) == 0 {
		return nil, ErrNoPrices
	}
	return prices, nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
func findClass(n *html.Node, class string) *html.Node {
	if n.Type == html.ElementNode && hasClass(n, class) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findClass(c, class); found != nil {
			return found
		}
	}
	return nil
}
func hasClass(n *html.Node, class string) bool {
	for _, item := range strings.Fields(attr(n, "class")) {
		if item == class {
			return true
		}
	}
	return false
}
func hidden(n *html.Node) bool {
	for current := n; current != nil; current = current.Parent {
		if current.Type != html.ElementNode {
			continue
		}
		style := strings.ToLower(strings.ReplaceAll(attr(current, "style"), " ", ""))
		if strings.Contains(style, "display:none") || hasAttr(current, "hidden") || strings.EqualFold(attr(current, "aria-hidden"), "true") || hasClass(current, "d-none") || hasClass(current, "hidden") {
			return true
		}
	}
	return false
}
func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
func parseBRL(value string) (int64, bool) {
	m := regexp.MustCompile(`R\$\s*([\d.]+),(\d{2})`).FindStringSubmatch(value)
	if len(m) == 0 {
		return 0, false
	}
	whole, err := strconv.ParseInt(strings.ReplaceAll(m[1], ".", ""), 10, 64)
	if err != nil {
		return 0, false
	}
	frac, _ := strconv.ParseInt(m[2], 10, 64)
	return whole*100 + frac, true
}
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c")
	return strings.Join(strings.Fields(replacer.Replace(s)), " ")
}

func canonicalRoomName(s string) string {
	s = normalize(s)
	s = strings.NewReplacer("super luxo", "superluxo", "vista para o mar", "vista mar").Replace(s)
	if strings.Contains(s, "superluxo") {
		s = strings.ReplaceAll(s, "duplo", "")
	}
	ignored := map[string]bool{"quarto": true, "suite": true, "com": true, "e": true, "de": true, "do": true, "da": true, "para": true, "o": true, "a": true}
	fields := strings.Fields(s)
	kept := fields[:0]
	for _, field := range fields {
		if !ignored[field] {
			kept = append(kept, field)
		}
	}
	return strings.Join(kept, " ")
}

func roomPrice(prices map[string]int64, sourceKey string) (int64, bool) {
	wanted := canonicalRoomName(roomNames[sourceKey])
	for name, cents := range prices {
		if canonicalRoomName(name) == wanted && cents > 0 {
			return cents, true
		}
	}
	return 0, false
}

func Format(s Search, prices map[string]int64) string {
	courtesy := 0
	for _, age := range s.Ages {
		if age <= 7 {
			courtesy++
		}
	}
	paying := s.Adults + s.Children - courtesy
	configuration := map[int]string{1: "Solo", 2: "Duplo", 3: "Triplo", 4: "Quádruplo", 5: "Quíntuplo"}[paying]
	if courtesy == 1 {
		configuration += " + 01 cortesia infantil"
	} else if courtesy > 1 {
		configuration += fmt.Sprintf(" + %02d cortesias infantis", courtesy)
	}
	lines := []string{}
	for _, category := range Categories(s, prices) {
		lines = append(lines, fmt.Sprintf("• %s: %s", category.Name, formatBRL(category.TotalCents)))
	}
	if len(lines) == 0 {
		lines = []string{"Nenhuma suíte disponível foi localizada."}
	}
	nights := int(s.CheckOut.Sub(s.CheckIn).Hours() / 24)
	nightLabel := "noites"
	if nights == 1 {
		nightLabel = "noite"
	}
	return fmt.Sprintf("Período: %s a %s (%d %s)\nRegime: Café da manhã\nConfiguração: %s\n\nSuítes disponíveis\n%s\n\nOrçamento válido por 24 horas", s.CheckIn.Format("02/01/2006"), s.CheckOut.Format("02/01/2006"), nights, nightLabel, configuration, strings.Join(lines, "\n"))
}

func formatBRL(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "." + whole[i:]
	}
	return fmt.Sprintf("%sR$ %s,%02d", sign, whole, cents%100)
}

type Category struct {
	Key        string `json:"key"`
	SourceKey  string `json:"source_key"`
	Name       string `json:"name"`
	TotalCents int64  `json:"total_cents"`
}

type Result struct {
	Text       string
	Categories []Category
}

// Categories mirrors the commercial slots used by the original browser
// extension. For up to three occupants, OmniBees reuses some physically larger
// room cards as commercial alternatives for the requested occupancy. Four and
// five occupants have their own exclusive sets. Classification remains
// semantic because OmniBees changes labels over time.
func Categories(s Search, prices map[string]int64) []Category {
	occupants := s.Adults + s.Children
	allowed := commercialSlots(occupants)
	bySource := map[string]Category{}
	for rawName, cents := range prices {
		category, _, ok := classifyRoom(rawName)
		displayName, wanted := allowed[category.SourceKey]
		if !ok || !wanted || cents <= 0 {
			continue
		}
		cents = (cents*int64(100-s.Discount) + 50) / 100
		category.Name = displayName
		category.TotalCents = cents
		if current, exists := bySource[category.SourceKey]; !exists || cents < current.TotalCents {
			bySource[category.SourceKey] = category
		}
	}
	order := map[string]int{"superluxo": 0, "familia": 1, "quadruploVista": 1, "triploDeluxe": 2, "quadruploDeluxe": 2, "triploVaranda": 3, "quadruploVaranda": 3, "duplo": 4, "triplo": 4}
	result := make([]Category, 0, len(bySource))
	for _, category := range bySource {
		result = append(result, category)
	}
	sort.Slice(result, func(i, j int) bool { return order[result[i].SourceKey] < order[result[j].SourceKey] })
	return result
}

func commercialSlots(occupants int) map[string]string {
	if occupants >= 5 {
		return map[string]string{"familia": roomNames["familia"]}
	}
	if occupants == 4 {
		return map[string]string{
			"quadruploVista":   "Suíte Deluxe com vista para o mar",
			"quadruploDeluxe":  "Suíte Deluxe com varanda",
			"quadruploVaranda": "Suíte com varanda",
		}
	}
	internal := "duplo"
	if occupants >= 3 {
		internal = "triplo"
	}
	return map[string]string{
		"superluxo":     "Suíte Superluxo com varanda e vista mar",
		"familia":       "Suíte Deluxe com vista para o mar",
		"triploDeluxe":  "Suíte Deluxe com varanda",
		"triploVaranda": "Suíte com varanda",
		internal:        "Suíte interna",
	}
}

func classifyRoom(rawName string) (Category, int, bool) {
	name := canonicalRoomName(rawName)
	has := func(value string) bool { return strings.Contains(name, value) }
	capacity := 0
	switch {
	case has("familia"):
		capacity = 5
	case has("quadruplo"):
		capacity = 4
	case has("triplo"):
		capacity = 3
	case has("duplo") || has("superluxo"):
		capacity = 2
	default:
		return Category{}, 0, false
	}
	if capacity == 5 {
		return Category{Key: "familia", SourceKey: "familia", Name: "Suíte Família Deluxe com vista mar"}, capacity, true
	}
	if has("superluxo") {
		return Category{Key: "superluxo", SourceKey: "superluxo", Name: "Suíte Superluxo com varanda e vista mar"}, capacity, true
	}
	prefix := map[int]string{2: "Duplo", 3: "Triplo", 4: "Quádruplo"}[capacity]
	sourcePrefix := map[int]string{2: "duplo", 3: "triplo", 4: "quadruplo"}[capacity]
	switch {
	case has("deluxe") && has("vista mar"):
		return Category{Key: "deluxe_vista_mar", SourceKey: sourcePrefix + "Vista", Name: "Suíte " + prefix + " Deluxe com varanda e vista mar"}, capacity, true
	case has("deluxe"):
		return Category{Key: "deluxe_varanda", SourceKey: sourcePrefix + "Deluxe", Name: "Suíte " + prefix + " Deluxe com varanda"}, capacity, true
	case has("varanda"):
		return Category{Key: "varanda", SourceKey: sourcePrefix + "Varanda", Name: "Suíte " + prefix + " com varanda"}, capacity, true
	default:
		return Category{Key: "interna", SourceKey: sourcePrefix, Name: "Suíte " + prefix + " interna"}, capacity, true
	}
}
