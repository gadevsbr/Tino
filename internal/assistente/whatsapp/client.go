package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
	qrCode "rsc.io/qr"

	"github.com/gadevsbr/tino/internal/assistente/advances"
	"github.com/gadevsbr/tino/internal/assistente/app"
	"github.com/gadevsbr/tino/internal/assistente/authorization"
	"github.com/gadevsbr/tino/internal/assistente/backup"
	"github.com/gadevsbr/tino/internal/assistente/cash"
	"github.com/gadevsbr/tino/internal/assistente/catalog"
	"github.com/gadevsbr/tino/internal/assistente/commands"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/config"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/queue"
	"github.com/gadevsbr/tino/internal/assistente/reports"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	"github.com/gadevsbr/tino/internal/assistente/utils"
)

type Service struct {
	client           *whatsmeow.Client
	store            *sqlstore.Container
	domainDB         *sql.DB
	app              *app.App
	auth             *authorization.Service
	queue            *queue.PerKey
	cfg              config.Config
	reports          *reports.Generator
	rooms            *rooms.Repository
	cash             *cash.Repository
	advances         *advances.Repository
	sent             sync.Map
	backup           *backup.Manager
	started          time.Time
	commercial       *commercial.Service
	catalog          *catalog.Service
	catalogRepo      *catalog.Repository
	sendOverride     func(context.Context, types.JID, *waE2E.Message) (whatsmeow.SendResponse, error)
	downloadOverride func(context.Context, whatsmeow.DownloadableMessage) ([]byte, error)
	uploadOverride   func(context.Context, []byte, whatsmeow.MediaType) (whatsmeow.UploadResponse, error)
	ownsClient       bool
	handlerID        uint32
}

func New(ctx context.Context, cfg config.Config, domainDB *sql.DB, application *app.App) (*Service, error) {
	if cfg.DNSServer != "" {
		if _, _, err := net.SplitHostPort(cfg.DNSServer); err != nil {
			return nil, fmt.Errorf("invalid DNS_SERVER: %w", err)
		}
		dialer := net.Dialer{Timeout: 5 * time.Second}
		net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "udp", cfg.DNSServer)
		}}
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, err
	}
	raw, err := sql.Open("sqlite", filepath.Join(cfg.DataDir, "whatsapp.db")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open whatsapp store: %w", err)
	}
	raw.SetMaxOpenConns(1)
	container := sqlstore.NewWithDB(raw, "sqlite3", nil)
	if err := container.Upgrade(ctx); err != nil {
		raw.Close()
		return nil, fmt.Errorf("upgrade whatsapp store: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		container.Close()
		return nil, err
	}
	auth, err := authorization.New(cfg, domainDB)
	if err != nil {
		container.Close()
		return nil, fmt.Errorf("load dynamic authorization: %w", err)
	}
	s := &Service{client: whatsmeow.NewClient(device, nil), store: container, domainDB: domainDB, app: application, auth: auth, queue: queue.New(), cfg: cfg, reports: reports.New(cfg.Timezone), rooms: rooms.NewRepository(domainDB, cfg.Timezone), cash: cash.NewRepository(domainDB, cfg.Timezone), advances: advances.NewRepository(domainDB, cfg.Timezone), backup: backup.New(domainDB, cfg.DataDir, cfg.BackupRetentionDays, cfg.Timezone), started: time.Now(), ownsClient: true}
	if err := s.enableCommercial(ctx); err != nil {
		container.Close()
		return nil, fmt.Errorf("initialize commercial transport: %w", err)
	}
	application.EnableOperations(s.runBackup, s.backupStatus, s.health)
	application.EnableMessaging(s.auth.Add, s.listGroups, s.sendReportsTo)
	s.handlerID = s.client.AddEventHandler(s.onEvent)
	return s, nil
}

