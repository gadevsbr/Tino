package catalog

import (
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
	"strings"
	"testing"
)

func TestParseProduct(t *testing.T) {
	m := &waE2E.Message{ProductMessage: &waE2E.ProductMessage{BusinessOwnerJID: proto.String("5511@s.whatsapp.net"), Product: &waE2E.ProductMessage_ProductSnapshot{ProductID: proto.String("p1"), Title: proto.String("Suíte"), CurrencyCode: proto.String("BRL"), PriceAmount1000: proto.Int64(150000)}}}
	p, ok := Parse(m)
	if !ok || p.ProductID != "p1" || p.Price1000 != 150000 || !strings.Contains(Summary(p), "ProductID: p1") {
		t.Fatalf("p=%#v ok=%v", p, ok)
	}
}
