//go:build windows

package bitz

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	if err := (&BrowserRunner{}).TestAccess(context.Background(), Config{BaseURL: server.URL, Username: "bot"}, "secret"); err != nil {
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
		<table><tbody id="table-quartos-disponiveis-reserva">
		<tr><td><i class="fa fa-square-o" onclick="this.dataset.picked='yes'"></i></td><td>202</td><td>QUARTO TRIPLO DELUXE COM VARANDA</td></tr>
		<tr><td><i class="fa fa-square-o" onclick="this.dataset.picked='yes'"></i></td><td>203</td><td>QUARTO TRIPLO DELUXE COM VARANDA</td></tr>
		</tbody></table>
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
	if err = selectAvailableCategory(ctx, RoomCategory{SourceKey: "triploDeluxe", Name: "Suíte Deluxe com varanda", AllowedRooms: []int{203}}); err != nil {
		t.Fatal(err)
	}
	var result string
	if err = chromedp.Run(ctx, chromedp.Evaluate(`([...document.querySelectorAll('tr')].find(r=>r.children[1]?.innerText==='203').querySelector('i').dataset.picked||'')+'|'+(document.querySelector('tr').querySelector('i').dataset.picked||'')+'|'+document.getElementById('btn-salvar-p').dataset.saved`, &result)); err != nil {
		t.Fatal(err)
	}
	if result != "yes||yes" {
		t.Fatalf("selection result: %q", result)
	}
}

