import test from "node:test";
import assert from "node:assert/strict";
import worker, { constantTimeEqual } from "./src.js";

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
