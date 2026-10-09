package main

import "github.com/gadevsbr/tino/internal/assistente/bills"

type BillInput struct {
	Description string
	Amount      string
	DueDate     string
}
type BillsView struct {
	Bills    []bills.Bill
	Settings bills.Settings
}

func (a *App) GetBills() (BillsView, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return BillsView{}, err
	}
	defer done()
	repo := bills.New(r.db)
	list, err := repo.List(a.ctx)
	if err != nil {
		return BillsView{}, err
	}
	cfg, err := repo.Settings(a.ctx)
	return BillsView{Bills: list, Settings: cfg}, err
}
func (a *App) AddBill(in BillInput) (bills.Bill, error) {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return bills.Bill{}, err
	}
	defer done()
	cents, err := bills.ParseMoney(in.Amount)
	if err != nil {
		return bills.Bill{}, err
	}
	return bills.New(r.db).Add(a.ctx, in.Description, cents, in.DueDate)
}
func (a *App) SetBillStatus(id int64, status string) error {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return err
	}
	defer done()
	return bills.New(r.db).SetStatus(a.ctx, id, status)
}
func (a *App) SaveBillReminders(cfg bills.Settings) error {
	r, done, err := a.operationalRuntime()
	if err != nil {
		return err
	}
	defer done()
	return bills.New(r.db).Configure(a.ctx, cfg)
}
