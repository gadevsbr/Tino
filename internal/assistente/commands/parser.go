package commands

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/rooms"
	"github.com/gadevsbr/tino/internal/assistente/utils"
)

type Kind int

const (
	Unknown Kind = iota
	Menu
	GeneralStatus
	RoomQuery
	RoomUpdate
	StartAll
	StartSelected
	Pause
	Continue
	Skip
	ListStatus
	BatchUpdate
	History
	SetObservation
	ClearObservation
	CompleteCleaning
	OpenCash
	CashEntry
	CashExit
	CashStatus
	CashReport
	CashMovements
	CashEdit
	CashDelete
	CashClose
	CashReopen
	CashHistoryDate
	CashHistoryWeek
	CashHistoryMonth
	CashOpeningEdit
	BackupNow
	BackupStatus
	Health
	StartAdvance
	AdvanceReport
	AdvancePDF
	SendReports
	AuthorizeNumber
	Quote
	StartQuote
	ReceiptList
	ReceiptGet
)

type Command struct {
	Kind       Kind
	Room       int
	Status     rooms.Status
	Rooms      []int
	Text       string
	Guests     int
	Cents      int64
	Method     string
	MovementID int64
	Date       string
	EndDate    string
	Discount   int
	ReceiptID  int64
}

var roomNumber = regexp.MustCompile(`\b([12][0-9]{2})\b`)