// NewWithClient attaches the complete operational bot to an existing
// whatsmeow client. The caller owns the client and its session store.
func NewWithClient(ctx context.Context, cfg config.Config, domainDB *sql.DB, application *app.App, client *whatsmeow.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("whatsapp client is required")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, err
	}
	auth, err := authorization.New(cfg, domainDB)
	if err != nil {
		return nil, fmt.Errorf("load dynamic authorization: %w", err)
	}
	s := &Service{client: client, domainDB: domainDB, app: application, auth: auth, queue: queue.New(), cfg: cfg, reports: reports.New(cfg.Timezone), rooms: rooms.NewRepository(domainDB, cfg.Timezone), cash: cash.NewRepository(domainDB, cfg.Timezone), advances: advances.NewRepository(domainDB, cfg.Timezone), backup: backup.New(domainDB, cfg.DataDir, cfg.BackupRetentionDays, cfg.Timezone), started: time.Now()}
	if err := s.enableCommercial(ctx); err != nil {
		return nil, fmt.Errorf("initialize commercial transport: %w", err)
	}
	application.EnableOperations(s.runBackup, s.backupStatus, s.health)
	application.EnableMessaging(s.auth.Add, s.listGroups, s.sendReportsTo)
	s.handlerID = client.AddEventHandler(s.onEvent)
	return s, nil
}

// StartAttached starts scheduled jobs without reconnecting the shared client.
func (s *Service) StartAttached(ctx context.Context) { go s.scheduler(ctx) }

func (s *Service) Start(ctx context.Context) error {
	if s.client.Store.ID != nil {
		if err := s.client.Connect(); err != nil {
			return err
		}
		go s.scheduler(ctx)
		return nil
	}
	pairCtx, cancel := context.WithCancel(ctx)
	qr, err := s.client.GetQRChannel(pairCtx)
	if err != nil {
		cancel()
		return err
	}
	if err := s.client.Connect(); err != nil {
		cancel()
		return err
	}
	first, ok := <-qr
	if !ok {
		cancel()
		return fmt.Errorf("pairing channel closed")
	}
	if s.cfg.PairingPhone != "" {
		code, err := s.client.PairPhone(pairCtx, s.cfg.PairingPhone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
		if err != nil {
			cancel()
			return fmt.Errorf("generate pairing code: %w", err)
		}
		slog.Info("pairing code generated", "code", code)
	} else {
		if first.Event != "code" {
			cancel()
			return fmt.Errorf("unexpected pairing event: %s", first.Event)
		}
		if s.cfg.PairingQRPath != "" {
			code, err := qrCode.Encode(first.Code, qrCode.L)
			if err != nil {
				cancel()
				return fmt.Errorf("encode pairing QR: %w", err)
			}
			if err := os.WriteFile(s.cfg.PairingQRPath, code.PNG(), 0o600); err != nil {
				cancel()
				return fmt.Errorf("write pairing QR: %w", err)
			}
			slog.Info("pairing QR written", "path", s.cfg.PairingQRPath)
		} else {
			qrterminal.GenerateHalfBlock(first.Code, qrterminal.L, os.Stdout)
		}
	}
	go s.observePairing(pairCtx, cancel, qr)
	go s.scheduler(ctx)
	return nil
}

func (s *Service) runBackup(ctx context.Context) (string, error) {
	path, err := s.backup.Run(ctx, time.Now())
	if err != nil {
		return "", err
	}
	return "✅ Backup criado e verificado.\n\nArquivo: " + filepath.Base(path), nil
}
func (s *Service) backupStatus(ctx context.Context) (string, error) {
	st, err := s.backup.Latest(ctx)
	if err == sql.ErrNoRows {
		return "📦 Nenhum backup registrado ainda.", nil
	}
	if err != nil {
		return "", err
	}
	return "📦 Último backup\n\nArquivo: " + filepath.Base(st.Path) + "\nCriado em: " + st.CreatedAt, nil
}
func (s *Service) health(ctx context.Context) string {
	dbState := "OK"
	if err := s.domainDB.PingContext(ctx); err != nil {
		dbState = "ERRO: " + err.Error()
	}
	wa := "desconectado"
	if s.client.IsConnected() {
		wa = "conectado"
	}
	backupState := "nenhum"
	if st, err := s.backup.Latest(ctx); err == nil {
		backupState = filepath.Base(st.Path)
	}
	return fmt.Sprintf("🩺 SAÚDE DO BOT\n\nWhatsApp: %s\nBanco: %s\nUptime: %s\nÚltimo backup: %s\nResumo automático: %02d:00", wa, dbState, time.Since(s.started).Round(time.Minute), backupState, s.cfg.DailySummaryHour)
}

func (s *Service) scheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	s.runScheduled(ctx, time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.runScheduled(ctx, now)
		}
	}
}
func (s *Service) runScheduled(ctx context.Context, now time.Time) {
	local := now.In(s.cfg.Timezone)
	date := local.Format("2006-01-02")
	if local.Hour() >= s.cfg.BackupHour && !s.settingDone(ctx, "backup:"+date) {
		if _, err := s.backup.Run(ctx, now); err != nil {
			slog.Error("automatic backup", "error", err)
		} else {
			s.markSetting(ctx, "backup:"+date)
		}
	}
	if local.Hour() >= s.cfg.DailySummaryHour && !s.settingDone(ctx, "summary:"+date) {
		roomsText, err1 := s.app.Handle(ctx, "system-summary", "summary-rooms-"+date, "status")
		cashText, err2 := s.app.Handle(ctx, "system-summary", "summary-cash-"+date, "caixa hoje")
		if err1 != nil || err2 != nil {
			slog.Error("daily summary build", "rooms", err1, "cash", err2)
			return
		}
		message := "🕔 RESUMO AUTOMÁTICO DAS 17H\n\n" + roomsText + "\n\n" + cashText
		ok := true
		for _, number := range s.auth.Numbers() {
			if _, err := s.sendMessage(ctx, types.NewJID(number, types.DefaultUserServer), &waE2E.Message{Conversation: proto.String(message)}); err != nil {
				ok = false
				slog.Error("daily summary send", "error", err)
			}
		}
		if ok {
			s.markSetting(ctx, "summary:"+date)
		}
	}
}
func (s *Service) settingDone(ctx context.Context, key string) bool {
	var n int
	_ = s.domainDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM settings WHERE key=?`, key).Scan(&n)
	return n > 0
}
func (s *Service) markSetting(ctx context.Context, key string) {
	_, _ = s.domainDB.ExecContext(ctx, `INSERT OR REPLACE INTO settings(key,value,updated_at) VALUES (?,'done',strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, key)
}

