package whatsapp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/catalog"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/config"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
	"github.com/gadevsbr/tino/internal/assistente/utils"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// Keys match the structured OmniBees result, never inferred from quote prose.
var catalogCategories = func() []catalog.Category {
	var result []catalog.Category
	for _, category := range omnibees.CategoryDefinitions() {
		result = append(result, catalog.Category{ID: category.Key, Name: category.Name})
	}
	return result
}()

func (s *Service) enableCommercial(ctx context.Context) error {
	if s.app == nil {
		return errors.New("operator application required")
	}
	var err error
	s.commercial, err = commercial.New(ctx, s.domainDB, s.app.PublicQuote)
	if err != nil {
		return err
	}
	s.catalogRepo = catalog.NewRepository(s.domainDB)
	if err := s.catalogRepo.EnsureSchema(ctx); err != nil {
		return err
	}
	s.catalog = catalog.NewService(s.catalogRepo, catalogCategories)
	if err := s.ensureTransportSchema(ctx); err != nil {
		return err
	}
	// Context-scoped cap bounds actual downloaded ciphertext, including servers
	// with dishonest or absent Content-Length. Other document downloads retain
	// their existing behavior.
	s.client.SetMediaHTTPClient(&http.Client{Transport: boundedMediaTransport{base: http.DefaultTransport}, Timeout: 45 * time.Second})
	return nil
}

func (s *Service) ensureTransportSchema(ctx context.Context) error {
	_, err := s.domainDB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS whatsapp_transport_sent(account TEXT NOT NULL,message_id TEXT NOT NULL,PRIMARY KEY(account,message_id));
CREATE TABLE IF NOT EXISTS whatsapp_catalog_deliveries(account TEXT NOT NULL,contact TEXT NOT NULL,quote_id TEXT NOT NULL,owner TEXT NOT NULL,product_id TEXT NOT NULL,category TEXT NOT NULL,state TEXT NOT NULL,PRIMARY KEY(account,contact,quote_id,owner,product_id));
CREATE TABLE IF NOT EXISTS whatsapp_commercial_tests(account TEXT NOT NULL,operator TEXT NOT NULL,PRIMARY KEY(account,operator));`)
	return err
}

func transportMessageKey(parts ...string) string {
	b, _ := json.Marshal(parts)
	h := sha256.Sum256(b)
	return "wa:" + hex.EncodeToString(h[:])
}

func (s *Service) accountJID() string {
	if s.client == nil || s.client.Store == nil || s.client.Store.ID == nil {
		return ""
	}
	return s.client.Store.ID.ToNonAD().String()
}

func (s *Service) wasSent(ctx context.Context, id string) bool {
	if _, ok := s.sent.Load(id); ok {
		return true
	}
	var n int
	err := s.domainDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM whatsapp_transport_sent WHERE account=? AND message_id=?`, s.accountJID(), id).Scan(&n)
	// A database failure must not turn our own outbound reply into an admin command.
	return err != nil || n > 0
}

func processableMessage(evt *events.Message) bool {
	if evt == nil || evt.Message == nil || evt.Info.ID == "" {
		return false
	}
	validChat := individualJID(evt.Info.Chat) || (evt.Info.IsGroup && evt.Info.Chat.User != "" && evt.Info.Chat.Server == types.GroupServer)
	if !validChat || !individualJID(evt.Info.Sender) {
		return false
	}
	m := evt.Message
	if m.GetProtocolMessage() != nil || m.GetReactionMessage() != nil || m.GetSenderKeyDistributionMessage() != nil {
		return false
	}
	return m.GetConversation() != "" || m.GetExtendedTextMessage() != nil || m.GetImageMessage() != nil || m.GetVideoMessage() != nil || m.GetAudioMessage() != nil || m.GetDocumentMessage() != nil || m.GetProductMessage() != nil
}

func individualJID(j types.JID) bool {
	return j.User != "" && (j.Server == types.DefaultUserServer || j.Server == types.LegacyUserServer || j.Server == types.HiddenUserServer)
}

func (s *Service) resolvePhone(ctx context.Context, primary, alternative types.JID) string {
	if phone := phoneUser(primary, alternative); phone != "" {
		return phone
	}
	if primary.Server != types.HiddenUserServer || s.client == nil || s.client.Store == nil || s.client.Store.LIDs == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	pn, err := s.client.Store.LIDs.GetPNForLID(ctx, primary.ToNonAD())
	if err != nil {
		return ""
	}
	return phoneUser(pn, types.EmptyJID)
}

