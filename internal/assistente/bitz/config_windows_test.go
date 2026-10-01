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

func TestPhysicalCategoryUsesOmniBeesSourceSlot(t *testing.T) {
	tests := []struct{ source, want string }{
		{"familia", "quarto familia deluxe com vista mar"},
		{"triploDeluxe", "quarto triplo deluxe com varanda"},
		{"quadruploVista", "quarto quadruplo deluxe com varanda e vista mar"},
		{"duplo", "quarto duplo"},
	}
	for _, test := range tests {
		if got := physicalCategory(RoomCategory{SourceKey: test.source}); got != test.want {
			t.Fatalf("source %s: got %q want %q", test.source, got, test.want)
		}
	}
}

func TestKnownReservationSelectorsAndRoomTable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1>SELECIONAR UH DISPONÍVEL</h1>
		<input id="reserva_cpf"><button title="Pesquisar por CPF"></button><button id="btn-avancar"></button>
		<input id="reserva_data_reserva"><input id="reserva_data_saida"><button id="btn-add-quarto-reserva"></button>
		<table><tbody id="table-quartos-disponiveis-reserva"><tr><td><i class="fa fa-square-o" onclick="this.dataset.picked='yes'"></i></td><td>203</td><td>QUARTO TRIPLO DELUXE COM VARANDA</td></tr></tbody></table>
		<button id="btn-salvar-p" onclick="this.dataset.saved='yes'"></button>
		</body></html>`))
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
	if err = setByID(ctx, "reserva_cpf", "52998224725"); err != nil {
		t.Fatal(err)
	}
	if err = setByID(ctx, "reserva_data_reserva", "01/10/2026"); err != nil {
		t.Fatal(err)
	}
	if err = setByID(ctx, "reserva_data_saida", "05/10/2026"); err != nil {
		t.Fatal(err)
	}
	if err = clickSelector(ctx, `[title="Pesquisar por CPF"]`); err != nil {
		t.Fatal(err)
	}
	if err = selectAvailableCategory(ctx, RoomCategory{SourceKey: "triploDeluxe", Name: "Suíte Deluxe com varanda"}); err != nil {
		t.Fatal(err)
	}
	var result string
	if err = chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('td:first-child i').dataset.picked+'|'+document.getElementById('btn-salvar-p').dataset.saved`, &result)); err != nil {
		t.Fatal(err)
	}
	if result != "yes|yes" {
		t.Fatalf("selection result: %q", result)
	}
}
