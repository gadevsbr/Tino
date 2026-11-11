import test from "node:test";
import assert from "node:assert/strict";
import { isOnline, stageLabel, stages } from "./stages.js";

test("funil comercial mantém as oito etapas na ordem", () => {
  assert.deepEqual(stages.map(([id]) => id), ["new", "collecting", "quote_sent", "suite_selected", "pre_reservation", "awaiting_approval", "approved", "human"]);
  assert.equal(stageLabel("approved"), "Aprovado");
});

test("indicador online tolera heartbeat de um minuto", () => {
  const now = Date.parse("2026-11-11T20:01:00.000Z");
  assert.equal(isOnline("2026-11-11T20:00:00.000Z", now), true);
  assert.equal(isOnline("2026-11-11T19:59:29.000Z", now), false);
});
