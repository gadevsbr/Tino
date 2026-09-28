package extratos

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
)

func TestRealBitzSamplesWhenConfigured(t *testing.T) {
	dir := os.Getenv("BITZ_SAMPLE_DIR")
	if dir == "" {
		t.Skip("real samples not configured")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.pdf"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("samples: count=%d err=%v", len(paths), err)
	}
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		stay, err := ParsePDF(filepath.Base(path), data)
		if err != nil {
			t.Fatalf("sample %d: %v", i+1, err)
		}
		t.Logf("sample %d: room=%t name=%t dates=%t/%t package=%t total=%t cash=%t pix=%t card=%t review=%t", i+1, stay.Room != "", stay.Name != "", stay.CheckIn != "", stay.CheckOut != "", stay.Package != 0, stay.Total != 0, stay.Cash != 0, stay.Pix != 0, stay.Card != 0, stay.Review)
		if stay.Room == "" || stay.Name == "" || stay.CheckIn == "" || stay.CheckOut == "" || stay.Package == 0 || stay.Total == 0 {
			t.Fatalf("sample %d: missing critical fields", i+1)
		}
	}
}

const sampleText = `Extrato da Hospedagem # 123
Titular: MARIA SILVA CPF / CNPJ: 000
UH: 104 / TESTE
Data Entrada: 18/09/2026
Data Saída: 20/09/2026
RECEBIMENTOS DA HOSPEDAGEM
18/09/2026 15:20 PIX R$ 200,00
19/09/2026 10:00 DINHEIRO R$ 50,00
TOTAIS
Diárias R$ 200,00
Produtos R$ 70,00
Desconto R$ 20,00
Total Hospedagem R$ 250,00
A Receber R$ 0,00`

func TestParseTextTotalsAndReceipts(t *testing.T) {
	s := ParseText("extrato.pdf", sampleText)
	if s.Room != "104" || s.Name != "MARIA SILVA" || s.Package != 20000 || s.Consumption != 5000 || s.Pix != 20000 || s.Cash != 5000 || s.Review {
		t.Fatalf("stay=%#v", s)
	}
}

func TestParseWrappedBitzTotals(t *testing.T) {
	wrapped := strings.Replace(sampleText, "Total Hospedagem R$ 250,00", "Total\nHospedagem\n(+)\nR$\n250,00", 1)
	wrapped = strings.Replace(wrapped, "Diárias R$ 200,00", "Diárias R$\n200,00", 1)
	s := ParseText("wrapped.pdf", wrapped)
	if s.Total != 25000 || s.Package != 20000 || s.Consumption != 5000 || s.Review {
		t.Fatalf("wrapped totals not recognized: %#v", s)
	}
}

func TestParseTextFlagsDivergence(t *testing.T) {
	s := ParseText("extrato.pdf", strings.Replace(sampleText, "PIX R$ 200,00", "PIX R$ 100,00", 1))
	if !s.Review || !strings.Contains(strings.Join(s.Notes, " "), "CONFERIR RECEBIMENTOS") {
		t.Fatalf("stay=%#v", s)
	}
}

func TestProcessWorkbookAndAudit(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	headers := []string{"QUARTO", "DATA ENTRADA", "DATA SAÍDA", "NOME", "VALOR PACOTE", "VALOR CONSUMO", "PAG. DINHEIRO", "PAG. PIX", "PAG. CARTÃO", "OBSERVAÇÕES"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue("Sheet1", cell, h)
	}
	for cell, value := range map[string]string{"A2": "104", "B2": "18/09/2026", "C2": "20/09/2026", "D2": "MARIA SILVA", "J2": "nota anterior", "A3": "105", "D3": "OUTRO HÓSPEDE"} {
		_ = f.SetCellValue("Sheet1", cell, value)
	}
	input, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	s := ParseText("extrato.pdf", sampleText)
	output, audit, results, err := Process(input.Bytes(), []Stay{s})
	if err != nil || len(results) != 1 || results[0].Status != "OK" {
		t.Fatalf("results=%#v err=%v", results, err)
	}
	if !bytes.Contains(audit, []byte("MARIA SILVA")) {
		t.Fatal("audit missing stay")
	}
	updated, err := excelize.OpenReader(bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	defer updated.Close()
	for cell, want := range map[string]string{"E2": "200", "F2": "50", "G2": "50", "H2": "200", "J2": "nota anterior"} {
		got, _ := updated.GetCellValue("Sheet1", cell)
		if got != want {
			t.Fatalf("%s=%q want %q", cell, got, want)
		}
	}
	if _, _, duplicate, err := Process(input.Bytes(), []Stay{s, s}); err != nil || duplicate[1].Status != "NÃO ENCONTRADO" {
		t.Fatalf("duplicate=%#v err=%v", duplicate, err)
	}
	wrongDate := s
	wrongDate.CheckIn = "21/09/2026"
	if _, _, mismatch, err := Process(input.Bytes(), []Stay{wrongDate}); err != nil || mismatch[0].Status != "NÃO ENCONTRADO" {
		t.Fatalf("wrong date=%#v err=%v", mismatch, err)
	}
	original, _ := f.GetCellValue("Sheet1", "E2")
	if original != "" {
		t.Fatal("original workbook was mutated")
	}
}

func TestParsePDFRejectsImageOnly(t *testing.T) {
	p := fpdf.New("P", "mm", "A4", "")
	p.AddPage()
	var out bytes.Buffer
	if err := p.Output(&out); err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePDF("scan.pdf", out.Bytes()); err == nil {
		t.Fatal("accepted PDF without selectable text")
	}
}

func TestParseTextBasedPDF(t *testing.T) {
	p := fpdf.New("P", "mm", "A4", "")
	p.AddPage()
	p.SetFont("Arial", "", 12)
	ascii := strings.NewReplacer("Saída", "Saida", "Diárias", "Diarias").Replace(sampleText)
	for _, line := range strings.Split(ascii, "\n") {
		p.CellFormat(0, 7, line, "", 1, "", false, 0, "")
	}
	var out bytes.Buffer
	if err := p.Output(&out); err != nil {
		t.Fatal(err)
	}
	s, err := ParsePDF("extrato.pdf", out.Bytes())
	if err != nil || s.Room != "104" || s.Package != 20000 || s.Pix != 20000 {
		t.Fatalf("stay=%#v err=%v", s, err)
	}
}
