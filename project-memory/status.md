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

- Status: concluído e publicado.
- Arquivo de referência: `telefones-whatsapp.csv`, 101 registros, UTF-8 BOM, coluna única `telefone`, valores com `+55` e aspas.
- Implementação: parser flexível com aliases, BOM e delimitadores comuns; mensagem global e confirmação de consentimento na interface.
- Evidência: testes específicos do layout de uma coluna e de aliases com ponto e vírgula passaram; suíte completa e `go vet` passaram; UI `0.5.0` abriu responsiva sem erro de inicialização.
- SHA-256 local: `38ABE265FE5039804A12E9D01CE30738E2AA75A4EF65395F966F1813F7A133FD`.
- Evidência de release: commit `1654331` enviado à `main`; release `v0.5.0` publicada com UI e CLI auxiliar.

## Etapa 8 — Conversas na home v0.6.0

- Status: concluído e publicado.
- Escopo: QR condicional, lista pesquisável de chats, histórico persistente, novas mensagens, não lidas, importação HistorySync e resposta manual.
- Persistência: `data/chats-<profile>.db`, ignorado pelo Git.
- Evidência: testes do armazenamento SQLite passaram; suíte completa e `go vet` passaram; UI `0.6.0` abriu responsiva sem erro de inicialização.
- Limite: volume do histórico antigo depende dos blocos enviados pelo WhatsApp; pareamento e sincronização reais ainda exigem conta/dispositivo autorizado.
- SHA-256 local: `0430742955D161C5D25E64733B164C0C8E7ABB7BC8C2EC5B8A7A9FE39A5F3F5D`.
- Evidência de release: commit `36d62c2` enviado à `main`; release `v0.6.0` publicada com UI e CLI auxiliar.

## Etapa 9 — Correção de autenticação v0.6.1

- Status: concluída e publicada.
- Causa: a UI usava `IsConnected()`, que representa somente a conexão do socket, e podia mostrar uma sessão local obsoleta como conectada sem vínculo ativo no celular.
- Correção: estado e operações agora exigem `IsLoggedIn()`; reconexão persistida aguarda autenticação; eventos de desconexão atualizam a UI; a ação **Gerar novo QR Code/Trocar conta** limpa somente a credencial local e preserva o histórico.
- Evidência: `go test ./...` e `go vet ./...` passaram; build Windows amd64 `0.6.1` passou; `Tino.exe` abriu responsivo com o título `Tino • Central de Comunicação 0.6.1`, sem log de erro de inicialização.
- Limite: o inventário da ferramenta de inspeção visual retornou vazio; pareamento real e confirmação em **Dispositivos conectados** ainda exigem o celular do usuário.
- SHA-256 local do `Tino.exe`: `052D4BC1A859A573AC8A6751FC741584ED1E416C00F7C962ED6CFB67C67777EA`.
- Evidência de release: commit `2a9f304` enviado à `main`; release `v0.6.1` publicada com UI e CLI auxiliar.

## Etapa 10 — Interface moderna e Central de Recursos v0.7.0

- Status: concluída e publicada.
- Interface: Wails v2.15 + React 19 + Vite + Lucide; navegação lateral, onboarding/QR, conversas, base/notificações, Flow Builder, recursos/perfis/arquivos e atividade foram reconstruídos.
- Governança: feature flags persistentes, bloqueio backend de notificações/auditoria/flow quando desativados, matriz inicial de perfis e seleção de pasta autorizada.
- Assistente Paraíso: quartos, caixa/OCR, extratos, vales, comercial/OmniBees/catálogo e backup foram mapeados na UI como planejados e documentados, sem importar segredos ou dados.
- Evidência: `go test -count=1 ./...` e `go vet ./...` passaram; teste de persistência da Central de Recursos passou; build Vite produziu bundle sem vulnerabilidades reportadas pelo npm; build Wails 0.7.0 gerou executável; `Tino.exe` abriu responsivo e iniciou o processo filho `msedgewebview2.exe`.
- Limite: a ferramenta de inspeção visual não expôs a janela nativa nem navegador nesta sessão; falta inspeção automatizada pixel a pixel. Pareamento e operações WhatsApp reais continuam dependentes de conta/dispositivo autorizado.
- SHA-256 local do `Tino.exe`: `20B1368A1B73EC131EDAC2F4AA5C1B3E4959FFB80800F027B150AA03D968FCA2`.
- Evidência de release: commit `3bdc89d` enviado à `main`; release `v0.7.0` publicada com a interface moderna e a CLI auxiliar.

