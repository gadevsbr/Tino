package rooms

import "strings"

type Category struct {
	Key, Label string
}

var BitzCategories = []Category{
	{Key: "superluxo", Label: "Duplo Superluxo com varanda e vista mar"},
	{Key: "familia", Label: "Família Deluxe com vista mar"},
	{Key: "triploDeluxe", Label: "Triplo Deluxe com varanda"},
	{Key: "quadruploDeluxe", Label: "Quádruplo Deluxe com varanda"},
	{Key: "quadruploVista", Label: "Quádruplo Deluxe com varanda e vista mar"},
	{Key: "triploVaranda", Label: "Triplo com varanda"},
	{Key: "quadruploVaranda", Label: "Quádruplo com varanda"},
	{Key: "duplo", Label: "Duplo"},
	{Key: "triplo", Label: "Triplo"},
}

func ValidBitzCategory(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return true
	}
	for _, category := range BitzCategories {
		if category.Key == key {
			return true
		}
	}
	return false
}

func BitzCategoryLabel(key string) string {
	for _, category := range BitzCategories {
		if category.Key == key {
			return category.Label
		}
	}
	return "Não configurada"
}
