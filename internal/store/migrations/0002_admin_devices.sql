-- Single admin credential for the management UI.
CREATE TABLE admin (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	password_hash BLOB NOT NULL,
	salt BLOB NOT NULL,
	iterations INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

-- Browser sessions. Only a SHA-256 of the cookie value is stored.
CREATE TABLE sessions (
	token_hash BLOB PRIMARY KEY,
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL
);

-- Speakers discovered from the linked Xiaomi account. mode: off | observe | takeover.
CREATE TABLE devices (
	device_id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	alias TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	hardware TEXT NOT NULL DEFAULT '',
	miot_did TEXT NOT NULL DEFAULT '',
	presence TEXT NOT NULL DEFAULT '',
	mode TEXT NOT NULL DEFAULT 'off' CHECK (mode IN ('off', 'observe', 'takeover')),
	first_seen_at TEXT NOT NULL,
	last_seen_at TEXT NOT NULL,
	in_account INTEGER NOT NULL DEFAULT 1
);
