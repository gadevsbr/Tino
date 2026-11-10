package whatsapp

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/authorization"
	"github.com/gadevsbr/tino/internal/assistente/bitz"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"github.com/gadevsbr/tino/internal/assistente/config"
	opdb "github.com/gadevsbr/tino/internal/assistente/database"
	"github.com/gadevsbr/tino/internal/assistente/omnibees"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func TestCommercialProofIsAcceptedOnlyAfterPaymentInstruction(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := opdb.Open(ctx, filepath.Join(dir, "hotel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	commercialService, err := commercial.New(ctx, db, func(_ context.Context, _, _, text string) (commercial.QuoteReply, error) {
		if text == "orçamento" {
			return commercial.QuoteReply{Text: "check-in", Active: true}, nil
		}
		categories := []omnibees.Category{{Key: "duplo", Name: "Duplo", TotalCents: 250420}}
		return commercial.QuoteReply{Categories: categories, Plan: &commercial.QuotePlan{CheckIn: "20/11/2026", CheckOut: "22/11/2026", Rooms: []commercial.QuoteRoom{{Adults: 2, Ages: []int{3}, Categories: categories}}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = commercialService.Configure(ctx, "account", commercial.Config{Mode: commercial.Public, FinalMessage3: "envie o comprovante"}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i, text := range []string{"oi", "1", "fim", "1"} {
		if _, err = commercialService.Handle(ctx, commercial.Input{Account: "account", Contact: "5573999999999", MessageID: string(rune('a' + i)), Text: text, Now: now.Add(time.Duration(i) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}
	auth, err := authorization.New(config.Config{}, db)
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := bitz.NewJobs(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	sent := 0
	s := &Service{domainDB: db, commercial: commercialService, auth: auth, bitzJobs: jobs, bitzStore: bitz.NewStore(filepath.Join(dir, "bitz.json")), downloadOverride: func(context.Context, whatsmeow.DownloadableMessage) ([]byte, error) {
		return []byte("%PDF-1.4 proof"), nil
	}, sendOverride: func(context.Context, types.JID, *waE2E.Message) (whatsmeow.SendResponse, error) {
		sent++
		return whatsmeow.SendResponse{}, nil
	}}
	if err = s.ensureTransportSchema(ctx); err != nil {
		t.Fatal(err)
	}
	handled, err := s.handleCommercialProof(ctx, "account", "5573999999999", "proof-1", types.NewJID("5573999999999", types.DefaultUserServer), nil, &waE2E.DocumentMessage{Mimetype: proto.String("application/pdf"), FileName: proto.String("pix.pdf")})
	if err == nil || !handled || sent != 0 {
		t.Fatalf("handled=%t sent=%d err=%v", handled, sent, err)
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM commercial_payment_proofs WHERE contact=?`, "5573999999999").Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	s.uploadOverride = func(context.Context, []byte, whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
		return whatsmeow.UploadResponse{}, nil
	}
	if err = s.sendCommercialProof(ctx, types.NewJID("5573999999999", types.DefaultUserServer), "validado", commercialProof{MediaType: "application/pdf", FileName: "pix.pdf", Data: []byte("%PDF-1.4 proof")}); err != nil || sent != 1 {
		t.Fatalf("confirmation proof sent=%d err=%v", sent, err)
	}
}
