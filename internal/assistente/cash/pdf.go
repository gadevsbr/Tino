package cash

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

func PDF(day Day, location *time.Location) ([]byte, error) {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetCompression(false)
	p.SetMargins(12, 10, 12)
	writeDay(p, day, location)
	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RangePDF keeps the daily cash layout, one page per recorded day.
func RangePDF(days []Day, location *time.Location) ([]byte, error) {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetCompression(false)
	p.SetMargins(12, 10, 12)
	if len(days) > 1 {
		writeRangeSummary(p, days)
	}
	for _, day := range days {
		writeDay(p, day, location)
	}
	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeRangeSummary(p *fpdf.Fpdf, days []Day) {
	p.AddPage()
	tr := p.UnicodeTranslatorFromDescriptor("")
	p.SetFont("Arial", "B", 14)
	p.CellFormat(186, 12, tr("RESUMO DO CAIXA - "+formatDate(days[0].Date)+" a "+formatDate(days[len(days)-1].Date)), "", 1, "L", false, 0, "")
	p.SetFont("Arial", "B", 8)
	for _, heading := range []string{"DATA", "ENTRADAS", "SAÍDAS", "CAIXA FINAL"} {
		p.CellFormat(46.5, 8, tr(heading), "1", 0, "C", false, 0, "")
	}
	p.Ln(-1)
	var entries, exits, final, cashTotal, cardTotal, pixTotal int64
	p.SetFont("Arial", "", 8)
	for _, day := range days {
		entries += day.EntryTotal()
		exits += day.ExitTotal()
		final += day.FinalCents()
		cashTotal += day.FinalMethodTotal("DINHEIRO")
		cardTotal += day.FinalMethodTotal("CARTAO")
		pixTotal += day.FinalMethodTotal("PIX")
		for _, value := range []string{formatDate(day.Date), Money(day.EntryTotal()), Money(day.ExitTotal()), Money(day.FinalCents())} {
			p.CellFormat(46.5, 7, tr(value), "1", 0, "C", false, 0, "")
		}
		p.Ln(-1)
	}
	p.SetFont("Arial", "B", 8)
	for _, value := range []string{"TOTAL", Money(entries), Money(exits), Money(final)} {
		p.CellFormat(46.5, 8, tr(value), "1", 0, "C", false, 0, "")
	}
	p.Ln(12)
	p.CellFormat(186, 7, tr(fmt.Sprintf("Fechamentos por método: Dinheiro %s | Cartão %s | PIX %s", Money(cashTotal), Money(cardTotal), Money(pixTotal))), "", 1, "L", false, 0, "")
}

func writeDay(p *fpdf.Fpdf, day Day, location *time.Location) {
	p.AddPage()
	tr := p.UnicodeTranslatorFromDescriptor("")
	p.SetFont("Arial", "B", 11)
	p.SetTextColor(220, 0, 0)
	title := "CAIXA - " + formatDate(day.Date)
	if day.ClosedAt != "" {
		title += " - FECHADO"
	}
	p.CellFormat(186, 6, tr(title), "", 1, "L", false, 0, "")
	p.SetTextColor(30, 80, 150)
	p.SetFont("Arial", "", 9)
	p.CellFormat(186, 5, tr("CAIXA INICIAL: "+Money(day.OpeningCents)), "", 1, "L", false, 0, "")
	p.Ln(2)
	p.SetTextColor(30, 130, 40)
	p.SetFont("Arial", "B", 9)
	p.CellFormat(186, 6, "ENTRADAS", "", 1, "C", false, 0, "")
	columns := []string{"DINHEIRO", "CARTÃO", "PIX"}
	widths := []float64{62, 62, 62}
	p.SetTextColor(0, 0, 0)
	p.SetFont("Arial", "B", 8)
	for i, h := range columns {
		p.CellFormat(widths[i], 7, tr(h), "1", 0, "C", false, 0, "")
	}
	p.Ln(-1)
	byMethod := map[string][]Movement{}
	for _, m := range day.Entries {
		byMethod[m.Method] = append(byMethod[m.Method], m)
	}
	rows := 1
	for _, method := range columns {
		key := method
		if key == "CARTÃO" {
			key = "CARTAO"
		}
		if len(byMethod[key]) > rows {
			rows = len(byMethod[key])
		}
	}
	if rows < 10 {
		rows = 10
	}
	p.SetFont("Arial", "", 7)
	for row := 0; row < rows; row++ {
		for i, method := range columns {
			key := method
			if key == "CARTÃO" {
				key = "CARTAO"
			}
			text := ""
			if row < len(byMethod[key]) {
				m := byMethod[key][row]
				text = movementText(m)
			}
			p.CellFormat(widths[i], 7, tr(text), "1", 0, "L", false, 0, "")
		}
		p.Ln(-1)
	}
	p.SetFont("Arial", "B", 7)
	p.CellFormat(186, 6, tr(fmt.Sprintf("DINHEIRO: %s   |   CARTÃO: %s   |   PIX: %s   |   TOTAL ENTRADAS: %s", Money(day.MethodTotal("DINHEIRO")), Money(day.MethodTotal("CARTAO")), Money(day.MethodTotal("PIX")), Money(day.EntryTotal()))), "1", 1, "C", false, 0, "")
	p.Ln(4)
	p.SetTextColor(220, 0, 0)
	p.SetFont("Arial", "B", 9)
	p.CellFormat(186, 6, tr("SAÍDAS"), "", 1, "C", false, 0, "")
	p.SetTextColor(0, 0, 0)
	p.SetFont("Arial", "", 7)
	exitRows := len(day.Exits)
	if exitRows < 6 {
		exitRows = 6
	}
	for i := 0; i < exitRows; i++ {
		text := ""
		if i < len(day.Exits) {
			text = movementText(day.Exits[i])
		}
		p.CellFormat(186, 7, tr(text), "1", 1, "L", false, 0, "")
	}
	p.SetTextColor(220, 0, 0)
	p.SetFont("Arial", "B", 9)
	p.CellFormat(186, 7, tr("CAIXA FINAL: "+Money(day.FinalCents())), "", 1, "L", false, 0, "")
	p.SetTextColor(0, 0, 0)
	p.SetFont("Arial", "B", 7)
	p.CellFormat(186, 6, tr(fmt.Sprintf("DINHEIRO: %s   |   CARTÃO: %s   |   PIX: %s", Money(day.FinalMethodTotal("DINHEIRO")), Money(day.FinalMethodTotal("CARTAO")), Money(day.FinalMethodTotal("PIX")))), "1", 1, "C", false, 0, "")
	p.SetTextColor(80, 80, 80)
	p.SetFont("Arial", "I", 7)
	p.CellFormat(186, 5, tr("Gerado em "+time.Now().In(location).Format("02/01/2006 às 15:04")), "", 1, "R", false, 0, "")
}

func formatDate(v string) string {
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return v
	}
	return t.Format("02/01/2006")
}
func movementText(m Movement) string { return fmt.Sprintf("%s - %s", Money(m.Cents), m.Description) }
