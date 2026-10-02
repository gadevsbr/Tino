# Estado

## Etapa 43 — Categoria física e bloqueio de UH v0.15.6

- Cadastro dos 42 quartos recebe categoria física Bitz com lista fechada e validação backend; a UI permite editar e exibe a categoria no card.
- Pré-reserva monta por categoria a lista de números permitidos, remove `INTERDITADO`, confere capacidade para pedidos repetidos e passa a lista ao Scrapling.
- Scrapling escolhe somente uma UH simultaneamente disponível no Bitz e permitida pelo Tino, sem repetir número entre quartos.
- Categorias ainda não mapeadas usam o comportamento anterior para permitir configuração gradual; categorias já mapeadas falham com aviso humano quando não houver capacidade operacional.
- Evidência: testes direcionados de migração, validação, filtro de interditado, capacidade e seletor de UH passaram; o runtime Scrapling empacotado iniciou e respondeu pelo protocolo JSON; `go test -count=1 ./...`, `go vet ./...`, testes Node e build Vite passaram.
- Build Windows v0.15.6 concluído e iniciado responsivo; SHA-256 de `dist/Tino.exe`: `263E0DAD6912F3AAB13EF8CBBEBB5A6BA3773B12B090B6A167737186AF391384`.
- Migração confirmada no banco operacional real: coluna `bitz_category` presente, 42 quartos preservados e nenhum mapeamento inventado automaticamente; a classificação deve ser feita conscientemente na UI.
- Evidência de release: commit funcional `2eaebc6` enviado à `main`; release `v0.15.6` publicada com `Tino.exe` e CLI auxiliar.

## Etapa 42 — Atendimento humanizado, fila e idempotência v0.15.5

- Respostas comerciais aguardam 3 segundos com presença `digitando...`, mantendo textos simples e acolhedores.
- Nova fila FIFO assíncrona preserva ordem por conversa e permite processar contatos diferentes em paralelo; operadores continuam serializados por conta.
- A reivindicação persistente por conta, contato e ID da mensagem ocorre antes do fluxo, bloqueando reentregas duplicadas inclusive após reinício.
- Testes direcionados cobrem ordem FIFO, paralelismo entre contatos, recuperação após pânico, chaves de isolamento, atraso configurado e idempotência SQLite.
- `go test -count=1 ./...`, `go vet ./...`, testes e build do frontend passaram. `go test -race` não pôde ser executado porque o ambiente está com CGO desativado.
- Build Windows v0.15.5 concluído; SHA-256 de `dist/Tino.exe`: `33FC05425953A55EA356A765FB7FA438E38B63F62BC6797D7BBEF6DAB218ED00`.
- Gate externo: ao reiniciar, o WhatsApp respondeu novamente `401 logged out from another device` e removeu a credencial; um novo QR é necessário para validar o atraso e a fila em conversa real.
- Release publicada: `v0.15.5` com `Tino.exe` e `tino-cli-windows-amd64.exe`.

## Etapa 41 — Recuperação segura de timeout no login Bitz v0.15.4

- A falha `13A457F3` ocorreu na etapa `login` com `browser_or_timeout`; nenhuma nova pré-reserva foi criada por essa tentativa.
- O teste real sem salvamento passou em seguida por login, CPF, datas, canal de venda, UH e revisão em 24 segundos, confirmando credenciais e seletores atuais.
- O Scrapling passa a tolerar até 45 segundos por controle e faz uma única nova tentativa com navegador limpo somente se ainda estiver no login; etapas de reserva e salvamento nunca são repetidas.
- Runtime empacotado reconstruído; teste de protocolo e novo probe real com o runtime empacotado passaram, sem salvar reserva.
- Validação: `go test -count=1 ./...`, `go vet ./...`, testes e build do frontend, protocolo do runtime empacotado e dois probes reais sem salvamento passaram.
- Build Windows v0.15.4 concluído; SHA-256 de `dist/Tino.exe`: `DC622ABCD372B38C4510276B8DA4576E12F6FE724357C5B6477B2774E494846C`.
- O executável v0.15.4 reiniciou autenticado, com dispositivo persistido e conexão estabelecida.
- Release publicada: `v0.15.4` com `Tino.exe` e `tino-cli-windows-amd64.exe`.

## Etapa 40 — Aviso completo ao aprovador e áudio para humano v0.15.3

