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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/bitz"
	"github.com/gadevsbr/tino/internal/assistente/catalog"
	"github.com/gadevsbr/tino/internal/assistente/commands"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/config"
	"github.com/gadevsbr/tino/internal/assistente/extratos"
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

func (s *Service) SetGuestAssistant(fn commercial.AssistantFunc) { s.commercial.SetAssistant(fn) }

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
	s.bitzStore = bitz.NewStore(filepath.Join(s.cfg.DataDir, "bitz-config.json"))
	s.bitzJobs, err = bitz.NewJobs(ctx, s.domainDB)
	if err != nil {
		return err
	}
	s.bitzRunner = bitz.NewBrowserRunner()
	if err := s.ensureTransportSchema(ctx); err != nil {
		return err
	}
	// Context-scoped cap bounds actual downloaded ciphertext, including servers
	// with dishonest or absent Content-Length. Other document downloads retain
	// their existing behavior.
	s.client.SetMediaHTTPClient(&http.Client{Transport: boundedMediaTransport{base: http.DefaultTransport}, Timeout: 45 * time.Second})
	return nil
}

func (s *Service) bitzApprovalCommand(ctx context.Context, sender, text string) bool {
	if s.bitzStore == nil || sender == "" {
		return false
	}
	n := utils.Normalize(text)
	if !strings.HasPrefix(n, "aprovar pre reserva ") && !strings.HasPrefix(n, "aprovar pre-reserva ") {
		return false
	}
	cfg, err := s.bitzStore.Public(ctx)
	return err == nil && cfg.ApproverPhone != "" && config.SamePhoneNumber(cfg.ApproverPhone, sender)
}