## Etapa 11 — Correção da Central de Recursos v0.7.1

- Status: concluída e publicada.
- Causa: o React tentou acessar `Modules`, `Roles` e `WorkspaceRoot`, enquanto o binding Wails serializa os campos como `modules`, `roles` e `workspaceRoot`.
- Correção: contrato normalizado, uso consistente de camelCase e estado de erro recuperável com botão **Tentar novamente**.
- Inventário: funções do Assistente Paraíso catalogadas em `docs/ASSISTENTE_PARAISO_FUNCTIONS.md` para definição funcional com o usuário.
- Evidência: dois testes Node do contrato passaram; build Vite passou; o binding Wails confirmou os nomes camelCase; `go test -count=1 ./...` e `go vet ./...` passaram; `Tino.exe` 0.7.1 abriu responsivo com WebView2 ativo.
- SHA-256 local do `Tino.exe`: `6D8CC2EDD2E477D840809CA6617D1689D8C5E555D836007F0E4E697F14D0810C`.
- Evidência de release: commit `152c46a` enviado à `main`; release `v0.7.1` publicada com UI e CLI auxiliar.

## Etapa 12 — Núcleo operacional unificado

- Status: integrado e validado localmente; telas administrativas especializadas ainda em desenvolvimento.
- Escopo concluído nesta etapa: código de quartos, relatórios, caixa/OCR/comprovantes, extratos, vales, OmniBees, comercial, catálogo e backup migrado; transporte anexado à sessão `whatsmeow` do Tino; cadastro de operadores na Central de Recursos.
- Persistência: `data/operations/hotel.db`, ignorada pelo Git; nenhuma sessão, segredo ou base real do Assistente foi copiada.
- Evidência: `go test -count=1 ./...` passou para todos os pacotes; testes Node do contrato passaram; build Vite passou; Wails gerou o executável 0.8.0, que abriu responsivo com WebView2 ativo.
- SHA-256 local do `Tino.exe`: `020076CAE1B9F2882417AFC7D0967549CE277E571B3DE82DBD21306DC041E838`.
- Evidência de release: commit `d7631e8` enviado à `main`; release `v0.8.0` publicada com UI e CLI auxiliar.
- Pendente após esta etapa: painéis de quartos, comercial, catálogo e backup; teste real com conta WhatsApp e provedores externos.

## Etapa 13 — Operação financeira visual v0.9.0

- Status: concluída e validada localmente; compartilhamento real depende de conta WhatsApp autenticada.
- Escopo: dashboard de caixa com hoje, sete dias, mês ou período personalizado; totais de entradas, saídas, saldo, dinheiro, PIX e cartão; PDFs de quartos, caixa e vales; envio para telefone ou grupo; consulta e preview de comprovantes; painéis de semanas de extratos e vales mensais.
- Segurança: preview valida que o arquivo permanece dentro de `data/operations`; intervalo máximo de consulta é 366 dias.
- Evidência: teste de integração criou caixa, movimentos, comprovante e vale em banco temporário e validou totais, listagem e preview; `go test -count=1 ./...`, `go vet ./...`, testes Node e build Vite passaram; build Wails 0.9.0 passou.
- SHA-256 local do `Tino.exe`: `B2C1354B55007E7D512EA88761A6909E3C9C13B30380B98E1C3B1BB776A58D3E`.
- Evidência de release: commit funcional `df6f90d` enviado à `main`; release `v0.9.0` publicada com UI e CLI auxiliar.
- Limite: a skill de controle do Windows não encontrou superfícies de aplicativos expostas nesta sessão. O processo abriu responsivo com WebView2, mas a inspeção visual automatizada não foi possível.

## Etapa 14 — Gestão completa do Assistente v0.10.0

