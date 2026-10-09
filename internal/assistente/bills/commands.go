package bills

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func IsCommand(text string) bool {
	n := strings.ToLower(strings.TrimSpace(text))
	return n == "boletos" || n == "boleto" || n == "contas a pagar" || strings.HasPrefix(n, "boleto ") || strings.HasPrefix(n, "cancelar boleto ") || strings.HasPrefix(n, "lembretes boletos ")
}
func (r *Repository) Command(ctx context.Context, text string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(text))
	if n == "boletos" || n == "contas a pagar" {
		list, err := r.List(ctx)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		b.WriteString("📅 BOLETOS PENDENTES\n")
		count := 0
		for _, x := range list {
			if x.Status != "OPEN" {
				continue
			}
			count++
			if count <= 30 {
				fmt.Fprintf(&b, "\n#%d — %s\nR$ %s • %s\n", x.ID, x.Description, Money(x.AmountCents), dateLabel(x.DueDate))
			}
		}
		if count == 0 {
			b.WriteString("\nNenhum boleto pendente.\n")
		}
		if count > 30 {
			fmt.Fprintf(&b, "\nMais %d boletos no aplicativo.\n", count-30)
		}
		b.WriteString("\nCadastrar: boleto 20/10/2026 150,00 | Energia\nDar baixa: boleto pago 1\nConfigurar avisos: lembretes boletos TELEFONE 3 09:00\nDesativar: lembretes boletos desativar")
		return b.String(), nil
	}
	if n == "lembretes boletos desativar" {
		cfg, err := r.Settings(ctx)
		if err != nil {
			return "", err
		}
		cfg.Enabled = false
		return "Lembretes de boletos desativados.", r.Configure(ctx, cfg)
	}
	if strings.HasPrefix(n, "lembretes boletos ") {
		parts := strings.Fields(n)
		if len(parts) != 5 {
			return "Use lembretes boletos TELEFONE_COM_DDI DIAS HH:MM. Exemplo: lembretes boletos 5573999999999 3 09:00", nil
		}
		days, e := strconv.Atoi(parts[3])
		clock, ce := time.Parse("15:04", parts[4])
		if e != nil || ce != nil {
			return "Informe dias inteiros e horário HH:MM.", nil
		}
		err := r.Configure(ctx, Settings{Enabled: true, Target: parts[2], DaysBefore: days, Hour: clock.Hour(), Minute: clock.Minute()})
		return "Lembretes configurados. O Tino precisa estar aberto e conectado para enviar os avisos.", err
	}
	if strings.HasPrefix(n, "boleto pago ") || strings.HasPrefix(n, "cancelar boleto ") {
		parts := strings.Fields(n)
		if len(parts) != 3 {
			return "Informe o número do boleto.", nil
		}
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || id <= 0 {
			return "Informe um número de boleto válido.", nil
		}
		status := "PAID"
		reply := "Boleto marcado como pago. Lembretes encerrados; o caixa não foi alterado."
		if strings.HasPrefix(n, "cancelar") {
			status = "CANCELLED"
			reply = "Boleto cancelado. Lembretes encerrados."
		}
		return reply, r.SetStatus(ctx, id, status)
	}
	match := regexp.MustCompile(`(?i)^boleto\s+(\d{2}/\d{2}/\d{4})\s+([0-9.,]+)\s*\|\s*(.+)$`).FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return "Cadastre assim: boleto 20/10/2026 150,00 | Energia\nConsulte com boletos; marque pago com boleto pago NUMERO.", nil
	}
	date, err := time.Parse("02/01/2006", match[1])
	if err != nil {
		return "Data de vencimento inválida.", nil
	}
	cents, err := ParseMoney(match[2])
	if err != nil {
		return "Valor inválido. Use 150,00.", nil
	}
	b, err := r.Add(ctx, match[3], cents, date.Format("2006-01-02"))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("✅ Boleto #%d cadastrado\n%s\nR$ %s • vence em %s\n\nPara dar baixa: boleto pago %d", b.ID, b.Description, Money(b.AmountCents), match[1], b.ID), nil
}
func ParseMoney(raw string) (int64, error) {
	if !regexp.MustCompile(`^(?:[0-9]+|[0-9]{1,3}(?:\.[0-9]{3})+)(?:,[0-9]{1,2})?$`).MatchString(raw) {
		return 0, fmt.Errorf("valor inválido")
	}
	parts := strings.Split(strings.ReplaceAll(raw, ".", ""), ",")
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > 9999999999 {
		return 0, fmt.Errorf("valor inválido")
	}
	frac := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		frac, _ = strconv.ParseInt(fraction, 10, 64)
	}
	return whole*100 + frac, nil
}
func Money(cents int64) string {
	whole := strconv.FormatInt(cents/100, 10)
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "." + whole[i:]
	}
	return fmt.Sprintf("%s,%02d", whole, cents%100)
}
func dateLabel(raw string) string {
	date, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return raw
	}
	return date.Format("02/01/2006")
}
