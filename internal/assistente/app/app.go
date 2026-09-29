package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/advances"
	"github.com/gadevsbr/tino/internal/assistente/cash"
	"github.com/gadevsbr/tino/internal/assistente/commands"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
)

type App struct {
	rooms        *rooms.Repository
	sessions     *conversation.Repository
	cash         *cash.Repository
	backupNow    func(context.Context) (string, error)
	backupStatus func(context.Context) (string, error)
	health       func(context.Context) string
	advances     *advances.Repository
	authorize    func(context.Context, string, string) (bool, error)
	listGroups   func(context.Context) ([]GroupOption, error)
	sendReports  func(context.Context, string, []string, []string) (string, error)
	quoteFetch   func(context.Context, omnibees.Search) (string, error)
}

type GroupOption struct {
	JID  string `json:"jid"`
	Name string `json:"name"`
}

type advancePayload struct {
	Employee string `json:"employee"`
	Cents    int64  `json:"cents"`
	Note     string `json:"note"`
}

type reportPayload struct {
	Kind   string        `json:"kind"`
	Groups []GroupOption `json:"groups"`
}

func New(roomRepo *rooms.Repository, sessionRepo *conversation.Repository) *App {
	return &App{rooms: roomRepo, sessions: sessionRepo, quoteFetch: omnibees.Fetch}
}
func (a *App) EnableCash(repo *cash.Repository) *App         { a.cash = repo; return a }
func (a *App) EnableAdvances(repo *advances.Repository) *App { a.advances = repo; return a }
func (a *App) EnableMessaging(authorize func(context.Context, string, string) (bool, error), listGroups func(context.Context) ([]GroupOption, error), sendReports func(context.Context, string, []string, []string) (string, error)) *App {
	a.authorize, a.listGroups, a.sendReports = authorize, listGroups, sendReports
	return a
}
func (a *App) EnableOperations(run func(context.Context) (string, error), status func(context.Context) (string, error), health func(context.Context) string) *App {
	a.backupNow, a.backupStatus, a.health = run, status, health
	return a
}

