package main

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/advances"
	"github.com/gadevsbr/tino/internal/assistente/cash"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"go.mau.fi/whatsmeow/types"
)

type FinanceDTO struct {
	From, To, Label                                   string
	OpeningCents, EntryCents, ExitCents, BalanceCents int64
	CashCents, PixCents, CardCents                    int64
	Days, Movements, Receipts                         int
}

type ReceiptDTO struct {
	ID, MovementID                              int64
	Date, Method, Description, Actor, MediaType string
	Cents                                       int64
	HasFile                                     bool
}

type ReceiptPreviewDTO struct {
	Receipt ReceiptDTO `json:"receipt"`
	DataURL string     `json:"dataURL"`
}

type AdvanceDTO struct {
	ID                          int64
	Employee, Date, Note, Actor string
	Cents                       int64
	DeductCash                  bool
}

type StatementWeekDTO struct {
	ID, Label, Status, CreatedBy, LastGeneratedAt string
	FileCount                                     int
	Files                                         []StatementFileDTO
}

type StatementFileDTO struct {
	ID         int64
	Name, Path string
}

type ShareReportRequest struct{ Kind, Target, From, To, Employee string }

func (a *App) FinanceDashboard(from, to string) (FinanceDTO, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return FinanceDTO{}, err
	}
	defer done()
	from, to, err = validRange(from, to)
	if err != nil {
		return FinanceDTO{}, err
	}
	days, err := r.cash.Range(a.ctx, from, to)
	if err != nil {
		return FinanceDTO{}, err
	}
	result := FinanceDTO{From: from, To: to, Label: periodLabel(from, to), Days: len(days)}
	for _, day := range days {
		result.OpeningCents += day.OpeningCents
		for _, item := range day.Entries {
			result.EntryCents += item.Cents
			result.Movements++
			switch item.Method {
			case "DINHEIRO":
				result.CashCents += item.Cents
			case "PIX":
				result.PixCents += item.Cents
			case "CARTAO":
				result.CardCents += item.Cents
			}
		}
		for _, item := range day.Exits {
			result.ExitCents += item.Cents
			result.Movements++
		}
	}
	receipts, err := r.cash.ReceiptsRange(a.ctx, from, to)
	if err != nil {
		return FinanceDTO{}, err
	}
	result.Receipts = len(receipts)
	result.BalanceCents = result.OpeningCents + result.EntryCents - result.ExitCents
	return result, nil
}

func (a *App) ListReceipts(from, to string) ([]ReceiptDTO, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return nil, err
	}
	defer done()
	from, to, err = validRange(from, to)
	if err != nil {
		return nil, err
	}
	items, err := r.cash.ReceiptsRange(a.ctx, from, to)
	if err != nil {
		return nil, err
	}
	result := make([]ReceiptDTO, 0, len(items))
	for _, x := range items {
		result = append(result, receiptDTO(x))
	}
	return result, nil
}

func (a *App) GetReceiptPreview(id int64) (ReceiptPreviewDTO, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return ReceiptPreviewDTO{}, err
	}
	defer done()
	item, err := r.cash.Receipt(a.ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ReceiptPreviewDTO{}, errors.New("comprovante não encontrado")
	}
	if err != nil {
		return ReceiptPreviewDTO{}, err
	}
	result := ReceiptPreviewDTO{Receipt: receiptDTO(item)}
	if item.FilePath == "" {
		return result, nil
	}
	path, err := safeOperationalPath(r.dataDir, item.FilePath)
	if err != nil {
		return result, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("abrir comprovante: %w", err)
	}
	mime := item.MediaType
	if mime == "" {
		mime = "application/octet-stream"
	}
	result.DataURL = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	return result, nil
}

func (a *App) ListAdvances(month, employee string) ([]AdvanceDTO, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return nil, err
	}
	defer done()
	date, err := time.Parse("2006-01", month)
	if err != nil {
		return nil, errors.New("mês inválido")
	}
	items, err := r.advances.Month(a.ctx, employee, date)
	if err != nil {
		return nil, err
	}
	result := make([]AdvanceDTO, 0, len(items))
	for _, x := range items {
		result = append(result, AdvanceDTO{ID: x.ID, Employee: x.Employee, Date: x.Date, Note: x.Note, Actor: x.Actor, Cents: x.Cents, DeductCash: x.DeductCash})
	}
	return result, nil
}

