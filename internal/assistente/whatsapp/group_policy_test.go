package whatsapp

import (
	"context"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestGroupsAllowOnlyRoomAndCashReports(t *testing.T) {
	for _, text := range []string{"relatorio em pdf", "relatorio caixa em pdf", "relatorio caixa em pdf 09/10/2026"} {
		if !groupReportCommand(text) {
			t.Fatal("report blocked", text)
		}
	}
	for _, text := range []string{"menu", "oi", "status", "101 verde", "caixa", "boletos", "boleto pago 1", "aprovar pre-reserva ABC", "relatorio vales em pdf", "comercial ativar"} {
		if groupReportCommand(text) {
			t.Fatal("group command allowed", text)
		}
	}
	group := types.NewJID("120363000000001", types.GroupServer)
	sent := 0
	s := &Service{sendOverride: func(context.Context, types.JID, *waE2E.Message) (whatsmeow.SendResponse, error) {
		sent++
		return whatsmeow.SendResponse{}, nil
	}}
	msg := &waE2E.Message{Conversation: proto.String("resposta comum")}
	if _, err := s.sendMessage(context.Background(), group, msg); err == nil || sent != 0 {
		t.Fatal("group reply escaped", err, sent)
	}
	if err := s.ShareReport(context.Background(), "ADVANCES", group, "", "", ""); err == nil || sent != 0 {
		t.Fatal("advance report escaped", err, sent)
	}
	if _, err := s.sendMessage(context.WithValue(context.Background(), groupReportKey{}, true), group, msg); err != nil || sent != 1 {
		t.Fatal("report egress blocked", err, sent)
	}
	if _, err := s.sendMessage(context.Background(), types.NewJID("5511999999999", types.DefaultUserServer), msg); err != nil || sent != 2 {
		t.Fatal("private reply blocked", err, sent)
	}
}
