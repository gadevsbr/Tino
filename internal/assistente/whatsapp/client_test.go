package whatsapp

import (
	"context"
	"database/sql"
	"github.com/gadevsbr/tino/internal/assistente/config"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"strings"
	"testing"
	"time"
)

func TestCashReceiptImageRequiresCaptionKeyword(t *testing.T) {
	tests := []struct {
		name    string
		message *waE2E.ImageMessage
		want    bool
	}{
		{name: "nil", message: nil},
		{name: "empty", message: &waE2E.ImageMessage{}},
		{name: "unrelated", message: &waE2E.ImageMessage{Caption: proto.String("foto do quarto")}},
		{name: "exact", message: &waE2E.ImageMessage{Caption: proto.String("comprovante")}, want: true},
		{name: "uppercase", message: &waE2E.ImageMessage{Caption: proto.String("COMPROVANTE PIX")}, want: true},
		{name: "sentence", message: &waE2E.ImageMessage{Caption: proto.String("foto do comprovante:")}, want: true},
		{name: "plural is not the keyword", message: &waE2E.ImageMessage{Caption: proto.String("comprovantes")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCashReceiptImage(tt.message); got != tt.want {
				t.Fatalf("isCashReceiptImage()=%t, want %t", got, tt.want)
			}
		})
	}
}

func TestRejectsInvalidDNSServer(t *testing.T) {
	_, err := New(context.Background(), config.Config{DNSServer: "invalid"}, &sql.DB{}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid DNS_SERVER") {
		t.Fatalf("err=%v", err)
	}
}

func TestPhoneUserPrefersPhoneAlternativeForLID(t *testing.T) {
	got := phoneUser(types.NewJID("123", types.HiddenUserServer), types.NewJID("5511987654321", types.DefaultUserServer))
	if got != "5511987654321" {
		t.Fatalf("got %s", got)
	}
}

func TestSelfChatAcceptsPhoneOrLIDOnlyWhenFromMe(t *testing.T) {
	id := types.NewJID("5511987654321", types.DefaultUserServer)
	lid := types.NewJID("123456789", types.HiddenUserServer)
	if !isSelfChat(&id, lid, types.NewJID(id.User, types.DefaultUserServer), true) {
		t.Fatal("phone self chat rejected")
	}
	if !isSelfChat(&id, lid, types.NewJID(lid.User, types.HiddenUserServer), true) {
		t.Fatal("LID self chat rejected")
	}
	if isSelfChat(&id, lid, types.NewJID("999", types.DefaultUserServer), true) {
		t.Fatal("other chat accepted")
	}
	if isSelfChat(&id, lid, id, false) {
		t.Fatal("incoming chat accepted as self")
	}
}

func TestDiagnosticCommandPreview(t *testing.T) {
	if got := diagnosticCommandPreview("extrato seana 1 setembro 2026"); got != "extrato seana 1 setembro 2026" {
		t.Fatalf("misspelled command not captured: %q", got)
	}
	if got := diagnosticCommandPreview("  orçamento\n  "); got != "orçamento" {
		t.Fatalf("quote not normalized: %q", got)
	}
	if got := diagnosticCommandPreview("hóspede João telefone 5573"); got != "" {
		t.Fatalf("unrelated message logged: %q", got)
	}
	if got := diagnosticCommandPreview("extratos " + strings.Repeat("x", 200)); len([]rune(got)) > 121 {
		t.Fatalf("command not truncated: %d runes", len([]rune(got)))
	}
}

func TestCashReportFilenameDate(t *testing.T) {
	if got := cashReportFilenameDate("2026-09-18"); got != "18-09-2026" {
		t.Fatalf("got %q", got)
	}
}

func TestProcessableMessageAcceptsOperatorCommandInGroup(t *testing.T) {
	evt := &events.Message{Info: types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:    types.NewJID("120363000000000000", types.GroupServer),
			Sender:  types.NewJID("5573988333657", types.DefaultUserServer),
			IsGroup: true,
		},
		ID: "group-command-1",
	}, Message: &waE2E.Message{Conversation: proto.String("menu")}}
	if !processableMessage(evt) {
		t.Fatal("group operator command was rejected before authorization")
	}
}

func TestCashReportRangeDefaultsToCurrentMonth(t *testing.T) {
	now := time.Date(2026, 9, 28, 14, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	from, until := cashReportRange("", "", now)
	if from != "2026-09-01" || until != "2026-09-28" {
		t.Fatalf("range=%s..%s", from, until)
	}
	from, until = cashReportRange("2026-09-18", "", now)
	if from != "2026-09-18" || until != "2026-09-18" {
		t.Fatalf("explicit range=%s..%s", from, until)
	}
}