- Status: implementada e validada localmente; fluxos com provedores e WhatsApp real permanecem gates externos.
- Quartos: painel dos 42 quartos, contagem por situação, hóspedes, observação, atualização auditada e histórico recente.
- Comercial: modos desativado, teste por números autorizados e público persistidos na mesma configuração lida pelo bot.
- Catálogo: inventário de produtos reais vinculados, metadados e remoção por categoria; captura continua pelo compartilhamento de produto no WhatsApp Business.
- Backup/saúde: diagnóstico de banco, socket e autenticação; criação manual de ZIP verificado contendo banco, comprovantes e extratos, sem sessão ou segredo.
- Migração: configurações `capabilities.json` antigas recebem os novos metadados de disponibilidade sem perder operadores, pasta autorizada ou opções previamente habilitadas.
- Evidência parcial: teste de API atualizou quarto, hóspedes e observação e conferiu os 42 registros e histórico; testes de comercial, catálogo e backup permanecem na suíte migrada.
- Evidência: `go test -count=1 ./...`, `go vet ./...`, testes Node, build Vite e build Wails passaram; `Tino.exe` abriu responsivo com WebView2 ativo.
- SHA-256 local do `Tino.exe`: `B5EDDE0E7C1A50A2CCEC4C474F13D4E4F3CD4E60F811EE3F4A2A2CC265F46D9E`.
- Limite: a configuração comercial, o catálogo e os comandos foram validados em testes locais; mensagens reais, captura de produto e OmniBees exigem conta e provedor externos.
- Evidência de release: commit funcional `ca783ca` enviado à `main`; release `v0.10.0` publicada com UI e CLI auxiliar.

## Etapa 15 — Importação de caixa por PDF v0.11.0

- Status: implementada, validada e publicada; os dados conciliados de setembro também foram importados localmente.
- Escopo: seleção múltipla, leitura dos dois layouts recebidos, preview de saldo inicial/entradas/saídas/final, validação contábil, deduplicação por SHA-256 e data, armazenamento do original e confirmação pela UI.
- Importação real: 18 dos 24 relatórios foram gravados em `data/operations/hotel.db`; 6 permaneceram fora do caixa por divergência documental (04, 09, 10, 11, 12 e 13/09/2026).
- Evidência: a segunda leitura classificou os 18 como `IMPORTED` e manteve os 6 como `REVIEW`; `go test -count=1 ./...`, `go vet ./...`, testes Node e build Vite passaram; o build Wails 0.11.0 gerou `Tino.exe`, que iniciou com processo responsivo.
- SHA-256 local do `Tino.exe`: `A04C65673751D622FDB9704F3D47699AA1AEB765B15446E45C6D59DA6E60EF4F`.
- Evidência de release: commit funcional `8831b7c` enviado à `main`; release `v0.11.0` publicada com a interface e a CLI auxiliar.

## Etapa 16 — Dashboard, quartos e categorias OmniBees v0.11.1

- Status: implementada, validada e publicada.
- Dashboard: abre no mês corrente, seleciona o período importado automaticamente e calcula abertura/fechamento sem somar saldos diários repetidos.
- Quartos: o resumo enviado antes do PDF lista separadamente os números sujos/para limpar e limpos/desforrados.
- OmniBees: nomes equivalentes de categorias são normalizados; quando a mesma suíte aparece em mais de uma tarifa, o menor total positivo é preservado. Teste com três categorias confirma que todas chegam ao orçamento.
- Evidência: testes direcionados, `go test -count=1 ./...`, `go vet ./...`, testes Node e build Vite passaram; uma consulta real da OmniBees retornou cinco categorias para ocupação dupla e três para ocupação quádrupla; o build Wails 0.11.1 gerou `Tino.exe`, que iniciou responsivo.
- SHA-256 local do `Tino.exe`: `88CE704875611EA10B0EB6662477C5F54AEF93A2415E11D5BE669207779A3130`.
- Evidência de release: commit funcional `482b844` enviado à `main`; release `v0.11.1` publicada com a interface e a CLI auxiliar.

## Etapa 17 — Comandos em grupos e relatório mensal de caixa v0.11.2