func (a *App) Handle(ctx context.Context, user, messageID, text string) (string, error) {
	if session, active, err := a.sessions.Active(ctx, user); err != nil {
		return "", err
	} else if active {
		if strings.HasPrefix(session.Type, "QUOTE_") {
			return a.handleQuoteSession(ctx, session, text)
		}
		if strings.HasPrefix(session.Type, "VALE_") {
			return a.handleAdvanceSession(ctx, session, user, messageID, text)
		}
		if strings.HasPrefix(session.Type, "REPORT_") {
			return a.handleReportSession(ctx, session, text)
		}
		if session.Type == "AUTH_NUMBER" {
			return a.handleAuthorization(ctx, session, user, text)
		}
		if session.Type == "CASH_DELETE" {
			n := strings.ToLower(strings.TrimSpace(text))
			if n == "confirmar" || n == "sim" {
				err := a.cash.DeleteToday(ctx, int64(session.CurrentRoom), user, time.Now())
				session.Status = "COMPLETED"
				if saveErr := a.sessions.Save(ctx, session); saveErr != nil {
					return "", saveErr
				}
				if errors.Is(err, cash.ErrCashClosed) {
					return "🔒 O caixa de hoje está fechado. Reabra antes de excluir.", nil
				}
				if errors.Is(err, cash.ErrMovementNotFound) {
					return "❌ Movimento não encontrado no caixa de hoje.", nil
				}
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("✅ Movimento #%d excluído.\n\nA exclusão ficou registrada na auditoria e os totais foram recalculados.", session.CurrentRoom), nil
			}
			if n == "cancelar" || n == "nao" || n == "não" {
				session.Status = "COMPLETED"
				if err := a.sessions.Save(ctx, session); err != nil {
					return "", err
				}
				return "✅ Exclusão cancelada. Nenhum movimento foi alterado.", nil
			}
			return "Responda `confirmar` para excluir ou `cancelar` para manter o movimento.", nil
		}
		if strings.HasPrefix(session.Type, "AWAITING_GUESTS:") {
			guests, err := strconv.Atoi(strings.TrimSpace(text))
			if err != nil || guests < 1 {
				return "Informe somente a quantidade de pessoas, por exemplo: 2", nil
			}
			status := rooms.Status(strings.TrimPrefix(session.Type, "AWAITING_GUESTS:"))
			session.Type = "UPDATE_ALL_ROOMS"
			return a.advanceWithGuests(ctx, session, status, guests, user, messageID)
		}
		kind := commands.Parse(text).Kind
		if kind == commands.Pause {
			session.Status = "PAUSED"
			if err := a.sessions.Save(ctx, session); err != nil {
				return "", err
			}
			return fmt.Sprintf("⏸️ Atualização interrompida.\n\n%d de %d quartos processados.\n%d restantes.\n\nEnvie \"continuar\" para retomar.", session.CurrentIndex, len(session.Selected), len(session.Selected)-session.CurrentIndex), nil
		}
		if kind == commands.Skip {
			return a.skip(ctx, session)
		}
		if status, ok := rooms.ParseStatus(text); ok {
			if status == rooms.Entry || status == rooms.OccupiedClean {
				session.Type = "AWAITING_GUESTS:" + string(status)
				if err := a.sessions.Save(ctx, session); err != nil {
					return "", err
				}
				action := "estão"
				if status == rooms.Entry {
					action = "vão entrar"
				}
				return fmt.Sprintf("👥 Quantas pessoas %s no quarto %d?\n\nResponda somente com o número.", action, session.CurrentRoom), nil
			}
			return a.advance(ctx, session, status, user, messageID)
		}
	}
	cmd := commands.Parse(text)
	switch cmd.Kind {
	case commands.Quote:
		search, err := omnibees.ParseLink(cmd.Text, cmd.Discount)
		if err != nil {
			return "❌ " + err.Error(), nil
		}
		quote, err := a.quoteFetch(ctx, search)
		if err != nil {
			return "❌ Não consegui consultar os valores totais na OmniBees agora. Confira o link e tente novamente; nenhum preço foi estimado.", nil
		}
		return quote, nil
	case commands.StartQuote:
		session := conversation.Session{User: user, Type: "QUOTE_CHECKIN", Status: conversation.Active, Payload: "{}"}
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return "📅 ORÇAMENTO — Qual a data de check-in? Envie DD/MM/AAAA. Para desistir, envie cancelar.", nil
	case commands.StartAdvance:
		if a.advances == nil {
			return "Vales indisponíveis.", nil
		}
		session := conversation.Session{User: user, Type: "VALE_NAME", Status: conversation.Active, Payload: "{}"}
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return "💵 NOVO VALE\n\nQual é o nome do funcionário?", nil
	case commands.AdvanceReport:
		if a.advances == nil {
			return "Vales indisponíveis.", nil
		}
		items, err := a.advances.Month(ctx, cmd.Text, time.Now())
		if err != nil {
			return "", err
		}
		return advances.Report(items, time.Now().Format("01/2006")), nil
	case commands.SendReports:
		if a.sendReports == nil || a.listGroups == nil {
			return "Envio de relatórios indisponível.", nil
		}
		session := conversation.Session{User: user, Type: "REPORT_KIND", Status: conversation.Active, Payload: "{}"}
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return "📤 ENVIAR RELATÓRIOS\n\nQual relatório deseja enviar?\n\n1 - Situação dos quartos\n2 - Caixa do dia\n3 - Ambos\n\nResponda 1, 2 ou 3.", nil
	case commands.AuthorizeNumber:
		if a.authorize == nil {
			return "Autorização dinâmica indisponível.", nil
		}
		if cmd.Text != "" {
			return a.authorizeNumber(ctx, user, cmd.Text)
		}
		session := conversation.Session{User: user, Type: "AUTH_NUMBER", Status: conversation.Active, Payload: "{}"}
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return "🔐 AUTORIZAR NÚMERO\n\nInforme o número com DDI e DDD.\nExemplo: 5573999999999", nil
	case commands.OpenCash:
		if a.cash == nil {
			return "Caixa indisponível.", nil
		}
		if err := a.cash.OpenDate(ctx, func() string {
			if cmd.Date != "" {
				return cmd.Date
			}
			return a.cash.BusinessDate(time.Now())
		}(), cmd.Cents, user); errors.Is(err, cash.ErrCashClosed) {
			return "🔒 O caixa de hoje está fechado. Use `reabrir caixa` antes de alterar.", nil
		} else if err != nil {
			return "", err
		}
		return "✅ Caixa aberto.\n\nCaixa inicial: " + cash.Money(cmd.Cents), nil
	case commands.CashOpeningEdit:
		if a.cash == nil {
			return "Caixa indisponível.", nil
		}
		if err := a.cash.EditOpening(ctx, cmd.Date, cmd.Cents, user); err != nil {
			return "❌ Não consegui editar a abertura desse dia: " + err.Error(), nil
		}
		return fmt.Sprintf("✅ Abertura do caixa de %s corrigida com auditoria.\n\nNovo valor: %s", cmd.Date, cash.Money(cmd.Cents)), nil
	case commands.CashEntry:
		if a.cash == nil {
			return "Caixa indisponível.", nil
		}
		if err := a.cash.AddDate(ctx, func() string {
			if cmd.Date != "" {
				return cmd.Date
			}
			return a.cash.BusinessDate(time.Now())
		}(), "ENTRY", cmd.Method, cmd.Cents, cmd.Text, user, messageID); errors.Is(err, cash.ErrCashClosed) {
			return "🔒 O caixa de hoje está fechado. Use `reabrir caixa` antes de alterar.", nil
		} else if err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ Entrada registrada.\n\n%s — %s\n%s", cmd.Method, cash.Money(cmd.Cents), cmd.Text), nil
	case commands.CashExit:
		if a.cash == nil {
			return "Caixa indisponível.", nil
		}
		if err := a.cash.AddDate(ctx, func() string {
			if cmd.Date != "" {
				return cmd.Date
			}
			return a.cash.BusinessDate(time.Now())
		}(), "EXIT", "", cmd.Cents, cmd.Text, user, messageID); errors.Is(err, cash.ErrCashClosed) {
			return "🔒 O caixa de hoje está fechado. Use `reabrir caixa` antes de alterar.", nil
		} else if err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ Saída registrada.\n\n%s\n%s", cash.Money(cmd.Cents), cmd.Text), nil
	case commands.CashStatus:
		if a.cash == nil {
			return "Caixa indisponível.", nil
		}
		day, err := a.cash.Today(ctx, time.Now())
		if err != nil {
			return "", err
		}
		state := "ABERTO"
		if day.ClosedAt != "" {
			state = "FECHADO"
		}
		return fmt.Sprintf("💰 CAIXA — %s [%s]\n\nCaixa inicial: %s\n\nENTRADAS\nDinheiro: %s\nCartão: %s\nPIX: %s\nTotal entradas: %s\n\nSaídas: %s\n\nCAIXA FINAL\nDinheiro: %s\nCartão: %s\nPIX: %s\nTotal final: %s", day.Date, state, cash.Money(day.OpeningCents), cash.Money(day.MethodTotal("DINHEIRO")), cash.Money(day.MethodTotal("CARTAO")), cash.Money(day.MethodTotal("PIX")), cash.Money(day.EntryTotal()), cash.Money(day.ExitTotal()), cash.Money(day.FinalMethodTotal("DINHEIRO")), cash.Money(day.FinalMethodTotal("CARTAO")), cash.Money(day.FinalMethodTotal("PIX")), cash.Money(day.FinalCents())), nil
	case commands.CashReport:
		return "", nil
	case commands.CashClose:
		if a.cash == nil {
			return "Caixa indisponível.", nil
		}
		day, err := a.cash.CloseToday(ctx, user, time.Now())
		if errors.Is(err, cash.ErrCashClosed) {
			return "🔒 O caixa de hoje já está fechado ou ainda não foi aberto.", nil
		}
		if err != nil {
			return "", err
		}
		return "🔒 CAIXA FECHADO\n\n" + cashDaySummary(day), nil
	case commands.CashReopen:
		if a.cash == nil {
			return "Caixa indisponível.", nil
		}
		if err := a.cash.ReopenToday(ctx, user, time.Now()); err != nil {
			return "❌ O caixa de hoje não está fechado.", nil
		}
		return "🔓 Caixa reaberto. A reabertura ficou registrada na auditoria.", nil
	case commands.CashHistoryDate:
		day, err := a.cash.ByDate(ctx, cmd.Date)
		if err != nil {
			return "", err
		}
		if len(day.Entries) == 0 && len(day.Exits) == 0 && day.OpeningCents == 0 {
			return "📅 Nenhum caixa encontrado nessa data.", nil
		}
		return cashDaySummary(day), nil
	case commands.CashHistoryWeek, commands.CashHistoryMonth:
		now := time.Now()
		local := now
		if cmd.Kind == commands.CashHistoryWeek {
			local = now.AddDate(0, 0, -6)
		} else {
			local = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		}
		days, err := a.cash.Range(ctx, local.Format("2006-01-02"), now.Format("2006-01-02"))
		if err != nil {
			return "", err
		}
		return cashRangeSummary(days), nil
	case commands.BackupNow:
		if a.backupNow == nil {
			return "Backup indisponível.", nil
		}
		return a.backupNow(ctx)
	case commands.BackupStatus:
		if a.backupStatus == nil {
			return "Backup indisponível.", nil
		}
		return a.backupStatus(ctx)
	case commands.Health:
		if a.health == nil {
			return "Diagnóstico indisponível.", nil
		}
		return a.health(ctx), nil
	case commands.CashMovements:
		if a.cash == nil {
			return "Caixa indisponível.", nil
		}
		day, err := a.cash.Today(ctx, time.Now())
		if err != nil {
			return "", err
		}
		items := append(append([]cash.Movement{}, day.Entries...), day.Exits...)
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		if len(items) == 0 {
			return "📋 Nenhum movimento registrado no caixa de hoje.", nil
		}
		var lines []string
		for _, m := range items {
			lines = append(lines, cashMovementLine(m))
		}
		return "📋 MOVIMENTOS DO CAIXA — " + day.Date + "\n\n" + strings.Join(lines, "\n") + "\n\nUse o código # para editar ou excluir.", nil
	case commands.CashEdit:
		if a.cash == nil {
			return "Caixa indisponível.", nil
		}
		kind := "EXIT"
		if cmd.Method != "" {
			kind = "ENTRY"
		}
		if err := a.cash.EditToday(ctx, cmd.MovementID, kind, cmd.Method, cmd.Cents, cmd.Text, user, time.Now()); errors.Is(err, cash.ErrCashClosed) {
			return "🔒 O caixa de hoje está fechado. Reabra antes de editar.", nil
		} else if errors.Is(err, cash.ErrMovementNotFound) {
			return "❌ Movimento não encontrado no caixa de hoje.", nil
		} else if err != nil {
			return "", err
		}
		updated, err := a.cash.MovementToday(ctx, cmd.MovementID, time.Now())
		if err != nil {
			return "", err
		}
		return "✅ Movimento editado e auditado.\n\n" + cashMovementLine(updated) + "\n\nOs totais foram recalculados.", nil
	case commands.CashDelete:
		if a.cash == nil {
			return "Caixa indisponível.", nil
		}
		movement, err := a.cash.MovementToday(ctx, cmd.MovementID, time.Now())
		if errors.Is(err, cash.ErrMovementNotFound) {
			return "❌ Movimento não encontrado no caixa de hoje.", nil
		}
		if err != nil {
			return "", err
		}
		s := conversation.Session{User: user, Type: "CASH_DELETE", Status: conversation.Active, CurrentRoom: int(cmd.MovementID)}
		if err := a.sessions.Save(ctx, s); err != nil {
			return "", err
		}
		return "⚠️ EXCLUIR MOVIMENTO\n\n" + cashMovementLine(movement) + "\n\nResponda `confirmar` para excluir ou `cancelar` para manter.", nil
	case commands.StartAll:
		s := conversation.Session{User: user, Type: "UPDATE_ALL_ROOMS", Status: conversation.Active, CurrentIndex: 0, CurrentRoom: rooms.OfficialNumbers[0], Selected: append([]int(nil), rooms.OfficialNumbers...)}
		if err := a.sessions.Save(ctx, s); err != nil {
			return "", err
		}
		return "🏨 ATUALIZAÇÃO DOS QUARTOS\n\n42 quartos cadastrados.\n\n🟢 verde = disponível\n🟢🟣 desforrado = limpo, mas desforrado\n🟠 laranja = manutenção de limpeza\n🩷 rosa = saída e entrada\n🟡 amarelo = entrada\n🟥 magenta = saída\n⬜ cinza = interditado\n🟣 roxo = limpar\n\nComandos:\npular\nparar\n\nQuarto 101:", nil
	case commands.Continue:
		s, err := a.sessions.Get(ctx, user)
		if err != nil {
			return "Não encontrei atualização para continuar.", nil
		}
		if s.Status != "PAUSED" {
			return "Não há atualização pausada.", nil
		}
		s.Status = conversation.Active
		if err := a.sessions.Save(ctx, s); err != nil {
			return "", err
		}
		last := "nenhum"
		if s.CurrentIndex > 0 {
			last = fmt.Sprint(s.Selected[s.CurrentIndex-1])
		}
		return fmt.Sprintf("🔄 Encontrei uma atualização em andamento.\n\nÚltimo quarto concluído:\n%s\n\nPróximo:\n%d\n\nQuarto %d:", last, s.CurrentRoom, s.CurrentRoom), nil
	case commands.RoomQuery:
		r, err := a.rooms.Get(ctx, cmd.Room)
		if errors.Is(err, rooms.ErrNotFound) {
			return fmt.Sprintf("❌ Quarto %d não está cadastrado.", cmd.Room), nil
		}
		if err != nil {
			return "", err
		}
		obs := r.Observation
		if obs == "" {
			obs = "—"
		}
		completion := ""
		if r.Status == rooms.OccupiedClean {
			if done, _ := a.rooms.CleaningCompletedToday(ctx, r.Number, time.Now()); done {
				completion = "\n✅ Manutenção concluída hoje"
			}
		}
		people := ""
		if r.Status == rooms.Entry || r.Status == rooms.OccupiedClean {
			people = fmt.Sprintf("\n👥 Pessoas: %d", r.GuestCount)
		}
		return fmt.Sprintf("🏨 QUARTO %d\n\nStatus:\n%s%s%s\n\nObservação:\n%s", r.Number, r.Status.StringWithIcon(), people, completion, obs), nil
	case commands.RoomUpdate:
		if (cmd.Status == rooms.Entry || cmd.Status == rooms.OccupiedClean) && cmd.Guests < 1 {
			color := "laranja"
			if cmd.Status == rooms.Entry {
				color = "amarelo"
			}
			return fmt.Sprintf("👥 Informe a quantidade de pessoas.\n\nExemplo: %d %s 2 pessoas", cmd.Room, color), nil
		}
		changed, _, err := a.rooms.UpdateStatusWithGuests(ctx, cmd.Room, cmd.Status, cmd.Guests, user, "INDIVIDUAL", messageID)
		if errors.Is(err, rooms.ErrNotFound) {
			return fmt.Sprintf("❌ Quarto %d não está cadastrado.", cmd.Room), nil
		}
		if errors.Is(err, rooms.ErrCleaningCompletedToday) {
			return fmt.Sprintf("⏳ A manutenção do quarto %d já foi concluída hoje. Uma nova manutenção só pode ser aberta amanhã.", cmd.Room), nil
		}
		if err != nil {
			return "", err
		}
		if !changed {
			return fmt.Sprintf("ℹ️ O quarto %d já está:\n\n%s", cmd.Room, cmd.Status.StringWithIcon()), nil
		}
		people := ""
		if cmd.Guests > 0 {
			people = fmt.Sprintf("\n👥 %d pessoas", cmd.Guests)
		}
		return fmt.Sprintf("✅ Quarto %d atualizado.\n\n%s%s", cmd.Room, cmd.Status.StringWithIcon(), people), nil
	case commands.CompleteCleaning:
		err := a.rooms.CompleteCleaning(ctx, cmd.Room, user, messageID, time.Now())
		switch {
		case errors.Is(err, rooms.ErrNotFound):
			return fmt.Sprintf("❌ Quarto %d não está cadastrado.", cmd.Room), nil
		case errors.Is(err, rooms.ErrNotCleaning):
			return fmt.Sprintf("ℹ️ O quarto %d não está laranja. O comando OK só conclui manutenção de limpeza.", cmd.Room), nil
		case errors.Is(err, rooms.ErrCleaningCompletedToday):
			return fmt.Sprintf("ℹ️ A manutenção do quarto %d já foi concluída hoje. Uma nova poderá ser aberta amanhã.", cmd.Room), nil
		case err != nil:
			return "", err
		}
		return fmt.Sprintf("✅ Manutenção de limpeza do quarto %d concluída.\n\nUma nova manutenção só poderá ser aberta amanhã.", cmd.Room), nil
	case commands.GeneralStatus:
		counts, err := a.rooms.Counts(ctx)
		if err != nil {
			return "", err
		}
		guests, err := a.rooms.GuestTotals(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("🏨 STATUS DOS QUARTOS\n\n🟢 Disponíveis: %d\n🟢🟣 Limpos, mas desforrados: %d\n🟠 Manutenção de limpeza: %d quartos — 👥 %d pessoas\n🩷 Saída e entrada: %d\n🟡 Entrada: %d quartos — 👥 %d pessoas\n🟥 Saída: %d\n⬜ Interditados: %d\n🟣 Limpar: %d\n\nTotal: 42 quartos", counts[rooms.AvailableClean], counts[rooms.CleanUnmade], counts[rooms.OccupiedClean], guests[rooms.OccupiedClean], counts[rooms.CheckoutEntry], counts[rooms.Entry], guests[rooms.Entry], counts[rooms.CheckoutToday], counts[rooms.Maintenance], counts[rooms.Dirty]), nil
	case commands.ListStatus:
		list, err := a.rooms.ListByStatus(ctx, cmd.Status)
		if err != nil {
			return "", err
		}
		numbers := make([]string, len(list))
		for i, r := range list {
			numbers[i] = fmt.Sprint(r.Number)
			if r.Status == rooms.Entry || r.Status == rooms.OccupiedClean {
				numbers[i] += fmt.Sprintf(" — %d pessoas", r.GuestCount)
			}
		}
		return fmt.Sprintf("%s %s\n\n%s\n\nTotal: %d", cmd.Status.Emoji(), cmd.Status.Label(), strings.Join(numbers, "\n"), len(list)), nil
	case commands.BatchUpdate:
		if cmd.Status == rooms.Entry || cmd.Status == rooms.OccupiedClean {
			return "👥 Para amarelo ou laranja, atualize cada quarto informando a quantidade de pessoas.\n\nExemplo: 101 amarelo 3 pessoas", nil
		}
		changed, err := a.rooms.BatchUpdate(ctx, cmd.Rooms, cmd.Status, user, "BATCH", messageID)
		if err != nil {
			if errors.Is(err, rooms.ErrCleaningCompletedToday) {
				return "⏳ Um dos quartos teve a manutenção concluída hoje. Nenhuma alteração foi feita; uma nova manutenção só pode ser aberta amanhã.", nil
			}
			if errors.Is(err, rooms.ErrNotFound) {
				return "❌ Um ou mais quartos não estão cadastrados; nenhuma alteração foi feita.", nil
			}
			return "", err
		}
		return fmt.Sprintf("✅ %d quartos atualizados.\n\n%s", changed, cmd.Status.StringWithIcon()), nil
	case commands.SetObservation:
		if err := a.rooms.SetObservation(ctx, cmd.Room, cmd.Text); errors.Is(err, rooms.ErrNotFound) {
			return fmt.Sprintf("❌ Quarto %d não está cadastrado.", cmd.Room), nil
		} else if err != nil {
			return "", err
		}
		return fmt.Sprintf("📝 Observação adicionada ao quarto %d:\n\n%s", cmd.Room, cmd.Text), nil
	case commands.ClearObservation:
		if err := a.rooms.SetObservation(ctx, cmd.Room, ""); errors.Is(err, rooms.ErrNotFound) {
			return fmt.Sprintf("❌ Quarto %d não está cadastrado.", cmd.Room), nil
		} else if err != nil {
			return "", err
		}
		return fmt.Sprintf("🧹 Observação removida do quarto %d.", cmd.Room), nil
	case commands.History:
		history, err := a.rooms.History(ctx, cmd.Room, 10)
		if err != nil {
			return "", err
		}
		if len(history) == 0 {
			if _, err := a.rooms.Get(ctx, cmd.Room); errors.Is(err, rooms.ErrNotFound) {
				return fmt.Sprintf("❌ Quarto %d não está cadastrado.", cmd.Room), nil
			}
			return fmt.Sprintf("📜 HISTÓRICO — QUARTO %d\n\nNenhuma alteração registrada.", cmd.Room), nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "📜 HISTÓRICO — QUARTO %d\n", cmd.Room)
		for _, h := range history {
			fmt.Fprintf(&b, "\n%s → %s", h.Old.Label(), h.New.Label())
		}
		return b.String(), nil
	case commands.Menu:
		return menuText, nil
	default:
		return "", nil
	}
}

func (a *App) handleAdvanceSession(ctx context.Context, session conversation.Session, user, messageID, text string) (string, error) {
	if strings.EqualFold(strings.TrimSpace(text), "cancelar") {
		session.Status = "COMPLETED"
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return "✅ Cadastro do vale cancelado.", nil
	}
	var payload advancePayload
	_ = json.Unmarshal([]byte(session.Payload), &payload)
	switch session.Type {
	case "VALE_NAME":
		name := strings.TrimSpace(text)
		if len([]rune(name)) < 2 || len([]rune(name)) > 80 {
			return "Informe o nome do funcionário, entre 2 e 80 caracteres.", nil
		}
		payload.Employee = name
		session.Type = "VALE_AMOUNT"
		session.Payload = marshalPayload(payload)
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return "Qual é o valor do vale?\n\nExemplo: 150,00", nil
	case "VALE_AMOUNT":
		cents, ok := parseCurrency(text)
		if !ok || cents <= 0 {
			return "Informe um valor válido. Exemplo: 150,00", nil
		}
		payload.Cents = cents
		session.Type = "VALE_NOTE"
		session.Payload = marshalPayload(payload)
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return "Digite uma observação para o vale ou responda `pular`.", nil
	case "VALE_NOTE":
		note := strings.TrimSpace(text)
		if strings.EqualFold(note, "pular") || strings.EqualFold(note, "sem observacao") || strings.EqualFold(note, "sem observação") {
			note = ""
		}
		if len([]rune(note)) > 200 {
			return "A observação deve ter no máximo 200 caracteres.", nil
		}
		payload.Note = note
		session.Type = "VALE_CASH"
		session.Payload = marshalPayload(payload)
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return fmt.Sprintf("💵 %s — %s\n\nDeseja subtrair este valor do caixa em dinheiro?\nResponda `sim` ou `não`.", payload.Employee, cash.Money(payload.Cents)), nil
	case "VALE_CASH":
		answer := strings.ToLower(strings.TrimSpace(text))
		if answer != "sim" && answer != "nao" && answer != "não" {
			return "Responda somente `sim` para lançar como saída do caixa ou `não` para registrar apenas no relatório mensal.", nil
		}
		deduct := answer == "sim"
		item, err := a.advances.Create(ctx, payload.Employee, payload.Cents, payload.Note, deduct, user, messageID, time.Now())
		if errors.Is(err, cash.ErrCashClosed) {
			return "🔒 O caixa de hoje está fechado. Reabra o caixa e responda `sim`, ou responda `não` para registrar somente no relatório mensal.", nil
		}
		if err != nil {
			return "", err
		}
		session.Status = "COMPLETED"
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		destination := "Registrado somente no relatório mensal."
		if item.DeductCash {
			destination = "Registrado também como saída do caixa em dinheiro."
		}
		return fmt.Sprintf("✅ Vale registrado.\n\nFuncionário: %s\nValor: %s\n%s\n\nConsulte com `vales %s`.", item.Employee, cash.Money(item.Cents), destination, item.Employee), nil
	}
	return "", errors.New("invalid advance session")
}

func (a *App) handleReportSession(ctx context.Context, session conversation.Session, text string) (string, error) {
	if strings.EqualFold(strings.TrimSpace(text), "cancelar") {
		session.Status = "COMPLETED"
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return "✅ Envio de relatórios cancelado.", nil
	}
	var payload reportPayload
	_ = json.Unmarshal([]byte(session.Payload), &payload)
	if session.Type == "REPORT_KIND" {
		switch strings.ToLower(strings.TrimSpace(text)) {
		case "1", "quartos", "situacao", "situação":
			payload.Kind = "ROOMS"
		case "2", "caixa":
			payload.Kind = "CASH"
		case "3", "ambos", "todos":
			payload.Kind = "BOTH"
		default:
			return "Responda 1 para quartos, 2 para caixa ou 3 para ambos.", nil
		}
		groups, err := a.listGroups(ctx)
		if err != nil {
			return "", err
		}
		payload.Groups = groups
		session.Type = "REPORT_TARGETS"
		session.Payload = marshalPayload(payload)
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		var lines []string
		for i, group := range groups {
			lines = append(lines, fmt.Sprintf("%d - %s", i+1, group.Name))
		}
		groupText := "Nenhum grupo disponível."
		if len(lines) > 0 {
			groupText = strings.Join(lines, "\n")
		}
		return "👥 GRUPOS DISPONÍVEIS\n\n" + groupText + "\n\nInforme os números da lista e/ou telefones com DDI, separados por espaço ou vírgula.\nExemplo: `1 3 5573999999999`", nil
	}
	if session.Type == "REPORT_TARGETS" {
		tokens := regexp.MustCompile(`[0-9]+`).FindAllString(text, -1)
		phones := []string{}
		groups := []string{}
		seen := map[string]struct{}{}
		for _, token := range tokens {
			if len(token) <= 3 {
				index, _ := strconv.Atoi(token)
				if index < 1 || index > len(payload.Groups) {
					return fmt.Sprintf("O grupo %s não existe na lista. Escolha um dos números exibidos.", token), nil
				}
				jid := payload.Groups[index-1].JID
				if _, ok := seen["g:"+jid]; !ok {
					groups = append(groups, jid)
					seen["g:"+jid] = struct{}{}
				}
				continue
			}
			if len(token) < 12 || len(token) > 15 {
				return "Telefone inválido. Use DDI + DDD + número, por exemplo 5573999999999.", nil
			}
			if _, ok := seen["p:"+token]; !ok {
				phones = append(phones, token)
				seen["p:"+token] = struct{}{}
			}
		}
		if len(phones)+len(groups) == 0 {
			return "Informe pelo menos um grupo da lista ou um telefone com DDI.", nil
		}
		result, err := a.sendReports(ctx, payload.Kind, phones, groups)
		if err != nil {
			return "", err
		}
		session.Status = "COMPLETED"
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return result, nil
	}
	return "", errors.New("invalid report session")
}

func (a *App) handleAuthorization(ctx context.Context, session conversation.Session, user, text string) (string, error) {
	if strings.EqualFold(strings.TrimSpace(text), "cancelar") {
		session.Status = "COMPLETED"
		if err := a.sessions.Save(ctx, session); err != nil {
			return "", err
		}
		return "✅ Autorização cancelada.", nil
	}
	reply, err := a.authorizeNumber(ctx, user, text)
	if err != nil || strings.HasPrefix(reply, "❌") {
		return reply, err
	}
	session.Status = "COMPLETED"
	if err := a.sessions.Save(ctx, session); err != nil {
		return "", err
	}
	return reply, nil
}

func (a *App) authorizeNumber(ctx context.Context, actor, value string) (string, error) {
	number := regexp.MustCompile(`\D`).ReplaceAllString(value, "")
	if len(number) < 12 || len(number) > 15 {
		return "❌ Número inválido. Informe DDI + DDD + número, por exemplo 5573999999999.", nil
	}
	added, err := a.authorize(ctx, actor, number)
	if err != nil {
		return "", err
	}
	if !added {
		return "ℹ️ Esse número já estava autorizado.", nil
	}
	return "✅ Número " + number + " autorizado com efeito imediato, sem reiniciar o bot.", nil
}

func marshalPayload(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func parseCurrency(value string) (int64, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "r$")
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ".", "")
	value = strings.ReplaceAll(value, ",", ".")
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, false
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, false
	}
	cents := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) == 1 {
			parts[1] += "0"
		}
		if len(parts[1]) != 2 {
			return 0, false
		}
		cents, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, false
		}
	}
	return whole*100 + cents, true
}