func Parse(input string) Command {
	n := utils.Normalize(input)
	raw := strings.ToLower(strings.TrimSpace(input))
	if strings.Contains(raw, "book.omnibees.com/hotelresults?") {
		parts := strings.Fields(strings.TrimSpace(input))
		for _, part := range parts {
			if strings.HasPrefix(strings.ToLower(part), "https://book.omnibees.com/hotelresults?") {
				discount := 0
				if m := regexp.MustCompile(`(?i)(?:desconto\s*)?(\d{1,2})%`).FindStringSubmatch(input); len(m) > 0 {
					discount, _ = strconv.Atoi(m[1])
				}
				return Command{Kind: Quote, Text: part, Discount: discount}
			}
		}
	}
	if n == "orcamento" || n == "novo orcamento" {
		return Command{Kind: StartQuote}
	}
	if n == "caixa" || n == "caixa hoje" || n == "saldo caixa" {
		return Command{Kind: CashStatus}
	}
	if n == "comprovantes" || n == "listar comprovantes" || n == "comprovantes hoje" {
		return Command{Kind: ReceiptList}
	}
	if m := regexp.MustCompile(`^comprovantes\s+(\d{2}/\d{2}/\d{4})$`).FindStringSubmatch(raw); len(m) > 0 {
		date, err := time.Parse("02/01/2006", m[1])
		if err != nil || date.Format("02/01/2006") != m[1] {
			return Command{Kind: Unknown}
		}
		return Command{Kind: ReceiptList, Date: date.Format("2006-01-02")}
	}
	if m := regexp.MustCompile(`^comprovante\s+(\d+)$`).FindStringSubmatch(raw); len(m) > 0 {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		return Command{Kind: ReceiptGet, ReceiptID: id}
	}
	if n == "fechar caixa" {
		return Command{Kind: CashClose}
	}
	if n == "reabrir caixa" {
		return Command{Kind: CashReopen}
	}
	if m := regexp.MustCompile(`^editar abertura caixa\s+(\d{2}/\d{2}/\d{4})\s+([^\s]+)$`).FindStringSubmatch(raw); len(m) > 0 {
		date, err := time.Parse("02/01/2006", m[1])
		cents, ok := parseMoney(m[2])
		if err != nil || date.Format("02/01/2006") != m[1] || !ok || cents < 0 {
			return Command{Kind: Unknown}
		}
		return Command{Kind: CashOpeningEdit, Date: date.Format("2006-01-02"), Cents: cents}
	}
	if m := regexp.MustCompile(`^abrir caixa\s+(\d{2}/\d{2}/\d{4})\s+([^\s]+)$`).FindStringSubmatch(raw); len(m) > 0 {
		date, err := time.Parse("02/01/2006", m[1])
		cents, ok := parseMoney(m[2])
		if err != nil || date.Format("02/01/2006") != m[1] || !ok {
			return Command{Kind: Unknown}
		}
		return Command{Kind: OpenCash, Date: date.Format("2006-01-02"), Cents: cents}
	}
	if n == "caixa semana" || n == "caixa da semana" {
		return Command{Kind: CashHistoryWeek}
	}
	if n == "caixa mes" || n == "caixa do mes" {
		return Command{Kind: CashHistoryMonth}
	}
	if n == "backup agora" || n == "fazer backup" {
		return Command{Kind: BackupNow}
	}
	if n == "status backup" || n == "backup status" {
		return Command{Kind: BackupStatus}
	}
	if n == "saude" || n == "diagnostico" || n == "status bot" {
		return Command{Kind: Health}
	}
	if n == "vale" || n == "novo vale" || n == "registrar vale" {
		return Command{Kind: StartAdvance}
	}
	if n == "vales" || n == "vales mes" || n == "relatorio vales" || n == "relatorio de vales" {
		return Command{Kind: AdvanceReport}
	}
	if n == "relatorio vales em pdf" || n == "relatorio de vales em pdf" || n == "pdf vales" {
		return Command{Kind: AdvancePDF}
	}
	if strings.HasPrefix(n, "vales ") {
		return Command{Kind: AdvanceReport, Text: strings.TrimSpace(raw[len("vales "):])}
	}
	if n == "enviar relatorios" || n == "enviar relatorio" {
		return Command{Kind: SendReports}
	}
	if n == "autorizar numero" || n == "autorizar telefone" {
		return Command{Kind: AuthorizeNumber}
	}
	if m := regexp.MustCompile(`^autorizar (?:numero|telefone)\s+\+?([0-9][0-9 .()\-]{9,})$`).FindStringSubmatch(strings.ToLower(strings.TrimSpace(input))); len(m) > 0 {
		digits := regexp.MustCompile(`\D`).ReplaceAllString(m[1], "")
		return Command{Kind: AuthorizeNumber, Text: digits}
	}
	if m := regexp.MustCompile(`^caixa\s+(\d{2})/(\d{2})/(\d{4})$`).FindStringSubmatch(raw); len(m) > 0 {
		return Command{Kind: CashHistoryDate, Date: m[3] + "-" + m[2] + "-" + m[1]}
	}
	if n == "relatorio caixa" || n == "relatorio caixa em pdf" || n == "pdf caixa" {
		return Command{Kind: CashReport}
	}
	if m := regexp.MustCompile(`^(?:relat[oó]rio caixa(?: em pdf)?|pdf caixa)\s+(\d{2}/\d{2}/\d{4})(?:\s+a\s+(\d{2}/\d{2}/\d{4}))?$`).FindStringSubmatch(raw); len(m) > 0 {
		from, err := time.Parse("02/01/2006", m[1])
		if err != nil || from.Format("02/01/2006") != m[1] {
			return Command{Kind: Unknown}
		}
		cmd := Command{Kind: CashReport, Date: from.Format("2006-01-02")}
		if m[2] != "" {
			to, err := time.Parse("02/01/2006", m[2])
			if err != nil || to.Format("02/01/2006") != m[2] || to.Before(from) {
				return Command{Kind: Unknown}
			}
			cmd.EndDate = to.Format("2006-01-02")
		}
		return cmd
	}
	if n == "movimentos caixa" || n == "movimentos do caixa" || n == "listar movimentos caixa" {
		return Command{Kind: CashMovements}
	}
	if cmd, ok := parseCash(strings.ToLower(strings.TrimSpace(input))); ok {
		return cmd
	}
	switch n {
	case "menu", "ajuda":
		return Command{Kind: Menu}
	case "status":
		return Command{Kind: GeneralStatus}
	case "atualizar status de limpeza", "atualizar limpeza", "atualizar status", "atualizar quartos", "comecar atualizacao", "iniciar atualizacao":
		return Command{Kind: StartAll}
	case "atualizar alguns":
		return Command{Kind: StartSelected}
	case "parar":
		return Command{Kind: Pause}
	case "continuar":
		return Command{Kind: Continue}
	case "pular":
		return Command{Kind: Skip}
	}
	listAliases := map[string]rooms.Status{"verdes": rooms.AvailableClean, "disponiveis": rooms.AvailableClean, "desforrados": rooms.CleanUnmade, "limpos desforrados": rooms.CleanUnmade, "verdes e roxos": rooms.CleanUnmade, "laranjas": rooms.OccupiedClean, "rosas": rooms.CheckoutEntry, "amarelos": rooms.Entry, "magenta": rooms.CheckoutToday, "magentas": rooms.CheckoutToday, "vermelhos": rooms.CheckoutToday, "saidas hoje": rooms.CheckoutToday, "cinzas": rooms.Maintenance, "interditados": rooms.Maintenance, "manutencao": rooms.Maintenance, "roxos": rooms.Dirty, "sujos": rooms.Dirty}
	if status, ok := listAliases[n]; ok {
		return Command{Kind: ListStatus, Status: status}
	}
	matches := roomNumber.FindAllString(n, -1)
	if len(matches) == 0 {
		return Command{Kind: Unknown}
	}
	room, _ := strconv.Atoi(matches[0])
	if n == "ok "+matches[0] || n == matches[0]+" ok" {
		return Command{Kind: CompleteCleaning, Room: room}
	}
	if strings.HasPrefix(n, "historico ") {
		return Command{Kind: History, Room: room}
	}
	if strings.HasPrefix(n, "limpar obs ") || strings.HasPrefix(n, "remover observacao ") {
		return Command{Kind: ClearObservation, Room: room}
	}
	if strings.HasPrefix(n, "obs ") || strings.HasPrefix(n, "observacao ") {
		parts := strings.SplitN(strings.TrimSpace(input), " ", 3)
		if len(parts) == 3 {
			return Command{Kind: SetObservation, Room: room, Text: strings.TrimSpace(parts[2])}
		}
	}
	if len(matches) > 1 {
		prefix := strings.TrimSpace(strings.TrimSuffix(n, strings.Join(matches, " ")))
		batchAliases := map[string]rooms.Status{"verdes": rooms.AvailableClean, "desforrados": rooms.CleanUnmade, "verdes e roxos": rooms.CleanUnmade, "laranjas": rooms.OccupiedClean, "rosas": rooms.CheckoutEntry, "amarelos": rooms.Entry, "magentas": rooms.CheckoutToday, "vermelhos": rooms.CheckoutToday, "cinzas": rooms.Maintenance, "roxos": rooms.Dirty}
		if status, ok := batchAliases[prefix]; ok {
			numbers := make([]int, 0, len(matches))
			for _, m := range matches {
				value, _ := strconv.Atoi(m)
				numbers = append(numbers, value)
			}
			return Command{Kind: BatchUpdate, Status: status, Rooms: numbers}
		}
	}
	if n == matches[0] || n == "status "+matches[0] {
		return Command{Kind: RoomQuery, Room: room}
	}
	without := strings.TrimSpace(strings.ReplaceAll(n, matches[0], " "))
	without = strings.TrimSpace(strings.TrimPrefix(without, "coloca "))
	without = strings.TrimSpace(strings.TrimPrefix(without, "esta "))
	guestPattern := regexp.MustCompile(`\b(\d+)\s*(pessoa|pessoas|hospede|hospedes)?$`)
	guests := 0
	if guestMatch := guestPattern.FindStringSubmatch(without); len(guestMatch) > 0 {
		guests, _ = strconv.Atoi(guestMatch[1])
		without = strings.TrimSpace(strings.TrimSuffix(without, guestMatch[0]))
	}
	if status, ok := rooms.ParseStatus(without); ok {
		return Command{Kind: RoomUpdate, Room: room, Status: status, Guests: guests}
	}
	return Command{Kind: Unknown}
}

