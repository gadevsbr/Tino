package omnibees

// CategoryDefinitions is the stable physical-room mapping for catalog setup.
// Zero totals are definitions only, never offers or confirmed prices.
func CategoryDefinitions() []Category {
	return []Category{
		{Key: "superluxo", Name: "Suíte Superluxo com varanda e vista mar"},
		{Key: "familia", Name: "Suíte Família"},
		{Key: "deluxe_vista_mar", Name: "Suíte Deluxe com vista para o mar"},
		{Key: "deluxe_varanda", Name: "Suíte Deluxe com varanda"},
		{Key: "varanda", Name: "Suíte com varanda"},
		{Key: "interna", Name: "Suíte interna"},
	}
}
