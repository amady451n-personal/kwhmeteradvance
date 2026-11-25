-- meteran schema. Idempotent; safe to run on every start.
CREATE TABLE IF NOT EXISTS readings (
    id              TEXT PRIMARY KEY,
    meter_name      TEXT    NOT NULL DEFAULT 'main',
    reading         REAL    NOT NULL,
    timestamp       TEXT    NOT NULL,
    notes           TEXT,
    recharge_amount REAL,
    cost            INTEGER,
    token_id        TEXT,
    source          TEXT    NOT NULL DEFAULT 'manual',
    old_id          TEXT,
    created_at      TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS readings_ts ON readings (timestamp DESC, id DESC);
CREATE INDEX IF NOT EXISTS readings_token ON readings (token_id);
CREATE UNIQUE INDEX IF NOT EXISTS readings_old_id ON readings (old_id) WHERE old_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS tokens (
    id                TEXT PRIMARY KEY,
    kwh               REAL    NOT NULL,
    cost              INTEGER,
    purchased_at      TEXT    NOT NULL,
    type              TEXT    NOT NULL DEFAULT 'additional',
    denomination      INTEGER,
    status            TEXT    NOT NULL DEFAULT 'unused',
    source            TEXT    NOT NULL DEFAULT 'manual',
    notes             TEXT,
    code_enc          BLOB,
    linked_reading_id TEXT,
    locked_at         TEXT,
    created_at        TEXT    NOT NULL,
    updated_at        TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS tokens_purchased ON tokens (purchased_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS settings (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    buffer             REAL    NOT NULL,
    avg_window         INTEGER NOT NULL,
    target_hour        INTEGER NOT NULL,
    reminder_hour      INTEGER NOT NULL,
    denominations      TEXT    NOT NULL,
    tariff_golongan    TEXT,
    tariff_per_kwh     REAL,
    push_enabled       INTEGER NOT NULL DEFAULT 0,
    import_completed_at TEXT,
    updated_at         TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS pin (
    id         INTEGER PRIMARY KEY CHECK (id = 1),
    hash       TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token        TEXT PRIMARY KEY,
    created_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_attempts (
    key TEXT NOT NULL,
    at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS auth_attempts_key ON auth_attempts (key, at);

CREATE TABLE IF NOT EXISTS push_subscriptions (
    id         TEXT PRIMARY KEY,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    created_at TEXT NOT NULL
);