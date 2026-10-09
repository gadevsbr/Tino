package whatsapp

import "github.com/gadevsbr/tino/internal/assistente/commands"

// Only the two report builders grant group egress; ordinary replies never do.
type groupReportKey struct{}

func groupReportCommand(text string) bool {
	return isReportCommand(text) || commands.Parse(text).Kind == commands.CashReport
}
