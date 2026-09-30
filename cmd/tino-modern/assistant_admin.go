package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
)

type RoomDTO struct {
	Number, Floor, Guests     int
	Status, StatusLabel, Note string
	UpdatedAt                 string
}
type RoomUpdateRequest struct {
	Number, Guests int
	Status, Note   string
}
type RoomHistoryDTO struct{ Old, OldLabel, New, NewLabel, At string }
type CommercialConfigDTO struct {
	Mode          string
	Allowlist     []string
	Epoch         int64
	GroupPhone    string
	FinalMessage1 string
	FinalMessage2 string
}
type CatalogDTO struct {
	Category, Title, ProductID, Owner, Description, Currency, Price string
	HasImage                                                        bool
}
type BackupDTO struct {
	Available                            bool
	Path, CreatedAt, DataDir             string
	Connected, Authenticated, DatabaseOK bool
	Account                              string
}

func (a *App) ListRooms() ([]RoomDTO, error) {
	if err := a.requireCapability("rooms"); err != nil {
		return nil, err
	}
	r, done, err := a.operationalRuntime()
	if err != nil {
		return nil, err
	}
	defer done()
	items, err := r.rooms.ListAll(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RoomDTO, 0, len(items))
	for _, x := range items {
		out = append(out, roomDTO(x))
	}
	return out, nil
}

func (a *App) UpdateRoom(req RoomUpdateRequest) error {
	if err := a.requireCapability("rooms"); err != nil {
		return err
	}
	status := rooms.Status(strings.TrimSpace(req.Status))
	if !status.Valid() {
		return errors.New("situação de quarto inválida")
	}
	if req.Number < 101 || req.Number > 226 {
		return errors.New("número de quarto inválido")
	}
	r, done, err := a.operationalRuntime()
	if err != nil {
		return err
	}
	defer done()
	id := fmt.Sprintf("ui-room-%d-%d", req.Number, time.Now().UnixNano())
	if _, _, err = r.rooms.UpdateStatusWithGuests(a.ctx, req.Number, status, req.Guests, "Tino UI", "UI", id); err != nil {
		return err
	}
	if err = r.rooms.SetObservation(a.ctx, req.Number, strings.TrimSpace(req.Note)); err != nil {
		return err
	}
	if a.eventsReady {
		a.emitActivity("Quarto atualizado", fmt.Sprintf("Quarto %d alterado pela interface", req.Number), "success")
	}
	return nil
}

func (a *App) RoomHistory(number int) ([]RoomHistoryDTO, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return nil, err
	}
	defer done()
	items, err := r.rooms.History(a.ctx, number, 50)
	if err != nil {
		return nil, err
	}
	out := make([]RoomHistoryDTO, 0, len(items))
	for _, x := range items {
		out = append(out, RoomHistoryDTO{Old: string(x.Old), OldLabel: x.Old.Label(), New: string(x.New), NewLabel: x.New.Label(), At: x.CreatedAt.Format(time.RFC3339)})
	}
	return out, nil
}

func (a *App) GetCommercialConfig() (CommercialConfigDTO, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return CommercialConfigDTO{}, err
	}
	defer done()
	account, err := a.accountJID()
	if err != nil {
		return CommercialConfigDTO{}, err
	}
	cfg, err := r.commercial.Configuration(a.ctx, account)
	return CommercialConfigDTO{Mode: string(cfg.Mode), Allowlist: cfg.Allowlist, Epoch: cfg.Epoch, GroupPhone: cfg.GroupPhone, FinalMessage1: cfg.FinalMessage1, FinalMessage2: cfg.FinalMessage2}, err
}

func (a *App) SaveCommercialConfig(cfg CommercialConfigDTO) error {
	if err := a.requireCapability("commercial"); err != nil {
		return err
	}
	r, done, err := a.operationalRuntime()
	if err != nil {
		return err
	}
	defer done()
	account, err := a.accountJID()
	if err != nil {
		return err
	}
	allow := make([]string, 0, len(cfg.Allowlist))
	for _, raw := range cfg.Allowlist {
		n := digitsOnly(raw)
		if len(n) < 10 || len(n) > 15 {
			return fmt.Errorf("telefone de teste inválido: %s", raw)
		}
		allow = append(allow, n)
	}
	groupPhone := digitsOnly(cfg.GroupPhone)
	if len(groupPhone) < 10 || len(groupPhone) > 15 {
		return errors.New("telefone do setor de grupos precisa ter DDI e entre 10 e 15 dígitos")
	}
	if err = r.commercial.Configure(a.ctx, account, commercial.Config{Mode: commercial.Mode(cfg.Mode), Allowlist: allow, GroupPhone: groupPhone, FinalMessage1: cfg.FinalMessage1, FinalMessage2: cfg.FinalMessage2}); err != nil {
		return err
	}
	if a.eventsReady {
		a.emitActivity("Atendimento comercial", "Modo alterado para "+cfg.Mode, "success")
	}
	return nil
}

