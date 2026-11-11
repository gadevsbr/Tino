import test from "node:test";
import assert from "node:assert/strict";
import worker, { constantTimeEqual, signSession, snapshotHash, syncPlan, verifySession } from "./src.js";

test("tokens exigem igualdade integral", () => {
  assert.equal(constantTimeEqual("segredo", "segredo"), true);
  assert.equal(constantTimeEqual("segredo", "segred0"), false);
  assert.equal(constantTimeEqual("", ""), false);
  assert.equal(constantTimeEqual("curto", "comprido"), false);
});

test("CORS aceita somente a origem configurada", async () => {
  const env = { DASHBOARD_ORIGIN: "https://painel.example" };
  const allowed = await worker.fetch(new Request("https://api.example/api/health", { headers: { origin: env.DASHBOARD_ORIGIN } }), env);
  assert.equal(allowed.headers.get("access-control-allow-origin"), env.DASHBOARD_ORIGIN);
  const denied = await worker.fetch(new Request("https://api.example/api/health", { headers: { origin: "https://evil.example" } }), env);
  assert.equal(denied.headers.get("access-control-allow-origin"), null);
});

test("sessão assinada respeita papel e rejeita adulteração", async () => {
  const token = await signSession("segredo-longo-de-teste", "proprietaria", "dashboard", 60);
  assert.equal((await verifySession("segredo-longo-de-teste", token, "dashboard")).sub, "proprietaria");
  assert.equal(await verifySession("segredo-longo-de-teste", token, "bridge"), null);
  assert.equal(await verifySession("outro-segredo", token, "dashboard"), null);
  assert.equal(await verifySession("segredo-longo-de-teste", `${token}x`, "dashboard"), null);
});

test("snapshot idêntico não grava e heartbeat ocorre uma vez por minuto", () => {
  const hash = "a".repeat(64), now = "2026-11-11T20:01:00.000Z";
  assert.deepEqual(syncPlan({ snapshot_hash: hash, last_seen: "2026-11-11T20:00:30.000Z" }, hash, now), { changed: false, heartbeat: false });
  assert.deepEqual(syncPlan({ snapshot_hash: hash, last_seen: "2026-11-11T20:00:00.000Z" }, hash, now), { changed: false, heartbeat: true });
  assert.deepEqual(syncPlan({ snapshot_hash: "b".repeat(64), last_seen: now }, hash, now), { changed: true, heartbeat: true });
});

test("cliente antigo recebe hash estável sem considerar horário online", async () => {
  const base = { version: "0.17.7", contacts: [{ Contact: "1" }], failures: [] };
  assert.equal(await snapshotHash({ ...base, onlineAt: "2026-11-11T20:00:00Z" }), await snapshotHash({ ...base, onlineAt: "2026-11-11T20:01:00Z" }));
});