- Aviso de pré-reserva passa a incluir WhatsApp do hóspede, período, categoria, adultos e crianças/idades de cada quarto.
- Destino do aprovador é validado e resolvido pelo WhatsApp antes do envio; falha deixa o chat do hóspede não lido.
- Áudio de hóspede pausa imediatamente o bot, responde com acolhimento simples e encaminha a conversa não lida para a equipe.
- Saudação, menu e transferências foram reescritos com instruções diretas e linguagem acessível.
- Validação: testes direcionados confirmam handoff de áudio, pausa persistente e resumo completo ao aprovador; `go test -count=1 ./...`, `go vet ./...`, testes e build do frontend passaram.
- Build Windows v0.15.3 concluído; SHA-256 de `dist/Tino.exe`: `6A04815A305066FA0F7FC0FD49ECFDF3D2ED0C4AB4728B075B031C310C7D97AB`.
- Gate externo: ao reabrir o executável, o WhatsApp respondeu `401 logged out from another device` e removeu a credencial local. Novo QR é necessário antes de validar a entrega real ao aprovador.
- Release publicada: `v0.15.3` com `Tino.exe` e `tino-cli-windows-amd64.exe`.

## Etapa 39 — Confirmação assíncrona do QR v0.15.2

- O QR foi aceito e persistiu um dispositivo `smba`; depois do falso timeout, o processo estabeleceu websocket com o WhatsApp e sincronizou mensagens.
- A espera sobe de 20 para 60 segundos e uma credencial já persistida deixa a sincronização terminar em segundo plano.
- Validação: `go test -count=1 ./...`, `go vet ./...`, testes e build do frontend passaram; o executável v0.15.2 reiniciou com 1 dispositivo persistido, 1 conexão estabelecida e log `whatsapp connected`, sem novo QR.
- Binário Windows: `dist/Tino.exe`, SHA-256 `2D6F970C37A101D6D735A010BA0344EFA44FB5DF78CA403F309CBE2A07B60022`.
- Release publicada: `v0.15.2` com `Tino.exe` e `tino-cli-windows-amd64.exe`.

## Etapa 38 — Reconexão automática do WhatsApp v0.15.1

- Causa da ausência de novos jobs Bitz confirmada: o `Tino.exe` v0.15.0 estava aberto sem conexão de rede e não havia mensagem processada desde 01/10/2026 02:09 UTC.
- A inicialização agora reconecta automaticamente a sessão persistida após instalar os handlers do chat e da operação hoteleira.
- A reconexão foi exercitada com a sessão existente: o WhatsApp respondeu `LoggedOut`, e o whatsmeow removeu corretamente a credencial inválida. `whatsmeow_device` ficou com zero registros; um novo QR precisa ser escaneado antes de validar mensagens.
- Testes Go direcionados, `go vet ./...`, testes/build React e build Wails passaram. SHA-256 do `Tino.exe`: `118F479F2E3B7FDD2C23F8C89621013513F93E01B61F4598CBC7B3B6FAEA2D4E`.
- Release `v0.15.1` publicada a partir do commit funcional `0117e81`. Validação WhatsApp/Bitz ponta a ponta bloqueada até novo pareamento.

## Etapa 37 — Migração Bitz para Scrapling v0.15.0

- Investigação real com Scrapling chegou à revisão final com uma UH, sem salvar. Confirmados: tela intermediária de canal, datas revertidas por `fill`, IDs duplicados e botão de salvar UH distinto do salvamento final.
- Driver novo implementado com checagem do estado aceito pelo Bitz, modo probe sem gravação e confirmação pelo ID/código retornado no salvamento. O salvamento nunca é repetido automaticamente.
- Evidência real sem gravação: fluxo completo até a tela final passou para `triploDeluxe`, 05/10/2026–09/10/2026.
- Evidência real com gravação: o Bitz confirmou a pré-reserva de teste código `3169`, UH `201`, no mesmo período. A equipe deve finalizar ou cancelar esse registro de teste.
- O Bitz retornou HTTP 429 durante tentativas rápidas de investigação; retentativas agressivas permanecem proibidas.
- Evidência local: `go test -count=1 ./...`, `go vet ./...`, testes e build React, runtime Scrapling empacotado, compilação com tag `bitzpackaged` e abertura do `Tino.exe` passaram. SHA-256: `2EF4E378333EDD7B02731DC6F166436DA67072F63ED6A14F55C574092298E11D`.
- Evidência de release: commit funcional `4c0993a` enviado à `main`; release `v0.15.0` publicada com a UI e a CLI auxiliar.

## Etapa 36 — Teste automatizado completo do Bitz v0.14.7

