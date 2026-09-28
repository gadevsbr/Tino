package extratos

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var monthNames = []string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

type Period struct {
	Week  int       `json:"week"`
	Month int       `json:"month"`
	Year  int       `json:"year"`
	From  time.Time `json:"-"`
	To    time.Time `json:"-"`
}

func ParseWeeklyCommand(text string, now time.Time) (Period, bool, error) {
	parts := strings.Fields(normalize(text))
	if len(parts) < 2 || (parts[0] != "EXTRATO" && parts[0] != "EXTRATOS") || parts[1] != "SEMANA" {
		return Period{}, false, nil
	}
	if len(parts) != 4 && len(parts) != 5 {
		return Period{}, true, fmt.Errorf("use `extrato semana 1 setembro` ou `extrato semana 1 setembro 2026`")
	}
	week, err := strconv.Atoi(parts[2])
	if err != nil || week < 1 || week > 6 {
		return Period{}, true, fmt.Errorf("semana deve ser de 1 a 6")
	}
	month := 0
	for i, name := range monthNames {
		if normalize(name) == parts[3] {
			month = i + 1
			break
		}
	}
	if month == 0 {
		return Period{}, true, fmt.Errorf("mês inválido; escreva o nome, como setembro")
	}
	year := now.Year()
	if len(parts) == 5 {
		year, err = strconv.Atoi(parts[4])
		if err != nil || year < 2000 || year > 2100 {
			return Period{}, true, fmt.Errorf("ano inválido")
		}
	}
	return Period{Week: week, Month: month, Year: year}, true, nil
}

func (p Period) WithDates(start, end string) (Period, error) {
	from, err := time.Parse("02/01/2006", strings.TrimSpace(start))
	if err != nil || from.Format("02/01/2006") != strings.TrimSpace(start) {
		return Period{}, fmt.Errorf("data inicial inválida; use DD/MM/AAAA")
	}
	to, err := time.Parse("02/01/2006", strings.TrimSpace(end))
	if err != nil || to.Format("02/01/2006") != strings.TrimSpace(end) {
		return Period{}, fmt.Errorf("data final inválida; use DD/MM/AAAA")
	}
	if int(from.Month()) != p.Month || from.Year() != p.Year {
		return Period{}, fmt.Errorf("a data inicial deve estar em %s/%d", monthNames[p.Month-1], p.Year)
	}
	if to.Before(from) || to.Sub(from) >= 7*24*time.Hour {
		return Period{}, fmt.Errorf("a semana deve ter de 1 a 7 dias, contando as duas datas")
	}
	p.From, p.To = from, to
	return p, nil
}

func (p Period) Label() string {
	return fmt.Sprintf("Semana %d de %s/%d (%s a %s)", p.Week, monthNames[p.Month-1], p.Year, p.From.Format("02/01/2006"), p.To.Format("02/01/2006"))
}
func MonthName(month int) string {
	if month < 1 || month > 12 {
		return ""
	}
	return monthNames[month-1]
}
func (p Period) Filename() string {
	return fmt.Sprintf("extrato_semana_%d_%s_%d.xlsx", p.Week, monthNames[p.Month-1], p.Year)
}
func (p Period) ContainsStay(entry, exit string) bool {
	from, e1 := time.Parse("02/01/2006", strings.TrimSpace(entry))
	to, e2 := time.Parse("02/01/2006", strings.TrimSpace(exit))
	return e1 == nil && e2 == nil && !from.After(p.To) && !to.Before(p.From)
}
