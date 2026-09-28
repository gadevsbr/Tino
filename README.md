# Tino

Aplicação desktop Windows com interface moderna React/Wails que mantém uma sessão WhatsApp Multi-Device com `whatsmeow`, apresenta conversas e histórico local, audita contatos/grupos, processa notificações consentidas e responde mensagens por regras declarativas. Uma CLI auxiliar também acompanha o projeto.

A interface utiliza identidade visual própria, navegação lateral, cards responsivos, ícones Lucide, estados operacionais visíveis e ícone incorporado ao executável. O React é embarcado no `Tino.exe`; nenhum servidor web precisa permanecer ativo.

> `whatsmeow` não usa MTProto. MTProto é do Telegram; o WhatsApp Multi-Device usa Noise e protocolos próprios. O projeto também não implementa aquecimento artificial de conta nem atrasos para imitar pessoas ou contornar controles. Os intervalos existem como limitação operacional explícita e não garantem aceitação pela plataforma.

## Requisitos e limites

- Go 1.26 e Node.js apenas para desenvolvimento, ou o executável pronto em `dist/`.
- Microsoft WebView2 Runtime, já presente por padrão no Windows 11 e na maioria das instalações atuais do Windows 10.
- Uma conta autorizada e uso em conformidade com os termos do WhatsApp e a legislação aplicável.
- O contato precisa ter consentido; o CSV exige `consent=true`.
- `whatsmeow` é um cliente não oficial. Para campanhas e uso empresarial em produção, prefira a WhatsApp Business Platform oficial.
- A exportação individual contém o cache de contatos sincronizado localmente, não um histórico completo de conversas.

## Estrutura

```text
cmd/tino-modern/    aplicação Wails, ponte Go e frontend React
cmd/tino-ui/        interface Win32 anterior mantida como referência temporária
cmd/tino/           CLI auxiliar
internal/session/   sessão SQLite, reconexão e QR
internal/audit/     coleta e exportação JSON/CSV
internal/batch/     leitura CSV, limites e envio sequencial
internal/chat/      histórico local e consultas de conversas
internal/flow/      roteamento de mensagens por templates
internal/assistente/ domínio operacional migrado: hotel, caixa e comercial
config/             configuração e regras de exemplo
scripts/            build reproduzível do executável
project-memory/     decisões e evidências curtas
```

## Uso

```powershell
go test ./...
.\scripts\build.ps1 -Version 0.7.0
.\dist\Tino.exe

# CLI auxiliar
.\dist\tino-cli-windows-amd64.exe login
.\dist\tino-cli-windows-amd64.exe status
```

No aplicativo desktop, o QR aparece na home somente durante o pareamento. Após autenticar, a mesma área exibe a lista de chats, pesquisa, histórico e resposta manual. A sessão e o histórico ficam em `data/`, excluídos do Git. Exportações com dados pessoais ficam em `exports/`, também excluídas.

O estado **conectada e autenticada** somente é exibido depois que o WhatsApp confirma a autenticação da sessão. Se o aparelho não listar o Tino em **Dispositivos conectados**, use **Gerar novo QR Code** (ou **Trocar conta**) para remover apenas as credenciais locais e fazer um novo pareamento; o histórico local é preservado.

O Tino armazena as novas mensagens localmente e importa os blocos de histórico recebidos durante a sincronização. A quantidade de mensagens antigas disponibilizada é controlada pelo WhatsApp; portanto, o aplicativo não garante recuperar todo o passado existente no celular.

## CSV de notificações

O importador aceita listas simples contendo somente `telefone`, inclusive arquivos UTF-8 com BOM e valores como `"+5511999999999"`. Também reconhece os cabeçalhos `phone`, `celular`, `whatsapp`, `numero` e `numero_telefone`, com separador vírgula, ponto e vírgula ou tabulação.

Quando o CSV não contém `message`/`mensagem`, a mensagem é preenchida na própria interface. Quando não contém `consent`/`consentimento`, o envio exige uma confirmação explícita de que os contatos autorizaram a comunicação. O telefone deve incluir DDI.

## Flow Builder visual

Na aba **Flow Builder**, use **Nova regra** para definir a condição e a resposta sem editar arquivos. As regras podem ser editadas, excluídas e movidas para cima ou para baixo; a primeira correspondência vence. O simulador mostra a resposta antes da ativação. A resposta padrão é usada quando nenhuma regra combina.

O fluxo continua persistido em YAML para portabilidade e backup, mas o arquivo não precisa ser manipulado manualmente. Apenas conversas individuais recebidas são respondidas; grupos e mensagens do próprio usuário são ignorados.

## Central de recursos

A tela **Central de recursos** permite ativar ou desativar módulos disponíveis, preparar permissões por perfil e escolher uma pasta explicitamente autorizada. As configurações ficam em `data/capabilities.json`, fora do Git. Recursos desativados são recusados também pelo backend, não apenas ocultados visualmente.

As funções mapeadas do Assistente Paraíso aparecem como **Planejado** e não podem ser ativadas antes de uma migração segura. A análise, os limites e a sequência proposta estão em [`docs/ASSISTENTE_PARAISO_INTEGRATION.md`](docs/ASSISTENTE_PARAISO_INTEGRATION.md). A configuração atual de perfis é a fundação da autorização; ela ainda não substitui autenticação local por usuário.

O núcleo operacional do Assistente já está anexado à mesma sessão WhatsApp do Tino. Cadastre telefones com DDI em **Central de recursos > Operadores do WhatsApp**. Os cards especializados continuam como planejados enquanto as respectivas telas e os controles seletivos não estiverem concluídos; veja [`docs/OPERATIONS_INTEGRATION_SPEC.md`](docs/OPERATIONS_INTEGRATION_SPEC.md).

Em **Operação financeira**, o dashboard consulta dia, sete dias, mês ou intervalo personalizado. A mesma área salva e compartilha PDFs de quartos, caixa e vales, apresenta os comprovantes originais e lista semanas de extratos e vales por funcionário.

## Evidência de entrega

Build e testes locais validam apenas código/artefato. Pareamento QR, estabilidade prolongada, entrega real, políticas do provedor e comportamento em uma máquina Windows diferente exigem validação externa separada.
