# Decisões

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
