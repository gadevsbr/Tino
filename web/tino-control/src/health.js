export function healthAlerts(health = {}, online = false) {
  const alerts = [];
  if (!online) alerts.push("Tino sem sincronização recente");
  for (const [name, signal] of [["WhatsApp", health.whatsapp], ["OmniBees", health.omnibees], ["Bitz", health.bitz]]) {
    if (signal?.state === "error") alerts.push(`${name}: ${signal.label || "requer atenção"}`);
  }
  if (health.stalledContacts) alerts.push(`${health.stalledContacts} atendimento(s) sem avanço há mais de 1 hora`);
  if (health.awaitingApproval) alerts.push(`${health.awaitingApproval} reserva(s) aguardando aprovação`);
  if (health.pendingProofs) alerts.push(`${health.pendingProofs} comprovante(s) pendente(s)`);
  if (health.actionErrors) alerts.push(`${health.actionErrors} erro(s) registrado(s) nas últimas 24 horas`);
  return alerts;
}