const menuText = `🏨 HOTEL ROOM BOT — COMO USAR

CORES REAIS
🟢 verde = DISPONÍVEL
🟢🟣 verde/roxo = LIMPO, MAS DESFORRADO
🟠 laranja = MANUTENÇÃO DE LIMPEZA
🩷 rosa = SAÍDA E ENTRADA
🟡 amarelo = ENTRADA
🟥 magenta = SAÍDA
⬜ cinza = INTERDITADO
🟣 roxo = LIMPAR

CONSULTAR
• status — resumo de todos
• 101 ou status 101 — consultar quarto
• verdes, desforrados, laranjas, rosas, amarelos, magentas, cinzas ou roxos — listar por cor
• historico 101 — últimas alterações

ATUALIZAR
• 101 verde — alterar um quarto
• 101 desforrado — quarto limpo, mas desforrado
• 101 amarelo 3 pessoas — entrada de 3 pessoas
• 101 laranja 2 pessoas — manutenção com 2 hóspedes
• verdes 101 102 — alterar vários de uma vez
• desforrados 103 104 — marcar vários limpos, mas desforrados
• atualizar status — passar pelos 42 quartos
• pular, parar e continuar — controlar a atualização

MANUTENÇÃO LARANJA
• ok 101 — concluir a manutenção do quarto 101
Após o OK, uma nova manutenção nesse quarto só pode ser aberta no dia seguinte.

OBSERVAÇÕES
• obs 101 texto da observação
• limpar obs 101

RELATÓRIO
• relatorio em pdf

ORÇAMENTO OMNIBEES
• orçamento — responder às perguntas de datas e hóspedes
• envie o link completo dos resultados da OmniBees — o bot responde com o orçamento
• opcional: acrescente 5% após o link para aplicar desconto de 1% a 8%

EXTRATOS BITZ
• extratos — envie PDFs e a planilha XLSX pelo WhatsApp
• extrato semana 1 setembro [2026] — criar ou retomar a semana; envie os PDFs de cada dia
• processar extratos — receber a versão completa atualizada e manter a semana salva
• cancelar extratos — sair do envio atual sem apagar a semana acumulada

CAIXA
• envie uma foto com a legenda comprovante, ou um PDF; confira o que o bot leu e responda confirmar ou corrija só o necessário
• correções: método PIX, valor 24,00, data 21/09/2026; também aceita PIX 24,00 21/09/2026
• depois digite a descrição e confirme o lançamento com 1 (2 cancela)
• comprovantes — listar os comprovantes de hoje
• comprovantes 21/09/2026 — listar por data
• comprovante 12 — receber o arquivo original pelo código
• abrir caixa 191,85
• entrada dinheiro 2100 hospedagem qto 104
• entrada cartao 350 hospedagem qto 205
• entrada pix 120 hospedagem qto 101
• saida 50 compra de material
• caixa hoje
• fechar caixa — trava o dia e gera o fechamento
• reabrir caixa — reabre com auditoria
• editar abertura caixa DD/MM/AAAA valor — corrige abertura passada com auditoria
• caixa 15/08/2026, caixa semana ou caixa mes
• movimentos caixa
• editar movimento 15 entrada pix 250 hospedagem qto 104
• editar movimento 16 saida 80 compra de material
• excluir movimento 15 — depois responda confirmar ou cancelar
• relatorio caixa em pdf
• relatorio caixa em pdf 18/09/2026 — PDF de um dia
• relatorio caixa em pdf 01/09/2026 a 30/09/2026 — PDF do período (um dia por página)
• backup agora e status backup
• saude ou diagnostico

VALES DE FUNCIONÁRIOS
• vale — cadastro guiado de adiantamento
• vales mes — relatório geral do mês
• vales Maria — relatório mensal da funcionária
• relatorio vales em pdf — PDF mensal dos vales

ATENDIMENTO COMERCIAL
• comercial status — consultar modo e configuração
• comercial teste <telefone com DDI> — atender somente o telefone de teste
• comercial ativar — atender hóspedes em conversas privadas
• comercial desativar — desligar o atendimento automático
• comercial retomar <telefone com DDI ou LID JID> — retomar um contato pausado
• testar atendimento — testar como hóspede (telefone precisa estar admitido)
• sair atendimento — voltar aos comandos de operador
• Hóspedes: orçamento, atendimento humano e retomar atendimento

CATÁLOGO
• configurar catalogo — vincular produtos às categorias de suítes
• status catalogo — consultar os vínculos da conta conectada
• limpar catalogo — remover vínculos após confirmação
• testar produto <número da categoria> — testar envio do produto vinculado

ENVIAR RELATÓRIOS
• enviar relatorios — envia quartos, caixa ou ambos
• aceita vários telefones e grupos numerados

OPERADORES
• autorizar numero — autorização guiada
• autorizar numero 5573999999999 — efeito imediato

AUTOMAÇÃO
• backup automático diário
• resumo automático aos operadores às 17h

Os demais operadores são avisados quando um status ou uma manutenção é atualizado.`

