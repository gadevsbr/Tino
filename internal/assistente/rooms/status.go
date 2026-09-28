package rooms

import (
	"fmt"
	"github.com/gadevsbr/tino/internal/assistente/utils"
)

type Status string

const (
	AvailableClean Status = "DISPONIVEL"
	OccupiedClean  Status = "MANUTENCAO_LIMPEZA"
	CheckoutEntry  Status = "SAIDA_ENTRADA"
	Entry          Status = "ENTRADA"
	CheckoutToday  Status = "SAIDA"
	Maintenance    Status = "INTERDITADO"
	Dirty          Status = "LIMPAR"
	CleanUnmade    Status = "LIMPO_DESFORRADO"
)

var orderedStatuses = []Status{AvailableClean, CleanUnmade, OccupiedClean, CheckoutEntry, Entry, CheckoutToday, Maintenance, Dirty}
var aliases = map[string]Status{
	"verde": AvailableClean, "disponivel": AvailableClean, "limpo": AvailableClean, "disponivel e limpo": AvailableClean,
	"verde roxo": CleanUnmade, "verde e roxo": CleanUnmade, "limpo desforrado": CleanUnmade, "limpo mas desforrado": CleanUnmade, "desforrado": CleanUnmade,
	"laranja": OccupiedClean, "manutencao de limpeza": OccupiedClean, "ocupado": OccupiedClean, "ocupado com manutencao de limpeza": OccupiedClean,
	"rosa": CheckoutEntry, "saida e entrada": CheckoutEntry,
	"amarelo": Entry, "entrada": Entry,
	"magenta": CheckoutToday, "vermelho": CheckoutToday, "saida": CheckoutToday, "saida hoje": CheckoutToday, "checkout": CheckoutToday, "check out hoje": CheckoutToday,
	"cinza": Maintenance, "interditado": Maintenance, "manutencao": Maintenance, "em manutencao": Maintenance, "bloqueado": Maintenance,
	"roxo": Dirty, "limpar": Dirty, "sujo": Dirty,
}

func ParseStatus(v string) (Status, bool) { s, ok := aliases[utils.Normalize(v)]; return s, ok }
func (s Status) Valid() bool {
	for _, v := range orderedStatuses {
		if s == v {
			return true
		}
	}
	return false
}
func (s Status) Label() string {
	switch s {
	case AvailableClean:
		return "DISPONÍVEL"
	case OccupiedClean:
		return "MANUTENÇÃO DE LIMPEZA"
	case CheckoutEntry:
		return "SAÍDA E ENTRADA"
	case Entry:
		return "ENTRADA"
	case CheckoutToday:
		return "SAÍDA"
	case Maintenance:
		return "INTERDITADO"
	case Dirty:
		return "LIMPAR"
	case CleanUnmade:
		return "LIMPO, MAS DESFORRADO"
	}
	return string(s)
}
func (s Status) Emoji() string {
	switch s {
	case AvailableClean:
		return "🟢"
	case OccupiedClean:
		return "🟠"
	case CheckoutEntry:
		return "🩷"
	case Entry:
		return "🟡"
	case CheckoutToday:
		return "🟥"
	case Maintenance:
		return "⬜"
	case Dirty:
		return "🟣"
	case CleanUnmade:
		return "🟢🟣"
	}
	return ""
}
func (s Status) StringWithIcon() string { return fmt.Sprintf("%s %s", s.Emoji(), s.Label()) }
func (s Status) RGB() (int, int, int) {
	switch s {
	case AvailableClean:
		return 70, 175, 80
	case OccupiedClean:
		return 255, 128, 0
	case CheckoutEntry:
		return 255, 170, 170
	case Entry:
		return 255, 245, 0
	case CheckoutToday:
		return 210, 0, 55
	case Maintenance:
		return 150, 150, 150
	case Dirty:
		return 100, 5, 85
	case CleanUnmade:
		return 70, 175, 80
	}
	return 255, 255, 255
}

func (s Status) IsSplitColor() bool { return s == CleanUnmade }
