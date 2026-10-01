package bitz

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

type ReservationRequest struct {
	ID                string
	CheckIn, CheckOut string
	Categories        []RoomCategory
}

type RoomCategory struct{ Key, SourceKey, Name string }

type Runner interface {
	TestAccess(context.Context, Config, string) error
	Create(context.Context, Config, string, ReservationRequest) error
}

type BrowserRunner struct{}

func NewBrowserRunner() *BrowserRunner { return &BrowserRunner{} }

func browserPath() (string, error) {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, pattern := range []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "EdgeCore", "*", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "EdgeCore", "*", "msedge.exe"),
	} {
		matches, _ := filepath.Glob(pattern)
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		candidates = append(candidates, matches...)
	}
	for _, candidate := range candidates {
		if candidate != "" {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", errors.New("Microsoft Edge ou Google Chrome não encontrado")
}

func browserContext(parent context.Context) (context.Context, context.CancelFunc, error) {
	executable, err := browserPath()
	if err != nil {
		return nil, nil, err
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(executable), chromedp.Headless,
		chromedp.Flag("disable-gpu", true), chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true), chromedp.Flag("disable-extensions", true),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, opts...)
	ctx, cancelBrowser := chromedp.NewContext(allocCtx)
	return ctx, func() { cancelBrowser(); cancelAlloc() }, nil
}

func (r *BrowserRunner) TestAccess(parent context.Context, cfg Config, password string) error {
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	ctx, closeBrowser, err := browserContext(ctx)
	if err != nil {
		return err
	}
	defer closeBrowser()
	return login(ctx, cfg, password)
}

