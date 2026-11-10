import test from "node:test";
import assert from "node:assert/strict";
import { stageLabel, stages } from "./stages.js";

test("funil comercial mantém as oito etapas na ordem", () => {
  assert.deepEqual(stages.map(([id]) => id), ["new", "collecting", "quote_sent", "suite_selected", "pre_reservation", "awaiting_approval", "approved", "human"]);
  assert.equal(stageLabel("approved"), "Aprovado");
});
