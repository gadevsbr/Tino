package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gadevsbr/tino/internal/capability"
	"github.com/gadevsbr/tino/internal/chat"
	"github.com/gadevsbr/tino/internal/config"
	"github.com/gadevsbr/tino/internal/flow"
	"github.com/gadevsbr/tino/internal/session"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfg, err := config.Load(filepath.Join("config", "config.yaml"))
	if err != nil {
		fatal(err)
	}
	ctx := context.Background()
	mgr, err := session.Open(ctx, cfg.DataDir, cfg.Profile, false)
	if err != nil {
		fatal(err)
	}
	chatStore, err := chat.Open(filepath.Join(cfg.DataDir, "chats-"+cfg.Profile+".db"))
	if err != nil {
		_ = mgr.Close()
		fatal(err)
	}
	flowDef, err := flow.LoadDefinition(cfg.Flow.RulesFile)
	if err != nil {
		flowDef = flow.Definition{DefaultReply: "Obrigado pela mensagem. Em breve continuaremos o atendimento."}
	}
	app := NewApp(cfg, mgr, chatStore, capability.NewStore(filepath.Join(cfg.DataDir, "capabilities.json")), flowDef, version)
	err = wails.Run(&options.App{
		Title:            "Tino • Central de Comunicação",
		Width:            1366,
		Height:           820,
		MinWidth:         1080,
		MinHeight:        700,
		Frameless:        false,
		DisableResize:    false,
		BackgroundColour: &options.RGBA{R: 247, G: 248, B: 250, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                windows.SystemDefault,
		},
	})
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, "Tino:", err)
	os.Exit(1)
}
