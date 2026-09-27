package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gadevsbr/tino/internal/audit"
	"github.com/gadevsbr/tino/internal/batch"
	"github.com/gadevsbr/tino/internal/config"
	"github.com/gadevsbr/tino/internal/flow"
	"github.com/gadevsbr/tino/internal/session"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/skip2/go-qrcode"
)

var version = "dev"

//go:embed default_flows.yaml
var defaultFlow []byte

type application struct {
	mw          *walk.MainWindow
	status      *walk.Label
	log         *walk.TextEdit
	qr          *walk.ImageView
	csvPath     *walk.LineEdit
	flowPath    *walk.LineEdit
	connectBtn  *walk.PushButton
	sendBtn     *walk.PushButton
	exportBtn   *walk.PushButton
	flowBtn     *walk.PushButton
	mgr         *session.Manager
	cfg         config.Config
	ctx         context.Context
	cancel      context.CancelFunc
	flowStarted bool
	mu          sync.Mutex
}

func main() {
	app := &application{}
	app.ctx, app.cancel = context.WithCancel(context.Background())
	defer app.cancel()

	cfgPath := filepath.Join("config", "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		writeStartupError(err)
		walk.MsgBox(nil, "Tino", "Não foi possível carregar a configuração: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	app.cfg = cfg
	if _, statErr := os.Stat(app.cfg.Flow.RulesFile); os.IsNotExist(statErr) {
		fallback := filepath.Join(app.cfg.DataDir, "default-flows.yaml")
		if mkErr := os.MkdirAll(filepath.Dir(fallback), 0o700); mkErr == nil {
			if writeErr := os.WriteFile(fallback, defaultFlow, 0o600); writeErr == nil {
				app.cfg.Flow.RulesFile = fallback
			}
		}
	}
	app.mgr, err = session.Open(app.ctx, cfg.DataDir, cfg.Profile, false)
	if err != nil {
		writeStartupError(err)
		walk.MsgBox(nil, "Tino", "Não foi possível abrir a sessão: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	defer app.mgr.Close()

	if err := app.createWindow(); err != nil {
		writeStartupError(err)
		walk.MsgBox(nil, "Tino", err.Error(), walk.MsgBoxIconError)
		return
	}
	app.refreshStatus()
	app.mw.Run()
}

func writeStartupError(err error) {
	_ = os.MkdirAll("data", 0o700)
	_ = os.WriteFile(filepath.Join("data", "ui-startup-error.log"), []byte(time.Now().Format(time.RFC3339)+" "+err.Error()+"\n"), 0o600)
}

func (a *application) createWindow() error {
	return MainWindow{
		AssignTo: &a.mw,
		Title:    "Tino — Comunicação Corporativa " + version,
		MinSize:  Size{Width: 820, Height: 650},
		Size:     Size{Width: 900, Height: 720},
		Layout:   VBox{MarginsZero: false, Spacing: 10},
		Children: []Widget{
			Composite{Layout: HBox{}, Children: []Widget{
				Label{Text: "Sessão:", Font: Font{Bold: true}},
				Label{AssignTo: &a.status, Text: "Carregando..."},
				HSpacer{},
				PushButton{AssignTo: &a.connectBtn, Text: "Conectar / Exibir QR", OnClicked: a.connect},
				PushButton{Text: "Atualizar status", OnClicked: a.refreshStatus},
			}},
			GroupBox{Title: "Autenticação", Layout: VBox{}, Children: []Widget{
				Label{Text: "No WhatsApp do celular, abra Dispositivos conectados e escaneie o QR Code."},
				ImageView{AssignTo: &a.qr, MinSize: Size{Width: 280, Height: 280}, Mode: ImageViewModeShrink},
			}},
			GroupBox{Title: "Auditoria e notificações", Layout: VBox{}, Children: []Widget{
				Composite{Layout: HBox{}, Children: []Widget{
					PushButton{AssignTo: &a.exportBtn, Text: "Exportar contatos e grupos", OnClicked: a.export},
					HSpacer{},
				}},
				Composite{Layout: HBox{}, Children: []Widget{
					Label{Text: "Arquivo CSV:"}, LineEdit{AssignTo: &a.csvPath, ReadOnly: true},
					PushButton{Text: "Selecionar...", OnClicked: a.chooseCSV},
					PushButton{AssignTo: &a.sendBtn, Text: "Enviar notificações consentidas", OnClicked: a.send},
				}},
			}},
			GroupBox{Title: "Fluxo de atendimento", Layout: HBox{}, Children: []Widget{
				LineEdit{AssignTo: &a.flowPath, Text: cfgOr(a.cfg.Flow.RulesFile, filepath.Join("config", "flows.yaml"))},
				PushButton{Text: "Selecionar YAML...", OnClicked: a.chooseFlow},
				PushButton{AssignTo: &a.flowBtn, Text: "Iniciar atendimento", OnClicked: a.startFlow},
			}},
			Label{Text: "Atividade", Font: Font{Bold: true}},
			TextEdit{AssignTo: &a.log, ReadOnly: true, VScroll: true, MinSize: Size{Height: 150}},
		},
	}.Create()
}

func cfgOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func (a *application) ui(fn func()) { a.mw.Synchronize(fn) }
func (a *application) appendLog(message string) {
	a.ui(func() { a.log.AppendText(time.Now().Format("15:04:05") + "  " + message + "\r\n") })
}
func (a *application) setBusy(busy bool) {
	a.ui(func() { a.connectBtn.SetEnabled(!busy); a.exportBtn.SetEnabled(!busy); a.sendBtn.SetEnabled(!busy) })
}
func (a *application) refreshStatus() {
	auth := a.mgr.Client.Store.ID != nil
	connected := a.mgr.Client.IsConnected()
	text := "não autenticada"
	if auth {
		text = "autenticada, desconectada"
	}
	if connected {
		text = "conectada"
	}
	a.status.SetText(text + " • perfil " + a.cfg.Profile)
}

func (a *application) ensureConnected() error {
	if a.mgr.Client.IsConnected() {
		return nil
	}
	return a.mgr.ConnectWithQR(a.ctx, false, nil)
}

func (a *application) connect() {
	a.setBusy(true)
	a.appendLog("Iniciando conexão...")
	go func() {
		defer a.setBusy(false)
		err := a.mgr.ConnectWithQR(a.ctx, true, func(content string) error {
			path := filepath.Join(a.cfg.DataDir, "pairing-qr.png")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			png, err := qrcode.Encode(content, qrcode.Medium, 360)
			if err != nil {
				return err
			}
			if err := os.WriteFile(path, png, 0o600); err != nil {
				return err
			}
			abs, _ := filepath.Abs(path)
			a.ui(func() {
				img, imageErr := walk.NewImageFromFile(abs)
				if imageErr == nil {
					_ = a.qr.SetImage(img)
				}
			})
			a.appendLog("QR Code atualizado. Escaneie com o celular.")
			return nil
		})
		if err != nil {
			a.appendLog("Falha na conexão: " + err.Error())
		} else {
			a.appendLog("Conta conectada com sucesso.")
		}
		a.ui(a.refreshStatus)
	}()
}

func (a *application) chooseCSV() {
	dlg := new(walk.FileDialog)
	dlg.Title = "Selecionar lista CSV"
	dlg.Filter = "Arquivos CSV (*.csv)|*.csv|Todos os arquivos (*.*)|*.*"
	if ok, _ := dlg.ShowOpen(a.mw); ok {
		a.csvPath.SetText(dlg.FilePath)
	}
}
func (a *application) chooseFlow() {
	dlg := new(walk.FileDialog)
	dlg.Title = "Selecionar fluxo YAML"
	dlg.Filter = "Fluxos YAML (*.yaml;*.yml)|*.yaml;*.yml|Todos os arquivos (*.*)|*.*"
	if ok, _ := dlg.ShowOpen(a.mw); ok {
		a.flowPath.SetText(dlg.FilePath)
	}
}

func (a *application) export() {
	a.setBusy(true)
	go func() {
		defer a.setBusy(false)
		if err := a.ensureConnected(); err != nil {
			a.appendLog("Exportação cancelada: " + err.Error())
			return
		}
		snap, err := audit.Collect(a.ctx, a.mgr.Client)
		if err != nil {
			a.appendLog("Erro na exportação: " + err.Error())
			return
		}
		dir := filepath.Join("exports", a.cfg.Profile)
		if err := audit.Write(snap, dir); err != nil {
			a.appendLog("Erro ao gravar exportação: " + err.Error())
			return
		}
		a.appendLog(fmt.Sprintf("Exportados %d contatos e %d grupos em %s.", len(snap.Contacts), len(snap.Groups), dir))
	}()
}

func (a *application) send() {
	path := a.csvPath.Text()
	if path == "" {
		walk.MsgBox(a.mw, "Tino", "Selecione um arquivo CSV.", walk.MsgBoxIconWarning)
		return
	}
	if walk.MsgBox(a.mw, "Confirmar envio", "Enviar somente para os contatos com consent=true?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	a.setBusy(true)
	go func() {
		defer a.setBusy(false)
		if err := a.ensureConnected(); err != nil {
			a.appendLog("Envio cancelado: " + err.Error())
			return
		}
		items, err := batch.LoadCSV(path)
		if err != nil {
			a.appendLog("CSV inválido: " + err.Error())
			return
		}
		p := batch.Processor{Client: a.mgr.Client, MinInterval: a.cfg.Batch.MinInterval, MaxInterval: a.cfg.Batch.MaxInterval, MaxPerRun: a.cfg.Batch.MaxPerRun}
		err = p.Run(a.ctx, items, func(r batch.Result) { b, _ := json.Marshal(r); a.appendLog(string(b)) })
		if err != nil {
			a.appendLog("Processamento encerrado: " + err.Error())
		} else {
			a.appendLog("Processamento concluído.")
		}
	}()
}

func (a *application) startFlow() {
	a.mu.Lock()
	if a.flowStarted {
		a.mu.Unlock()
		return
	}
	a.flowStarted = true
	a.mu.Unlock()
	path := a.flowPath.Text()
	a.flowBtn.SetEnabled(false)
	go func() {
		if err := a.ensureConnected(); err != nil {
			a.appendLog("Atendimento não iniciado: " + err.Error())
			a.mu.Lock()
			a.flowStarted = false
			a.mu.Unlock()
			a.ui(func() { a.flowBtn.SetEnabled(true) })
			return
		}
		engine, err := flow.Load(path, a.mgr.Client)
		if err != nil {
			a.appendLog("Fluxo inválido: " + err.Error())
			a.mu.Lock()
			a.flowStarted = false
			a.mu.Unlock()
			a.ui(func() { a.flowBtn.SetEnabled(true) })
			return
		}
		a.mgr.Client.AddEventHandler(engine.Handle)
		a.appendLog("Fluxo de atendimento ativo: " + path)
	}()
}