func (s *Service) observePairing(ctx context.Context, cancel context.CancelFunc, ch <-chan whatsmeow.QRChannelItem) {
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-ch:
			if !ok {
				return
			}
			slog.Info("pairing event", "event", item.Event)
		}
	}
}
func (s *Service) Close() {
	if !s.ownsClient && s.client != nil && s.handlerID != 0 {
		s.client.RemoveEventHandler(s.handlerID)
	}
	if s.ownsClient {
		s.client.Disconnect()
	}
	if s.store != nil {
		_ = s.store.Close()
	}
}

func (s *Service) onEvent(raw any) {
	switch raw.(type) {
	case *events.Connected:
		slog.Info("whatsapp connected")
	case *events.Disconnected:
		slog.Warn("whatsapp disconnected")
	}
	evt, ok := raw.(*events.Message)
	if !ok || !processableMessage(evt) {
		return
	}
	selfChat := isSelfChat(s.client.Store.ID, s.client.Store.LID, evt.Info.Chat, evt.Info.IsFromMe)
	if evt.Info.IsFromMe {
		if s.wasSent(context.Background(), string(evt.Info.ID)) {
			return
		}
		// Commands typed by the account owner in a group are valid operator
		// commands. Outbound messages to another private chat remain ignored.
		if !selfChat && !evt.Info.IsGroup {
			return
		}
	}
	sender := s.resolvePhone(context.Background(), evt.Info.Sender, evt.Info.SenderAlt)
	if selfChat && s.client.Store.ID != nil {
		sender = s.client.Store.ID.User
	}
	slog.Info("incoming message metadata", "addressing_mode", evt.Info.AddressingMode, "sender_server", evt.Info.Sender.Server, "has_phone_alt", evt.Info.SenderAlt.Server == types.DefaultUserServer)
	text := evt.Message.GetConversation()
	if text == "" && evt.Message.GetExtendedTextMessage() != nil {
		text = evt.Message.GetExtendedTextMessage().GetText()
	}
	operator := evt.Info.IsFromMe || selfChat || (sender != "" && s.auth.Allowed(sender))
	// Group conversations never enter the public guest flow. Only the account
	// owner or an explicitly authorized operator can trigger commands there.
	if evt.Info.IsGroup && !operator {
		return
	}
	document := evt.Message.GetDocumentMessage()
	imageMessage := evt.Message.GetImageMessage()
	product := evt.Message.GetProductMessage()
	if operator && text == "" && document == nil && imageMessage == nil && product == nil {
		return
	}
	account := s.accountJID()
	if account == "" {
		return
	}
	contact := sender
	if contact == "" {
		contact = evt.Info.Sender.ToNonAD().String()
	}
	// Serialize configuration, resume and guest replies for this account as well
	// as individual operator sessions, so disabling cannot race an in-flight quote.
	s.queue.Do("transport:"+account, func() {
		ctx := context.Background()
		claimed, err := database.ClaimMessage(ctx, s.domainDB, transportMessageKey(account, contact, string(evt.Info.ID)), contact)
		if err != nil {
			slog.Error("claim message", "error", err)
			return
		}
		if !claimed {
			return
		}
		if !operator {
			s.handleGuest(ctx, account, contact, string(evt.Info.ID), text, evt.Info.Chat)
			return
		}
		if s.handleCommercialOperator(ctx, account, sender, string(evt.Info.ID), text, product, evt.Info.Chat) {
			return
		}
		if isCashReceiptImage(imageMessage) {
			reply, handled, err := s.receiveCashReceiptImage(ctx, sender, imageMessage, evt.Info.Chat)
			if err != nil {
				slog.Error("read cash receipt image", "error", err)
				reply = "Não consegui ler a foto do comprovante. Confira a imagem e tente novamente."
			}
			if handled && reply != "" {
				_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String(reply)})
			}
			if handled {
				return
			}
		}
		if document != nil {
			reply, handled, err := s.receiveCashReceiptPDF(ctx, sender, document)
			if err != nil {
				slog.Error("read cash receipt PDF", "error", err)
				reply = "Não consegui ler o PDF do comprovante. Confira se é um PDF válido e tente novamente."
			}
			if handled && reply != "" {
				_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String(reply)})
			}
			if handled {
				return
			}
		}
		if reply, handled, err := s.handleCashReceiptText(ctx, sender, text, evt.Info.Chat); handled {
			if err != nil {
				slog.Error("handle cash receipt", "error", err)
				reply = "Não consegui continuar o lançamento do comprovante. Tente `cancelar comprovante` e reenvie a foto."
			}
			if reply != "" {
				_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String(reply)})
			}
			return
		}
		if document != nil {
			if reply, handled, err := s.receiveExtratoDocument(ctx, sender, document); handled {
				if err != nil {
					slog.Error("receive extrato document", "error", err)
					reply = "❌ Não consegui receber o arquivo. Confira o formato e tente novamente."
				}
				if reply != "" {
					_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String(reply)})
				}
				return
			}
		}
		if reply, handled, err := s.handleExtratosText(ctx, sender, text, evt.Info.Chat); handled {
			if err != nil {
				slog.Error("process extratos", "error", err)
				reply = "❌ Não consegui processar os extratos. Confira os arquivos e tente novamente."
			}
			if reply != "" {
				_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String(reply)})
			}
			return
		}
		if text == "" {
			return
		}
		parsedCommand := commands.Parse(text)
		if parsedCommand.Kind == commands.ReceiptList {
			if err := s.sendReceiptList(ctx, evt.Info.Chat, parsedCommand.Date); err != nil {
				slog.Error("list cash receipts", "error", err)
				_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String("Não consegui consultar os comprovantes.")})
			}
			return
		}
		if parsedCommand.Kind == commands.ReceiptGet {
			if err := s.sendStoredReceipt(ctx, evt.Info.Chat, parsedCommand.ReceiptID); err != nil {
				slog.Error("send stored cash receipt", "error", err)
				_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String("Não consegui recuperar esse comprovante.")})
			}
			return
		}
		if isReportCommand(text) {
			if err := s.sendReport(ctx, evt.Info.Chat); err != nil {
				slog.Error("send report", "error", err)
				_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String("❌ Não foi possível gerar o relatório.")})
				return
			}
			// The generic request is intentionally complete: send the room report
			// and the current month's imported cash report when it exists.
			if err := s.sendCashReport(ctx, evt.Info.Chat); err != nil {
				slog.Error("send cash report", "error", err)
				_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String("❌ O relatório de quartos foi enviado, mas não consegui gerar o relatório de caixa.")})
			}
			return
		}
		if report := commands.Parse(text); report.Kind == commands.CashReport {
			if err := s.sendCashReportFor(ctx, evt.Info.Chat, report.Date, report.EndDate); err != nil {
				slog.Error("send cash report", "error", err)
				_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String("❌ Não foi possível gerar o relatório de caixa.")})
			}
			return
		}
		if report := commands.Parse(text); report.Kind == commands.AdvancePDF {
			if err := s.sendAdvanceReport(ctx, evt.Info.Chat, ""); err != nil {
				slog.Error("send advances report", "error", err)
				_, _ = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String("❌ Não foi possível gerar o PDF de vales.")})
			}
			return
		}
		reply, err := s.app.Handle(ctx, sender, string(evt.Info.ID), text)
		if err != nil {
			slog.Error("handle message", "error", err)
			return
		}
		if reply == "" {
			return
		}
		if _, err = s.sendMessage(ctx, evt.Info.Chat, &waE2E.Message{Conversation: proto.String(reply)}); err != nil {
			slog.Error("send reply", "error", err)
		}
		if commands.Parse(text).Kind == commands.CashClose && strings.HasPrefix(reply, "🔒 CAIXA FECHADO") {
			if err := s.sendCashReport(ctx, evt.Info.Chat); err != nil {
				slog.Error("send closing cash report", "error", err)
			}
		}
		s.notifyOperators(ctx, sender, string(evt.Info.ID), text, reply)
	})
}