func (r *BrowserRunner) Create(parent context.Context, cfg Config, password string, req ReservationRequest) error {
	if len(req.Categories) == 0 || len(req.Categories) > 6 {
		return errors.New("a pré-reserva exige de 1 a 6 quartos")
	}
	ctx, cancel := context.WithTimeout(parent, 4*time.Minute)
	defer cancel()
	ctx, closeBrowser, err := browserContext(ctx)
	if err != nil {
		return err
	}
	defer closeBrowser()
	if err = login(ctx, cfg, password); err != nil {
		return err
	}
	if err = openNewReservation(ctx, 30*time.Second); err != nil {
		return fmt.Errorf("abrir nova reserva: %w", err)
	}
	if err = setByID(ctx, "reserva_cpf", cfg.BotCPF); err != nil {
		return fmt.Errorf("informar CPF operacional: %w", err)
	}
	if err = clickSelector(ctx, `[title="Pesquisar por CPF"]`); err != nil {
		return fmt.Errorf("pesquisar CPF operacional: %w", err)
	}
	if err = clickByID(ctx, "btn-avancar"); err != nil {
		return err
	}
	if err = waitSelector(ctx, "#reserva_data_reserva", 15*time.Second); err != nil {
		return err
	}
	if err = setByID(ctx, "reserva_data_reserva", req.CheckIn); err != nil {
		return err
	}
	if err = setByID(ctx, "reserva_data_saida", req.CheckOut); err != nil {
		return err
	}
	_ = clickText(ctx, "Pré Reserva")
	_ = clickText(ctx, "Aberto")
	if err = clickByID(ctx, "btn-avancar"); err != nil {
		return err
	}
	if err = waitSelector(ctx, "#btn-add-quarto-reserva", 15*time.Second); err != nil {
		return err
	}
	for _, category := range req.Categories {
		if err = clickByID(ctx, "btn-add-quarto-reserva"); err != nil {
			return err
		}
		if err = selectAvailableCategory(ctx, category); err != nil {
			return fmt.Errorf("categoria %q: %w", category.Name, err)
		}
	}
	if err = clickByID(ctx, "btn-avancar"); err != nil {
		return err
	}
	if err = clickByID(ctx, "btn-avancar"); err != nil {
		return err
	}
	if err = clickByID(ctx, "btn-salvar-p"); err != nil {
		return err
	}
	select {
	case <-time.After(20 * time.Second):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func login(ctx context.Context, cfg Config, password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("senha Bitz vazia")
	}
	if err := chromedp.Run(ctx,
		chromedp.Navigate(strings.TrimRight(cfg.BaseURL, "/")+"/login"),
		chromedp.WaitVisible(`#username`, chromedp.ByID),
		chromedp.SetValue(`#username`, cfg.Username, chromedp.ByID),
		chromedp.SetValue(`#password`, password, chromedp.ByID),
		chromedp.Click(`#loginButton`, chromedp.ByID),
		chromedp.WaitNotPresent(`#loginButton`, chromedp.ByID),
	); err != nil {
		return fmt.Errorf("login Bitz recusado ou indisponível: %w", err)
	}
	return nil
}

func openNewReservation(ctx context.Context, timeout time.Duration) error {
	// The login button disappears before Bitz finishes installing its global
	// keyboard shortcuts. Wait for a complete document and retry F2 instead of
	// treating that early DOM transition as an application-ready signal.
	if err := chromedp.Run(ctx,
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Poll(`document.readyState === "complete"`, nil, chromedp.WithPollingInterval(100*time.Millisecond)),
		chromedp.Evaluate(`window.focus(); document.body && document.body.focus()`, nil),
	); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for attempt := 0; time.Now().Before(deadline); attempt++ {
		var clicked bool
		_ = chromedp.Run(ctx, chromedp.Evaluate(`(()=>{const e=document.getElementById("btn-add-reserva");if(!e||e.disabled)return false;e.click();return true})()`, &clicked))
		if clicked && hasText(ctx, "NOVA RESERVA", 1500*time.Millisecond) {
			return nil
		}
		if err := chromedp.Run(ctx, chromedp.KeyEvent(kb.F2)); err != nil {
			return err
		}
		if hasText(ctx, "NOVA RESERVA", 1500*time.Millisecond) {
			return nil
		}
		// Some Bitz builds listen to DOM keyboard events rather than the browser's
		// native input dispatch. Use that path only after native F2 did not open it.
		if attempt == 0 {
			_ = chromedp.Run(ctx, chromedp.Evaluate(`(()=>{const o={key:"F2",code:"F2",keyCode:113,which:113,bubbles:true,cancelable:true};for(const t of [document,window]){t.dispatchEvent(new KeyboardEvent("keydown",o));t.dispatchEvent(new KeyboardEvent("keyup",o))}})()`, nil))
			if hasText(ctx, "NOVA RESERVA", 1500*time.Millisecond) {
				return nil
			}
		}
	}
	var location, title string
	_ = chromedp.Run(ctx, chromedp.Location(&location), chromedp.Title(&title))
	return fmt.Errorf("tela esperada não apareceu: NOVA RESERVA (página=%q, título=%q)", location, title)
}

func hasText(ctx context.Context, value string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ok bool
		_ = chromedp.Run(ctx, chromedp.Evaluate(`document.body && document.body.innerText.includes(`+jsString(value)+`)`, &ok))
		if ok {
			return true
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return false
		}
	}
	return false
}

