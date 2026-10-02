# Decisões

- 2026-10-01 — Aplicados ao atendimento os padrões de Customer Service e Workflow Architecture do pacote Agency Agents: linguagem simples, presença de digitação e atraso fixo de 3 segundos, sem atrasos aleatórios ou imitação enganosa.
- 2026-10-01 — O transporte usa fila FIFO assíncrona por conta e contato; hóspedes distintos executam em paralelo, operadores permanecem serializados por conta e pânico em uma tarefa não bloqueia as seguintes. A reivindicação SQLite anterior ao processamento mantém idempotência persistente.

- 2026-10-01 — Uma falha transitória pode repetir o navegador somente se a execução ainda estiver na etapa de login, antes de informar dados da reserva. Qualquer falha após o login continua sem repetição automática para impedir pré-reservas duplicadas.

- 2026-10-01 — O aviso de pré-reserva ao aprovador resolve primeiro o destino canônico no WhatsApp e inclui telefone do hóspede, período e resumo por quarto; falha de entrega mantém a conversa não lida e gera diagnóstico sem expor o conteúdo do hóspede em log.
- 2026-10-01 — Mensagens de áudio de hóspedes fazem transferência imediata ao humano, pausam a automação e mantêm o chat não lido. A linguagem comercial usa frases curtas, instruções explícitas e vocabulário simples para atender públicos com diferentes níveis de familiaridade digital.

- 2026-10-01 — Superseded: após o evento `success` do QR, o Tino exigia autenticação completa em 20 segundos e exibia falha mesmo quando a credencial já estava salva e a sincronização terminava em seguida.
- 2026-10-01 — O pareamento aguarda até 60 segundos; se o dispositivo já foi persistido, a sincronização pode continuar em segundo plano e o evento `Connected` atualiza o status, sem falso erro nem novo QR.

- 2026-10-01 — Superseded: iniciar a UI carregava a sessão e o motor hoteleiro, mas deixava a sessão WhatsApp persistida desconectada até o operador clicar em conectar.
- 2026-10-01 — O Tino reconecta automaticamente uma sessão WhatsApp persistida durante a inicialização, depois de registrar os handlers do chat e do motor hoteleiro; falhas continuam visíveis na atividade e no status.

- 2026-09-30 — Superseded: o E2E chromedp anterior reproduzia um wizard incompleto, ignorava a etapa de canal de venda e usava o salvamento final para adicionar UH. Seus resultados não provam o fluxo real.
- 2026-09-30 — Scrapling 0.4.15 passa a controlar o navegador na integração Bitz. Investigação autenticada confirmou digitação das datas (fill simples era revertido), IDs duplicados que exigem escopo de modal e `#modal-quarto-reserva #btn-salvar-quarto-reserva` para adicionar UHs.
- 2026-09-30 — Sucesso exige resposta positiva do salvamento com ID/código e estado confirmado, seguida da espera de 20 segundos. Nunca repetir automaticamente o salvamento. Credenciais passam por stdin, sem argumentos, arquivos temporários ou logs.

- 2026-09-30 — O fluxo Bitz tem um teste E2E padrão contra servidor simulado, cobrindo navegador, login, CPF, pesquisa, datas, categoria física, UH, avanços e salvamento sem tocar no Bitz real.
- 2026-09-30 — O teste real fica opt-in e exige a frase `CONFIRMAR_PRE_RESERVA_REAL`, datas e de uma a seis origens físicas; ele usa as credenciais protegidas já salvas e cria uma pré-reserva real.

- 2026-09-30 — Superseded: a matriz herdada oferecia a UH física Família como “Deluxe com vista para o mar” também para até três ocupantes.
- 2026-09-30 — `QUARTO FAMILIA DELUXE COM VISTA MAR` é exclusivo da categoria Família e de ocupação de cinco pessoas. Para até três ocupantes, “Deluxe com vista para o mar” usa a origem física Superluxo.

- 2026-09-30 — Superseded: campos e botões do assistente Bitz eram localizados principalmente por textos/labels, e a tabela de UHs procurava um checkbox inexistente.
- 2026-09-30 — O fluxo usa os IDs reais `reserva_cpf`, `btn-avancar`, `reserva_data_reserva`, `reserva_data_saida`, `btn-add-quarto-reserva`, `table-quartos-disponiveis-reserva` e `btn-salvar-p`. A UH é marcada pelo ícone da primeira célula e a categoria física deriva do `source_key` original da OmniBees.

