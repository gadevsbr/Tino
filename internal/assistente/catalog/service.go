package catalog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strconv"
	"strings"

	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

type Category struct{ ID, Name string }
type Input struct {
	AccountJID, OperatorJID, ChatJID, Text string
	Authorized                             bool
	Product                                *waE2E.ProductMessage
	// Image contains downloaded photo bytes, not a URL or encrypted media.
	Image []byte
}
type Result struct {
	Handled bool
	Text    string
}
type Service struct {
	repo       *Repository
	categories []Category
}

func NewService(repo *Repository, categories []Category) *Service {
	return &Service{repo: repo, categories: append([]Category(nil), categories...)}
}

type stage struct {
	Step, Category string
	Payload, Image []byte
	Previous       []byte
	Categories     []Category
}

const MaxImageBytes = 5 * 1024 * 1024

// AcceptsProduct is a preflight only; callers must authorize the operator first.
// Handle rechecks the session after download, preventing cancellation races.
func (s *Service) AcceptsProduct(ctx context.Context, account, operator, chat string) (bool, error) {
	account = NormalizeAccount(account)
	if account == "" || operator == "" || chat == "" {
		return false, nil
	}
	var raw []byte
	err := s.repo.db.QueryRowContext(ctx, `SELECT state FROM catalog_sessions WHERE account=? AND operator=? AND chat=?`, account, operator, chat).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var st stage
	err = json.Unmarshal(raw, &st)
	return err == nil && st.Step == "product", err
}

