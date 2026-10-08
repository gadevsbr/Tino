package whatsapp

import (
	"context"
	"fmt"
	assistapp "github.com/gadevsbr/tino/internal/assistente/app"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/config"
	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	"github.com/gadevsbr/tino/internal/session"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPhoneCommandsThroughAttachedTransport(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(dir, "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mgr, err := session.Open(ctx, dir, "test", false)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	owner := types.NewJID("5511999999999", types.DefaultUserServer)
	mgr.Client.Store.ID = &owner
	phone := types.NewJID("5511888888888", types.DefaultUserServer)
	cfg := config.Config{Authorized: map[string]struct{}{phone.User: {}}, Timezone: time.UTC, DataDir: dir}
	application := assistapp.New(rooms.NewRepository(db, time.UTC), conversation.NewRepository(db))
	service, err := NewWithClient(ctx, cfg, db, application, mgr.Client)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	replies := make(chan string, 5)
	service.sendOverride = func(_ context.Context, _ types.JID, message *waE2E.Message) (whatsmeow.SendResponse, error) {
		replies <- message.GetConversation()
		return whatsmeow.SendResponse{ID: "reply"}, nil
	}
	for i, source := range []types.MessageSource{{Chat: phone, Sender: phone}, {Chat: owner, Sender: owner, IsFromMe: true}, {Chat: types.NewJID("120363000000001", types.GroupServer), Sender: phone, IsGroup: true}} {
		for _, command := range []string{"menu", "status"} {
			evt := &events.Message{Info: types.MessageInfo{MessageSource: source, ID: types.MessageID(fmt.Sprintf("command-%d-%s", i, command))}, Message: &waE2E.Message{Conversation: proto.String(command)}}
			if service.AllowsFlow(evt) {
				t.Fatal("operator command admitted to AI flow")
			}
			service.onEvent(evt)
			select {
			case reply := <-replies:
				if strings.TrimSpace(reply) == "" {
					t.Fatal("empty command reply")
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("command %s timed out", command)
			}
		}
	}
	guest := types.NewJID("5511777777777", types.DefaultUserServer)
	evt := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: guest, Sender: guest}, ID: "guest"}, Message: &waE2E.Message{Conversation: proto.String("ola")}}
	if !service.AllowsFlow(evt) {
		t.Fatal("guest flow should be available while commercial is disabled")
	}
	if err := service.commercial.Configure(ctx, service.accountJID(), commercial.Config{Mode: commercial.Public}); err != nil {
		t.Fatal(err)
	}
	if service.AllowsFlow(evt) {
		t.Fatal("commercial guest must not receive a second AI flow reply")
	}
}
