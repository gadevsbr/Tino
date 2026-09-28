package catalog

import (
	"fmt"

	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
)

type Product struct {
	ProductID   string
	OwnerJID    string
	Title       string
	Description string
	Currency    string
	Price1000   int64
	HasPrice    bool
	RetailerID  string
}

func Parse(message *waE2E.Message) (Product, bool) {
	if message == nil || message.GetProductMessage() == nil || message.GetProductMessage().GetProduct() == nil {
		return Product{}, false
	}
	m := message.GetProductMessage()
	p := m.GetProduct()
	result := Product{ProductID: p.GetProductID(), OwnerJID: m.GetBusinessOwnerJID(), Title: p.GetTitle(), Description: p.GetDescription(), Currency: p.GetCurrencyCode(), Price1000: p.GetPriceAmount1000(), HasPrice: p.PriceAmount1000 != nil, RetailerID: p.GetRetailerID()}
	return result, true
}

func Summary(p Product) string {
	price := "não informado"
	if p.HasPrice {
		currency := p.Currency
		if currency == "BRL" {
			currency = "R$"
		}
		if currency == "" {
			currency = "moeda não informada"
		}
		// Avoid floating point rounding and preserve the protocol's thousandths.
		units, fraction := p.Price1000/1000, p.Price1000%1000
		sign := ""
		if p.Price1000 < 0 {
			sign = "-"
			units = -units
			fraction = -fraction
		}
		if fraction%10 == 0 {
			price = fmt.Sprintf("%s %s%d,%02d", currency, sign, units, fraction/10)
		} else {
			price = fmt.Sprintf("%s %s%d,%03d", currency, sign, units, fraction)
		}
	}
	return fmt.Sprintf("🛍️ PRODUTO DO CATÁLOGO\n\nNome: %s\nProductID: %s\nProprietário: %s\nPreço: %s\nDescrição: %s\n\nConfirme esses dados antes de relacionar ao orçamento.", p.Title, p.ProductID, p.OwnerJID, price, p.Description)
}