func isCashReceiptImage(message *waE2E.ImageMessage) bool {
	if message == nil {
		return false
	}
	caption := " " + utils.Normalize(message.GetCaption()) + " "
	return strings.Contains(caption, " comprovante ")
}

func (s *Service) sendReceiptList(ctx context.Context, to types.JID, date string) error {
	if date == "" {
		date = time.Now().In(s.cfg.Timezone).Format("2006-01-02")
	}
	items, err := s.cash.ReceiptsByDate(ctx, date)
	if err != nil {
		return err
	}
	formattedDate := formatReceiptDate(date)
	if len(items) == 0 {
		_, err = s.sendMessage(ctx, to, &waE2E.Message{Conversation: proto.String("Nenhum comprovante registrado em " + formattedDate + ".")})
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "🧾 COMPROVANTES — %s\n", formattedDate)
	for _, item := range items {
		availability := "arquivo disponível"
		if item.FilePath == "" {
			availability = "sem arquivo antigo"
		}
		fmt.Fprintf(&b, "\n#%d — %s — %s\n%s\n%s", item.ID, item.Method, cash.Money(item.Cents), item.Description, availability)
	}
	b.WriteString("\n\nPara receber o original: `comprovante ID`.")
	_, err = s.sendMessage(ctx, to, &waE2E.Message{Conversation: proto.String(b.String())})
	return err
}

