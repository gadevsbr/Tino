import test from "node:test";
import assert from "node:assert/strict";
import { healthAlerts } from "./health.js";

test("saúde reúne somente itens que exigem ação", () => {
  const alerts = healthAlerts({
    whatsapp: { state: "ok" }, omnibees: { state: "error", label: "Falha recente" }, bitz: { state: "unknown" },
    stalledContacts: 2, awaitingApproval: 1, pendingProofs: 0, actionErrors: 3
  }, true);
  assert.deepEqual(alerts, ["OmniBees: Falha recente", "2 atendimento(s) sem avanço há mais de 1 hora", "1 reserva(s) aguardando aprovação", "3 erro(s) registrado(s) nas últimas 24 horas"]);
  assert.deepEqual(healthAlerts({}, false), ["Tino sem sincronização recente"]);
});
