# Estado

## Etapa 1 — Fundação e implementação

- Status: concluída localmente.
- Evidência: `go test -count=1 ./...` passou; `go vet ./...` passou; build Windows amd64 com CGO desativado passou; `tino-windows-amd64.exe version` retornou `0.1.0`; `status` criou/leu SQLite e retornou sessão não autenticada corretamente.
- Limite de evidência: `go test -race` requer CGO e não foi executado; pareamento QR, entrega real e estabilidade prolongada exigem conta/dispositivo e não foram validados.

## Etapa 2 — Release v0.1.0

- Status: concluída.
- Evidência: commit inicial `68990c4` enviado à branch `main`; repositório público `https://github.com/gadevsbr/Tino`; release `v0.1.0` publicada com `tino-windows-amd64.exe`.
- SHA-256 do artefato: `D633F82FB74C95E2B111A7D9C0ACA032ABA122458CA59286429C8C54FAA33BC0`.

## Etapa 3 — Interface gráfica Windows

- Status: concluída localmente.
- Motivo: `v0.1.0` era somente CLI e fechava ao receber duplo clique sem argumentos.
- Escopo: janela nativa, QR na interface, status, exportação, seletor CSV, envio consentido, log de atividade e ativação de fluxo YAML.
- Evidência: `go test -count=1 ./...` e `go vet ./...` passaram; `Tino.exe` foi compilado com subsistema gráfico; processo abriu responsivo com título `Tino — Comunicação Corporativa 0.2.0`.
- Correção de runtime: o primeiro teste da UI revelou `TTM_ADDTOOL failed`; foi incorporado manifesto Common Controls v6 via recurso `.syso`, e a reabertura não gerou erro de inicialização.
- Limite: pareamento por QR e operações reais ainda dependem de conta/dispositivo autorizado.

## Etapa 4 — Release v0.2.0

- Status: em andamento.
- Artefato principal: `dist/Tino.exe`.
- SHA-256 local: `D540943ED44A841B184107DA529E73381108362BDC2D5139E86ABB0F7E1163B4`.