func (s *Service) sendStoredReceipt(ctx context.Context, to types.JID, id int64) error {
	item, err := s.cash.Receipt(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.sendMessage(ctx, to, &waE2E.Message{Conversation: proto.String("Comprovante não encontrado.")})
		return err
	}
	if err != nil {
		return err
	}
	if item.FilePath == "" {
		_, err = s.sendMessage(ctx, to, &waE2E.Message{Conversation: proto.String("Esse lançamento é anterior ao armazenamento de arquivos; somente os dados financeiros estão disponíveis.")})
		return err
	}
	base, err := filepath.Abs(filepath.Join(s.cfg.DataDir, "comprovantes"))
	if err != nil {
		return err
	}
	path, err := filepath.Abs(filepath.Join(s.cfg.DataDir, item.FilePath))
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid stored receipt path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var uploaded whatsmeow.UploadResponse
	if s.uploadOverride != nil {
		uploaded, err = s.uploadOverride(ctx, data, whatsmeow.MediaDocument)
	} else {
		uploaded, err = s.client.Upload(ctx, data, whatsmeow.MediaDocument)
	}
	if err != nil {
		return err
	}
	mime := item.MediaType
	if mime == "" {
		mime = "application/octet-stream"
	}
	ext := filepath.Ext(path)
	name := fmt.Sprintf("comprovante-%d-%s%s", item.ID, strings.ReplaceAll(item.BusinessDate, "-", ""), ext)
	caption := fmt.Sprintf("🧾 Comprovante #%d — %s — %s\n%s", item.ID, item.Method, cash.Money(item.Cents), item.Description)
	doc := &waE2E.DocumentMessage{URL: &uploaded.URL, DirectPath: &uploaded.DirectPath, MediaKey: uploaded.MediaKey, FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256, FileLength: &uploaded.FileLength, Mimetype: proto.String(mime), FileName: proto.String(name), Title: proto.String(name), Caption: proto.String(caption)}
	_, err = s.sendMessage(ctx, to, &waE2E.Message{DocumentMessage: doc})
	return err
}