func (s *Service) replyText(ctx context.Context, chat types.JID, text string) {
	if text == "" {
		return
	}
	if _, err := s.sendMessage(ctx, chat, &waE2E.Message{Conversation: proto.String(text)}); err != nil {
		slog.Error("commercial reply failed")
	}
}

func phoneArgument(raw string) (string, bool) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "+")
	if len(raw) < 10 || len(raw) > 15 {
		return "", false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return raw, true
}

func admitted(cfg commercial.Config, contact string) bool {
	if cfg.Mode == commercial.Public {
		return true
	}
	if cfg.Mode == commercial.Test {
		for _, number := range cfg.Allowlist {
			if number == contact {
				return true
			}
		}
	}
	return false
}

// Return the configured phone spelling when WhatsApp uses the Brazilian
// eight-digit variant. Never apply phone normalization to an unresolved LID.
func commercialContact(cfg commercial.Config, contact string) string {
	if _, valid := phoneArgument(contact); valid && cfg.Mode == commercial.Test {
		for _, number := range cfg.Allowlist {
			if config.SamePhoneNumber(number, contact) {
				return number
			}
		}
	}
	return contact
}

func (s *Service) handleCommercialOperator(ctx context.Context, account, operator, messageID, text string, product *waE2E.ProductMessage, chat types.JID) bool {
	if s.commercial == nil || s.catalog == nil {
		return false
	}
	n := utils.Normalize(text)
	var reply string
	var err error
	handled := true
	switch {
	case n == "comercial status" || n == "status comercial":
		var cfg commercial.Config
		cfg, err = s.commercial.Configuration(ctx, account)
		reply = fmt.Sprintf("Atendimento comercial: %s.\nPara configurar esta conta: comercial teste <telefone com DDI>, comercial ativar ou comercial desativar.\nCatálogo: configurar catalogo / status catalogo.\nRetomar hóspede: comercial retomar <telefone com DDI>.", cfg.Mode)
	case n == "comercial ativar" || n == "comercial desativar" || strings.HasPrefix(n, "comercial teste "):
		cfg := commercial.Config{Mode: commercial.Disabled}
		if n == "comercial ativar" {
			cfg.Mode = commercial.Public
		}
		if strings.HasPrefix(n, "comercial teste ") {
			phone, valid := phoneArgument(strings.TrimPrefix(n, "comercial teste "))
			if !valid {
				s.replyText(ctx, chat, "Use comercial teste <telefone com DDI, somente números>.")
				return true
			}
			cfg.Mode, cfg.Allowlist = commercial.Test, []string{phone}
		}
		err = s.commercial.Configure(ctx, account, cfg)
		if err == nil {
			_, err = s.domainDB.ExecContext(ctx, `DELETE FROM whatsapp_commercial_tests WHERE account=?`, account)
		}
		reply = fmt.Sprintf("Atendimento comercial: %s nesta conta. O catálogo deve ser configurado nesta conta. Para testar como operador, inclua seu número no modo teste e envie testar atendimento; para voltar, sair atendimento.", cfg.Mode)
	case strings.HasPrefix(n, "comercial retomar "):
		contact, valid := resumeArgument(strings.TrimPrefix(n, "comercial retomar "))
		if !valid {
			s.replyText(ctx, chat, "Use comercial retomar <telefone com DDI ou identificador do aviso>.")
			return true
		}
		var cfg commercial.Config
		cfg, err = s.commercial.Configuration(ctx, account)
		if err == nil {
			err = s.commercial.Resume(ctx, account, commercialContact(cfg, contact))
		}
		reply = "Atendimento automático retomado para esse contato."
	case n == "sair atendimento":
		_, err = s.domainDB.ExecContext(ctx, `DELETE FROM whatsapp_commercial_tests WHERE account=? AND operator=?`, account, operator)
		reply = "Teste encerrado. Comandos operacionais disponíveis."
	case n == "testar atendimento":
		var cfg commercial.Config
		cfg, err = s.commercial.Configuration(ctx, account)
		if err == nil && !admitted(cfg, commercialContact(cfg, operator)) {
			s.replyText(ctx, chat, "Primeiro configure comercial teste <seu telefone com DDI>. Depois envie testar atendimento. Para testar com hóspede externo, configure o número dele.")
			return true
		}
		if err == nil {
			_, err = s.domainDB.ExecContext(ctx, `INSERT OR IGNORE INTO whatsapp_commercial_tests(account,operator) VALUES (?,?)`, account, operator)
		}
		reply = "Teste comercial iniciado em sessão separada. Envie oi para começar; sair atendimento volta aos comandos operacionais."
	case strings.HasPrefix(n, "testar produto "):
		index, parseErr := strconv.Atoi(strings.TrimPrefix(n, "testar produto "))
		if parseErr != nil || index < 1 || index > len(catalogCategories) {
			s.replyText(ctx, chat, "Use testar produto <número da categoria de configurar catalogo>.")
			return true
		}
		var entry catalog.Entry
		entry, err = s.catalogRepo.Get(ctx, account, catalogCategories[index-1].ID)
		if err == nil {
			err = s.sendCatalogProduct(ctx, chat, entry)
		}
		reply = "Produto enviado somente para esta conversa."
	default:
		handled = false
	}
	if handled {
		if err != nil {
			reply = "Não foi possível concluir a operação comercial. Confira a configuração e tente novamente."
			slog.Error("commercial operator operation failed")
		}
		s.replyText(ctx, chat, reply)
		return true
	}
	input := catalog.Input{AccountJID: account, OperatorJID: operator, ChatJID: chat.ToNonAD().String(), Text: text, Authorized: true, Product: product}
	if product != nil {
		accept, checkErr := s.catalog.AcceptsProduct(ctx, account, operator, input.ChatJID)
		if checkErr != nil {
			s.replyText(ctx, chat, "Não foi possível consultar a configuração do catálogo.")
			return true
		}
		if !accept {
			s.replyText(ctx, chat, "Envie configurar catalogo antes de compartilhar o produto.")
			return true
		}
		input.Image, err = s.downloadProductImage(ctx, product)
		if err != nil {
			s.replyText(ctx, chat, "Não consegui receber a foto do produto. Compartilhe novamente um produto com foto JPEG/PNG de até 5 MB.")
			return true
		}
		if s.accountJID() != account {
			s.replyText(ctx, chat, "A conta do WhatsApp mudou durante a configuração. Inicie configurar catalogo novamente na conta atual.")
			return true
		}
	}
	result, err := s.catalog.Handle(ctx, input)
	if err != nil {
		s.replyText(ctx, chat, "Não foi possível concluir a configuração do catálogo. Tente novamente.")
		return true
	}
	if result.Handled {
		s.replyText(ctx, chat, result.Text)
		return true
	}
	var testing int
	err = s.domainDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM whatsapp_commercial_tests WHERE account=? AND operator=?`, account, operator).Scan(&testing)
	if err != nil {
		return true
	}
	if testing > 0 {
		s.handleGuest(ctx, account, operator, messageID, text, chat)
		return true
	}
	return false
}

func resumeArgument(raw string) (string, bool) {
	if phone, ok := phoneArgument(raw); ok {
		return phone, true
	}
	j, err := types.ParseJID(raw)
	return j.ToNonAD().String(), err == nil && j.Server == types.HiddenUserServer && j.User != ""
}

func (s *Service) handleGuest(ctx context.Context, account, contact, messageID, text string, chat types.JID) {
	if s.commercial == nil {
		return
	}
	cfg, err := s.commercial.Configuration(ctx, account)
	if err != nil {
		slog.Error("commercial configuration unavailable")
		return
	}
	contact = commercialContact(cfg, contact)
	if !admitted(cfg, contact) {
		return
	}
	result, err := s.commercial.Handle(ctx, commercial.Input{Account: account, Contact: contact, MessageID: messageID, Text: text, Now: time.Now()})
	if err != nil {
		slog.Error("commercial guest handling failed")
		return
	}
	if !result.Handled {
		return
	}
	if result.Handoff {
		if s.markUnread != nil {
			_ = s.markUnread(ctx, chat.String())
		}
		// No guest body or quote details are copied into notifications/logs.
		notice := "🔔 Hóspede solicitou atendimento humano.\nContato: " + contact + "\nO automático está pausado até retomada explícita.\nPara retomar: comercial retomar " + contact
		for _, number := range s.auth.Numbers() {
			s.replyText(ctx, types.NewJID(number, types.DefaultUserServer), notice)
		}
	}
	if result.GroupRequest != "" {
		destination := cfg.GroupPhone
		if destination == "" {
			destination = "5573988240413"
		}
		notice := result.GroupRequest + "\nContato solicitante: " + contact
		if _, err := s.sendMessage(ctx, types.NewJID(destination, types.DefaultUserServer), &waE2E.Message{Conversation: proto.String(notice)}); err != nil {
			slog.Error("group quote forwarding failed")
			s.replyText(ctx, chat, "Não consegui encaminhar ao setor de grupos agora. Um atendente humano continuará por aqui.")
			if s.markUnread != nil {
				_ = s.markUnread(ctx, chat.String())
			}
			return
		}
	}
	if result.Text != "" {
		if _, err := s.sendMessage(ctx, chat, &waE2E.Message{Conversation: proto.String(result.Text)}); err != nil {
			slog.Error("commercial quote reply failed")
			return
		}
	}
	for _, body := range result.Messages {
		if strings.TrimSpace(body) == "" {
			continue
		}
		if _, err := s.sendMessage(ctx, chat, &waE2E.Message{Conversation: proto.String(body)}); err != nil {
			slog.Error("commercial sequence reply failed")
			return
		}
	}
	if len(result.Categories) > 0 {
		if err := s.sendQuoteProducts(ctx, account, contact, messageID, chat, result.Categories); err != nil {
			slog.Error("commercial catalog delivery incomplete")
			s.replyText(ctx, chat, "Não consegui enviar todos os produtos do catálogo. O orçamento acima continua disponível; a equipe pode ajudar com as fotos.")
		}
	}
}

// Claim before sending: a crash or an ambiguous network failure must never
// automatically duplicate a product. The owner/product snapshot is durable.
func (s *Service) sendQuoteProducts(ctx context.Context, account, contact, quoteID string, chat types.JID, categories []omnibees.Category) error {
	var failures []error
	for _, category := range categories {
		entry, err := s.catalogRepo.Get(ctx, account, category.Key)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				failures = append(failures, err)
			}
			continue
		}
		productID := entry.Payload.GetProduct().GetProductID()
		res, err := s.domainDB.ExecContext(ctx, `INSERT OR IGNORE INTO whatsapp_catalog_deliveries(account,contact,quote_id,owner,product_id,category,state) VALUES (?,?,?,?,?,?,'attempted')`, account, contact, quoteID, entry.OwnerJID, productID, category.Key)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		n, err := res.RowsAffected()
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if n == 0 {
			continue
		}
		if err := s.sendCatalogProduct(ctx, chat, entry); err != nil {
			failures = append(failures, err)
			continue
		}
		_, err = s.domainDB.ExecContext(ctx, `UPDATE whatsapp_catalog_deliveries SET state='sent' WHERE account=? AND contact=? AND quote_id=? AND owner=? AND product_id=?`, account, contact, quoteID, entry.OwnerJID, productID)
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *Service) sendCatalogProduct(ctx context.Context, chat types.JID, entry catalog.Entry) error {
	if !individualJID(chat) || entry.Payload == nil || entry.Payload.GetProduct() == nil || len(entry.Image) == 0 || len(entry.Image) > catalog.MaxImageBytes {
		return errors.New("invalid catalog delivery")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	thumbnail, width, height, err := productThumbnail(entry.Image)
	if err != nil {
		return err
	}
	upload := s.uploadOverride
	if upload == nil {
		upload = s.client.Upload
	}
	up, err := upload(ctx, entry.Image, whatsmeow.MediaImage)
	if err != nil {
		return err
	}
	payload := proto.Clone(entry.Payload).(*waE2E.ProductMessage)
	payload.Product.ProductImage = &waE2E.ImageMessage{URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength), Mimetype: proto.String(http.DetectContentType(entry.Image))}
	payload.Product.ProductImage.JPEGThumbnail = thumbnail
	payload.Product.ProductImage.Width = proto.Uint32(width)
	payload.Product.ProductImage.Height = proto.Uint32(height)
	_, err = s.sendMessage(ctx, chat, &waE2E.Message{ProductMessage: payload})
	return err
}
