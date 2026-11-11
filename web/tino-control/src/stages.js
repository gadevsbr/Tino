export const stages = [
  ["new", "Novo contato"],
  ["collecting", "Coletando dados"],
  ["quote_sent", "Orçamento enviado"],
  ["suite_selected", "Suíte escolhida"],
  ["pre_reservation", "Pré-reserva"],
  ["awaiting_approval", "Aguardando aprovação"],
  ["approved", "Aprovado"],
  ["human", "Humano"]
];

export function stageLabel(value) { return stages.find(([id]) => id === value)?.[1] || value; }
export function isOnline(lastSeen, now = Date.now()) { return lastSeen && now - new Date(lastSeen).getTime() < 90000; }