func cashDaySummary(day cash.Day) string {
	state := "ABERTO"
	if day.ClosedAt != "" {
		state = "FECHADO"
	}
	return fmt.Sprintf("💰 CAIXA %s — %s\nDinheiro: %s\nCartão: %s\nPIX: %s\nSaídas: %s\nTotal final: %s", state, day.Date, cash.Money(day.FinalMethodTotal("DINHEIRO")), cash.Money(day.FinalMethodTotal("CARTAO")), cash.Money(day.FinalMethodTotal("PIX")), cash.Money(day.ExitTotal()), cash.Money(day.FinalCents()))
}

func cashRangeSummary(days []cash.Day) string {
	if len(days) == 0 {
		return "📅 Nenhum caixa encontrado no período."
	}
	var entries, exits, final int64
	lines := make([]string, 0, len(days))
	for _, d := range days {
		entries += d.EntryTotal()
		exits += d.ExitTotal()
		final += d.FinalCents()
		lines = append(lines, fmt.Sprintf("• %s — entradas %s | saídas %s | final %s", d.Date, cash.Money(d.EntryTotal()), cash.Money(d.ExitTotal()), cash.Money(d.FinalCents())))
	}
	return "📊 HISTÓRICO FINANCEIRO\n\n" + strings.Join(lines, "\n") + fmt.Sprintf("\n\nTotal entradas: %s\nTotal saídas: %s\nSoma dos fechamentos: %s", cash.Money(entries), cash.Money(exits), cash.Money(final))
}

