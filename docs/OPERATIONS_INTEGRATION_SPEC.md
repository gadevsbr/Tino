# Operação do Assistente no Tino

## Contrato definido

- O Tino é o runtime canônico e usa uma única sessão WhatsApp.
- Operadores autorizados administram quartos, relatórios, caixa, comprovantes, extratos, vales e backup pelo WhatsApp.
- Clientes externos entram no atendimento comercial, cotação OmniBees e catálogo.
- A UI deve oferecer gestão de PDFs, compartilhamento, dashboard de caixa por período, consulta de comprovantes e controles dos módulos comercial, catálogo, extratos, vales e backup.
- Recursos originalmente dependentes do Termux tornam-se inicialização, atualização, backup, saúde e recuperação nativos do Windows.
- Sessões, `.env`, bancos, comprovantes, backups, logs e dados reais do Assistente não são copiados.

## Estado de entrega

O motor de domínio e o transporte completo foram migrados e anexados ao cliente `whatsmeow` existente. A Central de Recursos permite cadastrar operadores. A área **Operação financeira** oferece dashboard por período, geração e compartilhamento de PDFs, consulta visual de comprovantes e painéis de extratos e vales. **Gestão Assistente** administra quartos, modo comercial, catálogo, backup e saúde sobre as mesmas tabelas e regras usadas pelos comandos do WhatsApp.

### Importação de relatórios de caixa

O Tino aceita seleção múltipla dos PDFs diários, reconhece os layouts legado e atual e mostra uma prévia antes de gravar. Apenas dias conciliados são importados. SHA-256 e data operacional impedem repetição, e o documento original é guardado em `data/operations/caixa-importados/`. Divergências ficam visíveis como **Revisar** e não alteram o banco.

O dashboard financeiro usa o primeiro saldo de abertura do período e o saldo final do último dia. Depois da importação, a UI seleciona o intervalo importado e atualiza os indicadores automaticamente.

Ao enviar a situação dos quartos por WhatsApp, o resumo textual que acompanha o PDF lista separadamente os números dos quartos **sujos/para limpar** e dos quartos **limpos, mas desforrados**.

## Gates de validação

- Testes automatizados e build provam integração local.
- Comandos reais, download/upload de mídia, OCR e entrega de PDFs exigem pareamento com conta autorizada.
- Cotação OmniBees e catálogo exigem validação dos provedores e configurações externas.
- O fluxo comercial multi-quarto está implementado localmente; o encaminhamento real ao setor de grupos, a sequência das mensagens e os produtos do catálogo ainda exigem validação em uma conta WhatsApp autenticada.
- A automação Bitz, a proteção local de credenciais, a idempotência e a aprovação final têm cobertura local. Login, seletores autenticados e criação de uma pré-reserva real permanecem gate externo até as credenciais serem configuradas e o teste controlado ser executado.
