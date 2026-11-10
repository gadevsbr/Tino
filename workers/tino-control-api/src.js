const JSON_HEADERS = { "content-type": "application/json; charset=utf-8", "cache-control": "no-store" };
const ACTIONS = new Set(["pause", "resume", "takeover", "restart_quote"]);
const encoder = new TextEncoder();

function json(data, status = 200) {
  return new Response(JSON.stringify(data), { status, headers: JSON_HEADERS });
}

function cors(response, request, env) {
  const origin = request.headers.get("origin") || "";
  const allowed = origin && (origin === env.DASHBOARD_ORIGIN || /^http:\/\/(localhost|127\.0\.0\.1)(:\d+)?$/.test(origin));
  if (!allowed) return response;
  const headers = new Headers(response.headers);
  headers.set("access-control-allow-origin", origin);
  headers.set("access-control-allow-headers", "authorization,content-type");
  headers.set("access-control-allow-methods", "GET,POST,OPTIONS");
  headers.set("vary", "Origin");
  return new Response(response.body, { status: response.status, headers });
}

function bearer(request) {
  const value = request.headers.get("authorization") || "";
  return value.startsWith("Bearer ") ? value.slice(7) : "";
}

function constantTimeEqual(left, right) {
  if (!left || !right || left.length !== right.length) return false;
  let value = 0;
  for (let i = 0; i < left.length; i++) value |= left.charCodeAt(i) ^ right.charCodeAt(i);
  return value === 0;
}

async function body(request) {
  const length = Number(request.headers.get("content-length") || 0);
  if (length > 1_000_000) throw new Error("payload muito grande");
  return request.json();
}

function clean(value, max = 500) {
  return String(value || "").trim().slice(0, max);
}

function requireToken(request, token) {
  return constantTimeEqual(bearer(request), token);
}

function base64url(bytes) {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/=/g, "").replace(/\+/g, "-").replace(/\//g, "_");
}

function unbase64url(value) {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(normalized + "=".repeat((4 - normalized.length % 4) % 4));
  return Uint8Array.from(binary, character => character.charCodeAt(0));
}

