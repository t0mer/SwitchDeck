package store

const schema = `
CREATE TABLE IF NOT EXISTS switches (
	id              TEXT PRIMARY KEY,
	name            TEXT NOT NULL,
	ip              TEXT NOT NULL,
	username        TEXT NOT NULL,
	password_enc    BLOB NOT NULL,
	insecure_tls    INTEGER NOT NULL DEFAULT 1,
	enabled         INTEGER NOT NULL DEFAULT 1,
	poll_stats_secs  INTEGER NOT NULL DEFAULT 60,
	poll_config_secs INTEGER NOT NULL DEFAULT 300,
	created_at      INTEGER NOT NULL,
	updated_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS snapshots (
	switch_id    TEXT PRIMARY KEY REFERENCES switches(id) ON DELETE CASCADE,
	collected_at INTEGER NOT NULL,
	data         BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS api_tokens (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	expiry     INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS notification_channels (
	id             TEXT PRIMARY KEY,
	name           TEXT NOT NULL UNIQUE,
	provider       TEXT NOT NULL,
	config_enc     BLOB NOT NULL,
	enabled        INTEGER NOT NULL DEFAULT 1,
	notify_offline INTEGER NOT NULL DEFAULT 1,
	notify_online  INTEGER NOT NULL DEFAULT 1,
	created_at     INTEGER NOT NULL,
	updated_at     INTEGER NOT NULL
);
`
