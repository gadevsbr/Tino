# Funções encontradas no Assistente Paraíso

Inventário para decidir como cada capacidade deverá funcionar no Tino. A presença nesta lista não significa que a função já foi integrada.

## 1. Conexão, segurança e processamento

- Sessão persistente WhatsApp/Whatsmeow e pareamento por QR ou código.
- Whitelist de operadores e autorização dinâmica de novos números.
- Comandos pelo chat pessoal “Você”, ignorando respostas produzidas pelo próprio bot.
- Processamento privado, fila sequencial por remetente, idempotência e bloqueio de grupos/eventos de controle.

## 2. Quartos e limpeza

- Cadastro fixo de 42 quartos e consulta individual.
- Estados: disponível/limpo, limpo desforrado, limpeza, entrada, saída/entrada, saída do dia, ocupado e interditado.
- Atualização individual, atualização de vários quartos e atualização guiada de todos.
- Categoria física do Bitz configurável por quarto na UI; o status `INTERDITADO` bloqueia a UH na pré-reserva automática.
- Quantidade de hóspedes para entrada/ocupação.
- Pausar, continuar e pular durante atualizações guiadas.
- Listas por estado/cor.
- Observações por quarto, remoção de observação e histórico.
- Confirmação de limpeza concluída e notificação aos outros operadores.

## 3. Relatórios operacionais

- PDF da situação dos quartos com resumo por estado e hóspedes.
- Escolha de relatório de quartos, caixa ou ambos.
- Envio para grupos limitado aos relatórios de quartos e caixa, solicitado por titular/operadores; demais comandos, atendimento e documentos ficam restritos ao privado.

## 4. Caixa

### Boletos a pagar (integrado ao Tino v0.15.14)

- Cadastro manual na UI e por comando administrativo no WhatsApp.
- Vencimento, valor, pendentes, pagos e cancelados.
- Lembretes diários configuráveis no WhatsApp, inclusive após vencimento, encerrados na baixa/cancelamento.
- Agendamento local com registro persistente de avisos; exige Tino aberto e conectado. Não realiza pagamentos nem altera o caixa.

- Abrir caixa por data e editar o valor de abertura.
- Registrar entrada em dinheiro, PIX ou cartão.
- Registrar saída em dinheiro.
- Consultar caixa do dia, data específica, semana e mês.
- Listar movimentos com identificadores.
- Editar movimento com auditoria.
- Excluir movimento com confirmação persistente.
- Fechar e reabrir caixa com trilha de auditoria.
- PDF do caixa atual, de uma data ou de um intervalo.

## 5. Comprovantes e OCR

- Receber imagem com legenda autorizadora ou PDF bancário com camada de texto.
- OCR local para imagens.
- Extrair valor, data e método e pedir revisão/correção do operador.
- Impedir duplicidade por hash.
- Guardar o original em pasta privada e vinculá-lo ao movimento.
- Listar comprovantes por data e recuperar o arquivo pelo identificador.

## 6. Orçamentos OmniBees

- Consulta guiada de check-in, check-out, adultos, crianças e idades.
- Leitura de link completo de resultados da OmniBees.
- Desconto opcional de 1% a 8% no fluxo por link.
- Regras de cortesia infantil e ocupação física.
- Uso apenas de preços totais reais retornados pela OmniBees.
- Recusa de hotel incorreto, parâmetros incompletos ou resultados sem preços.

## 7. Atendimento comercial