func (a *App) ListCatalog() ([]CatalogDTO, error) {
	if err := a.requireCapability("commercial"); err != nil {
		return nil, err
	}
	r, done, err := a.operationalRuntime()
	if err != nil {
		return nil, err
	}
	defer done()
	account, err := a.accountJID()
	if err != nil {
		return nil, err
	}
	items, err := r.catalog.List(a.ctx, account)
	if err != nil {
		return nil, err
	}
	out := make([]CatalogDTO, 0, len(items))
	for _, x := range items {
		p := x.Payload.GetProduct()
		price := ""
		if p.PriceAmount1000 != nil {
			price = fmt.Sprintf("%.2f", float64(p.GetPriceAmount1000())/1000)
		}
		out = append(out, CatalogDTO{Category: x.CategoryID, Title: p.GetTitle(), ProductID: p.GetProductID(), Owner: x.OwnerJID, Description: p.GetDescription(), Currency: p.GetCurrencyCode(), Price: price, HasImage: len(x.Image) > 0})
	}
	return out, nil
}

func (a *App) DeleteCatalogItem(category string) error {
	if err := a.requireCapability("commercial"); err != nil {
		return err
	}
	r, done, err := a.operationalRuntime()
	if err != nil {
		return err
	}
	defer done()
	account, err := a.accountJID()
	if err != nil {
		return err
	}
	if err = r.catalog.Delete(a.ctx, account, category); errors.Is(err, sql.ErrNoRows) {
		return errors.New("produto não encontrado")
	}
	return err
}

func (a *App) GetBackupStatus() (BackupDTO, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return BackupDTO{}, err
	}
	defer done()
	result := BackupDTO{DataDir: r.dataDir, Connected: a.mgr.Client.IsConnected(), Authenticated: a.mgr.Client.IsLoggedIn()}
	result.DatabaseOK = r.db.PingContext(a.ctx) == nil
	if a.mgr.Client.Store.ID != nil {
		result.Account = a.mgr.Client.Store.ID.ToNonAD().String()
	}
	latest, err := r.backup.Latest(a.ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return BackupDTO{}, err
	}
	result.Available = true
	result.Path = latest.Path
	result.CreatedAt = latest.CreatedAt
	return result, nil
}

func (a *App) RunBackup() (BackupDTO, error) {
	if err := a.requireCapability("backup"); err != nil {
		return BackupDTO{}, err
	}
	r, done, err := a.operationalRuntime()
	if err != nil {
		return BackupDTO{}, err
	}
	defer done()
	path, err := r.backup.Run(a.ctx, time.Now())
	if err != nil {
		return BackupDTO{}, err
	}
	if a.eventsReady {
		a.emitActivity("Backup operacional", filepathBase(path)+" criado e verificado", "success")
	}
	return BackupDTO{Available: true, Path: path, CreatedAt: time.Now().Format(time.RFC3339), DataDir: r.dataDir, Connected: a.mgr.Client.IsConnected(), Authenticated: a.mgr.Client.IsLoggedIn(), DatabaseOK: true}, nil
}

func (a *App) accountJID() (string, error) {
	if a.mgr.Client.Store.ID == nil {
		return "", errors.New("conecte uma conta WhatsApp para gerenciar este recurso")
	}
	return a.mgr.Client.Store.ID.ToNonAD().String(), nil
}
func roomDTO(x rooms.Room) RoomDTO {
	return RoomDTO{Number: x.Number, Floor: x.Floor, Guests: x.GuestCount, Status: string(x.Status), StatusLabel: x.Status.Label(), Note: x.Observation, UpdatedAt: x.UpdatedAt.Format(time.RFC3339)}
}
func filepathBase(path string) string {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}
