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
	"strings"
	"sync"
	"time"

	"github.com/gadevsbr/tino/internal/audit"
	"github.com/gadevsbr/tino/internal/batch"
	"github.com/gadevsbr/tino/internal/chat"
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
	mw              *walk.MainWindow
	status          *walk.Label
	log             *walk.TextEdit
	qr              *walk.ImageView
	qrHome          *walk.Composite
	chatHome        *walk.Composite
	chatSearch      *walk.LineEdit
	chatList        *walk.ListBox
	chatHistory     *walk.TextEdit
	chatCompose     *walk.TextEdit
	chatTitle       *walk.Label
	chatModel       *chatListModel
	chatStore       *chat.Store
	csvPath         *walk.LineEdit
	csvSummary      *walk.Label
	campaignMessage *walk.TextEdit
	consentCheck    *walk.CheckBox
	csvImport       batch.CSVImport
	connectBtn      *walk.PushButton
	sendBtn         *walk.PushButton
	exportBtn       *walk.PushButton
	flowBtn         *walk.PushButton
	flowList        *walk.ListBox
	defaultReply    *walk.TextEdit
	testMessage     *walk.LineEdit
	testResult      *walk.TextEdit
	flowDef         flow.Definition
	flowModel       *ruleListModel
	progress        *walk.ProgressBar
	brand           *walk.Bitmap
	icon            *walk.Icon
	mgr             *session.Manager
	cfg             config.Config
	ctx             context.Context
	cancel          context.CancelFunc
	flowStarted     bool
	mu              sync.Mutex
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
	app.chatStore, err = chat.Open(filepath.Join(cfg.DataDir, "chats-"+cfg.Profile+".db"))
	if err != nil {
		writeStartupError(err)
		walk.MsgBox(nil, "Tino", "Não foi possível abrir o histórico local: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	defer app.chatStore.Close()
	app.chatModel = &chatListModel{}
	app.flowDef, err = flow.LoadDefinition(app.cfg.Flow.RulesFile)
	if err != nil {
		writeStartupError(err)
		app.flowDef = flow.Definition{DefaultReply: "Obrigado pela mensagem. Em breve continuaremos o atendimento."}
	}
	app.flowModel = &ruleListModel{rules: app.flowDef.Rules}

	if err := app.createWindow(); err != nil {
		writeStartupError(err)
		walk.MsgBox(nil, "Tino", err.Error(), walk.MsgBoxIconError)
		return
	}
	app.mgr.Client.AddEventHandler(app.handleChatEvent)
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
					{Title: "  Conversas  ", Background: cardBrush, Layout: VBox{MarginsZero: true}, Children: []Widget{
						Composite{AssignTo: &a.qrHome, Background: cardBrush, Layout: HBox{Spacing: 22}, Children: []Widget{
							Composite{Background: cardBrush, MinSize: Size{Width: 390}, Layout: VBox{MarginsZero: true, Spacing: 12}, Children: []Widget{
								Label{Text: "Conecte sua conta", TextColor: ink, Font: Font{PointSize: 16, Bold: true}},
								Label{Text: "Depois da conexão, seus chats aparecerão aqui automaticamente.", TextColor: muted, Font: Font{PointSize: 9}},
								VSpacer{}, PushButton{AssignTo: &a.connectBtn, Text: "Conectar e exibir QR Code", MinSize: Size{Height: 42}, Font: Font{Bold: true}, Background: accentBrush, OnClicked: a.connect},
								PushButton{Text: "Atualizar estado da sessão", MinSize: Size{Height: 34}, OnClicked: a.refreshStatus}, VSpacer{},
								Label{Text: "Como conectar", TextColor: ink, Font: Font{Bold: true}},
								Label{Text: "1. Clique em conectar\r\n2. Abra o WhatsApp no celular\r\n3. Vá em Dispositivos conectados\r\n4. Escaneie o QR Code ao lado", TextColor: muted},
							}},
							Composite{Background: SolidColorBrush{Color: walk.Color(0x00FCFBF9)}, Border: true, Layout: VBox{Margins: Margins{Left: 18, Top: 18, Right: 18, Bottom: 18}, Spacing: 8}, Children: []Widget{
								Label{Text: "QR CODE DE PAREAMENTO", TextColor: muted, Font: Font{PointSize: 8, Bold: true}, TextAlignment: AlignCenter},
								ImageView{AssignTo: &a.qr, MinSize: Size{Width: 360, Height: 360}, Mode: ImageViewModeShrink, Background: cardBrush},
								Label{Text: "O código é renovado automaticamente quando expira.", TextColor: muted, Font: Font{PointSize: 8}, TextAlignment: AlignCenter},
							}},
						}},
						Composite{AssignTo: &a.chatHome, Visible: false, Background: cardBrush, Layout: VBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
							Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
								Label{Text: "Conversas", TextColor: ink, Font: Font{PointSize: 16, Bold: true}}, PushButton{Text: "Conectar / atualizar", OnClicked: a.connect}, HSpacer{},
								LineEdit{AssignTo: &a.chatSearch, CueBanner: "Buscar conversa...", MinSize: Size{Width: 260}, OnTextChanged: a.refreshChats},
							}},
							HSplitter{Children: []Widget{
								ListBox{AssignTo: &a.chatList, Model: a.chatModel, MinSize: Size{Width: 300}, OnCurrentIndexChanged: a.openSelectedChat},
								Composite{Layout: VBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
									Label{AssignTo: &a.chatTitle, Text: "Selecione uma conversa", TextColor: ink, Font: Font{PointSize: 13, Bold: true}},
									TextEdit{AssignTo: &a.chatHistory, ReadOnly: true, VScroll: true, Font: Font{Family: "Segoe UI", PointSize: 10}},
									Composite{Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
										TextEdit{AssignTo: &a.chatCompose, MinSize: Size{Height: 55}}, PushButton{Text: "Enviar", MinSize: Size{Width: 110, Height: 44}, Font: Font{Bold: true}, Background: accentBrush, OnClicked: a.sendChatMessage},
									}},
								}},
							}},
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
							Label{AssignTo: &a.csvSummary, Text: "Nenhuma lista selecionada.", TextColor: muted, Font: Font{PointSize: 8}},
							Label{Text: "Mensagem da notificação", TextColor: ink, Font: Font{Bold: true}},
							TextEdit{AssignTo: &a.campaignMessage, MinSize: Size{Height: 78}, ToolTipText: "Usada para listas que possuem apenas a coluna de telefone."},
							CheckBox{AssignTo: &a.consentCheck, Text: "Confirmo que estes contatos autorizaram o recebimento desta comunicação."},
							PushButton{AssignTo: &a.sendBtn, Text: "Revisar e iniciar notificações", MinSize: Size{Height: 42}, Font: Font{Bold: true}, Background: accentBrush, OnClicked: a.send},
						}},
						VSpacer{},
					}},
					{Title: "  Flow Builder  ", Background: cardBrush, Layout: VBox{Spacing: 10}, Children: []Widget{
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							Composite{Layout: VBox{MarginsZero: true, Spacing: 2}, Children: []Widget{
								Label{Text: "Construtor visual de atendimento", TextColor: ink, Font: Font{PointSize: 16, Bold: true}},
								Label{Text: "As regras são avaliadas de cima para baixo. A primeira condição encontrada responde ao cliente.", TextColor: muted},
							}},
							HSpacer{}, PushButton{AssignTo: &a.flowBtn, Text: "Ativar atendimento", MinSize: Size{Width: 170, Height: 40}, Font: Font{Bold: true}, Background: accentBrush, OnClicked: a.startFlow},
						}},
						HSplitter{Children: []Widget{
							Composite{Layout: VBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
								Label{Text: "CAMINHOS DE RESPOSTA", TextColor: muted, Font: Font{PointSize: 8, Bold: true}},
								ListBox{AssignTo: &a.flowList, Model: a.flowModel, MinSize: Size{Width: 430, Height: 260}, OnItemActivated: a.editRule},
								Composite{Layout: HBox{MarginsZero: true, Spacing: 6}, Children: []Widget{
									PushButton{Text: "+ Nova regra", OnClicked: a.addRule}, PushButton{Text: "Editar", OnClicked: a.editRule}, PushButton{Text: "Excluir", OnClicked: a.removeRule},
									HSpacer{}, PushButton{Text: "↑", ToolTipText: "Aumentar prioridade", MinSize: Size{Width: 38}, OnClicked: func() { a.moveRule(-1) }}, PushButton{Text: "↓", ToolTipText: "Diminuir prioridade", MinSize: Size{Width: 38}, OnClicked: func() { a.moveRule(1) }},
								}},
							}},
							Composite{Layout: VBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
								Label{Text: "RESPOSTA QUANDO NENHUMA REGRA COMBINA", TextColor: muted, Font: Font{PointSize: 8, Bold: true}},
								TextEdit{AssignTo: &a.defaultReply, Text: a.flowDef.DefaultReply, MinSize: Size{Height: 82}},
								Label{Text: "TESTAR ANTES DE ATIVAR", TextColor: muted, Font: Font{PointSize: 8, Bold: true}},
								LineEdit{AssignTo: &a.testMessage, CueBanner: "Digite uma mensagem como se fosse o cliente..."},
								PushButton{Text: "Simular resposta", OnClicked: a.testFlow},
								TextEdit{AssignTo: &a.testResult, ReadOnly: true, MinSize: Size{Height: 80}},
							}},
						}},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							Label{Text: "Crie e teste à vontade. Clique em Salvar alterações quando terminar.", TextColor: muted, Font: Font{PointSize: 8}},
							HSpacer{}, PushButton{Text: "Importar fluxo...", OnClicked: a.chooseFlow}, PushButton{Text: "Salvar alterações", MinSize: Size{Width: 150}, Font: Font{Bold: true}, OnClicked: a.saveFlow},
						}},
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
	if a.qrHome != nil && a.chatHome != nil {
		showChats := auth
		a.qrHome.SetVisible(!showChats)
		a.chatHome.SetVisible(showChats)
		if showChats {
			a.refreshChats()
		}
	}
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
			a.ui(func() { walk.MsgBox(a.mw, "Não foi possível conectar", err.Error(), walk.MsgBoxIconError) })
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
		result, err := batch.LoadCSVFlexible(dlg.FilePath)
		if err != nil {
			walk.MsgBox(a.mw, "Lista inválida", "Não foi possível usar este arquivo:\r\n\r\n"+err.Error(), walk.MsgBoxIconError)
			return
		}
		approved := 0
		for _, item := range result.Items {
			if item.Consented {
				approved++
			}
		}
		a.csvImport = result
		a.csvPath.SetText(dlg.FilePath)
		if result.HasConsent {
			a.csvSummary.SetText(fmt.Sprintf("%d contato(s) • %d marcados com consentimento • %d serão ignorados", len(result.Items), approved, len(result.Items)-approved))
		} else {
			a.csvSummary.SetText(fmt.Sprintf("%d telefone(s) encontrados • confirme o consentimento abaixo", len(result.Items)))
		}
		if result.HasMessage && len(result.Items) > 0 {
			a.campaignMessage.SetText(result.Items[0].Message)
		}
		a.appendLog(fmt.Sprintf("Lista validada: %d telefones na coluna %q.", len(result.Items), result.PhoneField))
	}
}
func (a *application) chooseFlow() {
	dlg := new(walk.FileDialog)
	dlg.Title = "Importar fluxo de atendimento"
	dlg.Filter = "Fluxos YAML (*.yaml;*.yml)|*.yaml;*.yml|Todos os arquivos (*.*)|*.*"
	if ok, _ := dlg.ShowOpen(a.mw); ok {
		def, err := flow.LoadDefinition(dlg.FilePath)
		if err != nil {
			walk.MsgBox(a.mw, "Fluxo inválido", err.Error(), walk.MsgBoxIconError)
			return
		}
		a.cfg.Flow.RulesFile = dlg.FilePath
		a.flowDef = def
		a.defaultReply.SetText(def.DefaultReply)
		a.syncFlowModel(0)
		a.appendLog(fmt.Sprintf("Fluxo importado com %d regra(s).", len(def.Rules)))
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
		a.ui(func() {
			walk.MsgBox(a.mw, "Exportação concluída", fmt.Sprintf("%d contatos e %d grupos foram exportados para:\r\n%s", len(snap.Contacts), len(snap.Groups), dir), walk.MsgBoxIconInformation)
		})
	}()
}