func waitText(text string, timeout time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error { return waitTextRun(ctx, text, timeout) })
}
func waitTextRun(ctx context.Context, value string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ok bool
		_ = chromedp.Run(ctx, chromedp.Evaluate(`document.body && document.body.innerText.includes(`+jsString(value)+`)`, &ok))
		if ok {
			return nil
		}
		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("tela esperada não apareceu: %s", value)
}
func jsString(value string) string { return fmt.Sprintf("%q", value) }
func waitSelector(ctx context.Context, selector string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ok bool
		_ = chromedp.Run(ctx, chromedp.Evaluate(`!!document.querySelector(`+jsString(selector)+`)`, &ok))
		if ok {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("controle não apareceu: %s", selector)
}
func clickSelector(ctx context.Context, selector string) error {
	var ok bool
	script := `(()=>{const e=document.querySelector(` + jsString(selector) + `);if(!e||e.disabled)return false;e.click();return true})()`
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &ok)); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("controle não encontrado: %s", selector)
	}
	time.Sleep(350 * time.Millisecond)
	return nil
}
func clickByID(ctx context.Context, id string) error { return clickSelector(ctx, "#"+id) }
func setByID(ctx context.Context, id, value string) error {
	var ok bool
	script := `(()=>{const e=document.getElementById(` + jsString(id) + `);if(!e)return false;e.focus();const s=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set;s.call(e,` + jsString(value) + `);e.dispatchEvent(new Event('input',{bubbles:true}));e.dispatchEvent(new Event('change',{bubbles:true}));e.dispatchEvent(new Event('blur',{bubbles:true}));return true})()`
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &ok)); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("campo não encontrado: #%s", id)
	}
	return nil
}
func clickText(ctx context.Context, value string) error {
	var ok bool
	script := `(()=>{const n=` + jsString(normalize(value)) + `;const els=[...document.querySelectorAll('button,a,label,span')];const e=els.find(x=>{const t=(x.innerText||x.textContent||'').normalize('NFD').replace(/[\u0300-\u036f]/g,'').trim().toLowerCase();return t===n});if(!e)return false;e.click();return true})()`
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &ok)); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("controle não encontrado: %s", value)
	}
	time.Sleep(350 * time.Millisecond)
	return nil
}
func setByLabel(ctx context.Context, label, value string) error {
	var ok bool
	script := `(()=>{const n=` + jsString(normalize(label)) + `;const ls=[...document.querySelectorAll('label')];const l=ls.find(x=>(x.innerText||'').normalize('NFD').replace(/[\u0300-\u036f]/g,'').trim().toLowerCase().includes(n));if(!l)return false;const id=l.getAttribute('for');const e=(id&&document.getElementById(id))||l.querySelector('input')||l.parentElement?.querySelector('input')||l.nextElementSibling?.querySelector?.('input');if(!e)return false;e.focus();e.value=` + jsString(value) + `;e.dispatchEvent(new Event('input',{bubbles:true}));e.dispatchEvent(new Event('change',{bubbles:true}));return true})()`
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &ok)); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("campo não encontrado: %s", label)
	}
	return nil
}
func selectAvailableCategory(ctx context.Context, category RoomCategory) error {
	if err := waitTextRun(ctx, "SELECIONAR UH DISPONÍVEL", 15*time.Second); err != nil {
		return err
	}
	var ok bool
	physical := physicalCategory(category)
	script := `(()=>{const n=` + jsString(physical) + `;const rows=[...document.querySelectorAll('#table-quartos-disponiveis-reserva tr')];const row=rows.find(r=>{const cell=r.children[2];const t=(cell?.innerText||'').normalize('NFD').replace(/[\u0300-\u036f]/g,'').trim().toLowerCase();return t===n});if(!row)return false;const pick=row.querySelector('input[type=checkbox]')||row.querySelector('td:first-child i')||row.children[0];if(!pick)return false;pick.click();const save=document.getElementById('btn-salvar-p');if(!save||save.disabled)return false;save.click();return true})()`
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &ok)); err != nil {
		return err
	}
	if !ok {
		return errors.New("nenhuma UH disponível encontrada")
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}
func physicalCategory(category RoomCategory) string {
	bySource := map[string]string{
		"superluxo":        "quarto duplo superluxo com varanda e vista mar",
		"familia":          "quarto familia deluxe com vista mar",
		"quadruploVista":   "quarto quadruplo deluxe com varanda e vista mar",
		"triploDeluxe":     "quarto triplo deluxe com varanda",
		"quadruploDeluxe":  "quarto quadruplo deluxe com varanda",
		"triploVaranda":    "quarto triplo com varanda",
		"quadruploVaranda": "quarto quadruplo com varanda",
		"duplo":            "quarto duplo",
		"triplo":           "quarto triplo",
	}
	if value := bySource[category.SourceKey]; value != "" {
		return value
	}
	if value := bySource[category.Key]; value != "" {
		return value
	}
	return normalize(strings.TrimPrefix(category.Name, "Suíte "))
}
func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c").Replace(value)))
}