func TestBrowserRunnerCompletesReservationWizard(t *testing.T) {
	saved := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/saved" {
			body, _ := io.ReadAll(r.Body)
			saved <- string(body)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`<!doctype html><html><body>
		<input id="username"><input id="password"><button id="loginButton" onclick="home()">Entrar</button>
		<script>
		let data={};
		function home(){document.body.innerHTML='<button id="btn-add-reserva" onclick="identity()">Nova reserva</button>'}
		function identity(){document.body.innerHTML='<h1>NOVA RESERVA</h1><input id="reserva_cpf"><button title="Pesquisar por CPF" onclick="data.cpf=document.getElementById(\'reserva_cpf\').value">Pesquisar</button><button id="btn-avancar" onclick="dates()">Avançar</button>'}
		function dates(){document.body.innerHTML='<input id="reserva_data_reserva"><input id="reserva_data_saida"><button>Pré Reserva</button><button>Aberto</button><button id="btn-avancar" onclick="data.checkin=document.getElementById(\'reserva_data_reserva\').value;data.checkout=document.getElementById(\'reserva_data_saida\').value;rooms()">Avançar</button>'}
		function rooms(){document.body.innerHTML='<button id="btn-add-quarto-reserva" onclick="picker()">Adicionar UH</button><button id="btn-avancar" onclick="review()">Avançar</button>'}
		function picker(){document.body.innerHTML='<h1>SELECIONAR UH DISPONÍVEL</h1><table><tbody id="table-quartos-disponiveis-reserva"><tr><td><i onclick="data.room=\'217\'"></i></td><td>217</td><td>QUARTO DUPLO SUPERLUXO COM VARANDA E VISTA MAR</td></tr></tbody></table><button id="btn-salvar-p" onclick="rooms()">Salvar</button>'}
		function review(){document.body.innerHTML='<button id="btn-avancar" onclick="finish()">Avançar</button>'}
		function finish(){document.body.innerHTML='<button id="btn-salvar-p" onclick="fetch(\'/saved\',{method:\'POST\',body:JSON.stringify(data)})">Salvar</button>'}
		</script></body></html>`))
	}))
	defer server.Close()
	runner := &BrowserRunner{completionWait: 250 * time.Millisecond}
	err := runner.Create(context.Background(), Config{BaseURL: server.URL, Username: "bot", BotCPF: "52998224725"}, "secret", ReservationRequest{
		ID: "test", CheckIn: "01/10/2026", CheckOut: "05/10/2026",
		Categories: []RoomCategory{{Key: "superluxo", SourceKey: "superluxo", Name: "Suíte Deluxe com vista para o mar"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case body := <-saved:
		for _, want := range []string{`"cpf":"52998224725"`, `"checkin":"01/10/2026"`, `"checkout":"05/10/2026"`, `"room":"217"`} {
			if !strings.Contains(body, want) {
				t.Fatalf("payload %s missing %s", body, want)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("final save was not reached")
	}
}

func TestLiveBitzCreateReservation(t *testing.T) {
	if os.Getenv("TINO_BITZ_LIVE_CREATE") != "CONFIRMAR_PRE_RESERVA_REAL" {
		t.Skip("set TINO_BITZ_LIVE_CREATE=CONFIRMAR_PRE_RESERVA_REAL to create a real pre-reservation")
	}
	checkIn, checkOut := os.Getenv("TINO_BITZ_CHECKIN"), os.Getenv("TINO_BITZ_CHECKOUT")
	if checkIn == "" || checkOut == "" {
		t.Fatal("TINO_BITZ_CHECKIN and TINO_BITZ_CHECKOUT are required")
	}
	store := NewStore(filepath.Join("..", "..", "..", "data", "operations", "bitz-config.json"))
	cfg, password, err := store.Credentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sources := strings.Split(strings.TrimSpace(os.Getenv("TINO_BITZ_SOURCES")), ",")
	if len(sources) == 1 && strings.TrimSpace(sources[0]) == "" {
		sources = []string{"superluxo"}
	}
	categories := make([]RoomCategory, 0, len(sources))
	for _, source := range sources {
		source = strings.TrimSpace(source)
		category := RoomCategory{Key: source, SourceKey: source, Name: source}
		if source == "" || physicalCategory(category) == normalize(source) {
			t.Fatalf("unsupported TINO_BITZ_SOURCES entry: %q", source)
		}
		categories = append(categories, category)
	}
	result, err := NewBrowserRunner().run(context.Background(), "create", cfg, password, ReservationRequest{
		ID: "live-test", CheckIn: checkIn, CheckOut: checkOut,
		Categories: categories,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pre-reservation confirmed by Bitz: reference=%s rooms=%v", result.Reference, result.Rooms)
}

func TestLiveBitzProbe(t *testing.T) {
	if os.Getenv("TINO_BITZ_LIVE_PROBE") != "1" {
		t.Skip("set TINO_BITZ_LIVE_PROBE=1 to verify the real wizard without saving")
	}
	checkIn, checkOut := os.Getenv("TINO_BITZ_CHECKIN"), os.Getenv("TINO_BITZ_CHECKOUT")
	if checkIn == "" || checkOut == "" {
		t.Fatal("TINO_BITZ_CHECKIN and TINO_BITZ_CHECKOUT are required")
	}
	store := NewStore(filepath.Join("..", "..", "..", "data", "operations", "bitz-config.json"))
	cfg, password, err := store.Credentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sources := strings.Split(strings.TrimSpace(os.Getenv("TINO_BITZ_SOURCES")), ",")
	if len(sources) == 1 && strings.TrimSpace(sources[0]) == "" {
		sources = []string{"triploDeluxe"}
	}
	categories := make([]RoomCategory, 0, len(sources))
	for _, source := range sources {
		source = strings.TrimSpace(source)
		category := RoomCategory{Key: source, SourceKey: source, Name: source}
		if source == "" || physicalCategory(category) == normalize(source) {
			t.Fatalf("unsupported source: %q", source)
		}
		categories = append(categories, category)
	}
	result, err := NewBrowserRunner().Probe(context.Background(), cfg, password, ReservationRequest{
		ID: "live-probe", CheckIn: checkIn, CheckOut: checkOut, Categories: categories,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != "verified_without_save" || len(result.Rooms) != len(categories) {
		t.Fatalf("unexpected result: %+v", result)
	}
}