func (s *Service) sendAdvanceReport(ctx context.Context, to types.JID, employee string) error {
	return s.sendAdvanceReportForMonth(ctx, to, employee, time.Now().In(s.cfg.Timezone))
}

func (s *Service) sendAdvanceReportForMonth(ctx context.Context, to types.JID, employee string, month time.Time) error {
	now := month.In(s.cfg.Timezone)
	items, err := s.advances.Month(ctx, employee, now)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		_, err = s.sendMessage(ctx, to, &waE2E.Message{Conversation: proto.String("Nenhum vale encontrado em " + now.Format("01/2006") + ".")})
		return err
	}
	data, err := advances.PDF(items, now.Format("01/2006"), employee, s.cfg.Timezone)
	if err != nil {
		return err
	}
	up, err := s.client.Upload(ctx, data, whatsmeow.MediaDocument)
	if err != nil {
		return err
	}
	name := "Vales - " + now.Format("01-2006") + ".pdf"
	doc := &waE2E.DocumentMessage{URL: &up.URL, DirectPath: &up.DirectPath, MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: &up.FileLength, Mimetype: proto.String("application/pdf"), FileName: &name, Title: &name, PageCount: proto.Uint32(1), Caption: proto.String("💵 Relatório mensal de vales.")}
	_, err = s.sendMessage(ctx, to, &waE2E.Message{DocumentMessage: doc})
	return err
}

// ShareReport exposes the same audited WhatsApp delivery used by chat commands
// to the desktop UI. Kind accepts ROOMS, CASH or ADVANCES.
func (s *Service) ShareReport(ctx context.Context, kind string, to types.JID, from, until, employee string) error {
	switch strings.ToUpper(strings.TrimSpace(kind)) {
	case "ROOMS":
		return s.sendReport(ctx, to)
	case "CASH":
		return s.sendCashReportFor(ctx, to, from, until)
	case "ADVANCES":
		month := time.Now().In(s.cfg.Timezone)
		if strings.TrimSpace(from) != "" {
			parsed, err := time.ParseInLocation("2006-01", from, s.cfg.Timezone)
			if err != nil {
				return fmt.Errorf("mês inválido: %w", err)
			}
			month = parsed
		}
		return s.sendAdvanceReportForMonth(ctx, to, employee, month)
	default:
		return fmt.Errorf("tipo de relatório inválido: %s", kind)
	}
}

func (s *Service) sendMessage(ctx context.Context, to types.JID, message *waE2E.Message) (whatsmeow.SendResponse, error) {
	if s.sendOverride != nil {
		return s.sendOverride(ctx, to, message)
	}
	id := s.client.GenerateMessageID()
	key := string(id)
	if _, err := s.domainDB.ExecContext(ctx, `INSERT OR IGNORE INTO whatsapp_transport_sent(account,message_id) VALUES (?,?)`, s.accountJID(), key); err != nil {
		return whatsmeow.SendResponse{}, err
	}
	s.sent.Store(key, struct{}{})
	resp, err := s.client.SendMessage(ctx, to, message, whatsmeow.SendRequestExtra{ID: id})
	if err != nil {
		s.sent.Delete(key)
		return resp, err
	}
	time.AfterFunc(10*time.Minute, func() { s.sent.Delete(key) })
	return resp, err
}

