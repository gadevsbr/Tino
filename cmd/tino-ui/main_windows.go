package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image/png"
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

//go:embed assets/tino-brand.png
var brandPNG []byte

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
	progress    *walk.ProgressBar
	brand       *walk.Bitmap
	icon        *walk.Icon
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
	brandImage, err := png.Decode(bytes.NewReader(brandPNG))
	if err != nil {
		return fmt.Errorf("decodificar identidade visual: %w", err)
	}
	a.brand, err = walk.NewBitmapFromImage(brandImage)
	if err != nil {
		return fmt.Errorf("criar imagem da marca: %w", err)
	}
	a.icon, err = walk.NewIconFromImage(brandImage)
	if err != nil {
		return fmt.Errorf("criar ícone da aplicação: %w", err)
	}

	const (
		navy  = walk.Color(0x0033210E)
		ink   = walk.Color(0x00382A19)
		muted = walk.Color(0x00766554)
		teal  = walk.Color(0x00A68A12)
		pale  = walk.Color(0x00F7F4EF)
		white = walk.Color(0x00FFFFFF)
	)
	pageBrush := SolidColorBrush{Color: pale}
	cardBrush := SolidColorBrush{Color: white}
	headerBrush := SolidColorBrush{Color: navy}
	accentBrush := SolidColorBrush{Color: teal}

	return MainWindow{
		AssignTo:   &a.mw,
		Title:      "Tino • Central de Comunicação " + version,
		Icon:       a.icon,
		MinSize:    Size{Width: 980, Height: 720},
		Size:       Size{Width: 1100, Height: 800},
		Font:       Font{Family: "Segoe UI", PointSize: 10},
		Background: pageBrush,
		Layout:     VBox{MarginsZero: true, Spacing: 0},
		Children: []Widget{
			Composite{Background: headerBrush, MinSize: Size{Height: 104}, Layout: HBox{Margins: Margins{Left: 24, Top: 16, Right: 24, Bottom: 16}, Spacing: 16}, Children: []Widget{
				ImageView{Image: a.brand, MinSize: Size{Width: 72, Height: 72}, MaxSize: Size{Width: 72, Height: 72}, Mode: ImageViewModeShrink, Background: headerBrush},
				Composite{Background: headerBrush, Layout: VBox{MarginsZero: true, Spacing: 2}, Children: []Widget{
					Label{Text: "TINO", TextColor: white, Font: Font{Family: "Segoe UI Semibold", PointSize: 20, Bold: true}},
					Label{Text: "Central segura de comunicação corporativa", TextColor: walk.Color(0x00E6D9CC), Font: Font{PointSize: 10}},
					Label{Text: "Sessões • Auditoria • Notificações • Atendimento", TextColor: walk.Color(0x00BFB0A0), Font: Font{PointSize: 9}},
				}},
				HSpacer{},
				Composite{Background: headerBrush, Layout: VBox{MarginsZero: true, Spacing: 6}, Children: []Widget{
					Label{Text: "STATUS DA SESSÃO", TextColor: walk.Color(0x00BFB0A0), Font: Font{PointSize: 8, Bold: true}},
					Label{AssignTo: &a.status, Text: "Carregando...", TextColor: white, Font: Font{PointSize: 11, Bold: true}},
				}},
			}},
			Composite{Background: pageBrush, Layout: VBox{Margins: Margins{Left: 20, Top: 18, Right: 20, Bottom: 14}, Spacing: 10}, Children: []Widget{
				TabWidget{ContentMargins: Margins{Left: 18, Top: 18, Right: 18, Bottom: 18}, Pages: []TabPage{
					{Title: "  Conexão  ", Background: cardBrush, Layout: HBox{Spacing: 22}, Children: []Widget{
						Composite{Background: cardBrush, MinSize: Size{Width: 390}, Layout: VBox{MarginsZero: true, Spacing: 12}, Children: []Widget{
							Label{Text: "Conecte sua conta", TextColor: ink, Font: Font{PointSize: 16, Bold: true}},
							Label{Text: "A sessão é criptografada e permanece salva neste computador.", TextColor: muted, Font: Font{PointSize: 9}},
							VSpacer{},
							PushButton{AssignTo: &a.connectBtn, Text: "  Conectar e exibir QR Code  ", MinSize: Size{Height: 42}, Font: Font{Bold: true}, Background: accentBrush, OnClicked: a.connect},
							PushButton{Text: "Atualizar estado da sessão", MinSize: Size{Height: 34}, OnClicked: a.refreshStatus},
							VSpacer{},
							Label{Text: "Como conectar", TextColor: ink, Font: Font{Bold: true}},
							Label{Text: "1. Clique em conectar\r\n2. Abra o WhatsApp no celular\r\n3. Vá em Dispositivos conectados\r\n4. Escaneie o QR Code ao lado", TextColor: muted},
						}},
						Composite{Background: SolidColorBrush{Color: walk.Color(0x00FCFBF9)}, Border: true, Layout: VBox{Margins: Margins{Left: 18, Top: 18, Right: 18, Bottom: 18}, Spacing: 8}, Children: []Widget{
							Label{Text: "QR CODE DE PAREAMENTO", TextColor: muted, Font: Font{PointSize: 8, Bold: true}, TextAlignment: AlignCenter},
							ImageView{AssignTo: &a.qr, MinSize: Size{Width: 360, Height: 360}, Mode: ImageViewModeShrink, Background: cardBrush},
							Label{Text: "O código é renovado automaticamente quando expira.", TextColor: muted, Font: Font{PointSize: 8}, TextAlignment: AlignCenter},
						}},
					}},
					{Title: "  Base e notificações  ", Background: cardBrush, Layout: VBox{Spacing: 16}, Children: []Widget{
						Label{Text: "Gestão da base de contatos", TextColor: ink, Font: Font{PointSize: 16, Bold: true}},
						Label{Text: "Exporte uma fotografia auditável da base ou processe uma lista com consentimento explícito.", TextColor: muted},
						GroupBox{Title: "Auditoria", Layout: HBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}}, Children: []Widget{
							Composite{Layout: VBox{MarginsZero: true}, Children: []Widget{Label{Text: "Contatos e participantes de grupos", TextColor: ink, Font: Font{Bold: true}}, Label{Text: "Gera JSON e CSV em uma pasta local protegida.", TextColor: muted}}},
							HSpacer{}, PushButton{AssignTo: &a.exportBtn, Text: "Exportar base", MinSize: Size{Width: 160, Height: 38}, OnClicked: a.export},
						}},
						GroupBox{Title: "Notificações consentidas", Layout: VBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}, Spacing: 10}, Children: []Widget{
							Label{Text: "Selecione um CSV com as colunas phone, message e consent.", TextColor: muted},
							Composite{Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
								LineEdit{AssignTo: &a.csvPath, ReadOnly: true}, PushButton{Text: "Selecionar CSV...", MinSize: Size{Width: 130}, OnClicked: a.chooseCSV},
							}},
							PushButton{AssignTo: &a.sendBtn, Text: "Revisar e iniciar notificações", MinSize: Size{Height: 42}, Font: Font{Bold: true}, Background: accentBrush, OnClicked: a.send},
						}},
						VSpacer{},
					}},
					{Title: "  Atendimento  ", Background: cardBrush, Layout: VBox{Spacing: 14}, Children: []Widget{
						Label{Text: "Automação de atendimento", TextColor: ink, Font: Font{PointSize: 16, Bold: true}},
						Label{Text: "Carregue regras YAML para responder conversas individuais com caminhos previsíveis.", TextColor: muted},
						GroupBox{Title: "Arquivo de fluxo", Layout: VBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}, Spacing: 10}, Children: []Widget{
							LineEdit{AssignTo: &a.flowPath, Text: cfgOr(a.cfg.Flow.RulesFile, filepath.Join("config", "flows.yaml"))},
							Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{PushButton{Text: "Selecionar YAML...", OnClicked: a.chooseFlow}, HSpacer{}, PushButton{AssignTo: &a.flowBtn, Text: "Ativar atendimento", MinSize: Size{Width: 180, Height: 40}, Font: Font{Bold: true}, Background: accentBrush, OnClicked: a.startFlow}}},
						}},
						GroupBox{Title: "Proteções ativas", Layout: VBox{Margins: Margins{Left: 14, Top: 12, Right: 14, Bottom: 12}}, Children: []Widget{
							Label{Text: "✓ Ignora mensagens enviadas pela própria conta\r\n✓ Não responde em grupos\r\n✓ Evita respostas duplicadas durante a execução", TextColor: muted},
						}},
						VSpacer{},
					}},
					{Title: "  Atividade  ", Background: cardBrush, Layout: VBox{Spacing: 10}, Children: []Widget{
						Label{Text: "Central de atividade", TextColor: ink, Font: Font{PointSize: 16, Bold: true}},
						Label{Text: "Acompanhe conexões, exportações, validações e envios em tempo real.", TextColor: muted},
						TextEdit{AssignTo: &a.log, ReadOnly: true, VScroll: true, Font: Font{Family: "Cascadia Mono", PointSize: 9}},
					}},
				}},
				Composite{Background: pageBrush, Layout: HBox{MarginsZero: true}, Children: []Widget{
					ProgressBar{AssignTo: &a.progress, MinSize: Size{Width: 180}, MaxSize: Size{Width: 180}},
					Label{Text: "Dados e sessões permanecem neste computador", TextColor: muted, Font: Font{PointSize: 8}},
					HSpacer{}, Label{Text: "Tino " + version, TextColor: muted, Font: Font{PointSize: 8}},
				}},
			}},
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
	a.ui(func() {
		a.connectBtn.SetEnabled(!busy)
		a.exportBtn.SetEnabled(!busy)
		a.sendBtn.SetEnabled(!busy)
		_ = a.progress.SetMarqueeMode(busy)
	})
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
		a.status.SetTextColor(walk.RGB(72, 224, 181))
	} else if auth {
		a.status.SetTextColor(walk.RGB(255, 204, 102))
	} else {
		a.status.SetTextColor(walk.RGB(255, 255, 255))
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
