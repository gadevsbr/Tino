package catalog

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

const accountA = "111@s.whatsapp.net"
const accountB = "222@s.whatsapp.net"

var testCategories = []Category{{"duplo", "Suíte Dupla"}, {"familia", "Suíte Família"}}

type harness struct {
	t       *testing.T
	db      *sql.DB
	repo    *Repository
	service *Service
	input   Input
	path    string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, path: filepath.Join(t.TempDir(), "catalog.db"), input: Input{AccountJID: accountA, OperatorJID: "operator", ChatJID: "private-chat", Authorized: true}}
	h.open()
	t.Cleanup(func() { h.db.Close() })
	return h
}
func (h *harness) open() {
	h.t.Helper()
	var err error
	h.db, err = sql.Open("sqlite", h.path)
	if err != nil {
		h.t.Fatal(err)
	}
	h.db.SetMaxOpenConns(1)
	h.repo = NewRepository(h.db)
	if err = h.repo.EnsureSchema(context.Background()); err != nil {
		h.t.Fatal(err)
	}
	h.service = NewService(h.repo, testCategories)
}
func (h *harness) handle(text string) Result {
	h.t.Helper()
	in := h.input
	in.Text = text
	r, e := h.service.Handle(context.Background(), in)
	if e != nil {
		h.t.Fatal(e)
	}
	return r
}
func photo(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func product(owner, id string) *waE2E.ProductMessage {
	return &waE2E.ProductMessage{BusinessOwnerJID: proto.String(owner), Product: &waE2E.ProductMessage_ProductSnapshot{ProductID: proto.String(id), Title: proto.String("Synthetic suite"), CurrencyCode: proto.String("BRL"), PriceAmount1000: proto.Int64(100000), SignedURL: proto.String("https://private.invalid/signed?token=secret"), URL: proto.String("https://private.invalid/media"), ProductImage: &waE2E.ImageMessage{URL: proto.String("https://private.invalid/photo"), DirectPath: proto.String("/secret/path"), MediaKey: []byte("secret-key")}}, ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("private-context")}}
}
func (h *harness) capture(owner, id string) {
	h.t.Helper()
	in := h.input
	in.Product = product(owner, id)
	in.Image = photo(h.t)
	r, e := h.service.Handle(context.Background(), in)
	if e != nil || !strings.Contains(r.Text, "Escolha") {
		h.t.Fatalf("capture: %+v %v", r, e)
	}
}
func (h *harness) get(account string) Entry {
	h.t.Helper()
	e, err := h.repo.Get(context.Background(), account, "duplo")
	if err != nil {
		h.t.Fatal(err)
	}
	return e
}
func (h *harness) absent(account string) {
	h.t.Helper()
	_, err := h.repo.Get(context.Background(), account, "duplo")
	if !errors.Is(err, sql.ErrNoRows) {
		h.t.Fatalf("expected absent mapping: %v", err)
	}
}

func TestDurableStageConfirmationAndSanitization(t *testing.T) {
	h := newHarness(t)
	h.handle("configurar catálogo")
	h.capture(accountA, "p1")
	h.absent(accountA)
	h.db.Close()
	h.open()
	h.handle("1")
	h.absent(accountA)
	h.handle("confirmar")
	h.db.Close()
	h.open()
	e := h.get(accountA)
	if e.OwnerJID != accountA || e.Payload.GetProduct().GetProductID() != "p1" || !bytes.Equal(e.Image, photo(t)) {
		t.Fatalf("entry not durable: %+v", e)
	}
	p := e.Payload.GetProduct()
	if p.ProductImage != nil || p.SignedURL != nil || p.URL != nil || e.Payload.ContextInfo != nil || e.Payload.Catalog != nil {
		t.Fatal("unsafe protobuf persisted")
	}
	raw, err := proto.Marshal(e.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("secret")) {
		t.Fatal("secret persisted")
	}
	if r := h.handle("confirmar"); r.Handled {
		t.Fatal("confirmation replay should not act")
	}
	var count int
	if err = h.db.QueryRow(`SELECT count(*) FROM catalog_sessions`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("stage left behind %d %v", count, err)
	}
}