func isSelfChat(id *types.JID, lid, chat types.JID, fromMe bool) bool {
	if !fromMe || id == nil {
		return false
	}
	return chat.User != "" && (chat.ToNonAD() == id.ToNonAD() || (lid.User != "" && chat.ToNonAD() == lid.ToNonAD()))
}

func (s *Service) sendCashReport(ctx context.Context, to types.JID) error {
	return s.sendCashReportFor(ctx, to, "", "")
}

func (s *Service) sendCashReportFor(ctx context.Context, to types.JID, from, until string) error {
	from, until = cashReportRange(from, until, time.Now().In(s.cfg.Timezone))
	days, err := s.cash.Range(ctx, from, until)
	if err != nil {
		return err
	}
	if len(days) == 0 {
		_, err = s.sendMessage(ctx, to, &waE2E.Message{Conversation: proto.String("Nenhum caixa registrado nesse dia ou período.")})
		return err
	}
	data, err := cash.RangePDF(days, s.cfg.Timezone)
	if err != nil {
		return err
	}
	up, err := s.client.Upload(ctx, data, whatsmeow.MediaDocument)
	if err != nil {
		return err
	}
	name := cashReportFilenameDate(from) + ".pdf"
	if until != from {
		name = cashReportFilenameDate(from) + " a " + cashReportFilenameDate(until) + ".pdf"
	}
	pages := uint32(len(days))
	if len(days) > 1 {
		pages++
	}
	caption := "💰 Relatório diário do caixa."
	if until != from {
		caption = "💰 Relatório de caixa do período."
	}
	doc := &waE2E.DocumentMessage{URL: &up.URL, DirectPath: &up.DirectPath, MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: &up.FileLength, Mimetype: proto.String("application/pdf"), FileName: &name, Title: &name, PageCount: &pages, Caption: proto.String(caption)}
	_, err = s.sendMessage(ctx, to, &waE2E.Message{DocumentMessage: doc})
	return err
}

func cashReportRange(from, until string, now time.Time) (string, string) {
	if from == "" {
		// A request without dates means the current month, matching the financial
		// dashboard and including PDFs imported earlier in the month.
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		until = now.Format("2006-01-02")
		return from, until
	}
	if until == "" {
		until = from
	}
	return from, until
}

func cashReportFilenameDate(value string) string {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return value
	}
	return date.Format("02-01-2006")
}

func (s *Service) notifyOperators(ctx context.Context, actor, messageID, input, reply string) {
	changes, err := s.rooms.ChangesByMessage(ctx, messageID)
	if err != nil {
		slog.Error("load notification changes", "error", err)
		return
	}
	var notice string
	if len(changes) > 0 {
		lines := make([]string, 0, len(changes))
		for _, change := range changes {
			line := fmt.Sprintf("• Quarto %d: %s → %s", change.Number, change.Old.StringWithIcon(), change.New.StringWithIcon())
			if change.New == rooms.Entry || change.New == rooms.OccupiedClean {
				line += fmt.Sprintf(" — 👥 %d pessoas", change.GuestCount)
			}
			lines = append(lines, line)
		}
		notice = "🔔 ATUALIZAÇÃO DE QUARTOS\n\n" + fmt.Sprintf("Operador: %s\n", actor) + strings.Join(lines, "\n")
	} else if commands.Parse(input).Kind == commands.CompleteCleaning && strings.HasPrefix(reply, "✅") {
		notice = "🔔 MANUTENÇÃO CONCLUÍDA\n\n" + reply + "\n\nOperador: " + actor
	} else {
		return
	}
	for _, number := range s.auth.Numbers() {
		if config.SamePhoneNumber(number, actor) {
			continue
		}
		to := types.NewJID(number, types.DefaultUserServer)
		if _, err := s.sendMessage(ctx, to, &waE2E.Message{Conversation: proto.String(notice)}); err != nil {
			slog.Error("notify operator", "error", err)
		}
	}
}

