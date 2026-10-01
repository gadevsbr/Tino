//go:build windows

package bitz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestStoreProtectsPasswordAndNeverReturnsItPublicly(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "bitz.json"))
	ctx := context.Background()
	err := store.Save(ctx, PublicConfig{Enabled: true, BaseURL: DefaultURL, Username: "bot", BotCPF: "52998224725", ApproverPhone: "5573999999999", ApprovedText: "confirmada"}, "segredo")
	if err != nil {
		t.Fatal(err)
	}
	public, err := store.Public(ctx)
	if err != nil || !public.PasswordSet {
		t.Fatal(public, err)
	}
	_, password, err := store.Credentials(ctx)
	if err != nil || password != "segredo" {
		t.Fatal(err, password)
	}
}

func TestBrowserDiscoveryIncludesInstalledEdgeCore(t *testing.T) {
	path, err := browserPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".exe" {
		t.Fatalf("unexpected browser path: %s", path)
	}
}

func TestBrowserRunnerLoginDoesNotOpenReservation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><input id="username"><input id="password"><button id="loginButton" onclick="this.remove()">Entrar</button></body></html>`))
	}))
	defer server.Close()
	if err := NewBrowserRunner().TestAccess(context.Background(), Config{BaseURL: server.URL, Username: "bot"}, "secret"); err != nil {
		t.Fatal(err)
	}
}

func TestOpenNewReservationWaitsForShortcutAndRetriesF2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body tabindex="-1"><script>
		setTimeout(() => document.addEventListener('keydown', e => {
			if (e.key === 'F2') document.body.insertAdjacentHTML('beforeend', '<div>NOVA RESERVA</div>')
		}), 700)
		</script></body></html>`))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx, closeBrowser, err := browserContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBrowser()
	if err = chromedp.Run(ctx, chromedp.Navigate(server.URL)); err != nil {
		t.Fatal(err)
	}
	if err = openNewReservation(ctx, 5*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestOpenNewReservationPrefersKnownButtonID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><button id="btn-add-reserva" onclick="document.body.insertAdjacentHTML('beforeend','<div>NOVA RESERVA</div>')">Nova reserva</button></body></html>`))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx, closeBrowser, err := browserContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBrowser()
	if err = chromedp.Run(ctx, chromedp.Navigate(server.URL)); err != nil {
		t.Fatal(err)
	}
	if err = openNewReservation(ctx, 5*time.Second); err != nil {
		t.Fatal(err)
	}
}
