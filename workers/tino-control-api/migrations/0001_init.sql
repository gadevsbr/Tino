CREATE TABLE instances (
  id TEXT PRIMARY KEY,
  version TEXT NOT NULL DEFAULT '',
  last_seen TEXT NOT NULL
);

CREATE TABLE contacts (
  instance_id TEXT NOT NULL,
  account TEXT NOT NULL,
  contact TEXT NOT NULL,
  stage TEXT NOT NULL,
  last_seen TEXT NOT NULL DEFAULT '',
  check_in TEXT NOT NULL DEFAULT '',
  check_out TEXT NOT NULL DEFAULT '',
  paused INTEGER NOT NULL DEFAULT 0,
  quote_active INTEGER NOT NULL DEFAULT 0,
  awaiting_choice INTEGER NOT NULL DEFAULT 0,
  adults INTEGER NOT NULL DEFAULT 0,
  children INTEGER NOT NULL DEFAULT 0,
  rooms INTEGER NOT NULL DEFAULT 0,
  categories TEXT NOT NULL DEFAULT '[]',
  selected_categories TEXT NOT NULL DEFAULT '[]',
  prereservation_code TEXT NOT NULL DEFAULT '',
  prereservation_status TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY(instance_id,account,contact)
);
CREATE INDEX contacts_stage ON contacts(instance_id,stage,updated_at DESC);

CREATE TABLE failures (
  instance_id TEXT NOT NULL,
  source TEXT NOT NULL,
  source_id INTEGER NOT NULL,
  contact TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY(instance_id,source,source_id)
);
CREATE INDEX failures_created ON failures(instance_id,created_at DESC);

CREATE TABLE commands (
  id TEXT PRIMARY KEY,
  instance_id TEXT NOT NULL,
  account TEXT NOT NULL,
  contact TEXT NOT NULL,
  action TEXT NOT NULL CHECK(action IN ('pause','resume','takeover','restart_quote')),
  status TEXT NOT NULL CHECK(status IN ('pending','delivered','executed','failed')),
  message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX commands_pending ON commands(instance_id,status,created_at);

CREATE TABLE audit_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  instance_id TEXT NOT NULL,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  account TEXT NOT NULL DEFAULT '',
  contact TEXT NOT NULL DEFAULT '',
  details TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX audit_created ON audit_log(instance_id,created_at DESC);