- Status: implementada e validada localmente; entrega real das mensagens ainda depende do WhatsApp conectado.
- Bot: comandos enviados pelo titular da conta em grupos deixam de ser descartados; em grupos, terceiros continuam bloqueados salvo operadores explicitamente autorizados.
- Caixa: `relatorio caixa em pdf` sem data agora abrange o mês corrente até hoje, incluindo os caixas importados. Datas explícitas continuam gerando um dia ou intervalo específico.
- Relatório genérico: `relatorio em pdf` envia quartos e, em seguida, o caixa do mês corrente.
- Evidência: testes direcionados de filtro de grupo e intervalo mensal passaram; `go test -count=1 ./...`, `go vet ./...` e build Wails 0.11.2 passaram.
- SHA-256 local do `Tino.exe`: `14C0DD540C5CB462A894ED38D277A1933DA56D0C199581EBF2E316D98B44FA97`.
- Evidência de release: commit funcional `75202b3` enviado à `main`; release `v0.11.2` publicada com a interface e a CLI auxiliar.
- Limite: falta confirmar recebimento em conversa/grupo reais após a reabertura do executável.

## Etapa 18 — Paridade de categorias OmniBees v0.11.3

- Status: corrigida e validada contra consulta real da OmniBees.
- Consulta: 15/10/2026 a 18/10/2026, um quarto, dois adultos e nenhuma criança.
- Causa: o Tino aplicava um filtro local por capacidade e escondia quartos maiores que a OmniBees oferecia para a mesma ocupação.
- Resultado: as seis acomodações confirmadas na resposta real agora aparecem no orçamento, com identidade física e preço preservados.
- Evidência: teste ao vivo retornou Superluxo, Triplo Deluxe, Quádruplo Deluxe, Quádruplo Deluxe com vista mar, Triplo com varanda e Quádruplo com varanda; testes determinísticos cobrem a retenção das seis categorias.
- Evidência de build: `go test -count=1 ./...`, `go vet ./...` e Wails 0.11.3 passaram; SHA-256 do `Tino.exe`: `0F08F931647B4692C88B86737A0FEC73159446B8698E4D2A4CBF7331DC2E83E2`.
- Evidência de release: commit funcional `f8bed17` enviado à `main`; release `v0.11.3` publicada com a interface e a CLI auxiliar.

## Etapa 19 — Capacidade exata nos orçamentos v0.11.4

- Status: corrigida após esclarecimento da regra comercial.
- Regra: 1–2 hóspedes veem somente duplos, 3 somente triplos, 4 somente quádruplos e 5 somente família; todas as opções ainda precisam ter preço confirmado na OmniBees.
- Consulta real de 15/10/2026 a 18/10/2026: 2 pessoas retornaram apenas Superluxo dupla; 3 retornaram dois triplos; 4 retornaram três quádruplos; 5 não tinham preço disponível.
- Evidência: testes determinísticos cobrem cada capacidade e impedem regressão para quartos maiores.
- Evidência de build: `go test -count=1 ./...`, `go vet ./...` e Wails 0.11.4 passaram; SHA-256 do `Tino.exe`: `CB8B9407CE06F91293ABD159ED532D4468BA13B550AF6C20B4C6E79CEE65090B`.
- Evidência de release: commit funcional `d31a4ee` enviado à `main`; release `v0.11.4` publicada com a interface e a CLI auxiliar.

## Etapa 20 — Formatação monetária brasileira v0.11.5

- Status: corrigida e validada.
- Orçamentos passam a usar ponto para milhares e vírgula para centavos; por exemplo, `R$ 2.504,20`.
- Evidência: testes cobrem zero, centenas, milhares e milhões; consulta real de 15/10/2026 a 18/10/2026 exibiu `R$ 2.235,54`.
- Robustez: comandos de caixa passam a usar a data operacional do repositório, evitando divergência na virada do dia quando o fuso configurado difere do fuso do processo.
- Evidência de build: `go test -count=1 ./...`, `go vet ./...` e Wails 0.11.5 passaram; SHA-256 do `Tino.exe`: `B0B2B4C316296E495121A3FB8E330800A5E7D4C401BE06CF3CC1FF53294E415A`.
- Evidência de release: commit funcional `5a0da2d` enviado à `main`; release `v0.11.5` publicada com a interface e a CLI auxiliar.