func (a *application) send() {
	path := a.csvPath.Text()
	if path == "" {
		walk.MsgBox(a.mw, "Tino", "Selecione um arquivo CSV.", walk.MsgBoxIconWarning)
		return
	}
	result := a.csvImport
	if len(result.Items) == 0 {
		var err error
		result, err = batch.LoadCSVFlexible(path)
		if err != nil {
			walk.MsgBox(a.mw, "Lista inválida", err.Error(), walk.MsgBoxIconError)
			return
		}
	}
	items := append([]batch.Item(nil), result.Items...)
	message := strings.TrimSpace(a.campaignMessage.Text())
	if !result.HasMessage && message == "" {
		walk.MsgBox(a.mw, "Mensagem obrigatória", "Escreva a mensagem que será enviada para esta lista.", walk.MsgBoxIconWarning)
		return
	}
	if !result.HasConsent && !a.consentCheck.Checked() {
		walk.MsgBox(a.mw, "Confirmação necessária", "Confirme que os contatos autorizaram esta comunicação.", walk.MsgBoxIconWarning)
		return
	}
	for i := range items {
		if strings.TrimSpace(items[i].Message) == "" {
			items[i].Message = message
		}
		if !result.HasConsent {
			items[i].Consented = true
		}
	}
	approved := 0
	for _, item := range items {
		if item.Consented {
			approved++
		}
	}
	confirmation := fmt.Sprintf("A lista possui %d contato(s) com consentimento.\r\n\r\nO Tino enviará uma mensagem por vez, respeitando intervalos de %s a %s. Deseja continuar?", approved, a.cfg.Batch.MinInterval, a.cfg.Batch.MaxInterval)
	if walk.MsgBox(a.mw, "Revisar envio", confirmation, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	a.setBusy(true)
	go func() {
		defer a.setBusy(false)
		if err := a.ensureConnected(); err != nil {
			a.appendLog("Envio cancelado: " + err.Error())
			return
		}
		p := batch.Processor{Client: a.mgr.Client, MinInterval: a.cfg.Batch.MinInterval, MaxInterval: a.cfg.Batch.MaxInterval, MaxPerRun: a.cfg.Batch.MaxPerRun}
		runErr := p.Run(a.ctx, items, func(r batch.Result) { b, _ := json.Marshal(r); a.appendLog(string(b)) })
		if runErr != nil {
			a.appendLog("Processamento encerrado: " + runErr.Error())
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
	def := a.currentFlowDefinition()
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
		if err := flow.SaveDefinition(a.cfg.Flow.RulesFile, def); err != nil {
			a.appendLog("Não foi possível salvar o fluxo: " + err.Error())
			a.mu.Lock()
			a.flowStarted = false
			a.mu.Unlock()
			a.ui(func() { a.flowBtn.SetEnabled(true) })
			return
		}
		engine, err := flow.New(a.mgr.Client, def)
		if err != nil {
			a.appendLog("Fluxo inválido: " + err.Error())
			a.mu.Lock()
			a.flowStarted = false
			a.mu.Unlock()
			a.ui(func() { a.flowBtn.SetEnabled(true) })
			return
		}
		a.mgr.Client.AddEventHandler(engine.Handle)
		a.flowDef = def
		a.appendLog(fmt.Sprintf("Atendimento ativo com %d regra(s).", len(def.Rules)))
	}()
}
