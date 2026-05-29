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
`
