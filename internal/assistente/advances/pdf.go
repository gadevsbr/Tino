package advances

import (
	"bytes"
	"fmt"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/cash"
	"github.com/go-pdf/fpdf"
)

// PDF creates the monthly employee-advances report in A4 landscape.
func PDF(items []Advance, month, employee string, location *time.Location) ([]byte, error) {
	p := fpdf.New("L", "mm", "A4", "")
	p.SetCompression(false)
	p.SetMargins(10, 10, 10)
	p.AddPage()
	tr := p.UnicodeTranslatorFromDescriptor("")
	p.SetFont("Arial", "B", 15)
	title := "VALES DE FUNCIONÁRIOS - " + month
	if employee != "" {
		title += " - " + employee
	}
	p.CellFormat(277, 9, tr(title), "", 1, "L", false, 0, "")
	p.SetFont("Arial", "", 8)
	p.CellFormat(277, 6, tr("Gerado em "+time.Now().In(location).Format("02/01/2006 às 15:04")), "", 1, "L", false, 0, "")
	p.Ln(3)
	widths := []float64{27, 48, 32, 112, 36, 22}
	for i, h := range []string{"DATA", "FUNCIONÁRIO", "VALOR", "OBSERVAÇÃO", "CAIXA", "ID"} {
		p.SetFont("Arial", "B", 8)
		p.CellFormat(widths[i], 8, tr(h), "1", 0, "C", false, 0, "")
	}
	p.Ln(-1)
	p.SetFont("Arial", "", 8)
	var total int64
	for _, item := range items {
		total += item.Cents
		cashText := "não descontado"
		if item.DeductCash {
			cashText = "dinheiro"
		}
		values := []string{formatAdvanceDate(item.Date), item.Employee, cash.Money(item.Cents), item.Note, cashText, fmt.Sprintf("%d", item.ID)}
		for i, value := range values {
			p.CellFormat(widths[i], 7, tr(value), "1", 0, "L", false, 0, "")
		}
		p.Ln(-1)
	}
	p.SetFont("Arial", "B", 9)
	p.CellFormat(219, 9, tr("TOTAL"), "1", 0, "R", false, 0, "")
	p.CellFormat(58, 9, tr(cash.Money(total)), "1", 0, "L", false, 0, "")
	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatAdvanceDate(value string) string {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return value
	}
	return t.Format("02/01/2006")
}
