PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;

CREATE TABLE IF NOT EXISTS apps (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  cwd TEXT NOT NULL,
  command_json TEXT NOT NULL,
  health_mode TEXT NOT NULL DEFAULT 'url',
  health_url TEXT NOT NULL DEFAULT '',
  health_command_json TEXT NOT NULL DEFAULT '[]',
  startup_timeout_seconds INTEGER NOT NULL DEFAULT 60,
  sound_repeat_count INTEGER NOT NULL DEFAULT 1,
  codex_environment TEXT NOT NULL DEFAULT 'host',
  runner_environment TEXT NOT NULL DEFAULT 'host',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS prompt_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  prompt_index INTEGER NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT,
  prompt_duration_seconds REAL,
  user_wait_seconds REAL,
  permission_count INTEGER NOT NULL DEFAULT 0,
  restart_result TEXT,
  health_result TEXT
);

CREATE TABLE IF NOT EXISTS restart_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  prompt_run_id INTEGER REFERENCES prompt_runs(id) ON DELETE SET NULL,
  requested_at TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT,
  healthy_at TEXT,
  result TEXT,
  failure_reason TEXT
);

CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS notifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  repeat_count INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_prompt_runs_app_started ON prompt_runs(app_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_restart_runs_app_requested ON restart_runs(app_id, requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_app_created ON events(app_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_app_created ON notifications(app_id, created_at DESC);
