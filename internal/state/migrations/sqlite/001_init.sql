-- All timestamps are RFC3339Nano UTC strings. Booleans/ints are INTEGER.

CREATE TABLE IF NOT EXISTS runs (
    id          TEXT PRIMARY KEY,
    project     TEXT NOT NULL,
    provider    TEXT NOT NULL,
    started_at  TEXT NOT NULL,
    ended_at    TEXT,
    status      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS features (
    id          TEXT NOT NULL,
    run_id      TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL,
    priority    INTEGER NOT NULL DEFAULT 0,
    depends_on  TEXT NOT NULL DEFAULT '[]',
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (id, run_id)
);

CREATE TABLE IF NOT EXISTS attempts (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL,
    feature_id  TEXT NOT NULL,
    provider    TEXT NOT NULL,
    prompt      TEXT NOT NULL,
    output      TEXT NOT NULL DEFAULT '',
    stderr      TEXT NOT NULL DEFAULT '',
    exit_code   INTEGER NOT NULL DEFAULT 0,
    success     INTEGER NOT NULL DEFAULT 0,
    session_id  TEXT NOT NULL DEFAULT '',
    started_at  TEXT NOT NULL,
    ended_at    TEXT,
    duration_ms INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS log_lines (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id      TEXT NOT NULL,
    attempt_id  TEXT NOT NULL DEFAULT '',
    ts          TEXT NOT NULL,
    level       TEXT NOT NULL,
    msg         TEXT NOT NULL,
    attrs       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS limiter_windows (
    provider    TEXT PRIMARY KEY,
    limited_at  TEXT NOT NULL,
    reset_at    TEXT,
    reason      TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_features_run ON features(run_id);
CREATE INDEX IF NOT EXISTS idx_attempts_run ON attempts(run_id);
CREATE INDEX IF NOT EXISTS idx_log_lines_run ON log_lines(run_id, id);