func (a *App) ListStatementWeeks() ([]StatementWeekDTO, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return nil, err
	}
	defer done()
	weeks, err := r.extratos.ListWeeks(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]StatementWeekDTO, 0, len(weeks))
	for _, week := range weeks {
		files, err := r.extratos.Files(a.ctx, week.ID)
		if err != nil {
			return nil, err
		}
		dto := StatementWeekDTO{ID: week.ID, Label: week.Period.Label(), Status: week.Status, CreatedBy: week.CreatedBy, LastGeneratedAt: week.LastGeneratedAt, FileCount: len(files)}
		for _, f := range files {
			dto.Files = append(dto.Files, StatementFileDTO{ID: f.ID, Name: f.OriginalName, Path: f.Path})
		}
		result = append(result, dto)
	}
	return result, nil
}

func (a *App) ExportOperationalPDF(kind, from, to, employee string) (string, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return "", err
	}
	defer done()
	kind = strings.ToUpper(strings.TrimSpace(kind))
	var data []byte
	var name string
	switch kind {
	case "ROOMS":
		items, e := r.rooms.ListAll(a.ctx)
		if e != nil {
			return "", e
		}
		data, err = r.reports.Generate(items, time.Now())
		name = "Relatorio de Quartos.pdf"
	case "CASH":
		from, to, err = validRange(from, to)
		if err != nil {
			return "", err
		}
		days, e := r.cash.Range(a.ctx, from, to)
		if e != nil {
			return "", e
		}
		if len(days) == 0 {
			return "", errors.New("não há caixa no período")
		}
		data, err = cash.RangePDF(days, r.zone)
		name = "Relatorio de Caixa " + from + " a " + to + ".pdf"
	case "ADVANCES":
		date, e := time.Parse("2006-01", from)
		if e != nil {
			return "", errors.New("mês inválido")
		}
		items, e := r.advances.Month(a.ctx, employee, date)
		if e != nil {
			return "", e
		}
		if len(items) == 0 {
			return "", errors.New("nenhum vale encontrado")
		}
		data, err = advances.PDF(items, date.Format("01/2006"), employee, r.zone)
		name = "Vales " + from + ".pdf"
	default:
		return "", errors.New("tipo de relatório inválido")
	}
	if err != nil {
		return "", err
	}
	path, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{Title: "Salvar relatório PDF", DefaultFilename: name, Filters: []wailsRuntime.FileFilter{{DisplayName: "Documento PDF", Pattern: "*.pdf"}}})
	if err != nil || path == "" {
		return path, err
	}
	if strings.ToLower(filepath.Ext(path)) != ".pdf" {
		path += ".pdf"
	}
	if err = os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) ShareOperationalPDF(req ShareReportRequest) error {
	if err := a.ensureConnected(); err != nil {
		return err
	}
	r, done, err := a.operationalRuntime()
	if err != nil {
		return err
	}
	defer done()
	target := strings.TrimSpace(req.Target)
	var jid types.JID
	if strings.Contains(target, "@") {
		jid, err = types.ParseJID(target)
	} else {
		phone := digitsOnly(target)
		if len(phone) < 10 || len(phone) > 15 {
			return errors.New("destino deve conter telefone com DDI ou JID de grupo")
		}
		jid = types.NewJID(phone, types.DefaultUserServer)
	}
	if err != nil {
		return fmt.Errorf("destino inválido: %w", err)
	}
	return r.service.ShareReport(a.ctx, req.Kind, jid, req.From, req.To, req.Employee)
}

func (a *App) operationalRuntime() (*hotelRuntime, func(), error) {
	a.hotelMu.Lock()
	if a.hotel == nil {
		a.hotelMu.Unlock()
		return nil, func() {}, errors.New("motor operacional indisponível")
	}
	return a.hotel, a.hotelMu.Unlock, nil
}
func receiptDTO(x cash.Receipt) ReceiptDTO {
	return ReceiptDTO{ID: x.ID, MovementID: x.MovementID, Date: x.BusinessDate, Method: x.Method, Description: x.Description, Actor: x.Actor, MediaType: x.MediaType, Cents: x.Cents, HasFile: x.FilePath != ""}
}
func validRange(from, to string) (string, string, error) {
	if from == "" || to == "" {
		return "", "", errors.New("informe data inicial e final")
	}
	a, e := time.Parse("2006-01-02", from)
	if e != nil {
		return "", "", errors.New("data inicial inválida")
	}
	b, e := time.Parse("2006-01-02", to)
	if e != nil || b.Before(a) {
		return "", "", errors.New("data final inválida")
	}
	if b.Sub(a) > 366*24*time.Hour {
		return "", "", errors.New("o período máximo é de 366 dias")
	}
	return from, to, nil
}
func periodLabel(from, to string) string {
	if from == to {
		return from
	}
	return from + " a " + to
}
func safeOperationalPath(root, relative string) (string, error) {
	base, e := filepath.Abs(root)
	if e != nil {
		return "", e
	}
	path, e := filepath.Abs(filepath.Join(root, relative))
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(base, path)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("caminho do comprovante inválido")
	}
	return path, nil
}