func cashMovementLine(m cash.Movement) string {
	kind := "SAÍDA"
	method := "DINHEIRO"
	if m.Kind == "ENTRY" {
		kind = "ENTRADA"
		method = m.Method
	}
	return fmt.Sprintf("#%d — %s %s — %s — %s", m.ID, kind, method, cash.Money(m.Cents), m.Description)
}

func (a *App) skip(ctx context.Context, s conversation.Session) (string, error) {
	number := s.Selected[s.CurrentIndex]
	s.CurrentIndex++
	if s.CurrentIndex >= len(s.Selected) {
		s.Status = "COMPLETED"
		s.CurrentRoom = 0
		if err := a.sessions.Save(ctx, s); err != nil {
			return "", err
		}
		return fmt.Sprintf("⏭️ Quarto %d não alterado.\n\n✅ ATUALIZAÇÃO FINALIZADA", number), nil
	}
	s.CurrentRoom = s.Selected[s.CurrentIndex]
	if err := a.sessions.Save(ctx, s); err != nil {
		return "", err
	}
	return fmt.Sprintf("⏭️ Quarto %d não alterado.\n\nQuarto %d:", number, s.CurrentRoom), nil
}

func (a *App) advance(ctx context.Context, s conversation.Session, status rooms.Status, user, messageID string) (string, error) {
	return a.advanceWithGuests(ctx, s, status, 0, user, messageID)
}

