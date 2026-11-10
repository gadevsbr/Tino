const base = (import.meta.env.VITE_API_URL || "").replace(/\/$/, "");

export function savedToken() { return sessionStorage.getItem("tino-control-token") || ""; }
export function saveToken(token) { sessionStorage.setItem("tino-control-token", token.trim()); }
export function clearToken() { sessionStorage.removeItem("tino-control-token"); }

if (location.hash.startsWith("#token=")) {
  saveToken(decodeURIComponent(location.hash.slice(7)));
  history.replaceState(null, "", location.pathname + location.search);
}

async function request(path, options = {}) {
  const response = await fetch(base + path, {
    ...options,
    headers: { "content-type": "application/json", authorization: `Bearer ${savedToken()}`, ...(options.headers || {}) }
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
  return data;
}

export function loadDashboard() { return request("/api/dashboard"); }
export function sendCommand(contact, action, instanceId) {
  return request("/api/commands", { method: "POST", body: JSON.stringify({ instanceId, account: contact.account, contact: contact.contact, action }) });
}
