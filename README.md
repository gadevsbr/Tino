# Tino

Aplicação desktop Windows com interface gráfica nativa que mantém uma sessão WhatsApp Multi-Device com `whatsmeow`, audita contatos/grupos, processa notificações consentidas e responde mensagens por regras declarativas. Uma CLI auxiliar também acompanha o projeto.

> `whatsmeow` não usa MTProto. MTProto é do Telegram; o WhatsApp Multi-Device usa Noise e protocolos próprios. O projeto também não implementa aquecimento artificial de conta nem atrasos para imitar pessoas ou contornar controles. Os intervalos existem como limitação operacional explícita e não garantem aceitação pela plataforma.

## Requisitos e limites

- Go 1.26 ou o executável pronto em `dist/`.
- Uma conta autorizada e uso em conformidade com os termos do WhatsApp e a legislação aplicável.
- O contato precisa ter consentido; o CSV exige `consent=true`.
- `whatsmeow` é um cliente não oficial. Para campanhas e uso empresarial em produção, prefira a WhatsApp Business Platform oficial.
- A exportação individual contém o cache de contatos sincronizado localmente, não um histórico completo de conversas.

## Estrutura

```text
cmd/tino-ui/        aplicação desktop e recursos incorporados
cmd/tino/           CLI auxiliar
internal/session/   sessão SQLite, reconexão e QR
internal/audit/     coleta e exportação JSON/CSV
internal/batch/     leitura CSV, limites e envio sequencial
internal/flow/      roteamento de mensagens por templates
config/             configuração e regras de exemplo
scripts/            build reproduzível do executável
project-memory/     decisões e evidências curtas
```

## Uso

```powershell
go test ./...
.\scripts\build.ps1 -Version 0.2.0
.\dist\Tino.exe

# CLI auxiliar
.\dist\tino-cli-windows-amd64.exe login
.\dist\tino-cli-windows-amd64.exe status
```

No aplicativo desktop, o QR aparece dentro da janela. A sessão fica em `data/sessions/<profile>.db`, excluída do Git. Exportações com dados pessoais ficam em `exports/`, também excluídas.

## CSV de notificações

Cabeçalho obrigatório: `phone,message,consent`; `name` é opcional. O telefone deve incluir DDI. Linhas sem consentimento são registradas como `skipped_no_consent`. A saída JSON por linha contém o status e, quando enviado, o ID da mensagem.

## Fluxos

Edite `config/flows.yaml`. As regras são avaliadas em ordem e a primeira correspondência vence. Apenas conversas individuais recebidas são respondidas; grupos e mensagens do próprio usuário são ignorados. O cache em memória evita resposta duplicada durante o processo atual.

## Evidência de entrega

Build e testes locais validam apenas código/artefato. Pareamento QR, estabilidade prolongada, entrega real, políticas do provedor e comportamento em uma máquina Windows diferente exigem validação externa separada.