func parseCash(text string) (Command, bool) {
	text = strings.ReplaceAll(text, "r$ ", "r$")
	if m := regexp.MustCompile(`^excluir movimento\s+(\d+)$`).FindStringSubmatch(text); len(m) > 0 {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		return Command{Kind: CashDelete, MovementID: id}, true
	}
	if m := regexp.MustCompile(`^editar movimento\s+(\d+)\s+(entrada\s+(dinheiro|cartao|cartão|pix)|saida)\s+([^\s]+)(?:\s+(.*))?$`).FindStringSubmatch(text); len(m) > 0 {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		cents, ok := parseMoney(m[4])
		desc := strings.TrimSpace(m[5])
		if !ok || cents <= 0 || desc == "" {
			return Command{Kind: Unknown}, true
		}
		if strings.HasPrefix(m[2], "entrada") {
			return Command{Kind: CashEdit, MovementID: id, Method: strings.ToUpper(utils.Normalize(m[3])), Cents: cents, Text: desc}, true
		}
		return Command{Kind: CashEdit, MovementID: id, Cents: cents, Text: desc}, true
	}
	patterns := []struct {
		re     *regexp.Regexp
		kind   Kind
		method int
	}{
		{regexp.MustCompile(`^abrir caixa\s+([^\s]+)(?:\s+(.*))?$`), OpenCash, 0},
		{regexp.MustCompile(`^entrada\s+(dinheiro|cartao|cartão|pix)\s+([^\s]+)(?:\s+(.*))?$`), CashEntry, 1},
		{regexp.MustCompile(`^saida\s+([^\s]+)(?:\s+(.*))?$`), CashExit, 0},
	}
	if m := regexp.MustCompile(`^(entrada\s+(?:dinheiro|cartao|cartÃ£o|pix)|saida)\s+([^\s]+)\s+(.+?)\s+dia\s+(\d{2}/\d{2}/\d{4})$`).FindStringSubmatch(text); len(m) > 0 {
		date, err := time.Parse("02/01/2006", m[4])
		cents, ok := parseMoney(m[2])
		if err != nil || date.Format("02/01/2006") != m[4] || !ok {
			return Command{Kind: Unknown}, true
		}
		kind := CashExit
		method := ""
		if strings.HasPrefix(m[1], "entrada") {
			kind = CashEntry
			method = strings.ToUpper(utils.Normalize(strings.Fields(m[1])[1]))
		}
		return Command{Kind: kind, Method: method, Cents: cents, Text: strings.TrimSpace(m[3]), Date: date.Format("2006-01-02")}, true
	}
	for _, p := range patterns {
		m := p.re.FindStringSubmatch(text)
		if len(m) == 0 {
			continue
		}
		amountIndex := 1
		method := ""
		descIndex := 2
		if p.kind == CashEntry {
			method = strings.ToUpper(utils.Normalize(m[1]))
			amountIndex = 2
			descIndex = 3
		}
		cents, ok := parseMoney(m[amountIndex])
		if !ok {
			return Command{Kind: Unknown}, true
		}
		desc := ""
		if descIndex < len(m) {
			desc = strings.TrimSpace(m[descIndex])
		}
		if p.kind != OpenCash && desc == "" {
			return Command{Kind: Unknown}, true
		}
		return Command{Kind: p.kind, Cents: cents, Method: method, Text: desc}, true
	}
	return Command{}, false
}

func parseMoney(v string) (int64, bool) {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.TrimPrefix(v, "r$")
	v = strings.TrimSpace(v)
	if strings.Contains(v, ",") {
		if strings.Count(v, ",") != 1 {
			return 0, false
		}
		parts := strings.Split(v, ",")
		if len(parts[0]) > 3 && !regexp.MustCompile(`^\d{1,3}(\.\d{3})+$`).MatchString(parts[0]) {
			return 0, false
		}
		parts[0] = strings.ReplaceAll(parts[0], ".", "")
		v = parts[0] + "." + parts[1]
	} else if strings.Count(v, ".") > 1 {
		return 0, false
	}
	parts := strings.Split(v, ".")
	if len(parts) > 2 {
		return 0, false
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, false
	}
	cents := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) == 1 {
			parts[1] += "0"
		}
		if len(parts[1]) != 2 {
			return 0, false
		}
		cents, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, false
		}
	}
	return whole*100 + cents, true
}
