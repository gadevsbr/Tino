package extratos

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type weeklyAudit struct {
	Period  Period   `json:"periodo"`
	Basis   string   `json:"criterio"`
	Stays   []Stay   `json:"extratos"`
	Results []Result `json:"resultados"`
}

// WeeklyWorkbook creates a new spreadsheet; it never needs or modifies a template.
func WeeklyWorkbook(period Period, stays []Stay) ([]byte, []byte, []Result, error) {
	if len(stays) == 0 {
		return nil, nil, nil, fmt.Errorf("nenhum PDF recebido")
	}
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Hospedagens"); err != nil {
		return nil, nil, nil, err
	}
	if _, err := f.NewSheet("Resumo"); err != nil {
		return nil, nil, nil, err
	}
	if err := f.SetCellValue("Resumo", "A1", "EXTRATOS BITZ — "+period.Label()); err != nil {
		return nil, nil, nil, err
	}
	_ = f.SetCellValue("Resumo", "A2", "Período definido manualmente. Hospedagens que cruzam o período são incluídas.")
	_ = f.SetCellValue("Resumo", "A3", "Itens CONFERIR e FORA DO PERÍODO não entram nos totais confirmados.")
	for col, value := range []string{"STATUS", "QUANTIDADE", "PACOTE", "CONSUMO", "DINHEIRO", "PIX", "CARTÃO", "TOTAL HOSPEDAGEM"} {
		c, _ := excelize.CoordinatesToCellName(col+1, 5)
		_ = f.SetCellValue("Resumo", c, value)
	}
	headers := []string{"QUARTO", "DATA ENTRADA", "DATA SAÍDA", "NOME", "VALOR PACOTE", "VALOR CONSUMO", "PAG. DINHEIRO", "PAG. PIX", "PAG. CARTÃO", "OBSERVAÇÕES"}
	for i, name := range headers {
		c, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue("Hospedagens", c, name)
	}
	type totals struct {
		count                                             int
		packageCents, consumption, cash, pix, card, total int64
	}
	aggregates := map[string]*totals{}
	results := make([]Result, 0, len(stays))
	seen := map[string]bool{}
	for i, stay := range stays {
		row := i + 2
		key := stay.ID
		if key == "" {
			key = normalize(stay.Room + " " + stay.Name + " " + stay.CheckIn + " " + stay.CheckOut)
		}
		status := "OK"
		switch {
		case seen[key]:
			status = "DUPLICADO"
		case stay.Review:
			status = "CONFERIR"
		case stay.CheckIn == "" || stay.CheckOut == "":
			status = "CONFERIR"
		case !period.ContainsStay(stay.CheckIn, stay.CheckOut):
			status = "FORA DO PERÍODO"
		}
		seen[key] = true
		notes := strings.Join(stay.Notes, "; ")
		if status == "FORA DO PERÍODO" {
			notes = appendNote(notes, "Hospedagem fora de "+period.Label())
		}
		if status == "DUPLICADO" {
			notes = appendNote(notes, "Extrato de hospedagem duplicado no lote")
		}
		if status != "OK" {
			notes = appendNote(notes, status)
		}
		var room any = stay.Room
		if numericRoom, err := strconv.Atoi(stay.Room); err == nil {
			room = numericRoom
		}
		var checkIn, checkOut any = stay.CheckIn, stay.CheckOut
		if parsed, err := time.Parse("02/01/2006", stay.CheckIn); err == nil {
			checkIn = parsed
		}
		if parsed, err := time.Parse("02/01/2006", stay.CheckOut); err == nil {
			checkOut = parsed
		}
		values := []any{room, checkIn, checkOut, stay.Name, moneyValue(stay.Package), moneyValue(stay.Consumption), moneyValue(stay.Cash), moneyValue(stay.Pix), moneyValue(stay.Card), notes}
		for col, value := range values {
			if err := setCell(f, "Hospedagens", col+1, row, value); err != nil {
				return nil, nil, nil, err
			}
		}
		results = append(results, Result{File: stay.File, Status: status, Row: row, Detail: "UH " + stay.Room + "; saída " + stay.CheckOut})
		t := aggregates[status]
		if t == nil {
			t = &totals{}
			aggregates[status] = t
		}
		t.count++
		t.packageCents += stay.Package
		t.consumption += stay.Consumption
		t.cash += stay.Cash
		t.pix += stay.Pix
		t.card += stay.Card
		t.total += stay.Total
	}
	for i, status := range []string{"OK", "CONFERIR", "FORA DO PERÍODO", "DUPLICADO"} {
		row := i + 6
		t := aggregates[status]
		if t == nil {
			t = &totals{}
		}
		for col, value := range []any{status, t.count, moneyValue(t.packageCents), moneyValue(t.consumption), moneyValue(t.cash), moneyValue(t.pix), moneyValue(t.card), moneyValue(t.total)} {
			if err := setCell(f, "Resumo", col+1, row, value); err != nil {
				return nil, nil, nil, err
			}
		}
	}
	format := `"R$" #,##0.00`
	moneyStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &format})
	if err != nil {
		return nil, nil, nil, err
	}
	dateFormat := "dd/mm/yyyy"
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &dateFormat})
	if err != nil {
		return nil, nil, nil, err
	}
	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1}})
	if err != nil {
		return nil, nil, nil, err
	}
	_ = f.SetCellStyle("Hospedagens", "A1", "J1", headerStyle)
	_ = f.SetCellStyle("Resumo", "A5", "H5", headerStyle)
	if len(stays) > 0 {
		_ = f.SetCellStyle("Hospedagens", "E2", fmt.Sprintf("I%d", len(stays)+1), moneyStyle)
		_ = f.SetCellStyle("Hospedagens", "B2", fmt.Sprintf("C%d", len(stays)+1), dateStyle)
	}
	_ = f.SetCellStyle("Resumo", "C6", "H9", moneyStyle)
	for column, width := range map[string]float64{"A": 14.75, "B": 19.38, "C": 17, "D": 34, "E": 19.13, "F": 21.38, "G": 16.13, "H": 14.63, "I": 18, "J": 50} {
		_ = f.SetColWidth("Hospedagens", column, column, width)
	}
	_ = f.SetColWidth("Resumo", "A", "A", 24)
	_ = f.SetColWidth("Resumo", "B", "H", 18)
	_ = f.SetPanes("Hospedagens", &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	_ = f.AutoFilter("Hospedagens", fmt.Sprintf("A1:J%d", len(stays)+1), nil)
	buffer, err := f.WriteToBuffer()
	if err != nil {
		return nil, nil, nil, err
	}
	audit, err := json.MarshalIndent(weeklyAudit{Period: period, Basis: "sobreposição com período manual", Stays: stays, Results: results}, "", "  ")
	if err != nil {
		return nil, nil, nil, err
	}
	return buffer.Bytes(), audit, results, nil
}
