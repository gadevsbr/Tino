CREATE TABLE auth_attempts (
  id TEXT PRIMARY KEY,
  window_start INTEGER NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0
);
