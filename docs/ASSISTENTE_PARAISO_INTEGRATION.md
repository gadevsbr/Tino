# Integração do Assistente Paraíso ao Tino

## Conclusão

A integração é tecnicamente viável, mas deve ocorrer por módulos de domínio, não pela cópia integral do bot. Os dois projetos usam Go, Whatsmeow e SQLite, porém o Assistente Paraíso foi desenhado para um hotel específico, execução em Termux e comandos por WhatsApp. O Tino é um desktop multiuso com interface gráfica.

## Capacidades mapeadas

| Capacidade | Reuso recomendado | Estado no Tino |
| --- | --- | --- |
| Atendimento comercial, catálogo e handoff humano | Extrair serviços independentes da conta e expor configuração/teste na UI | Planejado |
| Orçamento OmniBees | Adaptador opcional com teste e credenciais fora do banco principal | Planejado |
| Quartos, limpeza e ocupação | Plugin vertical “Hotel”, com banco e telas próprios | Planejado |
| Caixa, comprovantes e OCR | Plugin financeiro com trilha de auditoria e confirmação forte | Planejado |
| Extratos e planilhas | Job assíncrono com upload, revisão e download do resultado | Planejado |
| Vales e relatórios PDF | Módulo de RH restrito a perfis autorizados | Planejado |
| Backup, saúde e retenção | Serviço comum da plataforma, sem incluir sessão ou segredos | Planejado |
| Autorização por número | Migrar para identidade, perfis e permissões granulares | Fundação criada |

## Arquitetura proposta

1. `core`: sessão WhatsApp, eventos, arquivos autorizados, auditoria e jobs.
2. `capabilities`: registro persistente de recursos e feature flags.
3. `identity`: usuários locais, autenticação e perfis; a configuração de papéis criada na v0.7.0 ainda não substitui login.
4. `plugins/hotel`: quartos, caixa, vales, comprovantes, extratos e relatórios.
5. `plugins/commercial`: atendimento, OmniBees, catálogo e handoff.
6. `files`: acesso somente dentro de raízes explicitamente autorizadas, com extensão/tamanho permitidos e log de cada operação.

## Regras de segurança

- Não importar `.env`, sessões WhatsApp, bancos, comprovantes, logs ou backups do Assistente Paraíso.
- Não oferecer acesso arbitrário ao disco; cada plugin recebe apenas uma pasta autorizada.
- Operações financeiras, exclusões e ativação pública exigem confirmação e auditoria.
- Modo comercial permanece desativado por padrão e passa primeiro por teste restrito.
- Perfis na UI só se tornam controle de segurança após autenticação local e sessão de usuário.
- Validar catálogo, OmniBees, OCR e envio real separadamente em uma conta/dispositivo autorizado.

## Sequência recomendada

1. Autenticação local e enforcement de perfis.
2. Sandbox de arquivos e trilha de auditoria.
3. Plugin comercial em modo de teste.
4. Plugin de quartos e relatórios.
5. Caixa/OCR/vales com revisão humana obrigatória.
6. Backup e operação remota do dispositivo Termux.
