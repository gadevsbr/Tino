package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"

	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// Repository uses the operational database, never the WhatsApp credential store.
type Repository struct {
	db *sql.DB
	mu sync.Mutex
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }
func (r *Repository) EnsureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS catalog_entries (
 account TEXT NOT NULL, owner TEXT NOT NULL, category TEXT NOT NULL,
 payload BLOB NOT NULL, image BLOB NOT NULL,
 PRIMARY KEY(account,category));
 CREATE TABLE IF NOT EXISTS catalog_sessions (
 account TEXT NOT NULL, operator TEXT NOT NULL, chat TEXT NOT NULL, state BLOB NOT NULL,
 PRIMARY KEY(account,operator,chat));`)
	return err
}

type Entry struct {
	AccountJID string
	OwnerJID   string
	CategoryID string
	Payload    *waE2E.ProductMessage
	Image      []byte
}

// NormalizeAccount removes device suffixes; transports must resolve LIDs to their
// stable connected account phone JID before calling this package.
func NormalizeAccount(value string) string {
	jid, err := types.ParseJID(strings.TrimSpace(value))
	if err != nil || jid.User == "" || jid.Server != types.DefaultUserServer {
		return ""
	}
	return jid.ToNonAD().String()
}

func (r *Repository) Get(ctx context.Context, account, category string) (Entry, error) {
	account = NormalizeAccount(account)
	if account == "" {
		return Entry{}, errors.New("conta inválida")
	}
	e := Entry{AccountJID: account, CategoryID: category}
	var data []byte
	err := r.db.QueryRowContext(ctx, `SELECT owner,payload,image FROM catalog_entries WHERE account=? AND category=?`, account, category).Scan(&e.OwnerJID, &data, &e.Image)
	if err != nil {
		return Entry{}, err
	}
	e.Payload = &waE2E.ProductMessage{}
	err = proto.Unmarshal(data, e.Payload)
	return e, err
}
func (r *Repository) List(ctx context.Context, account string) ([]Entry, error) {
	account = NormalizeAccount(account)
	if account == "" {
		return nil, errors.New("conta inválida")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT owner,category,payload,image FROM catalog_entries WHERE account=? ORDER BY owner,category`, account)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []Entry{}
	for rows.Next() {
		e := Entry{AccountJID: account}
		var data []byte
		if err = rows.Scan(&e.OwnerJID, &e.CategoryID, &data, &e.Image); err != nil {
			return nil, err
		}
		e.Payload = &waE2E.ProductMessage{}
		if err = proto.Unmarshal(data, e.Payload); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Sanitize reconstructs an allowlisted snapshot. Media, URLs, context, catalog
// thumbnails and unknown protobuf fields are intentionally never persisted.
// The transport must upload Entry.Image freshly and set Product.ProductImage.
func Sanitize(m *waE2E.ProductMessage) (*waE2E.ProductMessage, error) {
	if m == nil || m.GetProduct() == nil || strings.TrimSpace(m.GetProduct().GetProductID()) == "" || NormalizeAccount(m.GetBusinessOwnerJID()) == "" {
		return nil, errors.New("produto sem ID ou proprietário válido")
	}
	p := m.GetProduct()
	return &waE2E.ProductMessage{BusinessOwnerJID: proto.String(NormalizeAccount(m.GetBusinessOwnerJID())), Product: &waE2E.ProductMessage_ProductSnapshot{
		ProductID: copyOptional(p.ProductID), Title: copyOptional(p.Title), Description: copyOptional(p.Description), CurrencyCode: copyOptional(p.CurrencyCode), PriceAmount1000: copyOptional(p.PriceAmount1000), SalePriceAmount1000: copyOptional(p.SalePriceAmount1000), RetailerID: copyOptional(p.RetailerID), ProductImageCount: copyOptional(p.ProductImageCount),
	}}, nil
}

func copyOptional[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