async function sessionKey(secret) {
  return crypto.subtle.importKey("raw", encoder.encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign", "verify"]);
}

async function signSession(secret, username, role, lifetimeSeconds) {
  const payload = base64url(encoder.encode(JSON.stringify({ sub: username, role, exp: Math.floor(Date.now() / 1000) + lifetimeSeconds })));
  const signature = await crypto.subtle.sign("HMAC", await sessionKey(secret), encoder.encode(payload));
  return `${payload}.${base64url(new Uint8Array(signature))}`;
}

async function verifySession(secret, token, role) {
  try {
    const [payload, signature, extra] = token.split(".");
    if (!payload || !signature || extra) return null;
    const valid = await crypto.subtle.verify("HMAC", await sessionKey(secret), unbase64url(signature), encoder.encode(payload));
    if (!valid) return null;
    const session = JSON.parse(new TextDecoder().decode(unbase64url(payload)));
    return session.role === role && session.exp > Math.floor(Date.now() / 1000) ? session : null;
  } catch {
    return null;
  }
}

async function authorizeBridge(request, env) {
  return requireToken(request, env.BRIDGE_TOKEN) || Boolean(await verifySession(env.SESSION_SECRET, bearer(request), "bridge"));
}

async function authorizeDashboard(request, env) {
  return verifySession(env.SESSION_SECRET, bearer(request), "dashboard");
}

async function login(request, env) {
  const data = await body(request);
  const username = clean(data.username, 100), password = String(data.password || "").slice(0, 200);
  const address = request.headers.get("cf-connecting-ip") || request.headers.get("x-forwarded-for") || "unknown";
  const attemptID = base64url(new Uint8Array(await crypto.subtle.digest("SHA-256", encoder.encode(address))));
  const windowStart = Math.floor(Date.now() / 900000) * 900000;
  const attempt = await env.DB.prepare("SELECT attempts FROM auth_attempts WHERE id=? AND window_start=?").bind(attemptID, windowStart).first();
  if ((attempt?.attempts || 0) >= 8) return json({ error: "muitas tentativas; aguarde 15 minutos" }, 429);
  await env.DB.prepare("INSERT INTO auth_attempts(id,window_start,attempts) VALUES(?,?,1) ON CONFLICT(id) DO UPDATE SET window_start=excluded.window_start,attempts=CASE WHEN auth_attempts.window_start=excluded.window_start THEN auth_attempts.attempts+1 ELSE 1 END").bind(attemptID, windowStart).run();
  if (!constantTimeEqual(username, env.DASHBOARD_USERNAME) || !constantTimeEqual(password, env.DASHBOARD_PASSWORD)) return json({ error: "usuário ou senha inválidos" }, 401);
  await env.DB.prepare("DELETE FROM auth_attempts WHERE id=?").bind(attemptID).run();
  const role = data.purpose === "bridge" ? "bridge" : "dashboard";
  const lifetime = role === "bridge" ? 90 * 86400 : 12 * 3600;
  return json({ token: await signSession(env.SESSION_SECRET, username, role, lifetime), username, expiresIn: lifetime });
}

async function syncBridge(request, env) {
  if (!await authorizeBridge(request, env)) return json({ error: "não autorizado" }, 401);
  const data = await body(request);
  const instanceId = clean(data.instanceId, 100);
  if (!instanceId || !Array.isArray(data.contacts) || !Array.isArray(data.failures)) return json({ error: "snapshot inválido" }, 400);
  const now = new Date().toISOString();
  const statements = [
    env.DB.prepare("INSERT INTO instances(id,version,last_seen) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET version=excluded.version,last_seen=excluded.last_seen").bind(instanceId, clean(data.version, 40), now),
    env.DB.prepare("DELETE FROM contacts WHERE instance_id=?").bind(instanceId)
  ];
  for (const item of data.contacts.slice(0, 5000)) {
    const account = clean(item.Account, 150), contact = clean(item.Contact, 150), stage = clean(item.Stage, 40);
    if (!account || !contact || !stage) continue;
    statements.push(env.DB.prepare(`INSERT INTO contacts(instance_id,account,contact,stage,last_seen,check_in,check_out,paused,quote_active,awaiting_choice,adults,children,rooms,categories,selected_categories,prereservation_code,prereservation_status,updated_at)
      VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(instance_id,account,contact) DO UPDATE SET stage=excluded.stage,last_seen=excluded.last_seen,check_in=excluded.check_in,check_out=excluded.check_out,paused=excluded.paused,quote_active=excluded.quote_active,awaiting_choice=excluded.awaiting_choice,adults=excluded.adults,children=excluded.children,rooms=excluded.rooms,categories=excluded.categories,selected_categories=excluded.selected_categories,prereservation_code=excluded.prereservation_code,prereservation_status=excluded.prereservation_status,updated_at=excluded.updated_at`)
      .bind(instanceId, account, contact, stage, clean(item.LastSeen, 40), clean(item.CheckIn, 20), clean(item.CheckOut, 20), item.Paused ? 1 : 0, item.QuoteActive ? 1 : 0, item.AwaitingChoice ? 1 : 0, Number(item.Adults) || 0, Number(item.Children) || 0, Number(item.Rooms) || 0, JSON.stringify(item.Categories || []), JSON.stringify(item.SelectedCategories || []), clean(item.PreReservationCode, 100), clean(item.PreReservationStatus, 40), now));
  }
  for (const failure of data.failures.slice(0, 500)) {
    statements.push(env.DB.prepare("INSERT OR IGNORE INTO failures(instance_id,source,source_id,contact,message,created_at) VALUES(?,?,?,?,?,?)")
      .bind(instanceId, clean(failure.Source, 30), Number(failure.id) || 0, clean(failure.Contact, 150), clean(failure.Message, 1000), clean(failure.CreatedAt, 40) || now));
  }
  await env.DB.batch(statements);
  const pending = await env.DB.prepare("SELECT id,account,contact,action FROM commands WHERE instance_id=? AND status IN ('pending','delivered') ORDER BY created_at LIMIT 20").bind(instanceId).all();
  if (pending.results.length) {
    await env.DB.batch(pending.results.map((command) => env.DB.prepare("UPDATE commands SET status='delivered',updated_at=? WHERE id=? AND status='pending'").bind(now, command.id)));
  }
  return json({ commands: pending.results.map(({ id, account, contact, action }) => ({ ID: id, Account: account, Contact: contact, Action: action })) });
}

async function ackCommand(request, env, id) {
  if (!await authorizeBridge(request, env)) return json({ error: "não autorizado" }, 401);
  const data = await body(request);
  const status = data.status === "executed" ? "executed" : "failed";
  const result = await env.DB.prepare("UPDATE commands SET status=?,message=?,updated_at=? WHERE id=? AND instance_id=? AND status IN ('pending','delivered')")
    .bind(status, clean(data.message, 500), new Date().toISOString(), id, clean(data.instanceId, 100)).run();
  return result.meta.changes ? json({ ok: true }) : json({ error: "comando não encontrado" }, 404);
}

async function dashboard(request, env) {
  if (!await authorizeDashboard(request, env)) return json({ error: "não autorizado" }, 401);
  const url = new URL(request.url), instance = clean(url.searchParams.get("instance"), 100);
  const selected = instance || (await env.DB.prepare("SELECT id FROM instances ORDER BY last_seen DESC LIMIT 1").first())?.id;
  if (!selected) return json({ instance: null, contacts: [], failures: [], commands: [] });
  const [machine, contacts, failures, commands] = await Promise.all([
    env.DB.prepare("SELECT id,version,last_seen FROM instances WHERE id=?").bind(selected).first(),
    env.DB.prepare("SELECT * FROM contacts WHERE instance_id=? ORDER BY updated_at DESC LIMIT 1000").bind(selected).all(),
    env.DB.prepare("SELECT * FROM failures WHERE instance_id=? ORDER BY created_at DESC LIMIT 200").bind(selected).all(),
    env.DB.prepare("SELECT * FROM commands WHERE instance_id=? ORDER BY created_at DESC LIMIT 100").bind(selected).all()
  ]);
  return json({ instance: machine, contacts: contacts.results.map((item) => ({ ...item, categories: JSON.parse(item.categories), selected_categories: JSON.parse(item.selected_categories) })), failures: failures.results, commands: commands.results });
}

async function createCommand(request, env) {
  const session = await authorizeDashboard(request, env);
  if (!session) return json({ error: "não autorizado" }, 401);
  const data = await body(request), action = clean(data.action, 40);
  if (!ACTIONS.has(action)) return json({ error: "ação inválida" }, 400);
  const instanceId = clean(data.instanceId, 100), account = clean(data.account, 150), contact = clean(data.contact, 150);
  if (!instanceId || !account || !contact) return json({ error: "atendimento incompleto" }, 400);
  const exists = await env.DB.prepare("SELECT 1 ok FROM contacts WHERE instance_id=? AND account=? AND contact=?").bind(instanceId, account, contact).first();
  if (!exists) return json({ error: "atendimento não encontrado" }, 404);
  const id = crypto.randomUUID(), now = new Date().toISOString();
  await env.DB.batch([
    env.DB.prepare("INSERT INTO commands(id,instance_id,account,contact,action,status,created_at,updated_at) VALUES(?,?,?,?,?,'pending',?,?)").bind(id, instanceId, account, contact, action, now, now),
    env.DB.prepare("INSERT INTO audit_log(instance_id,actor,action,account,contact,created_at) VALUES(?,?,?,?,?,?)").bind(instanceId, clean(session.sub, 200), action, account, contact, now)
  ]);
  return json({ id, status: "pending" }, 202);
}

export { constantTimeEqual, signSession, verifySession };

export default {
  async fetch(request, env) {
    try {
      const url = new URL(request.url);
      if (request.method === "OPTIONS") return cors(new Response(null, { status: 204 }), request, env);
      let response;
      if (request.method === "GET" && url.pathname === "/api/health") response = json({ ok: true });
      else if (request.method === "POST" && url.pathname === "/api/auth/login") response = await login(request, env);
      else if (request.method === "POST" && url.pathname === "/api/bridge/sync") response = await syncBridge(request, env);
      const ack = url.pathname.match(/^\/api\/bridge\/commands\/([^/]+)\/ack$/);
      if (!response && request.method === "POST" && ack) response = await ackCommand(request, env, ack[1]);
      else if (!response && request.method === "GET" && url.pathname === "/api/dashboard") response = await dashboard(request, env);
      else if (!response && request.method === "POST" && url.pathname === "/api/commands") response = await createCommand(request, env);
      return cors(response || json({ error: "rota não encontrada" }, 404), request, env);
    } catch (error) {
      return cors(json({ error: error instanceof Error ? error.message : "erro interno" }, 500), request, env);
    }
  }
};
