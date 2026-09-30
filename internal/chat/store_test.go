package chat

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestStoreConversationHistory(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "chats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("5511", types.DefaultUserServer), Sender: types.NewJID("5511", types.DefaultUserServer)}, ID: "m1", PushName: "Ana", Timestamp: time.Unix(100, 0)}, Message: &waE2E.Message{Conversation: proto.String("Olá")}}
	if err := s.SaveEvent(context.Background(), e, true); err != nil {
		t.Fatal(err)
	}
	chats, err := s.Conversations(context.Background(), "")
	if err != nil || len(chats) != 1 || chats[0].Unread != 1 {
		t.Fatalf("chats=%#v err=%v", chats, err)
	}
	messages, err := s.Messages(context.Background(), e.Info.Chat.String(), 50)
	if err != nil || len(messages) != 1 || messages[0].Text != "Olá" {
		t.Fatalf("messages=%#v err=%v", messages, err)
	}
	if err := s.UpdateName(context.Background(), e.Info.Chat.String(), "Ana Souza"); err != nil {
		t.Fatal(err)
	}
	chats, err = s.Conversations(context.Background(), "Ana Souza")
	if err != nil || len(chats) != 1 || chats[0].Name != "Ana Souza" {
		t.Fatalf("renamed chats=%#v err=%v", chats, err)
	}
}