- 2026-09-30 — Superseded parcialmente: F2 deixa de ser o acionamento principal da nova reserva.
- 2026-09-30 — O seletor estável `#btn-add-reserva`, confirmado pelo operador, passa a ser a ação primária. F2 permanece somente como compatibilidade de fallback.

- 2026-09-30 — Superseded: após o login Bitz, o desaparecimento do botão de entrada era tratado como aplicação pronta e apenas um `F2` era enviado; o atalho podia chegar antes de o sistema instalar seus listeners.
- 2026-09-30 — A abertura de “NOVA RESERVA” aguarda o documento completo, dá foco à página, repete `F2` e usa evento DOM como fallback. Falhas incluem URL e título seguros para diagnóstico, sem credenciais nem conteúdo do hóspede.

- 2026-09-30 — Superseded: o comando `testar atendimento` afirmava abrir sessão separada, mas reutilizava o estado comercial do telefone admitido; se esse contato estivesse pausado, `oi` era consumido sem resposta.
- 2026-09-30 — O teste do operador usa uma identidade sintética isolada por conta e operador. Iniciar ou sair do teste reinicia somente esse estado, sem retomar, cancelar ou alterar a conversa real do hóspede.

- 2026-09-30 — Superseded: manter a ativação do Bitz apenas como subaba de Gestão Assistente tornou a integração difícil de localizar.
- 2026-09-30 — **Integração Bitz** passa a ser uma entrada de primeiro nível no menu lateral e abre diretamente credenciais, teste, status e ativação.

- 2026-09-30 — Superseded: comandos `comercial ativar`, `comercial desativar` e `comercial teste` recriavam a configuração e podiam apagar telefone de grupos e as duas mensagens definidas na UI.
- 2026-09-30 — Comandos de modo alteram exclusivamente modo e allowlist. A tela Bitz usa a ação explícita **Salvar e ativar**; falhas técnicas continuam genéricas para o hóspede e são detalhadas somente ao WhatsApp aprovador.

- 2026-09-30 — Superseded: uma única categoria escolhida sobre a união das opções de todos os quartos podia associar uma acomodação incompatível a outro quarto.
- 2026-09-30 — Cada quarto conserva suas categorias OmniBees e exige uma escolha própria. Somente depois de todas as escolhas nasce uma pré-reserva Bitz idempotente.
- 2026-09-30 — A automação Bitz usa Chromium local controlado pelo Go. A senha é cifrada pelo DPAPI e nunca retorna à UI; o teste de acesso não executa F2 nem grava reserva.
- 2026-09-30 — A criação é serializada e persistida por identificador único. A mensagem final ao hóspede exige confirmação do WhatsApp aprovador configurado por código de uso único.

- 2026-09-30 — Uma categoria apresentada no orçamento é descrita como disponível para o período consultado. Após a escolha, o atendente conclui a reserva; o texto não volta a colocar a disponibilidade em dúvida.

- 2026-09-30 — Ao concluir a cotação, os produtos do catálogo são seguidos por uma lista numerada das categorias e instrução explícita para responder pelo número. `*atendente*` é destacado em negrito e `sair` tem prioridade sobre a seleção, cancela qualquer cotação ativa, limpa o estado e faz a próxima mensagem começar uma nova triagem.

- 2026-09-30 — A opção numérica `1` significa “fazer orçamento” somente enquanto a triagem aguarda uma escolha. Depois que a cotação está ativa, `1` é dado da etapa corrente, como quantidade de quartos ou adultos, e nunca reinicia a sessão.

- 2026-09-30 — A primeira mensagem e a primeira após 24 horas sempre recebem triagem. Qualquer escolha diferente de orçamento transfere a conversa ao humano, pausa o bot persistentemente e força um marcador não lido no Tino.
- 2026-09-30 — Orçamentos de até seis quartos coletam todas as ocupações antes de qualquer consulta à OmniBees; cada resultado é enviado com sua configuração em negrito. Depois seguem duas mensagens configuráveis e, por último, os produtos únicos associados às categorias retornadas.
- 2026-09-30 — Pedidos acima de seis quartos não consultam a OmniBees automaticamente. O bot coleta período, total de adultos, crianças e idades, encaminha o resumo ao telefone de grupos configurado (padrão `5573988240413`) e confirma o encaminhamento ao solicitante.

- 2026-09-30 — A tela de conversas segue a densidade e os padrões de interação familiares do WhatsApp Web, mantendo identidade Tino e sem usar logotipo ou arte proprietária do WhatsApp. Atualização primária ocorre por eventos Wails do Whatsmeow, com reconciliação periódica apenas como proteção contra perda de evento.

