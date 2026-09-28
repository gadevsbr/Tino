package main

import (
	"errors"
	"fmt"

	"github.com/gadevsbr/tino/internal/assistente/cashimport"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ChooseCashPDFs() ([]cashimport.Preview, error) {
	paths, err := wailsRuntime.OpenMultipleFilesDialog(a.ctx, wailsRuntime.OpenDialogOptions{Title: "Selecionar relatórios diários de caixa", Filters: []wailsRuntime.FileFilter{{DisplayName: "Relatórios PDF", Pattern: "*.pdf"}}})
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	r, done, err := a.operationalRuntime()
	if err != nil {
		return nil, err
	}
	defer done()
	return cashimport.PreviewFiles(a.ctx, r.db, paths)
}

func (a *App) ImportCashPDFs(paths []string) (cashimport.Result, error) {
	if len(paths) == 0 {
		return cashimport.Result{}, errors.New("selecione ao menos um PDF")
	}
	r, done, err := a.operationalRuntime()
	if err != nil {
		return cashimport.Result{}, err
	}
	defer done()
	result, err := cashimport.ImportReady(a.ctx, r.db, r.dataDir, paths)
	if err != nil {
		return result, err
	}
	a.emitActivity("Importação de caixa", fmt.Sprintf("%d dias importados e %d encaminhados para revisão", result.Imported, result.Skipped), "success")
	return result, nil
}