- `TestBrowserRunnerCompletesReservationWizard` executa o mesmo encadeamento do chatbot em navegador real contra um Bitz simulado e confirma CPF, período, UH física e salvamento final.
- `scripts/test-bitz-live.ps1` permite testar diretamente no Bitz real sem percorrer o WhatsApp; exige confirmação explícita porque cria uma pré-reserva real.
- O teste real aceita de uma a seis origens: `superluxo`, `familia`, `quadruploVista`, `triploDeluxe`, `quadruploDeluxe`, `triploVaranda`, `quadruploVaranda`, `duplo` e `triplo`.
- Evidência: o E2E simulado passou; o teste real permaneceu corretamente ignorado sem confirmação. Parser PowerShell, `go vet ./...`, `go test -count=1 ./...`, testes React, build Vite e build Wails passaram. SHA-256 do `Tino.exe`: `6A3A4D4CAE0CB929B4FB781BF397AC732905842D8FCC9D9DCED2FAB4013C0706`.
- Evidência de release: commit funcional `efeb068` enviado à `main`; release `v0.14.7` publicada com UI e CLI auxiliar.

## Etapa 35 — Seletores reais e tabela de UHs Bitz v0.14.6

- Campos de CPF e datas, pesquisa, avanços, adição de UH e salvamento usam os IDs reais fornecidos pelo operador.
- O HTML real confirmou que a tabela usa `#table-quartos-disponiveis-reserva` e marca a linha pelo ícone da primeira célula, não por `input[type=checkbox]`.
- O transporte agora preserva `key`, `source_key` e nome da categoria OmniBees até o Bitz; isso diferencia corretamente família, superluxo, triplo, quádruplo e variações deluxe/varanda/interna.
- Evidência: testes DOM reproduzem os seletores e a estrutura da tabela fornecida; regressões direcionadas, `go vet ./...`, `go test -count=1 ./...`, testes React, build Vite e build Wails passaram. SHA-256 do `Tino.exe`: `3891E3C098A11EE5C3388108A21D0EBEAF877EB148D502DDEC401791835F154A`.
- Correção de capacidade: a UH Família foi removida das alternativas de até três ocupantes e permanece exclusiva para cinco pessoas; a opção vista-mar nessa faixa usa a UH Superluxo.
- Evidência de release: commit funcional `551565e` enviado à `main`; release `v0.14.6` publicada com UI e CLI auxiliar.

## Etapa 34 — Botão direto de nova reserva Bitz v0.14.5

- O robô agora clica diretamente em `#btn-add-reserva`, ID confirmado no sistema real.
- F2 permanece apenas como fallback caso o botão ainda não exista ou esteja desabilitado durante o carregamento.
- Evidência: regressão específica do botão, regressão do fallback F2, `go vet ./...`, `go test -count=1 ./...`, testes React, build Vite e build Wails passaram. SHA-256 do `Tino.exe`: `6AC229B354AEF52029DC98861D738F7C8175B19B4279DD21C1D379151EC4AC4B`.
- Evidência de release: commit funcional `16b93ff` enviado à `main`; release `v0.14.5` publicada com UI e CLI auxiliar.

## Etapa 33 — Abertura resiliente da pré-reserva Bitz v0.14.4

- Causa confirmada no job `428F96D1`: login aceito, seguido de `abrir nova reserva: tela esperada não apareceu: NOVA RESERVA`.
- Correção: o robô aguarda carregamento completo, garante foco, repete o atalho F2 e possui fallback por evento de teclado da própria página.
- Diagnóstico: em nova falha, o erro inclui URL e título da página, sem senha, CPF ou dados do hóspede.
- Evidência: regressão local do atalho, `go vet ./...`, `go test -count=1 ./...`, testes React, build Vite e build Wails passaram. Validação controlada no Bitz real também passou em 8,04 s, limitada a login e abertura de “NOVA RESERVA”; nenhum campo foi preenchido e nenhuma reserva foi salva. SHA-256 do `Tino.exe`: `AE846B8C76980E3E3F92F4E35843EFB22DEB8187F6075F5E654B8E4B701D4275`.
- Evidência de release: commit funcional `051e33f` enviado à `main`; release `v0.14.4` publicada com UI e CLI auxiliar.

## Etapa 32 — Sessão comercial de operador isolada v0.14.3