- 2026-09-30 — Superseded: usar a capacidade escrita no nome físico como filtro exclusivo para 1–3 hóspedes divergia da extensão original e ocultava alternativas comerciais válidas da OmniBees.
- 2026-09-30 — O orçamento replica a matriz `getPrices()` da extensão original: até 3 ocupantes usam Superluxo, Família como Deluxe vista mar, Triplo Deluxe como Deluxe varanda, Triplo com varanda e a interna adequada; 4 usam apenas os três quádruplos; 5 usam apenas Família. Categorias sem preço continuam omitidas.

- 2026-09-30 — A matriz de capacidade exata passa a ter regressão baseada em cinco consultas reais fornecidas pelo operador, incluindo cortesia infantil e período de Réveillon. A resposta HTML pode conter quartos maiores, mas eles não entram no orçamento se excederem a ocupação física solicitada.

- 2026-09-27 — O motor usa `whatsmeow` e o protocolo WhatsApp Multi-Device/Noise. A solicitação de MTProto foi corrigida porque MTProto pertence ao Telegram.
- 2026-09-27 — Envios em lote exigem consentimento registrado, lista de supressão, limite configurável e intervalo uniforme com jitter operacional. Não há imitação humana, aquecimento de conta ou troca artificial de mensagens para contornar controles da plataforma.
- 2026-09-27 — SQLite puro Go (`modernc.org/sqlite`) foi escolhido para permitir compilação Windows sem CGO. O banco e os dados pessoais permanecem fora do Git.
- 2026-09-27 — Superseded: a decisão inicial de entregar apenas CLI não atendeu à expectativa de software desktop com UI.
- 2026-09-27 — A interface principal passa a ser uma janela Windows nativa (`Tino.exe`) com QR visual, estado da sessão, exportação, seleção de CSV, atividade e controle do fluxo. A CLI permanece apenas como ferramenta auxiliar.
- 2026-09-27 — A UI profissional usa navegação por áreas, Segoe UI, paleta navy/teal, cards claros, estados de sessão e atividade visíveis. A identidade visual é própria e não imita marcas do WhatsApp.
- 2026-09-27 — A skill `imagegen` disponível foi usada somente para criar o símbolo visual bitmap; não havia skill agency-agents de UI instalada. O layout, acessibilidade e comportamento permanecem implementados deterministicamente em Go/Walk.
- 2026-09-27 — Superseded: selecionar um YAML não constitui um flow builder visual.
- 2026-09-27 — O Flow Builder passa a ser operado inteiramente pela UI: regras ordenadas, formulário de condição/resposta, prioridade, resposta padrão, simulação, importação e salvamento. YAML permanece apenas como formato interno portátil.
- 2026-09-27 — O importador aceita CSV telefônico simples (`telefone` e aliases), BOM UTF-8 e delimitadores comuns. A ausência de mensagem é resolvida por um campo na UI; a ausência de consentimento nunca é inferida e exige confirmação explícita antes do envio.
- 2026-09-27 — A home usa o QR apenas durante o pareamento; depois da autenticação, o mesmo espaço mostra conversas e histórico. Mensagens novas e blocos de HistorySync são persistidos em SQLite local, sem prometer recuperação integral do histórico antigo controlado pelo WhatsApp.
- 2026-09-27 — Superseded: considerar `Client.IsConnected()` suficiente para exibir a conta como conectada estava incorreto, pois esse método confirma apenas o socket.
- 2026-09-27 — O estado conectado, a home de conversas e as operações protegidas passam a exigir `Client.IsLoggedIn()`. Sessões locais obsoletas podem ser removidas pela UI para gerar outro QR, preservando o banco de histórico.
- 2026-09-28 — Superseded: a UI Win32/Walk, mesmo tematizada, mantém controles e composição visual com aparência legada e deixa de ser a interface principal.
- 2026-09-28 — A interface principal passa a usar Wails v2.15 com React, Vite e Lucide, embarcada no executável Windows via WebView2. O motor Go, Whatsmeow e os bancos SQLite permanecem locais.
- 2026-09-28 — A Central de Recursos persiste feature flags, perfis e uma raiz de arquivos autorizada em `data/capabilities.json`. Módulos desativados são validados no backend; perfis só serão barreira de segurança após autenticação local.
- 2026-09-28 — As funções do Assistente Paraíso serão migradas como plugins de domínio. Bancos, sessões, `.env`, comprovantes e dados reais não serão copiados; recursos ainda não integrados aparecem como planejados e não podem ser ativados.
- 2026-09-28 — O contrato React da Central de Recursos usa os nomes JSON `modules`, `roles` e `workspaceRoot`. A normalização fica isolada e testada; falhas de carregamento devem mostrar recuperação visível, nunca uma página branca.
- 2026-09-28 — O Tino passa a ser o runtime canônico das funções do Assistente Paraíso. O transporte operacional é anexado ao mesmo cliente `whatsmeow`, sem segundo QR ou banco de sessão; somente o banco de domínio fica separado em `data/operations/hotel.db`.
- 2026-09-28 — Operadores de comandos internos são cadastrados pela UI com telefone e DDI. Sessões, segredos e dados reais do projeto de origem não são migrados.
- 2026-09-28 — Superseded: a integração deixou de ser apenas um inventário planejado. O núcleo operacional está integrado, mas os cards continuam planejados até existirem telas administrativas e bloqueio seletivo completos.
- 2026-09-28 — Superseded parcialmente: caixa, comprovantes, extratos e vales deixam de ser apenas planejados. Eles passam a ter uma área operacional comum com filtros por período; comercial, catálogo, quartos e backup continuam aguardando painéis completos.
- 2026-09-28 — PDFs de quartos, caixa e vales podem ser salvos localmente ou enviados pela mesma sessão WhatsApp para telefone com DDI ou JID de grupo. O envio continua condicionado a uma sessão autenticada.
- 2026-09-28 — Superseded: quartos, comercial, catálogo e backup deixam de aguardar painéis. A UI usa os mesmos repositórios do transporte WhatsApp; não existe estado administrativo paralelo.
- 2026-09-28 — O atendimento comercial tem três modos persistentes: desativado, teste por allowlist e público. A captura de produto continua no WhatsApp Business porque depende de uma mensagem de produto real; a UI consulta e remove os vínculos resultantes.
- 2026-09-28 — Relatórios diários de caixa em PDF só entram no banco quando `saldo inicial + entradas - saídas = saldo final`. O hash e a data impedem duplicidade; arquivos divergentes permanecem fora do caixa e aparecem como revisão na UI. O PDF original importado é preservado no diretório operacional privado.
- 2026-09-28 — Em dashboards com vários dias, saldos de abertura não são somados: o período mostra a abertura do primeiro dia, entradas e saídas acumuladas e o fechamento do último dia. O envio do relatório de quartos inclui listas numéricas separadas para sujos e limpos/desforrados.
- 2026-09-28 — A identificação de categorias OmniBees tolera variações semânticas de nomes (`Quarto`/`Suíte`, `Super Luxo`/`Superluxo`, conectores e vista para o mar), mantendo preços e categorias estruturados e sem inferência a partir do texto final.
- 2026-09-28 — Superseded: ignorar toda mensagem de grupo impedia o titular e operadores autorizados de administrar o bot no destino em que os relatórios são usados.
- 2026-09-28 — Grupos aceitam somente comandos do titular da conta ou de operadores autorizados; o atendimento automático de hóspedes não roda em grupos. `relatorio caixa em pdf` sem data representa o mês corrente até hoje, alinhado ao dashboard financeiro.
- 2026-09-28 — Superseded: filtrar categorias OmniBees por uma capacidade presumida localmente descartava acomodações maiores que o motor de reservas oferecia legitimamente para menos hóspedes.
- 2026-09-28 — A resposta real da OmniBees é a fonte de disponibilidade: toda categoria visível com total confirmado entra no orçamento, preservando a acomodação física e o menor total quando houver tarifas repetidas.
- 2026-09-28 — Superseded: incluir toda acomodação devolvida no HTML da OmniBees oferecia quartos maiores sem necessidade e contrariava a regra comercial definida pelo usuário.
- 2026-09-28 — O orçamento cruza disponibilidade real com capacidade física exata: 1–2 pessoas recebem apenas duplos, 3 apenas triplos, 4 apenas quádruplos e 5 apenas família. Cortesia infantil altera a cobrança, não a ocupação física.
- 2026-09-28 — Valores monetários do orçamento OmniBees usam o padrão brasileiro completo, com ponto de milhar e vírgula decimal (`R$ 2.504,20`).
- 2026-09-30 — Superseded: casar categorias apenas contra uma lista fixa de nomes podia eliminar toda a disponibilidade quando a OmniBees alterava rótulos.
- 2026-09-30 — Categorias OmniBees são classificadas semanticamente por capacidade e atributos (`duplo`, `triplo`, `quádruplo`, `família`, `deluxe`, `varanda`, `vista mar`, `superluxo`). A capacidade exata continua obrigatória.
