-- Takeover runs: one attempt to interrupt Xiao AI and play TrackFerry's own audio on a
-- speaker, either for a heard command or as a manual test from the UI.
-- steps is a JSON array of {step, at_ms (since started_ms), ok, detail}.
CREATE TABLE takeover_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	device_id TEXT NOT NULL,
	command_id INTEGER,
	kind TEXT NOT NULL,            -- test | command
	source TEXT NOT NULL,          -- what was played, for people (e.g. 提示音)
	started_ms INTEGER NOT NULL,
	upstream_ms INTEGER NOT NULL DEFAULT 0, -- Xiaomi's recognition time of the command, 0 for tests
	finished_ms INTEGER NOT NULL DEFAULT 0,
	outcome TEXT NOT NULL DEFAULT 'running',
	detail TEXT NOT NULL DEFAULT '',
	steps TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX takeover_runs_started ON takeover_runs(started_ms DESC);

-- A speaker TrackFerry muted while cutting Xiao AI off, with the volume to put back.
-- Kept on disk so a crash or restart mid-takeover never leaves a speaker silent.
CREATE TABLE pending_volume (
	device_id TEXT PRIMARY KEY,
	volume INTEGER NOT NULL,
	set_ms INTEGER NOT NULL
);
