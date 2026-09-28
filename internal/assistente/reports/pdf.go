package reports

import (
	"bytes"
	"fmt"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	"github.com/go-pdf/fpdf"
	"time"
)

type Generator struct{ location *time.Location }

func New(location *time.Location) *Generator { return &Generator{location: location} }

func Summary(list []rooms.Room, now time.Time, location *time.Location) string {
	counts := map[rooms.Status]int{}
	guests := map[rooms.Status]int{}
	for _, room := range list {
		counts[room.Status]++
		if room.Status == rooms.Entry || room.Status == rooms.OccupiedClean {
			guests[room.Status] += room.GuestCount
		}
	}
	return fmt.Sprintf("🏨 RESUMO DO RELATÓRIO\nAtualizado em %s\n\n🟢 Disponíveis: %d\n🟢🟣 Limpos, mas desforrados: %d\n🟠 Manutenção de limpeza: %d quartos — 👥 %d pessoas\n🩷 Saída e entrada: %d\n🟡 Entrada: %d quartos — 👥 %d pessoas\n🟥 Saída: %d\n⬜ Interditados: %d\n🟣 Limpar: %d\n\nTotal: %d quartos", now.In(location).Format("02/01/2006 às 15:04"), counts[rooms.AvailableClean], counts[rooms.CleanUnmade], counts[rooms.OccupiedClean], guests[rooms.OccupiedClean], counts[rooms.CheckoutEntry], counts[rooms.Entry], guests[rooms.Entry], counts[rooms.CheckoutToday], counts[rooms.Maintenance], counts[rooms.Dirty], len(list))
}

func (g *Generator) Generate(list []rooms.Room, now time.Time) ([]byte, error) {
	if len(list) != 42 {
		return nil, fmt.Errorf("report requires 42 rooms, got %d", len(list))
	}
	p := fpdf.New("L", "mm", "A4", "")
	p.SetCompression(false)
	tr := p.UnicodeTranslatorFromDescriptor("")
	p.SetMargins(10, 8, 10)
	p.AddPage()
	p.SetFont("Arial", "B", 13)
	p.CellFormat(277, 7, tr("RELATÓRIO DE QUARTOS"), "", 1, "C", false, 0, "")
	p.SetFont("Arial", "", 8)
	p.CellFormat(277, 5, tr(fmt.Sprintf("Atualizado em %s | Total: 42 quartos", now.In(g.location).Format("02/01/2006 às 15:04"))), "", 1, "C", false, 0, "")
	legend := []struct {
		name   string
		status rooms.Status
	}{{"Laranja", rooms.OccupiedClean}, {"Rosa", rooms.CheckoutEntry}, {"Amarelo", rooms.Entry}, {"Magenta", rooms.CheckoutToday}, {"Cinza", rooms.Maintenance}, {"Roxo", rooms.Dirty}, {"Verde", rooms.AvailableClean}, {"Verde/Roxo", rooms.CleanUnmade}}
	x, y := 10.0, p.GetY()+1
	p.SetFont("Arial", "", 7)
	for i, v := range legend {
		col := i % 4
		row := i / 4
		lx := x + float64(col)*69
		ly := y + float64(row)*5
		p.SetXY(lx, ly)
		p.CellFormat(15, 4, v.name, "", 0, "L", false, 0, "")
		p.SetXY(lx+15, ly)
		drawStatusCell(p, v.status, 20, 4)
		p.SetXY(lx+35, ly)
		p.CellFormat(34, 4, tr(v.status.Label()), "", 0, "L", false, 0, "")
	}
	top := y + 11
	drawTable(p, tr, 10, top, list[:21])
	drawTable(p, tr, 151, top, list[21:])
	p.SetY(198)
	p.SetFont("Arial", "I", 7)
	p.CellFormat(277, 4, tr("Gerado automaticamente pelo Hotel Room Bot"), "", 0, "C", false, 0, "")
	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
func drawTable(p *fpdf.Fpdf, tr func(string) string, x, y float64, list []rooms.Room) {
	widths := []float64{24, 50, 67}
	p.SetXY(x, y)
	p.SetFont("Arial", "B", 8)
	for i, h := range []string{"Quarto", "Situação", "Formatação"} {
		p.CellFormat(widths[i], 6, tr(h), "1", 0, "C", false, 0, "")
	}
	p.Ln(-1)
	for _, room := range list {
		p.SetX(x)
		p.SetFont("Arial", "", 7)
		p.CellFormat(widths[0], 6, fmt.Sprint(room.Number), "1", 0, "R", false, 0, "")
		drawStatusCell(p, room.Status, widths[1], 6)
		text := roomFormatting(room)
		p.CellFormat(widths[2], 6, tr(text), "1", 1, "L", false, 0, "")
	}
}

func drawStatusCell(p *fpdf.Fpdf, status rooms.Status, width, height float64) {
	r, g, b := status.RGB()
	p.SetFillColor(r, g, b)
	if !status.IsSplitColor() {
		p.CellFormat(width, height, "", "1", 0, "", true, 0, "")
		return
	}
	p.CellFormat(width/2, height, "", "LTB", 0, "", true, 0, "")
	p.SetFillColor(100, 5, 85)
	p.CellFormat(width/2, height, "", "RTB", 0, "", true, 0, "")
}

func roomFormatting(room rooms.Room) string {
	text := room.Observation
	if text == "" {
		text = room.Status.Label()
	}
	if room.Status == rooms.Entry || room.Status == rooms.OccupiedClean {
		text = fmt.Sprintf("%s | %d pessoa(s)", text, room.GuestCount)
	}
	return text
}