- Causa confirmada no banco: o telefone admitido tinha `paused:true`, e o teste do operador reutilizava esse mesmo estado apesar da mensagem dizer “sessão separada”.
- Correção: `testar atendimento` roteia para uma identidade sintética exclusiva do operador e reinicia somente essa sessão; `sair atendimento` também limpa o estado de teste.
- Proteção: a identidade sintética só é admitida quando o transporte autenticado marca explicitamente a entrada como sessão de teste no modo comercial `test`.
- Evidência: regressão direcionada, testes dos pacotes comercial/WhatsApp, `go vet ./...`, `go test ./...`, testes React, build Vite e build Wails passaram. O `Tino.exe` abriu responsivo; SHA-256 `FD1E2608368D37709C0A8B3B0B03B7419A5B2E83CAAA42DF1A968A04ADF0C348`.
- Evidência de release: commit funcional `c5cfdf0` enviado à `main`; release `v0.14.3` publicada com UI e CLI auxiliar.

## Etapa 31 — Acesso direto à Integração Bitz v0.14.2

- Correção de encontrabilidade: nova entrada **Integração Bitz** no menu lateral.
- A tela direta reúne credenciais, CPF operacional, aprovador, mensagem final, teste de acesso, status e **Salvar e ativar**.
- Evidência: testes React e Go direcionados, build Vite e build Wails passaram; o executável abriu responsivo. SHA-256 do `Tino.exe`: `7220A352699CFDBE5AF151128E869DBDBEF38FDC36A71CFCF294AF226B1A1A4F`.
- Limite visual: o controlador de janelas não expôs o processo Wails para captura, portanto a presença do item foi validada pelo bundle compilado e pelo processo responsivo, não por screenshot automatizado.
- Evidência de release: commit funcional `10d4e35` enviado à `main`; release `v0.14.2` publicada com UI e CLI auxiliar.

## Etapa 30 — Preservação das mensagens e diagnóstico Bitz v0.14.1

- Causa confirmada no banco: um comando de modo comercial gravou `final_message_1` e `final_message_2` vazias; a tentativa Bitz falhou com `integração Bitz desativada`.
- Correção: comandos de modo preservam todos os campos definidos na UI; regressão multi-quarto exige as duas mensagens antes da escolha; falha detalhada vai ao aprovador.
- UI: status Ativa/Inativa permanece visível e o botão principal agora diz **Salvar e ativar**, com ação separada para desativar.
- Evidência: regressões direcionadas, `go test -count=1 ./...`, `go vet ./...`, testes Node, build Vite e build Wails passaram. SHA-256 do `Tino.exe`: `3C86516067E20DE4B70B57D75ADFD5C476E12AF8C7B27D10FCAEA3B66CCB83A3`.
- Gate operacional: a configuração local continua intencionalmente inativa até o operador clicar **Salvar e ativar**; nenhuma credencial foi impressa ou alterada durante o diagnóstico.
- Evidência de release: commit funcional `c2dbb90` enviado à `main`; release `v0.14.1` publicada com UI e CLI auxiliar.

## Etapa 29 — Pré-reserva Bitz v0.14.0

- Status: implementada e validada localmente; a criação real depende de credenciais e validação controlada no Bitz autenticado.
- Escopo: configuração visual; senha DPAPI; teste de login sem gravação; escolha de categoria por quarto; Chromium local; até seis UHs; espera de 20 segundos; aviso ao aprovador; confirmação por código; mensagem final ao hóspede.
- Segurança: senha nunca é retornada à UI; arquivo privado permanece em `data/`; execução serializada e identificador persistente impedem repetção automática.
- Evidência atual: testes Go de DPAPI, descoberta do EdgeCore, login Chromium local sem reserva, idempotência/aprovação e seleção multi-quarto passaram; suíte completa, `go vet`, testes Node e build Vite passaram.
- Gate: falta executar **Testar acesso** com as credenciais reais e uma pré-reserva controlada para confirmar o DOM autenticado do Bitz e o recebimento no WhatsApp aprovador.
- Build Windows: `Tino.exe` 0.14.0 abriu responsivo; SHA-256 `3160E3655C2FC38942CA42D814F20E3E50C29F5E5820E4BA8217A24FC2E21323`.
- Evidência de release: commit funcional `1db033f` enviado à `main`; release `v0.14.0` publicada com UI e CLI auxiliar.

## Etapa 28 — Confirmação objetiva da categoria selecionada v0.13.3