func ValidateImage(data []byte) error {
	if len(data) == 0 || len(data) > MaxImageBytes {
		return errors.New("foto ausente ou maior que 5 MB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 25_000_000 {
		return errors.New("foto deve ser JPEG/PNG válido de até 25 megapixels")
	}
	_, _, err = image.Decode(bytes.NewReader(data))
	if err != nil {
		return errors.New("foto JPEG/PNG incompleta ou inválida")
	}
	return nil
}

// Handle requires the transport's authorization decision on every input, and
// serializes state changes. Confirmation and saving/deleting are one transaction.
func (s *Service) Handle(ctx context.Context, in Input) (Result, error) {
	if !in.Authorized {
		return Result{}, nil
	}
	in.AccountJID = NormalizeAccount(in.AccountJID)
	if in.AccountJID == "" || strings.TrimSpace(in.OperatorJID) == "" || strings.TrimSpace(in.ChatJID) == "" {
		return Result{}, errors.New("conta, operador e conversa são obrigatórios")
	}
	s.repo.mu.Lock()
	defer s.repo.mu.Unlock()
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	st := stage{}
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT state FROM catalog_sessions WHERE account=? AND operator=? AND chat=?`, in.AccountJID, in.OperatorJID, in.ChatJID).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	err = nil
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &st); err != nil {
			return Result{}, err
		}
	}
	text := strings.Join(strings.Fields(strings.ToLower(in.Text)), " ")
	text = strings.ReplaceAll(text, "catálogo", "catalogo")
	reply := ""
	handled := true
	remove := false
	save := false
	switch {
	case text == "cancelar" || text == "cancelar catalogo":
		if st.Step == "" {
			return Result{}, nil
		}
		remove = true
		reply = "Configuração do catálogo cancelada."
	case text == "configurar catalogo":
		if len(s.categories) == 0 {
			return Result{true, "Categorias do orçamento indisponíveis."}, nil
		}
		ids := map[string]bool{}
		for _, c := range s.categories {
			if c.ID == "" || c.Name == "" || ids[c.ID] {
				return Result{}, errors.New("categorias canônicas inválidas")
			}
			ids[c.ID] = true
		}
		st = stage{Step: "product", Categories: append([]Category(nil), s.categories...)}
		save = true
		reply = "Envie um produto do catálogo Business com foto. Use cancelar para sair."
	case text == "limpar catalogo":
		st = stage{Step: "clear"}
		save = true
		reply = "Isso removerá todos os produtos configurados somente nesta conta conectada. Digite confirmar ou cancelar."
	case text == "status catalogo" || text == "catalogo status" || text == "listar catalogo" || text == "catalogo":
		rows, e := tx.QueryContext(ctx, `SELECT owner,category,payload FROM catalog_entries WHERE account=? ORDER BY owner,category`, in.AccountJID)
		if e != nil {
			return Result{}, e
		}
		var b strings.Builder
		b.WriteString("Catálogo da conta " + in.AccountJID + ":\n")
		count := 0
		for rows.Next() {
			var owner, category string
			var data []byte
			if e = rows.Scan(&owner, &category, &data); e != nil {
				rows.Close()
				return Result{}, e
			}
			p := &waE2E.ProductMessage{}
			if e = proto.Unmarshal(data, p); e != nil {
				rows.Close()
				return Result{}, e
			}
			fmt.Fprintf(&b, "%s: %s (ProductID: %s; proprietário: %s)\n", category, p.GetProduct().GetTitle(), p.GetProduct().GetProductID(), owner)
			count++
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return Result{}, e
		}
		if count == 0 {
			b.WriteString("Nenhum produto configurado.")
		}
		reply = b.String()
	case st.Step == "":
		handled = false
	case st.Step == "clear":
		if text != "confirmar" {
			reply = "Digite confirmar para limpar o catálogo desta conta ou cancelar."
			break
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM catalog_entries WHERE account=?`, in.AccountJID); err != nil {
			return Result{}, err
		}
		// Invalidate all pending captures for this account so another operator cannot
		// restore cleared products by confirming an old staged product.
		if _, err = tx.ExecContext(ctx, `DELETE FROM catalog_sessions WHERE account=?`, in.AccountJID); err != nil {
			return Result{}, err
		}
		remove = true
		reply = "Catálogo desta conta removido."
	case st.Step == "product":
		if in.Product == nil {
			reply = "Envie o produto do catálogo Business com foto ou cancelar."
			break
		}
		p, e := Sanitize(in.Product)
		if e != nil {
			return Result{true, e.Error()}, nil
		}
		if e = ValidateImage(in.Image); e != nil {
			return Result{true, e.Error() + ". Reenvie o produto com foto."}, nil
		}
		st.Payload, e = proto.Marshal(p)
		if e != nil {
			return Result{}, e
		}
		st.Image = append([]byte(nil), in.Image...)
		st.Step = "category"
		save = true
		reply = "Produto recebido: " + p.GetProduct().GetTitle() + ".\nEscolha a categoria:\n" + categoryMenu(st.Categories)
	case st.Step == "category":
		n, e := strconv.Atoi(text)
		if e != nil || n < 1 || n > len(st.Categories) {
			reply = "Escolha o número da categoria:\n" + categoryMenu(st.Categories)
			break
		}
		st.Category = st.Categories[n-1].ID
		st.Step = "confirm"
		save = true
		p := &waE2E.ProductMessage{}
		if err = proto.Unmarshal(st.Payload, p); err != nil {
			return Result{}, err
		}
		reply = fmt.Sprintf("Relacionar %s (ProductID: %s; proprietário: %s) à categoria %s nesta conta %s? Digite confirmar ou cancelar.", p.GetProduct().GetTitle(), p.GetProduct().GetProductID(), p.GetBusinessOwnerJID(), st.Categories[n-1].Name, in.AccountJID)
		err = tx.QueryRowContext(ctx, `SELECT payload FROM catalog_entries WHERE account=? AND category=?`, in.AccountJID, st.Category).Scan(&st.Previous)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Result{}, err
		}
		err = nil
		if len(st.Previous) > 0 {
			reply += "\nJá existe um produto nesta categoria. Confirmar substituirá o vínculo anterior."
		}
		if p.GetBusinessOwnerJID() != in.AccountJID {
			st.Step = "owner"
			reply += "\nO proprietário é diferente da conta conectada. Para autorizar explicitamente, digite confirmar proprietario."
		}
	case st.Step == "owner":
		if text != "confirmar proprietario" && text != "confirmar proprietário" {
			reply = "O proprietário é diferente da conta conectada. Digite confirmar proprietario ou cancelar."
			break
		}
		st.Step = "confirm"
		save = true
		reply = "Proprietário diferente autorizado. Digite confirmar para salvar o vínculo ou cancelar."
	case st.Step == "confirm":
		if text != "confirmar" {
			reply = "Digite confirmar para salvar ou cancelar."
			break
		}
		p := &waE2E.ProductMessage{}
		if err = proto.Unmarshal(st.Payload, p); err != nil {
			return Result{}, err
		}
		var current []byte
		err = tx.QueryRowContext(ctx, `SELECT payload FROM catalog_entries WHERE account=? AND category=?`, in.AccountJID, st.Category).Scan(&current)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Result{}, err
		}
		err = nil
		if !bytes.Equal(current, st.Previous) {
			st.Previous = current
			save = true
			reply = "O vínculo desta categoria mudou durante a configuração. Digite confirmar novamente para substituir pelo produto escolhido ou cancelar."
			break
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO catalog_entries(account,owner,category,payload,image) VALUES(?,?,?,?,?) ON CONFLICT(account,category) DO UPDATE SET owner=excluded.owner,payload=excluded.payload,image=excluded.image`, in.AccountJID, p.GetBusinessOwnerJID(), st.Category, st.Payload, st.Image)
		if err != nil {
			return Result{}, err
		}
		remove = true
		reply = "Produto salvo no catálogo desta conta. Use configurar catalogo para adicionar outro."
	default:
		return Result{}, errors.New("etapa de catálogo desconhecida")
	}
	if remove {
		_, err = tx.ExecContext(ctx, `DELETE FROM catalog_sessions WHERE account=? AND operator=? AND chat=?`, in.AccountJID, in.OperatorJID, in.ChatJID)
	} else if save {
		raw, err = json.Marshal(st)
		if err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO catalog_sessions(account,operator,chat,state) VALUES(?,?,?,?) ON CONFLICT(account,operator,chat) DO UPDATE SET state=excluded.state`, in.AccountJID, in.OperatorJID, in.ChatJID, raw)
		}
	}
	if err != nil {
		return Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{handled, reply}, nil
}
func categoryMenu(categories []Category) string {
	var b strings.Builder
	for i, c := range categories {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c.Name)
	}
	return b.String()
}
