package whatsapp

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/bitz"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/config"
	opdb "github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
	"github.com/gadevsbr/tino/internal/assistente/rooms"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
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

func TestPreReservationNoticeIncludesGuestAndRoomSummary(t *testing.T) {
	notice := preReservationNotice("ABC123", "5573999999999", commercial.PreReservation{
		CheckIn: "10/10/2026", CheckOut: "12/10/2026",
		Categories: []omnibees.Category{{Name: "Suíte Deluxe"}, {Name: "Suíte Interna"}},
		Rooms:      []commercial.QuoteRoom{{Adults: 2, Ages: []int{1, 7}}, {Adults: 1}},
	})
	for _, expected := range []string{"*+5573999999999*", "*Quarto 1 — Suíte Deluxe*", "2 adulto(s)", "2 criança(s): 1 ano, 7 anos", "*Quarto 2 — Suíte Interna*", "*aprovar pre-reserva ABC123*"} {
		if !strings.Contains(notice, expected) {
			t.Fatalf("notice missing %q:\n%s", expected, notice)
		}
	}
}

func TestTransportQueueSeparatesGuestsAndSerializesOperators(t *testing.T) {
	if a, b := transportQueueKey("hotel", "guest-a", false), transportQueueKey("hotel", "guest-b", false); a == b {
		t.Fatalf("different guests share queue key %q", a)
	}
	if a, b := transportQueueKey("hotel", "operator-a", true), transportQueueKey("hotel", "operator-b", true); a != b {
		t.Fatalf("operators must share account queue: %q != %q", a, b)
	}
}

func TestGuestReplyDelayWaitsConfiguredDuration(t *testing.T) {
	s := &Service{guestReplyDelay: 25 * time.Millisecond}
	started := time.Now()
	if !s.waitGuestReply(context.Background(), types.NewJID("5511999999999", types.DefaultUserServer)) {
		t.Fatal("delay cancelled unexpectedly")
	}
	if elapsed := time.Since(started); elapsed < 20*time.Millisecond {
		t.Fatalf("delay too short: %s", elapsed)
	}
}

func TestRoomAllocationPolicyExcludesInterdictedAndChecksCapacity(t *testing.T) {
	ctx := context.Background()
	db, err := opdb.Open(ctx, filepath.Join(t.TempDir(), "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := rooms.NewRepository(db)
	for _, number := range []int{201, 202} {
		if err := repo.SetBitzCategory(ctx, number, "triploDeluxe"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := repo.UpdateStatus(ctx, 201, rooms.Maintenance, "test", "test", "block-201"); err != nil {
		t.Fatal(err)
	}
	s := &Service{rooms: repo}
	one := []bitz.RoomCategory{{SourceKey: "triploDeluxe"}}
	if err := s.applyRoomAllocationPolicy(ctx, one); err != nil {
		t.Fatal(err)
	}
	if len(one[0].AllowedRooms) != 1 || one[0].AllowedRooms[0] != 202 {
		t.Fatalf("allowed=%v", one[0].AllowedRooms)
	}
	two := []bitz.RoomCategory{{SourceKey: "triploDeluxe"}, {SourceKey: "triploDeluxe"}}
	if err := s.applyRoomAllocationPolicy(ctx, two); err == nil || !strings.Contains(err.Error(), "somente 1 UH") {
		t.Fatalf("capacity err=%v", err)
	}
}