- Status: corrigido e validado localmente após retorno do teste real.
- Correção: a escolha confirma que a suíte está disponível no período consultado e informa que o atendente concluirá a reserva; foram removidas as frases que pediam nova confirmação de disponibilidade ou sugeriam incerteza.
- Evidência: regressão seleciona a categoria `1`, exige disponibilidade afirmativa e rejeita o texto antigo; `go test -count=1 ./...`, `go vet ./...` e build Wails 0.13.3 passaram. SHA-256: `59247A862C6DA707FF17E85DE7CB63C50198AA418F9D408515AC896FBC5B748C`.

## Etapa 27 — Seleção numerada e encerramento comercial v0.13.2

- Status: corrigido e validado localmente após retorno do teste real.
- Categorias: depois dos produtos do catálogo, o bot envia a relação numerada e pede explicitamente que o cliente escolha pelo número.
- Atendimento: a alternativa `*atendente*` usa negrito do WhatsApp.
- Encerramento: `sair` cancela a cotação ativa, limpa categorias/pausa/seleção e encerra; a próxima mensagem inicia uma triagem nova.
- Evidência: regressões validam numeração, destaque de atendente e `sair` seguido de nova saudação; testes direcionados, `go test -count=1 ./...`, `go vet ./...` e build Wails 0.13.2 passaram. SHA-256: `9804A6AA1E836DE79165536F9AF662EE77E3D07B9EA4909094F22AC5A97F7830`.

## Etapa 26 — Correção do reinício por resposta `1` v0.13.1

- Status: corrigido e validado localmente após teste real do operador.
- Causa: o roteador comercial tratava `1` como comando global para iniciar orçamento, inclusive quando a sessão já aguardava quantidade de quartos ou adultos. O valor `01` avançava apenas porque não correspondia ao comando.
- Correção: `1` só seleciona orçamento durante a triagem; em sessão de cotação ativa, o valor é encaminhado à etapa atual.
- Evidência: teste de regressão reproduz `oi → 1 → 1` e confirma chamadas `orçamento → 1`, sem reinício; testes direcionados, `go test -count=1 ./...`, `go vet ./...` e build Wails 0.13.1 passaram. SHA-256: `1FEFF84725EF21CF0DB0324748F8EC2A5EF7261997A21D145A59576B67FC5074`.

## Etapa 25 — Orçamento multi-quarto e triagem comercial v0.13.0

- Status: implementado, testado e compilado localmente; entrega real no WhatsApp permanece um gate externo.
- Triagem: primeira mensagem e retorno após 24 horas exibem orçamento ou outros assuntos; o handoff pausa o bot e mantém a conversa não lida para o atendente.
- Até seis quartos: período e composição de cada quarto são coletados integralmente antes das consultas; os resultados saem separados, com a configuração em negrito.
- Grupos: acima de seis quartos, o resumo geral é encaminhado ao número configurável do setor de grupos, com padrão `5573988240413`, sem consulta automática à OmniBees.
- Sequência: duas mensagens configuráveis na UI são enviadas depois dos orçamentos e antes dos produtos deduplicados do catálogo associados às categorias disponíveis.
- Evidência: testes direcionados de coleta multi-quarto, grupo, handoff não lido e configuração passaram; `go test -count=1 ./...`, `go vet ./...`, testes Node, Vite e Wails passaram. `Tino.exe` 0.13.0 iniciou responsivo. SHA-256: `91B5D014940FC2AB26BBD930D1BCD3BF42EE5C8198ECE11DC45D6DB860A08740`.
- Limite: falta validar a ordem e a entrega das mensagens, o encaminhamento para grupos e os produtos do catálogo em uma conta WhatsApp autenticada.

## Etapa 24 — Conversas em tempo real v0.12.0

- Status: implementação visual aprovada e integrada ao runtime Wails/Whatsmeow.
- Interface: lista compacta, busca, filtros de não lidas e grupos, nomes sincronizados, conversa ativa, separadores de data, balões, horários, confirmação visual de envio, composer e estado de conexão ao vivo.
- Tempo real: mensagens recebidas e enviadas emitem o JID alterado; a conversa aberta e a lista são reconciliadas imediatamente. Eventos reais de `ChatPresence` exibem “digitando”; uma reconciliação periódica cobre eventos eventualmente perdidos.
- Histórico: nomes de contatos são enriquecidos pelo armazenamento local do Whatsmeow e nomes de conversas/grupos vindos do HistorySync são persistidos.
- Referência aprovada: `docs/design/tino-conversations-approved.png`.
- Evidência: `go test -count=1 ./...`, `go vet ./...`, testes Node, Vite e Wails passaram; `Tino.exe` 0.12.0 abriu responsivo com título correto. SHA-256: `DD5AC04ACEFC652040FFE19D4E5C5D9FF163D47FA037865855D2E48F670AC86B`.
- Limite de evidência: a ferramenta de controle visual não expôs janelas nativas nesta sessão; portanto, a comparação pixel a pixel com o mockup aprovado permanece pendente, embora build e runtime tenham sido confirmados.
- Evidência de release: commit funcional `8dce63e` enviado à `main`; release `v0.12.0` publicada com a interface e a CLI auxiliar.