func (a *App) advanceWithGuests(ctx context.Context, s conversation.Session, status rooms.Status, guests int, user, messageID string) (string, error) {
	number := s.Selected[s.CurrentIndex]
	changed, _, err := a.rooms.UpdateStatusWithGuests(ctx, number, status, guests, user, "GUIDED", messageID)
	if errors.Is(err, rooms.ErrCleaningCompletedToday) {
		return fmt.Sprintf("⏳ A manutenção do quarto %d já foi concluída hoje. Escolha outra cor; uma nova manutenção laranja só pode ser aberta amanhã.", number), nil
	}
	if err != nil {
		return "", err
	}
	s.CurrentIndex++
	prefix := fmt.Sprintf("%s %d — %s", status.Emoji(), number, status.Label())
	if guests > 0 {
		prefix += fmt.Sprintf(" — 👥 %d pessoas", guests)
	}
	if !changed {
		prefix = fmt.Sprintf("ℹ️ %d já estava — %s", number, status.Label())
	}
	if s.CurrentIndex >= len(s.Selected) {
		s.Status = "COMPLETED"
		s.CurrentRoom = 0
		if err := a.sessions.Save(ctx, s); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s\n\n✅ ATUALIZAÇÃO FINALIZADA\n\n42 quartos processados.", prefix), nil
	}
	s.CurrentRoom = s.Selected[s.CurrentIndex]
	if err := a.sessions.Save(ctx, s); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s\n\nQuarto %d:", prefix, s.CurrentRoom), nil
}