func isReportCommand(text string) bool {
	switch utils.Normalize(text) {
	case "relatorio em pdf", "relatorio pdf", "pdf", "gerar pdf", "gerar relatorio":
		return true
	}
	return false
}
func (s *Service) sendReport(ctx context.Context, to types.JID) error {
	list, err := s.rooms.ListAll(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	if _, err := s.sendMessage(ctx, to, &waE2E.Message{Conversation: proto.String(reports.Summary(list, now, s.cfg.Timezone))}); err != nil {
		return fmt.Errorf("send written report summary: %w", err)
	}
	data, err := s.reports.Generate(list, now)
	if err != nil {
		return err
	}
	up, err := s.client.Upload(ctx, data, whatsmeow.MediaDocument)
	if err != nil {
		return err
	}
	name := "Relatorio de Quartos - " + now.In(s.cfg.Timezone).Format("02-01-2006") + ".pdf"
	pages := uint32(1)
	doc := &waE2E.DocumentMessage{URL: &up.URL, DirectPath: &up.DirectPath, MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: &up.FileLength, Mimetype: proto.String("application/pdf"), FileName: &name, Title: &name, PageCount: &pages, Caption: proto.String("🏨 Relatório atualizado dos quartos.")}
	_, err = s.sendMessage(ctx, to, &waE2E.Message{DocumentMessage: doc})
	return err
}

func (s *Service) listGroups(ctx context.Context) ([]app.GroupOption, error) {
	if !s.client.IsConnected() || !s.client.IsLoggedIn() {
		return nil, fmt.Errorf("WhatsApp não está conectado para consultar os grupos")
	}
	groups, err := s.client.GetJoinedGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar grupos do WhatsApp: %w", err)
	}
	result := make([]app.GroupOption, 0, len(groups))
	for _, group := range groups {
		if group == nil || group.JID.User == "" || strings.TrimSpace(group.Name) == "" {
			continue
		}
		result = append(result, app.GroupOption{JID: group.JID.String(), Name: strings.TrimSpace(group.Name)})
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}

func (s *Service) sendReportsTo(ctx context.Context, kind string, phones, groupIDs []string) (string, error) {
	targets := make([]types.JID, 0, len(phones)+len(groupIDs))
	for _, number := range phones {
		targets = append(targets, types.NewJID(number, types.DefaultUserServer))
	}
	for _, raw := range groupIDs {
		jid, err := types.ParseJID(raw)
		if err != nil || jid.Server != types.GroupServer {
			return "", fmt.Errorf("grupo inválido: %s", raw)
		}
		targets = append(targets, jid)
	}
	for _, target := range targets {
		if kind == "ROOMS" || kind == "BOTH" {
			if err := s.sendReport(ctx, target); err != nil {
				return "", fmt.Errorf("enviar relatório de quartos para %s: %w", target, err)
			}
		}
		if kind == "CASH" || kind == "BOTH" {
			if err := s.sendCashReport(ctx, target); err != nil {
				return "", fmt.Errorf("enviar relatório de caixa para %s: %w", target, err)
			}
		}
	}
	return fmt.Sprintf("✅ Relatório(s) enviado(s) para %d destino(s): %d telefone(s) e %d grupo(s).", len(targets), len(phones), len(groupIDs)), nil
}

func phoneUser(primary, alternative types.JID) string {
	if primary.Server == types.DefaultUserServer || primary.Server == types.LegacyUserServer {
		return primary.User
	}
	if alternative.Server == types.DefaultUserServer || alternative.Server == types.LegacyUserServer {
		return alternative.User
	}
	return ""
}

// Keep command diagnostics short and avoid logging arbitrary guest messages.
func diagnosticCommandPreview(text string) string {
	clean := strings.Join(strings.Fields(text), " ")
	lower := strings.ToLower(clean)
	if !strings.HasPrefix(lower, "extrat") && !strings.HasPrefix(lower, "orçament") && !strings.HasPrefix(lower, "orcament") {
		return ""
	}
	runes := []rune(clean)
	if len(runes) > 120 {
		return string(runes[:120]) + "…"
	}
	return clean
}
