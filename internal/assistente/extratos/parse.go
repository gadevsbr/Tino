package extratos

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

type Stay struct {
	File, ID, Name, Room, CheckIn, CheckOut                                   string
	Package, Consumption, Total, Due, Cash, Pix, Card, Deposit, Other, Change int64
	Notes                                                                     []string
	Review                                                                    bool
}

var moneyPattern = regexp.MustCompile(`(?i)R\$\s*([0-9.]+,[0-9]{2})`)

func cents(value string) int64 {
	value = strings.ReplaceAll(value, ".", "")
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return 0
	}
	whole, err1 := strconv.ParseInt(parts[0], 10, 64)
	fraction, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0
	}
	return whole*100 + fraction
}

func textMatch(text, pattern string) string {
	m := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.Join(strings.Fields(m[1]), " ")
}

func sectionValue(text, label string) int64 {
	upper := strings.ToUpper(text)
	if pos := strings.LastIndex(upper, "TOTAIS"); pos >= 0 {
		text = text[pos:]
	}
	// Bitz often wraps labels, the sign and the currency onto separate lines.
	text = strings.Join(strings.Fields(text), " ")
	pattern := `(?is)` + label + `\s*(?:\([+-]\))?\s*R\$\s*([0-9.]+,[0-9]{2})`
	return cents(textMatch(text, pattern))
}

func ParseText(filename, text string) Stay {
	s := Stay{File: filepath.Base(filename)}
	s.ID = textMatch(text, `(?i)Extrato\s+da\s+Hospedagem\s*#\s*(\d+)`)
	s.Name = textMatch(text, `(?is)Titular:\s*(.*?)\s+(?:CPF\s*/\s*CNPJ|CPF|CNPJ):`)
	s.Room = textMatch(text, `(?i)UH:\s*(\d+)\s*/`)
	s.CheckIn = textMatch(text, `(?i)Data\s+Entrada:\s*(\d{2}/\d{2}/\d{4})`)
	s.CheckOut = textMatch(text, `(?i)Data\s+Sa[ií]da:\s*(\d{2}/\d{2}/\d{4})`)
	s.Package = sectionValue(text, `Di[aá]rias`)
	products := sectionValue(text, `Produtos`)
	services := sectionValue(text, `Servi[cç]os`)
	serviceFee := sectionValue(text, `Taxa de Servi[cç]os`)
	discount := sectionValue(text, `Desconto`)
	s.Total = sectionValue(text, `Total Hospedagem`)
	s.Due = sectionValue(text, `A Receber`)
	s.Change = sectionValue(text, `Troco`)
	if s.Total > 0 {
		s.Consumption = s.Total - s.Package
	} else {
		s.Consumption = products + services + serviceFee - discount
	}
	upper := strings.ToUpper(text)
	start := strings.Index(upper, "RECEBIMENTOS DA HOSPEDAGEM")
	if start < 0 {
		start = strings.Index(upper, "RECEBIMENTOS DA RESERVA")
	}
	end := strings.LastIndex(upper, "TOTAIS")
	if start >= 0 && end > start {
		area := strings.Join(strings.Fields(text[start:end]), " ")
		chunks := regexp.MustCompile(`(?is)\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}(?::\d{2})?.*?R\$\s*[0-9.]+,[0-9]{2}`).FindAllString(area, -1)
		for _, chunk := range chunks {
			all := moneyPattern.FindAllStringSubmatch(chunk, -1)
			if len(all) == 0 {
				continue
			}
			amount := cents(all[len(all)-1][1])
			u := strings.ToUpper(chunk)
			switch {
			case strings.Contains(u, "PIX"):
				s.Pix += amount
			case strings.Contains(u, "ESPÉCIE"), strings.Contains(u, "ESPECIE"), strings.Contains(u, "DINHEIRO"):
				s.Cash += amount
			case strings.Contains(u, "DEPÓSITO"), strings.Contains(u, "DEPOSITO"):
				s.Deposit += amount
			case strings.Contains(u, "CRÉDITO"), strings.Contains(u, "CREDITO"), strings.Contains(u, "DÉBITO"), strings.Contains(u, "DEBITO"), strings.Contains(u, "VISA"), strings.Contains(u, "MASTERCARD"), strings.Contains(u, "ELO"), strings.Contains(u, "MAESTRO"), strings.Contains(u, "AMEX"):
				s.Card += amount
			default:
				s.Other += amount
			}
		}
	}
	if s.Deposit != 0 {
		s.Notes = append(s.Notes, "DEPÓSITO: "+formatMoney(s.Deposit))
	}
	if s.Other != 0 {
		s.Notes = append(s.Notes, "OUTROS RECEBIMENTOS: "+formatMoney(s.Other))
	}
	if s.Change != 0 {
		s.Notes = append(s.Notes, "TROCO: "+formatMoney(s.Change))
	}
	if s.Due > 5 {
		s.Notes = append(s.Notes, "ATENÇÃO - A RECEBER: "+formatMoney(s.Due))
	} else if s.Due > 0 {
		s.Notes = append(s.Notes, "A RECEBER: "+formatMoney(s.Due))
	}
	received := s.Cash + s.Pix + s.Card + s.Deposit + s.Other
	if s.Total > 0 && abs(received-(s.Total+s.Change)) > 6 && s.Due <= 5 {
		s.Review = true
		s.Notes = append(s.Notes, fmt.Sprintf("CONFERIR RECEBIMENTOS: recebido %s / total %s", formatMoney(received), formatMoney(s.Total)))
	}
	if s.Room == "" || s.Name == "" || s.Total == 0 || s.Package == 0 || s.CheckIn == "" || s.CheckOut == "" {
		s.Review = true
		s.Notes = append(s.Notes, "EXTRAÇÃO INCOMPLETA")
	}
	return s
}

func ParsePDF(filename string, data []byte) (Stay, error) {
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return Stay{}, errors.New("arquivo não é PDF")
	}
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Stay{}, err
	}
	plain, err := reader.GetPlainText()
	if err != nil {
		return Stay{}, err
	}
	text, err := io.ReadAll(io.LimitReader(plain, 2<<20))
	if err != nil {
		return Stay{}, err
	}
	if len(strings.TrimSpace(string(text))) == 0 {
		return Stay{}, errors.New("PDF sem texto selecionável; confira se é extrato exportado, não digitalização")
	}
	return ParseText(filename, string(text)), nil
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
func formatMoney(n int64) string { return fmt.Sprintf("R$ %d,%02d", n/100, abs(n%100)) }