func approvalCode(text string) string {
	n := utils.Normalize(text)
	n = strings.TrimPrefix(n, "aprovar pre reserva ")
	n = strings.TrimPrefix(n, "aprovar pre-reserva ")
	return strings.ToUpper(strings.TrimSpace(n))
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

// AllowsFlow keeps operator commands and commercial conversations with their owner.
func (s *Service) AllowsFlow(evt *events.Message) bool {
	if !processableMessage(evt) || evt.Info.IsFromMe || evt.Info.IsGroup {
		return false
	}
	contact := s.resolvePhone(context.Background(), evt.Info.Sender, evt.Info.SenderAlt)
	if contact != "" && s.auth.Allowed(contact) {
		return false
	}
	if s.commercial == nil {
		return true
	}
	cfg, err := s.commercial.Configuration(context.Background(), s.accountJID())
	if err != nil {
		return false
	}
	if contact == "" {
		contact = evt.Info.Sender.ToNonAD().String()
	}
	return !admitted(cfg, commercialContact(cfg, contact))
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
	case s.bitzApprovalCommand(ctx, operator, text):
		var job bitz.Job
		job, err = s.bitzJobs.Approve(ctx, approvalCode(text))
		if err == nil {
			var cfg bitz.PublicConfig
			cfg, err = s.bitzStore.Public(ctx)
			if err == nil {
				var guest types.JID
				guest, err = types.ParseJID(job.ChatJID)
				if err == nil {
					s.replyText(ctx, guest, cfg.ApprovedText)
				}
			}
		}
		reply = "Pré-reserva aprovada. A mensagem final foi enviada ao hóspede."
	case n == "comercial status" || n == "status comercial":
		var cfg commercial.Config
		cfg, err = s.commercial.Configuration(ctx, account)
		reply = fmt.Sprintf("Atendimento comercial: %s.\nPara configurar esta conta: comercial teste <telefone com DDI>, comercial ativar ou comercial desativar.\nCatálogo: configurar catalogo / status catalogo.\nRetomar hóspede: comercial retomar <telefone com DDI>.", cfg.Mode)
	case n == "comercial ativar" || n == "comercial desativar" || strings.HasPrefix(n, "comercial teste "):
		cfg, configErr := s.commercial.Configuration(ctx, account)
		if configErr != nil {
			err = configErr
			break
		}
		cfg = commercialModeConfig(cfg, n)
		if strings.HasPrefix(n, "comercial teste ") {
			phone, valid := phoneArgument(strings.TrimPrefix(n, "comercial teste "))
			if !valid {
				s.replyText(ctx, chat, "Use comercial teste <telefone com DDI, somente números>.")
				return true
			}
			cfg.Allowlist = []string{phone}
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
		if err == nil {
			err = s.commercial.Reset(ctx, account, commercialTestContact(operator))
		}
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
		if err == nil {
			err = s.commercial.Reset(ctx, account, commercialTestContact(operator))
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
		if administrativeTestCommand(text) {
			_, err = s.domainDB.ExecContext(ctx, `DELETE FROM whatsapp_commercial_tests WHERE account=? AND operator=?`, account, operator)
			if err == nil {
				err = s.commercial.Reset(ctx, account, commercialTestContact(operator))
			}
			if err != nil {
				slog.Error("end commercial test for operational command", "error", err)
				s.replyText(ctx, chat, "Não consegui encerrar o teste de atendimento. Tente sair atendimento e reenvie o comando.")
				return true
			}
			return false
		}
		s.handleGuestAs(ctx, account, commercialTestContact(operator), operator, messageID, text, chat, true, false)
		return true
	}
	return false
}

// Named operational commands exit the operator's guest simulation. Numeric
// answers and quote controls remain inputs of that isolated simulation.
func administrativeTestCommand(text string) bool {
	n := utils.Normalize(text)
	_, weekly, _ := extratos.ParseWeeklyCommand(text, time.Now())
	if isReportCommand(text) || n == "extratos" || n == "processar extratos" || n == "cancelar extratos" || weekly {
		return true
	}
	switch commands.Parse(text).Kind {
	case commands.Unknown, commands.StartQuote, commands.Pause, commands.Continue, commands.Skip:
		return false
	case commands.RoomQuery:
		return strings.HasPrefix(n, "status ") || strings.HasPrefix(n, "quarto ")
	default:
		return true
	}
}

func commercialModeConfig(current commercial.Config, normalizedCommand string) commercial.Config {
	current.Allowlist = nil
	switch {
	case normalizedCommand == "comercial ativar":
		current.Mode = commercial.Public
	case strings.HasPrefix(normalizedCommand, "comercial teste "):
		current.Mode = commercial.Test
	default:
		current.Mode = commercial.Disabled
	}
	return current
}

func resumeArgument(raw string) (string, bool) {
	if phone, ok := phoneArgument(raw); ok {
		return phone, true
	}
	j, err := types.ParseJID(raw)
	return j.ToNonAD().String(), err == nil && j.Server == types.HiddenUserServer && j.User != ""
}

func (s *Service) handleGuest(ctx context.Context, account, contact, messageID, text string, chat types.JID, audio bool) {
	s.handleGuestAs(ctx, account, contact, contact, messageID, text, chat, false, audio)
}

func commercialTestContact(operator string) string {
	return "operator-test:" + operator
}

func (s *Service) handleGuestAs(ctx context.Context, account, stateContact, displayContact, messageID, text string, chat types.JID, testSession, audio bool) {
	if s.commercial == nil {
		return
	}
	cfg, err := s.commercial.Configuration(ctx, account)
	if err != nil {
		slog.Error("commercial configuration unavailable")
		return
	}
	if !testSession {
		stateContact = commercialContact(cfg, stateContact)
		displayContact = stateContact
	}
	if !testSession && !admitted(cfg, stateContact) {
		return
	}
	result, err := s.commercial.Handle(ctx, commercial.Input{Account: account, Contact: stateContact, MessageID: messageID, Text: text, Now: time.Now(), TestSession: testSession, Audio: audio})
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
		notice := "🔔 Hóspede solicitou atendimento humano.\nContato: " + displayContact + "\nO automático está pausado até retomada explícita.\nPara retomar: comercial retomar " + displayContact
		for _, number := range s.auth.Numbers() {
			s.replyText(ctx, types.NewJID(number, types.DefaultUserServer), notice)
		}
	}
	if result.GroupRequest != "" {
		destination := cfg.GroupPhone
		if destination == "" {
			destination = "5573988240413"
		}
		notice := result.GroupRequest + "\nContato solicitante: " + displayContact
		if _, err := s.sendMessage(ctx, types.NewJID(destination, types.DefaultUserServer), &waE2E.Message{Conversation: proto.String(notice)}); err != nil {
			slog.Error("group quote forwarding failed")
			s.replyText(ctx, chat, "Não consegui encaminhar ao setor de grupos agora. Um atendente humano continuará por aqui.")
			if s.markUnread != nil {
				_ = s.markUnread(ctx, chat.String())
			}
			return
		}
	}
	if !s.waitGuestReply(ctx, chat) {
		return
	}
	if result.Text != "" {
		if _, err := s.sendMessage(ctx, chat, &waE2E.Message{Conversation: proto.String(result.Text)}); err != nil {
			slog.Error("commercial quote reply failed")
			return
		}
	}
	for _, body := range result.Messages {
		if strings.EqualFold(strings.TrimSpace(body), "mensagem 1") || strings.EqualFold(strings.TrimSpace(body), "mensagem 2") {
			continue
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		if _, err := s.sendMessage(ctx, chat, &waE2E.Message{Conversation: proto.String(body)}); err != nil {
			slog.Error("commercial sequence reply failed")
			return
		}
	}
	if len(result.Categories) > 0 {
		if err := s.sendQuoteProducts(ctx, account, stateContact, messageID, chat, result.Categories); err != nil {
			slog.Error("commercial catalog delivery incomplete")
			s.replyText(ctx, chat, "Não consegui enviar todos os produtos do catálogo. O orçamento acima continua disponível; a equipe pode ajudar com as fotos.")
		}
	}
	if strings.TrimSpace(result.SelectionPrompt) != "" {
		if _, err := s.sendMessage(ctx, chat, &waE2E.Message{Conversation: proto.String(result.SelectionPrompt)}); err != nil {
			slog.Error("commercial category prompt failed")
		}
	}
	if result.PreReservation != nil {
		s.startPreReservation(account, displayContact, messageID, chat, *result.PreReservation)
	}
}

func (s *Service) waitGuestReply(ctx context.Context, chat types.JID) bool {
	delay := s.guestReplyDelay
	if delay <= 0 {
		return true
	}
	if s.client != nil && s.client.IsLoggedIn() {
		_ = s.client.SendChatPresence(ctx, chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
		defer func() {
			_ = s.client.SendChatPresence(context.Background(), chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
		}()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Service) startPreReservation(account, contact, messageID string, chat types.JID, request commercial.PreReservation) {
	jobID := transportMessageKey("bitz", account, contact, messageID)
	job, claimed, err := s.bitzJobs.Claim(context.Background(), jobID, account, contact, chat.ToNonAD().String())
	if err != nil || !claimed {
		if err != nil {
			slog.Error("claim Bitz pre-reservation")
		}
		return
	}
	categories := make([]bitz.RoomCategory, len(request.Categories))
	for i, category := range request.Categories {
		categories[i] = bitz.RoomCategory{Key: category.Key, SourceKey: category.SourceKey, Name: category.Name}
	}
	go func() {
		s.bitzMu.Lock()
		defer s.bitzMu.Unlock()
		ctx := context.Background()
		publicCfg, publicErr := s.bitzStore.Public(ctx)
		cfg, password, runErr := s.bitzStore.Credentials(ctx)
		if publicErr != nil && runErr == nil {
			runErr = publicErr
		}
		if runErr == nil {
			runErr = s.applyRoomAllocationPolicy(ctx, categories)
		}
		if runErr == nil {
			runErr = s.bitzRunner.Create(ctx, cfg, password, bitz.ReservationRequest{ID: job.ID, CheckIn: request.CheckIn, CheckOut: request.CheckOut, Categories: categories})
		}
		if runErr != nil {
			_ = s.bitzJobs.Fail(ctx, job.ID, runErr)
			s.replyText(ctx, chat, "Não consegui concluir a pré-reserva automaticamente. Deixei a conversa para nossa equipe continuar.")
			if publicCfg.ApproverPhone != "" {
				notice := fmt.Sprintf("⚠️ Falha na pré-reserva %s.\nHóspede: %s\nMotivo: %s\nA conversa foi marcada para atendimento humano.", job.Code, contact, operatorSafeBitzError(runErr))
				if err := s.sendTextToPhone(ctx, publicCfg.ApproverPhone, notice); err != nil {
					slog.Error("Bitz failure notice delivery failed", "error", err)
				}
			}
			if s.markUnread != nil {
				_ = s.markUnread(ctx, chat.String())
			}
			slog.Error("Bitz pre-reservation failed")
			return
		}
		if err := s.bitzJobs.AwaitingApproval(ctx, job.ID); err != nil {
			slog.Error("persist Bitz approval state")
			return
		}
		notice := preReservationNotice(job.Code, contact, request)
		if err := s.sendTextToPhone(ctx, cfg.ApproverPhone, notice); err != nil {
			slog.Error("Bitz approval notice delivery failed", "error", err)
			if s.markUnread != nil {
				_ = s.markUnread(ctx, chat.String())
			}
		}
	}()
}

func (s *Service) applyRoomAllocationPolicy(ctx context.Context, categories []bitz.RoomCategory) error {
	type pool struct {
		configured, eligible []int
		demand               int
	}
	pools := make(map[string]*pool)
	for _, category := range categories {
		key := category.SourceKey
		if key == "" {
			key = category.Key
		}
		p := pools[key]
		if p == nil {
			configured, eligible, err := s.rooms.BitzAllocationCandidates(ctx, key)
			if err != nil {
				return err
			}
			p = &pool{configured: configured, eligible: eligible}
			pools[key] = p
		}
		p.demand++
	}
	for key, p := range pools {
		// No configured mapping keeps backward compatibility while the hotel
		// classifies its inventory in the UI. Once mapped, the local block applies.
		if len(p.configured) > 0 && len(p.eligible) < p.demand {
			return fmt.Errorf("categoria %s: %d quarto(s) solicitado(s), mas somente %d UH(s) não interditada(s)", key, p.demand, len(p.eligible))
		}
	}
	for i := range categories {
		key := categories[i].SourceKey
		if key == "" {
			key = categories[i].Key
		}
		if p := pools[key]; p != nil && len(p.configured) > 0 {
			categories[i].AllowedRooms = append([]int(nil), p.eligible...)
		}
	}
	return nil
}

func preReservationNotice(code, contact string, request commercial.PreReservation) string {
	guest := strings.TrimSpace(contact)
	if _, ok := phoneArgument(guest); ok {
		guest = "+" + guest
	}
	var b strings.Builder
	fmt.Fprintf(&b, "✅ *Pré-reserva criada no Bitz*\n\nCódigo: *%s*\nWhatsApp do hóspede: *%s*\nPeríodo: *%s a %s*\n", code, guest, request.CheckIn, request.CheckOut)
	if len(request.Rooms) > 0 {
		b.WriteString("\n*Resumo da hospedagem*\n")
		for i, room := range request.Rooms {
			category := "Categoria não informada"
			if i < len(request.Categories) && strings.TrimSpace(request.Categories[i].Name) != "" {
				category = request.Categories[i].Name
			}
			fmt.Fprintf(&b, "\n*Quarto %d — %s*\n%d adulto(s)", i+1, category, room.Adults)
			if len(room.Ages) == 0 {
				b.WriteString("\nSem crianças")
			} else {
				fmt.Fprintf(&b, "\n%d criança(s): %s", len(room.Ages), formatChildAges(room.Ages))
			}
			b.WriteByte('\n')
		}
	} else {
		fmt.Fprintf(&b, "\nQuartos: *%d*\n", len(request.Categories))
	}
	fmt.Fprintf(&b, "\nFinalize o atendimento e, quando estiver tudo certo, responda:\n*aprovar pre-reserva %s*", code)
	return b.String()
}

func formatChildAges(ages []int) string {
	parts := make([]string, len(ages))
	for i, age := range ages {
		unit := "anos"
		if age == 1 {
			unit = "ano"
		}
		parts[i] = fmt.Sprintf("%d %s", age, unit)
	}
	return strings.Join(parts, ", ")
}

func (s *Service) sendTextToPhone(ctx context.Context, phone, body string) error {
	phone, ok := phoneArgument(phone)
	if !ok {
		return errors.New("telefone de destino inválido")
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := s.client.IsOnWhatsApp(lookupCtx, []string{"+" + phone})
	if err != nil {
		return fmt.Errorf("consultar destino no WhatsApp: %w", err)
	}
	if len(result) != 1 || !result[0].IsIn || result[0].JID.IsEmpty() {
		return errors.New("telefone de destino não está registrado no WhatsApp")
	}
	_, err = s.sendMessage(ctx, result[0].JID, &waE2E.Message{Conversation: proto.String(body)})
	return err
}

func operatorSafeBitzError(err error) string {
	if err == nil {
		return "erro desconhecido"
	}
	message := err.Error()
	if len(message) > 300 {
		message = message[:300]
	}
	return message
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
