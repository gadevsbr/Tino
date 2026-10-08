package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gadevsbr/tino/internal/audit"
	"github.com/gadevsbr/tino/internal/batch"
	"github.com/gadevsbr/tino/internal/capability"
	"github.com/gadevsbr/tino/internal/config"
	"github.com/gadevsbr/tino/internal/flow"
	"github.com/gadevsbr/tino/internal/session"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func run() error {
	global := flag.NewFlagSet("tino", flag.ContinueOnError)
	cfgPath := global.String("config", "config/config.yaml", "arquivo de configuração")
	debug := global.Bool("debug", false, "logs detalhados")
	if err := global.Parse(os.Args[1:]); err != nil {
		return err
	}
	args := global.Args()
	if len(args) == 0 {
		usage()
		return errors.New("subcomando obrigatório")
	}
	if args[0] == "version" {
		fmt.Println(version)
		return nil
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mgr, err := session.Open(ctx, cfg.DataDir, cfg.Profile, *debug)
	if err != nil {
		return err
	}
	defer mgr.Close()
	switch args[0] {
	case "login":
		if err := mgr.Connect(ctx, true); err != nil {
			return err
		}
		fmt.Println("sessão autenticada e persistida")
	case "status":
		auth := mgr.Client.Store.ID != nil
		fmt.Printf("autenticada=%t conectada=%t perfil=%s\n", auth, mgr.Client.IsConnected(), cfg.Profile)
	case "export":
		if err := mgr.Connect(ctx, false); err != nil {
			return err
		}
		snap, err := audit.Collect(ctx, mgr.Client)
		if err != nil {
			return err
		}
		dir := filepath.Join("exports", cfg.Profile)
		if err := audit.Write(snap, dir); err != nil {
			return err
		}
		fmt.Printf("exportados %d contatos e %d grupos em %s\n", len(snap.Contacts), len(snap.Groups), dir)
	case "send":
		fs := flag.NewFlagSet("send", flag.ContinueOnError)
		input := fs.String("csv", "", "CSV com phone,name,message,consent")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *input == "" {
			return errors.New("use send -csv arquivo.csv")
		}
		if err := mgr.Connect(ctx, false); err != nil {
			return err
		}
		items, err := batch.LoadCSV(*input)
		if err != nil {
			return err
		}
		p := batch.Processor{Client: mgr.Client, MinInterval: cfg.Batch.MinInterval, MaxInterval: cfg.Batch.MaxInterval, MaxPerRun: cfg.Batch.MaxPerRun}
		return p.Run(ctx, items, func(r batch.Result) { b, _ := json.Marshal(r); fmt.Println(string(b)) })
	case "serve":
		if err := mgr.Connect(ctx, false); err != nil {
			return err
		}
		// Get AI configuration from capability store
		capStore := capability.NewStore(filepath.Join(cfg.DataDir, "capabilities.json"))
		aiService, err := flow.ConfiguredAI(capStore)
		if err != nil {
			return err
		}
		engine, err := flow.Load(cfg.Flow.RulesFile, mgr.Client, aiService)
		if err != nil {
			return err
		}
		mgr.Client.AddEventHandler(engine.Handle)
		fmt.Println("roteador ativo; Ctrl+C para encerrar")
		<-ctx.Done()
	default:
		usage()
		return fmt.Errorf("subcomando desconhecido: %s", args[0])
	}
	return nil
}

func usage() {
	fmt.Println("Tino", version, "\nuso: tino [-config arquivo] <login|status|export|send|serve|version>")
}
