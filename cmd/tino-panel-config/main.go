package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gadevsbr/tino/internal/controlpanel"
)

func main() {
	path := flag.String("config", "data/control-panel.json", "arquivo de configuração do painel")
	copyFrom := flag.String("copy-from", "", "copia uma configuração protegida existente para esta instalação")
	endpoint := flag.String("api", "", "endereço HTTPS da API")
	dashboard := flag.String("dashboard", "", "endereço HTTPS do painel")
	instance := flag.String("instance", "", "identificador desta instalação")
	smoke := flag.Bool("smoke", false, "testa a ponte já configurada sem alterar atendimentos")
	flag.Parse()
	store := controlpanel.NewStore(*path)
	if *copyFrom != "" {
		source := controlpanel.NewStore(*copyFrom)
		cfg, token, err := source.Load()
		if err != nil {
			panic(err)
		}
		dashboardToken, _ := source.DashboardToken()
		if err := store.Save(cfg, token, dashboardToken); err != nil {
			panic(err)
		}
		fmt.Println("configuração copiada; token protegido pelo Windows")
		return
	}
	if *smoke {
		cfg, token, err := store.Load()
		if err != nil {
			panic(err)
		}
		_, err = (controlpanel.Client{Endpoint: cfg.Endpoint, Token: token, InstanceID: cfg.InstanceID}).Sync(context.Background(), controlpanel.Snapshot{Contacts: []controlpanel.Contact{}, Failures: []controlpanel.Failure{}})
		if err != nil {
			panic(err)
		}
		fmt.Println("ponte autenticada e sincronizada")
		return
	}
	tokens, err := io.ReadAll(io.LimitReader(os.Stdin, 8192))
	if err != nil {
		panic(err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(tokens)), "\n", 2)
	if len(parts) != 2 {
		panic("informe os tokens da ponte e do painel em duas linhas")
	}
	err = store.Save(controlpanel.Config{Enabled: true, Endpoint: *endpoint, DashboardURL: *dashboard, InstanceID: *instance}, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	if err != nil {
		panic(err)
	}
	fmt.Println("painel configurado; token protegido pelo Windows")
}