func TestAccountOperatorChatIsolationAndOwnerConfirmation(t *testing.T) {
	h := newHarness(t)
	h.handle("configurar catalogo")
	h.capture(accountB, "foreign")
	h.handle("1")
	h.handle("confirmar")
	h.absent(accountA)
	for _, mutate := range []func(*Input){func(i *Input) { i.AccountJID = accountB }, func(i *Input) { i.OperatorJID = "other" }, func(i *Input) { i.ChatJID = "other" }, func(i *Input) { i.Authorized = false }} {
		in := h.input
		mutate(&in)
		in.Text = "confirmar proprietario"
		r, e := h.service.Handle(context.Background(), in)
		if e != nil || r.Handled {
			t.Fatalf("isolated session handled: %+v %v", r, e)
		}
	}
	h.handle("confirmar proprietario")
	h.absent(accountA)
	h.handle("confirmar")
	if h.get(accountA).OwnerJID != accountB {
		t.Fatal("explicit foreign owner not saved")
	}
	h.absent(accountB)
	list, err := h.repo.List(context.Background(), accountB)
	if err != nil || len(list) != 0 {
		t.Fatalf("wrong-account list: %v %v", list, err)
	}
	e, err := h.repo.Get(context.Background(), "111:5@s.whatsapp.net", "duplo")
	if err != nil || e.OwnerJID != accountB {
		t.Fatalf("device normalization: %+v %v", e, err)
	}
}

func TestCancelClearAndReplacement(t *testing.T) {
	h := newHarness(t)
	h.handle("configurar catalogo")
	h.capture(accountA, "original")
	h.handle("1")
	h.handle("cancelar")
	h.handle("confirmar")
	h.absent(accountA)
	h.handle("configurar catalogo")
	h.capture(accountA, "original")
	h.handle("1")
	h.handle("confirmar")
	h.handle("configurar catalogo")
	h.capture(accountB, "replacement")
	if r := h.handle("1"); !strings.Contains(r.Text, "substituirá") {
		t.Fatal(r.Text)
	}
	h.handle("confirmar proprietario")
	if h.get(accountA).Payload.GetProduct().GetProductID() != "original" {
		t.Fatal("premature replacement")
	}
	h.handle("confirmar")
	entries, err := h.repo.List(context.Background(), accountA)
	if err != nil || len(entries) != 1 || entries[0].OwnerJID != accountB {
		t.Fatalf("ambiguous replacement %+v %v", entries, err)
	}
	h.input.AccountJID = accountB
	h.handle("configurar catalogo")
	h.capture(accountB, "other-account")
	h.handle("1")
	h.handle("confirmar")
	h.input.AccountJID = accountA
	h.handle("limpar catalogo")
	h.get(accountA)
	h.handle("cancelar")
	h.get(accountA)
	h.handle("limpar catalogo")
	h.db.Close()
	h.open()
	h.handle("confirmar")
	h.absent(accountA)
	h.get(accountB)
}