## Etapa 23 — Paridade com a extensão original OmniBees v0.11.8

- Status: corrigida e validada comparando diretamente `orçamentoominibees/content.js` com o motor do Tino.
- Causa: o Tino interpretava o nome físico do card como filtro exclusivo. A extensão usa cinco posições comerciais para até três ocupantes e reaproveita os cards Família e Triplo como Deluxe vista mar, Deluxe varanda e varanda.
- Correção: o Tino agora replica `getPrices()` e os rótulos da extensão: até 3 ocupantes usam os cinco slots comerciais disponíveis; 4 usam somente os três slots quádruplos; 5 usam somente Família.
- Evidência ao vivo: as cinco URLs fornecidas produziram exatamente 2, 5, 4, 3 e 1 opções, na mesma ordem e com os mesmos rótulos da extensão original.
- Evidência automatizada: testes permanentes cobrem os slots, indisponibilidade, cortesia infantil, variações de nomes e as cinco composições fornecidas.
- Evidência de build: `go test -count=1 ./...`, `go vet ./...`, testes Node, Vite e Wails passaram; o `Tino.exe` 0.11.8 iniciou responsivo. SHA-256: `AE3E5DD5F3F6D036E0B0BAC851C550102058E214DF65EE111CEDAC95E32B5F0D`.
- Evidência de release: commit funcional `9feda2f` enviado à `main`; release `v0.11.8` publicada com a interface e a CLI auxiliar.

## Etapa 22 — Matriz real de ocupação OmniBees v0.11.7

- Status: cinco configurações fornecidas pelo operador foram consultadas ao vivo e convertidas em regressões determinísticas.
- Consultas: dois adultos; três adultos; dois adultos com duas crianças (idades 1/10 e 5/10); três adultos com duas crianças (5/10).
- Resultado: o HTML da OmniBees oferece acomodações maiores em algumas buscas, mas o Tino manteve exatamente duplos para 2, triplos para 3, quádruplos para 4 e família para 5 hóspedes. Cortesia infantil não reduziu a capacidade física.
- Evidência ao vivo: as cinco buscas retornaram categorias e preços; nenhuma terminou vazia no Tino. A virada do ano retornou dois quádruplos, a busca dupla retornou dois duplos, a tripla retornou três triplos, a quádrupla comum retornou três quádruplos e a quíntupla retornou uma família.
- Evidência automatizada: o teste permanente reproduz os nomes reais observados e valida as cinco ocupações; `go test -count=1 ./...` e `go vet ./...` passaram.
- Evidência de build: testes Node e Vite passaram; Wails gerou o executável 0.11.7, iniciado com processo responsivo. SHA-256 do `Tino.exe`: `90DFC01BFA08BB1C4090CB657C19F802E331D32CCC3D4E41F383AE49F74C1B46`.
- Evidência de release: commit funcional `db2f2c7` enviado à `main`; release `v0.11.7` publicada com a interface e a CLI auxiliar.

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

## Etapa 21 — Reconhecimento semântico de categorias v0.11.6

- Status: corrigido e validado localmente e contra consulta real.
- Causa: nomes fora da lista textual fixa, embora contivessem capacidade e características válidas, não eram associados a nenhuma categoria.
- Correção: classificação por capacidade e atributos tolera variações de `Quarto`/`Suíte`/`Apartamento`, mantendo a regra de capacidade exata e escolhendo o menor total entre variações equivalentes.
- Evidência: testes cobrem Duplo Deluxe, Duplo Standard, Triplo Super Deluxe e Quádruplo com Vista para o Mar; consulta real continuou retornando a categoria dupla esperada com preço formatado.
- Evidência de build: `go test -count=1 ./...`, `go vet ./...` e Wails 0.11.6 passaram; SHA-256 do `Tino.exe`: `24319FB96930AD76D018D97CC2C7DD3D5B8078DEABB72E44F907B9AB8AF049E7`.
- Evidência de release: commit funcional `8a7279f` enviado à `main`; release `v0.11.6` publicada com a interface e a CLI auxiliar.
