package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/chat"
	"github.com/lxn/walk"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type chatListModel struct {
	walk.ListModelBase
	items []chat.Conversation
}

func (m *chatListModel) ItemCount() int { return len(m.items) }
func (m *chatListModel) Value(i int) interface{} {
	if i < 0 || i >= len(m.items) {
		return ""
	}
	c := m.items[i]
	name := c.Name
	if name == "" {
		name = c.JID
	}
	unread := ""
	if c.Unread > 0 {
		unread = fmt.Sprintf("  • %d nova(s)", c.Unread)
	}
	return fmt.Sprintf("%s%s\r\n%s  ·  %s", name, unread, c.LastAt.Format("02/01 15:04"), truncate(c.LastMessage, 42))
}

func (a *application) handleChatEvent(raw any) {
	switch evt := raw.(type) {
	case *events.Message:
		if err := a.chatStore.SaveEvent(context.Background(), evt, true); err != nil {
			a.appendLog("Erro ao salvar mensagem: " + err.Error())
			return
		}
		a.ui(func() { a.refreshChats(); a.refreshOpenChat(evt.Info.Chat.String()) })
	case *events.HistorySync:
		go a.importHistory(evt)
	}
}

func (a *application) importHistory(evt *events.HistorySync) {
	if evt == nil || evt.Data == nil {
		return
	}
	count := 0
	for _, conv := range evt.Data.GetConversations() {
		jid, err := types.ParseJID(conv.GetID())
		if err != nil {
			continue
		}
		for _, history := range conv.GetMessages() {
			parsed, err := a.mgr.Client.ParseWebMessage(jid, history.GetMessage())
			if err != nil {
				continue
			}
			if a.chatStore.SaveEvent(context.Background(), parsed, false) == nil {
				count++
			}
		}
	}
	a.appendLog(fmt.Sprintf("Sincronização de histórico: %d mensagem(ns) processada(s).", count))
	a.ui(a.refreshChats)
}

func (a *application) refreshChats() {
	if a.chatStore == nil || a.chatModel == nil || a.chatList == nil {
		return
	}
	search := ""
	if a.chatSearch != nil {
		search = a.chatSearch.Text()
	}
	items, err := a.chatStore.Conversations(context.Background(), search)
	if err != nil {
		a.appendLog("Erro ao listar conversas: " + err.Error())
		return
	}
	selected := ""
	idx := a.chatList.CurrentIndex()
	if idx >= 0 && idx < len(a.chatModel.items) {
		selected = a.chatModel.items[idx].JID
	}
	a.chatModel.items = items
	a.chatModel.PublishItemsReset()
	newIndex := -1
	for i, c := range items {
		if c.JID == selected {
			newIndex = i
			break
		}
	}
	if newIndex < 0 && len(items) > 0 {
		newIndex = 0
	}
	a.chatList.SetCurrentIndex(newIndex)
}

func (a *application) openSelectedChat() {
	idx := a.chatList.CurrentIndex()
	if idx < 0 || idx >= len(a.chatModel.items) {
		return
	}
	c := a.chatModel.items[idx]
	a.chatTitle.SetText(c.Name)
	if c.Name == "" {
		a.chatTitle.SetText(c.JID)
	}
	_ = a.chatStore.MarkRead(context.Background(), c.JID)
	a.renderChat(c.JID)
}

func (a *application) refreshOpenChat(jid string) {
	idx := a.chatList.CurrentIndex()
	if idx >= 0 && idx < len(a.chatModel.items) && a.chatModel.items[idx].JID == jid {
		a.renderChat(jid)
	}
}

func (a *application) renderChat(jid string) {
	messages, err := a.chatStore.Messages(context.Background(), jid, 300)
	if err != nil {
		a.appendLog("Erro ao abrir conversa: " + err.Error())
		return
	}
	var b strings.Builder
	for _, m := range messages {
		author := "Contato"
		if m.FromMe {
			author = "Você"
		}
		fmt.Fprintf(&b, "%s  %s\r\n%s\r\n\r\n", author, m.Timestamp.Format("02/01/2006 15:04"), m.Text)
	}
	a.chatHistory.SetText(b.String())
}

func (a *application) sendChatMessage() {
	idx := a.chatList.CurrentIndex()
	if idx < 0 || idx >= len(a.chatModel.items) {
		walk.MsgBox(a.mw, "Conversas", "Selecione uma conversa.", walk.MsgBoxIconInformation)
		return
	}
	text := strings.TrimSpace(a.chatCompose.Text())
	if text == "" {
		return
	}
	target := a.chatModel.items[idx]
	jid, err := types.ParseJID(target.JID)
	if err != nil {
		walk.MsgBox(a.mw, "Conversa inválida", err.Error(), walk.MsgBoxIconError)
		return
	}
	a.chatCompose.SetEnabled(false)
	go func() {
		resp, sendErr := a.mgr.Client.SendMessage(a.ctx, jid, &waE2E.Message{Conversation: proto.String(text)})
		if sendErr != nil {
			a.appendLog("Falha ao enviar resposta: " + sendErr.Error())
			a.ui(func() { a.chatCompose.SetEnabled(true) })
			return
		}
		evt := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: jid, Sender: types.EmptyJID, IsFromMe: true, IsGroup: jid.Server == types.GroupServer}, ID: resp.ID, Timestamp: time.Now()}, Message: &waE2E.Message{Conversation: proto.String(text)}}
		_ = a.chatStore.SaveEvent(context.Background(), evt, false)
		a.ui(func() {
			a.chatCompose.SetText("")
			a.chatCompose.SetEnabled(true)
			a.refreshChats()
			a.renderChat(target.JID)
		})
	}()
}