- Modos desativado, teste restrito a um número e público.
- Menu inicial e nova saudação após 24 horas de inatividade.
- Orçamento guiado integrado à OmniBees.
- Triagem entre orçamento e outros assuntos; o segundo caminho pausa a automação e mantém o chat não lido para atendimento humano.
- Até seis quartos: coleta todas as ocupações, incluindo idades das crianças, antes de consultar e enviar um orçamento destacado para cada quarto.
- Acima de seis quartos: coleta período e ocupação geral, encaminha ao WhatsApp configurado do setor de grupos e confirma o encaminhamento ao solicitante.
- Duas mensagens finais editáveis na UI são enviadas depois dos orçamentos e antes dos produtos de catálogo associados às categorias disponíveis.
- Após os produtos, o bot envia uma lista numerada das categorias e pede a escolha pelo número; `*atendente*` aparece destacado e `sair` encerra e limpa a sessão comercial.
- Para até seis quartos, cada quarto recebe sua própria escolha numerada de categoria disponível.
- Depois de todas as escolhas, a integração Bitz cria a pré-reserva com o CPF operacional configurado, seleciona uma UH disponível por quarto e avisa o WhatsApp aprovador.
- A seleção cruza disponibilidade do Bitz com categoria e status locais, usa outra UH da mesma categoria quando necessário e nunca reutiliza a mesma UH na pré-reserva.
- O aviso ao aprovador usa o destino canônico consultado no WhatsApp e inclui número do hóspede, período, categoria, adultos e idades das crianças de cada quarto.
- Somente o aprovador configurado pode confirmar com `aprovar pre-reserva CODIGO`; então a mensagem final definida na UI é enviada ao hóspede.
- Áudios de hóspedes não são interpretados: pausam o bot, deixam a conversa não lida e encaminham diretamente para atendimento humano.
- Respostas automáticas comerciais usam `digitando...` e espera fixa de 3 segundos; mensagens do mesmo contato seguem uma fila FIFO, enquanto contatos diferentes são processados em paralelo.
- A idempotência é persistente: a mesma mensagem do WhatsApp não altera o estado nem dispara resposta, orçamento ou pré-reserva duas vezes.
- Encaminhamento para atendimento humano.
- Pausa persistente da automação por contato e retomada explícita.
- Teste do atendimento usando o número do operador.

## 8. Catálogo WhatsApp Business

- Cadastro assistido compartilhando um produto real do catálogo.
- Extração de ProductID, proprietário, foto e categoria.
- Confirmação, substituição e limpeza de vínculos.
- Status do catálogo e teste individual de produto.
- Dados isolados por conta WhatsApp.
- Armazenamento privado da imagem e novo upload para envio.
- Cards apenas para categorias disponíveis no orçamento, com deduplicação e thumbnail.

## 9. Extratos e conciliação

- Receber PDFs do Bitz e uma planilha XLSX modelo.
- Cruzar quarto, hóspede e datas.
- Separar dinheiro, PIX e cartão; calcular pacote e consumo.
- Marcar divergências como `CONFERIR` sem lançar automaticamente no caixa.
- Criar planilha semanal sem XLSX inicial.
- Acumular novos PDFs durante a semana e ignorar duplicados por SHA-256.
- Gerar planilha completa, aba de resumo e auditoria JSON.
- Limites de tamanho/quantidade e cancelamento do envio temporário.

## 10. Vales de funcionários

- Cadastro guiado de funcionário, valor e observação.
- Opção de abater o vale do caixa em uma transação atômica.
- Consulta mensal e por funcionário.
- Relatório textual e PDF mensal.

## 11. Backup, saúde e agendamento

- Backup ZIP diário com banco verificado e comprovantes, sem `.env` ou sessão WhatsApp.
- Retenção configurável de backups, relatórios e logs.
- Backup manual e consulta do estado do backup.
- Resumo automático de quartos e caixa às 17h.
- Idempotência dos agendamentos após reinício.
- Saúde e diagnóstico do bot.

## 12. Operação do dispositivo Termux

- Serviço em segundo plano, wake lock e Termux:Boot.
- Comandos de iniciar, parar, consultar estado e logs.
- Atualização pública com SHA-256, self-test, backup prévio e rollback.
- Reinstalação limpa com confirmação literal e QR automático.
- Distribuição separada para ARM64 e ARMv7.

## 13. Componentes auxiliares

- Extensão de navegador para apoiar orçamento OmniBees.
- Aplicativo separado de extratos usado como referência de layout/processamento.
- PDFs locais de quartos, caixa e vales.

## Perguntas para cada módulo

Para definir a versão do Tino, responder para cada item:

1. Deve existir no Tino?
2. Qual perfil pode visualizar, operar, configurar e excluir?
3. A ação acontece pela UI, WhatsApp ou pelos dois?
4. Quais campos, confirmações e telas são necessários?
5. Quais arquivos pode ler e onde pode gravar?
6. Precisa funcionar localmente, no Moto/Termux ou em ambos?
7. Qual evento deve entrar na auditoria?
