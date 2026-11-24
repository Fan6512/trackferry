-- Music commands heard on observed speakers. Only utterances recognised as music requests
-- or playback controls are stored; other conversation is counted, never saved.
-- Times are Unix milliseconds: upstream_ms is Xiaomi's recognition time (its clock),
-- prev_poll_ms / seen_ms bound when the record became visible (our clock).
CREATE TABLE commands (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	device_id TEXT NOT NULL,
	request_key TEXT NOT NULL,
	query TEXT NOT NULL,
	answer TEXT NOT NULL DEFAULT '',
	answer_types TEXT NOT NULL DEFAULT '',
	upstream_ms INTEGER NOT NULL,
	prev_poll_ms INTEGER NOT NULL,
	seen_ms INTEGER NOT NULL,
	stale INTEGER NOT NULL DEFAULT 0,
	intent_kind TEXT NOT NULL,
	intent_title TEXT NOT NULL DEFAULT '',
	intent_artist TEXT NOT NULL DEFAULT '',
	intent_control TEXT NOT NULL DEFAULT '',
	-- what TrackFerry did: observed (watch only), later dispatched / handed_to_xiaoai / ...
	stage TEXT NOT NULL,
	-- field note on what Xiao AI had already done when the record appeared: '' | silent | spoke | playing
	native_note TEXT NOT NULL DEFAULT '',
	UNIQUE (device_id, request_key)
);
CREATE INDEX commands_seen ON commands(seen_ms DESC);
