package extratos

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/unicode/norm"
)

type Result struct {
	File   string `json:"arquivo"`
	Status string `json:"status"`
	Row    int    `json:"linha,omitempty"`
	Detail string `json:"detalhe"`
}

type Audit struct {
	Stays   []Stay   `json:"extratos"`
	Results []Result `json:"resultados"`
}

func normalize(text string) string {
	text = strings.ToUpper(norm.NFKD.String(text))
	var b strings.Builder
	space := false
	for _, r := range text {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func nameSimilarity(a, b string) float64 {
	a, b = normalize(a), normalize(b)
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return .94
	}
	// A conservative token overlap avoids selecting a similarly named guest by room alone.
	at, bt := strings.Fields(a), strings.Fields(b)
	counts := map[string]int{}
	for _, t := range at {
		counts[t]++
	}
	common := 0
	for _, t := range bt {
		if counts[t] > 0 {
			common++
			counts[t]--
		}
	}
	return float64(2*common) / float64(len(at)+len(bt))
}

func cell(f *excelize.File, sheet string, col, row int) string {
	if col == 0 {
		return ""
	}
	name, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return ""
	}
	value, _ := f.GetCellValue(sheet, name)
	return strings.TrimSpace(value)
}

func normalizeDate(value string) string {
	for _, layout := range []string{"02/01/2006", "2006-01-02", "02-01-2006"} {
		if date, err := time.Parse(layout, value); err == nil {
			return date.Format("02/01/2006")
		}
	}
	if serial, err := strconv.ParseFloat(value, 64); err == nil && serial > 10000 {
		if date, err := excelize.ExcelDateToTime(serial, false); err == nil {
			return date.Format("02/01/2006")
		}
	}
	return ""
}

func setCell(f *excelize.File, sheet string, col, row int, value any) error {
	name, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return err
	}
	return f.SetCellValue(sheet, name, value)
}

func moneyValue(n int64) any {
	if n == 0 {
		return nil
	}
	return float64(n) / 100
}

func appendNote(old, note string) string {
	old, note = strings.TrimSpace(old), strings.TrimSpace(note)
	if note == "" || strings.Contains(old, note) {
		return old
	}
	if old == "" {
		return note
	}
	return old + "; " + note
}

func Process(workbook []byte, stays []Stay) ([]byte, []byte, []Result, error) {
	if len(stays) == 0 {
		return nil, nil, nil, errors.New("nenhum extrato recebido")
	}
	if len(workbook) > 15<<20 {
		return nil, nil, nil, errors.New("planilha excede 15 MB")
	}
	f, err := excelize.OpenReader(bytes.NewReader(workbook))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("abrir planilha: %w", err)
	}
	defer f.Close()
	var sheet string
	var header map[string]int
	for _, candidate := range f.GetSheetList() {
		rows, err := f.GetRows(candidate)
		if err != nil || len(rows) == 0 {
			continue
		}
		h := map[string]int{}
		for i, v := range rows[0] {
			h[normalize(v)] = i + 1
		}
		if h["QUARTO"] != 0 && h["NOME"] != 0 {
			sheet, header = candidate, h
			break
		}
	}
	if sheet == "" {
		return nil, nil, nil, errors.New("planilha sem aba com QUARTO e NOME na primeira linha")
	}
	for _, key := range []string{"VALOR PACOTE", "VALOR CONSUMO", "PAG DINHEIRO", "PAG PIX", "PAG CARTAO", "OBSERVACOES"} {
		if header[key] == 0 {
			return nil, nil, nil, fmt.Errorf("coluna %s não encontrada", key)
		}
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, nil, nil, err
	}
	results := make([]Result, 0, len(stays))
	used := map[int]bool{}
	for _, stay := range stays {
		bestRow, bestScore, second := 0, 0.0, 0.0
		for row := 2; row <= len(rows); row++ {
			if cell(f, sheet, header["QUARTO"], row) != stay.Room || stay.Room == "" {
				continue
			}
			name := cell(f, sheet, header["NOME"], row)
			sim := nameSimilarity(name, stay.Name)
			din := normalizeDate(cell(f, sheet, header["DATA ENTRADA"], row))
			dout := normalizeDate(cell(f, sheet, header["DATA SAIDA"], row))
			if din != "" && stay.CheckIn != "" && din != stay.CheckIn {
				continue
			}
			if dout != "" && stay.CheckOut != "" && dout != stay.CheckOut {
				continue
			}
			if sim < .72 && (din == "" || dout == "" || din != stay.CheckIn || dout != stay.CheckOut) {
				continue
			}
			score := .62 + sim*.28
			if din != "" && din == stay.CheckIn {
				score += .05
			}
			if dout != "" && dout == stay.CheckOut {
				score += .05
			}
			if score > bestScore {
				second, bestScore, bestRow = bestScore, score, row
			} else if score > second {
				second = score
			}
		}
		result := Result{File: stay.File}
		if bestRow == 0 || bestScore < .72 || bestScore-second < .04 || used[bestRow] {
			result.Status = "NÃO ENCONTRADO"
			result.Detail = "sem correspondência única por quarto, nome e datas; nenhuma linha alterada"
			results = append(results, result)
			continue
		}
		used[bestRow] = true
		result.Row = bestRow
		name := cell(f, sheet, header["NOME"], bestRow)
		obs := strings.Join(stay.Notes, "; ")
		if nameSimilarity(name, stay.Name) < .72 {
			obs = appendNote(obs, "Extrato UH "+stay.Room+" em nome de "+stay.Name)
		}
		if stay.Review || nameSimilarity(name, stay.Name) < .72 {
			result.Status = "CONFERIR"
			obs = appendNote(obs, "CONFERIR")
		} else {
			result.Status = "OK"
		}
		for _, pair := range []struct {
			column string
			value  int64
		}{{"VALOR PACOTE", stay.Package}, {"VALOR CONSUMO", stay.Consumption}, {"PAG DINHEIRO", stay.Cash}, {"PAG PIX", stay.Pix}, {"PAG CARTAO", stay.Card}} {
			if err := setCell(f, sheet, header[pair.column], bestRow, moneyValue(pair.value)); err != nil {
				return nil, nil, nil, err
			}
		}
		original := cell(f, sheet, header["OBSERVACOES"], bestRow)
		if err := setCell(f, sheet, header["OBSERVACOES"], bestRow, appendNote(original, obs)); err != nil {
			return nil, nil, nil, err
		}
		result.Detail = fmt.Sprintf("UH %s; planilha=%s; extrato=%s", stay.Room, name, stay.Name)
		results = append(results, result)
	}
	output, err := f.WriteToBuffer()
	if err != nil {
		return nil, nil, nil, err
	}
	audit, err := json.MarshalIndent(Audit{Stays: stays, Results: results}, "", "  ")
	if err != nil {
		return nil, nil, nil, err
	}
	return output.Bytes(), audit, results, nil
}
