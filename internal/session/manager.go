package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"
)

type Manager struct {
	Client *whatsmeow.Client
	store  *sqlstore.Container
	logger waLog.Logger
	once   sync.Once
}

func Open(ctx context.Context, dataDir, profile string, debug bool) (*Manager, error) {
	if strings.ContainsAny(profile, `\\/:*?"<>|`) || profile == "." || profile == ".." {
		return nil, errors.New("nome de perfil inválido")
	}
	dir := filepath.Join(dataDir, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("criar diretório de sessão: %w", err)
	}
	dbPath := filepath.Join(dir, profile+".db")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abrir SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	container := sqlstore.NewWithDB(db, "sqlite3", waLog.Noop)
	if err := container.Upgrade(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrar armazenamento whatsmeow: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		_ = container.Close()
		return nil, fmt.Errorf("carregar dispositivo: %w", err)
	}
	level := "INFO"
	if debug {
		level = "DEBUG"
	}
	logger := waLog.Stdout("WhatsApp", level, true)
	client := whatsmeow.NewClient(device, logger)
	client.EnableAutoReconnect = true
	return &Manager{Client: client, store: container, logger: logger}, nil
}

func (m *Manager) Connect(ctx context.Context, showQR bool) error {
	return m.ConnectWithQR(ctx, showQR, func(content string) error {
		printQR(content)
		return nil
	})
}

// ConnectWithQR connects a persisted session or pairs a new one. onQR is called
// whenever WhatsApp rotates the pairing code, allowing CLI and GUI frontends to
// render it without duplicating session logic.
func (m *Manager) ConnectWithQR(ctx context.Context, showQR bool, onQR func(string) error) error {
	if m.Client.Store.ID != nil {
		if m.Client.IsLoggedIn() {
			return nil
		}
		if !m.Client.IsConnected() {
			if err := m.Client.Connect(); err != nil {
				return fmt.Errorf("conectar sessão persistida: %w", err)
			}
		}
		if !m.Client.WaitForConnection(20 * time.Second) {
			m.Client.Disconnect()
			return errors.New("a sessão salva não foi autenticada; o vínculo pode ter sido removido no celular. Use 'Gerar novo QR Code' para conectar novamente")
		}
		return nil
	}
	if !showQR {
		return errors.New("sessão não autenticada; execute login")
	}
	qrChan, err := m.Client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("iniciar pareamento QR: %w", err)
	}
	if err := m.Client.Connect(); err != nil {
		return fmt.Errorf("conectar para pareamento: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-qrChan:
			if !ok {
				return errors.New("canal QR encerrado antes da autenticação")
			}
			switch item.Event {
			case "code":
				if onQR != nil {
					if err := onQR(item.Code); err != nil {
						return fmt.Errorf("renderizar QR: %w", err)
					}
				}
			case "success":
				if !m.Client.WaitForConnection(60 * time.Second) {
					// WhatsApp persists the device before the initial history sync is
					// necessarily complete. Keep the live connection running: the
					// Connected event will update the UI as soon as sync finishes.
					if m.Client.Store.ID != nil {
						return nil
					}
					return errors.New("o QR foi aceito, mas a credencial não foi persistida")
				}
				return nil
			case "timeout":
				return errors.New("QR expirou; execute login novamente")
			default:
				if item.Error != nil {
					return fmt.Errorf("pareamento (%s): %w", item.Event, item.Error)
				}
			}
		}
	}
}

// ResetLocalSession removes only the local WhatsApp credentials and prepares a
// fresh client for a new QR pairing. Application data such as chat history is
// intentionally preserved.
func (m *Manager) ResetLocalSession(ctx context.Context) error {
	m.Client.Disconnect()
	if m.Client.Store.ID != nil {
		if err := m.Client.Store.Delete(ctx); err != nil {
			return fmt.Errorf("remover sessão local: %w", err)
		}
	}
	client := whatsmeow.NewClient(m.store.NewDevice(), m.logger)
	client.EnableAutoReconnect = true
	m.Client = client
	return nil
}

func printQR(content string) {
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		fmt.Println("QR recebido, mas não foi possível renderizá-lo:", err)
		return
	}
	fmt.Println("Escaneie em WhatsApp > Dispositivos conectados:")
	bitmap := code.Bitmap()
	for y := 0; y < len(bitmap); y += 2 {
		var line strings.Builder
		for x := range bitmap[y] {
			top := bitmap[y][x]
			bottom := y+1 < len(bitmap) && bitmap[y+1][x]
			switch {
			case top && bottom:
				line.WriteRune('█')
			case top:
				line.WriteRune('▀')
			case bottom:
				line.WriteRune('▄')
			default:
				line.WriteRune(' ')
			}
		}
		fmt.Println(line.String())
	}
}

func (m *Manager) Close() error {
	var err error
	m.once.Do(func() { m.Client.Disconnect(); err = m.store.Close() })
	return err
}
