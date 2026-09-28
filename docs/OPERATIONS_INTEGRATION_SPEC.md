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

## Gates de validação

- Testes automatizados e build provam integração local.
- Comandos reais, download/upload de mídia, OCR e entrega de PDFs exigem pareamento com conta autorizada.
- Cotação OmniBees e catálogo exigem validação dos provedores e configurações externas.
