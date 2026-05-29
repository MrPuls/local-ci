package store

// schemaVersion is the current schema revision. Bump it (and add migration
// steps in migrate) when the tables below change.
const schemaVersion = 1

// schemaSQL creates the run-history tables if they do not already exist.
// Timestamps are unix milliseconds. Columns that are unknown until a run/job
// finishes (finished_at, duration_ms, exit_code) are nullable.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS runs (
  id           TEXT PRIMARY KEY,
  project_path TEXT    NOT NULL,
  config_path  TEXT    NOT NULL,
  mode         TEXT    NOT NULL,
  status       TEXT    NOT NULL,
  started_at   INTEGER NOT NULL,
  finished_at  INTEGER,
  duration_ms  INTEGER,
  error        TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_runs_project_started ON runs(project_path, started_at DESC);

CREATE TABLE IF NOT EXISTS jobs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id      TEXT    NOT NULL REFERENCES runs(id),
  name        TEXT    NOT NULL,
  stage       TEXT    NOT NULL DEFAULT '',
  exec_kind   TEXT    NOT NULL DEFAULT '',
  group_label TEXT    NOT NULL DEFAULT '',
  status      TEXT    NOT NULL,
  started_at  INTEGER,
  finished_at INTEGER,
  duration_ms INTEGER,
  exit_code   INTEGER,
  error       TEXT    NOT NULL DEFAULT '',
  log_path    TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_jobs_run ON jobs(run_id);

CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL);
`