func TestCapturePreflightAndImageValidation(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	accepts := func(want bool) {
		t.Helper()
		got, err := h.service.AcceptsProduct(ctx, h.input.AccountJID, h.input.OperatorJID, h.input.ChatJID)
		if err != nil || got != want {
			t.Fatalf("AcceptsProduct %v %v want %v", got, err, want)
		}
	}
	accepts(false)
	h.handle("configurar catalogo")
	accepts(true)
	for _, bad := range [][]byte{nil, []byte("not an image"), photo(t)[:30], make([]byte, MaxImageBytes+1)} {
		in := h.input
		in.Product = product(accountA, "p1")
		in.Image = bad
		r, err := h.service.Handle(ctx, in)
		if err != nil || !r.Handled {
			t.Fatalf("invalid image: %+v %v", r, err)
		}
		accepts(true)
		h.absent(accountA)
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	if err := ValidateImage(jpg.Bytes()); err != nil {
		t.Fatal(err)
	}
	h.handle("cancelar")
	accepts(false)
	in := h.input
	in.Product = product(accountA, "late")
	in.Image = photo(t)
	r, err := h.service.Handle(ctx, in)
	if err != nil || r.Handled {
		t.Fatal("download after cancellation accepted")
	}
	h.handle("configurar catalogo")
	h.capture(accountA, "p1")
	accepts(false)
	h.handle("0")
	h.handle("confirmar")
	h.absent(accountA)
}

func TestConcurrentReplacementRequiresNewConfirmation(t *testing.T) {
	h := newHarness(t)
	h.handle("configurar catalogo")
	h.capture(accountA, "first")
	h.handle("1")
	h.input.OperatorJID = "other"
	h.handle("configurar catalogo")
	h.capture(accountA, "second")
	h.handle("1")
	h.handle("confirmar")
	h.input.OperatorJID = "operator"
	r := h.handle("confirmar")
	if !strings.Contains(r.Text, "mudou") {
		t.Fatal(r.Text)
	}
	if h.get(accountA).Payload.GetProduct().GetProductID() != "second" {
		t.Fatal("stale confirmation replaced data")
	}
	h.handle("confirmar")
	if h.get(accountA).Payload.GetProduct().GetProductID() != "first" {
		t.Fatal("confirmed replacement failed")
	}
}

func TestInspectorDoesNotInventCatalogID(t *testing.T) {
	m := product(accountA, "p1")
	m.Catalog = &waE2E.ProductMessage_CatalogSnapshot{Title: proto.String("catalog title is not an ID")}
	p, ok := Parse(&waE2E.Message{ProductMessage: m})
	if !ok {
		t.Fatal("parse failed")
	}
	summary := Summary(p)
	if strings.Contains(summary, "CatalogID") || strings.Contains(summary, "catalog title") || strings.Contains(summary, "secret") {
		t.Fatal(summary)
	}
	if !strings.Contains(summary, "Proprietário: "+accountA) {
		t.Fatal(summary)
	}
	clean, err := Sanitize(m)
	if err != nil || clean.Catalog != nil {
		t.Fatalf("catalog persisted: %v %v", clean, err)
	}
}

func TestOptionalPriceNeverBecomesFree(t *testing.T) {
	m := product(accountA, "no-price")
	m.Product.PriceAmount1000 = nil
	m.Product.CurrencyCode = nil
	clean, err := Sanitize(m)
	if err != nil {
		t.Fatal(err)
	}
	if clean.Product.PriceAmount1000 != nil || clean.Product.SalePriceAmount1000 != nil || clean.Product.CurrencyCode != nil || clean.Product.ProductImageCount != nil {
		t.Fatal("invented optional fields")
	}
	p, _ := Parse(&waE2E.Message{ProductMessage: clean})
	if !strings.Contains(Summary(p), "Preço: não informado") {
		t.Fatal(Summary(p))
	}
	m.Product.PriceAmount1000 = proto.Int64(150000)
	m.Product.SalePriceAmount1000 = proto.Int64(125500)
	m.Product.CurrencyCode = proto.String("BRL")
	m.Product.ProductImageCount = proto.Uint32(3)
	clean, err = Sanitize(m)
	if err != nil {
		t.Fatal(err)
	}
	*m.Product.SalePriceAmount1000 = 1
	if clean.Product.GetSalePriceAmount1000() != 125500 || clean.Product.GetProductImageCount() != 3 {
		t.Fatal("optional values not preserved independently")
	}
	p, _ = Parse(&waE2E.Message{ProductMessage: clean})
	if !strings.Contains(Summary(p), "Preço: R$ 150,00") {
		t.Fatal(Summary(p))
	}
	h := newHarness(t)
	h.handle("configurar catalogo")
	in := h.input
	in.Product = product(accountA, "no-price")
	in.Product.Product.PriceAmount1000 = nil
	in.Product.Product.CurrencyCode = nil
	in.Image = photo(t)
	if _, err = h.service.Handle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	h.handle("1")
	h.handle("confirmar")
	h.db.Close()
	h.open()
	persisted := h.get(accountA).Payload.Product
	if persisted.PriceAmount1000 != nil || persisted.CurrencyCode != nil {
		t.Fatal("restart fabricated price")
	}
}

func TestClearInvalidatesOtherPendingCaptures(t *testing.T) {
	h := newHarness(t)
	h.handle("configurar catalogo")
	h.capture(accountA, "pending")
	h.handle("1")
	h.input.OperatorJID = "other"
	h.handle("limpar catalogo")
	h.handle("confirmar")
	h.input.OperatorJID = "operator"
	if h.handle("confirmar").Handled {
		t.Fatal("old stage survived clear")
	}
	h.absent(accountA)
}

func TestUnauthorizedCannotBeginOrCapture(t *testing.T) {
	h := newHarness(t)
	h.input.Authorized = false
	if h.handle("configurar catalogo").Handled {
		t.Fatal("unauthorized start")
	}
	h.input.Authorized = true
	h.handle("configurar catalogo")
	in := h.input
	in.Authorized = false
	in.Product = product(accountA, "denied")
	in.Image = photo(t)
	r, err := h.service.Handle(context.Background(), in)
	if err != nil || r.Handled {
		t.Fatalf("unauthorized capture %+v %v", r, err)
	}
	allowed, err := h.service.AcceptsProduct(context.Background(), accountA, in.OperatorJID, in.ChatJID)
	if err != nil || !allowed {
		t.Fatal("unauthorized capture changed stage")
	}
	h.absent(accountA)
}

func TestSanitizeDropsUnknownFieldsWithoutMutatingSource(t *testing.T) {
	m := product(accountA, "p1")
	m.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	m.Product.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	clean, err := Sanitize(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(clean.ProtoReflect().GetUnknown()) != 0 || len(clean.Product.ProtoReflect().GetUnknown()) != 0 {
		t.Fatal("unknown fields persisted")
	}
	if m.Product.ProductImage == nil || m.Product.SignedURL == nil || m.ContextInfo == nil || len(m.ProtoReflect().GetUnknown()) == 0 {
		t.Fatal("source mutated")
	}
}
