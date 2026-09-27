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

- Status: concluída.
- Artefato principal: `dist/Tino.exe`.
- SHA-256 local: `D540943ED44A841B184107DA529E73381108362BDC2D5139E86ABB0F7E1163B4`.
- Evidência: commit `036c6c6` enviado à branch `main`; release `v0.2.0` publicada com `Tino.exe` e CLI auxiliar.

## Etapa 5 — Redesign profissional v0.3.0

- Status: concluído e publicado.
- Escopo: identidade visual própria, ícone incorporado, cabeçalho de produto, navegação em quatro áreas, onboarding de QR, cards de operação, indicador de atividade e hierarquia visual consistente.
- Evidência: `go test -count=1 ./...` e `go vet ./...` passaram; `Tino.exe` abriu responsivo com título `Tino • Central de Comunicação 0.3.0`; nenhuma falha foi gravada no diagnóstico de inicialização.
- SHA-256 local do `Tino.exe`: `DB215D0807A1DFFA16FAF23C90D51D9F1159D1536B732438CF5677F049B3BB0C`.
- Limite: a ferramenta de inspeção visual nativa não estava disponível nesta sessão; runtime e estrutura foram validados, mas a inspeção pixel a pixel deve ser confirmada no monitor do usuário.
- Evidência de release: commit `a946e38` na branch `main`; release `v0.3.0` publicada com `Tino.exe` e CLI auxiliar.

## Etapa 6 — Flow Builder visual v0.4.0

- Status: concluído e publicado.
- Escopo: criar, editar, excluir e reordenar regras; resposta padrão; simulador de mensagem; importação; persistência e ativação pela UI.
- Usabilidade adicional: validação do CSV na seleção, resumo de contatos consentidos, confirmação contextual antes do envio e diálogo de conclusão da exportação.
- Evidência: testes do motor incluem salvar/carregar, matching e validação; `go test -count=1 ./...` e `go vet ./...` passaram; `Tino.exe` abriu responsivo como versão `0.4.0` sem erro de inicialização.
- SHA-256 local: `DB71D3CC87F7EC0219EBCEC8A3D0522C6B47AF4176BCE99E62B8B92DAA574EE2`.
- Evidência de release: commit `ff072a6` enviado à `main`; release `v0.4.0` publicada com UI e CLI auxiliar.

## Etapa 7 — CSV telefônico simples v0.5.0

- Status: concluído localmente; publicação em andamento.
- Arquivo de referência: `telefones-whatsapp.csv`, 101 registros, UTF-8 BOM, coluna única `telefone`, valores com `+55` e aspas.
- Implementação: parser flexível com aliases, BOM e delimitadores comuns; mensagem global e confirmação de consentimento na interface.
- Evidência: testes específicos do layout de uma coluna e de aliases com ponto e vírgula passaram; suíte completa e `go vet` passaram; UI `0.5.0` abriu responsiva sem erro de inicialização.
- SHA-256 local: `38ABE265FE5039804A12E9D01CE30738E2AA75A4EF65395F966F1813F7A133FD`.
