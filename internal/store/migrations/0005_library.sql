-- The local library index. Files are never modified; rows mirror what the last scan saw.
CREATE TABLE tracks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	path TEXT NOT NULL UNIQUE,     -- relative to the music directory, with slashes
	root TEXT NOT NULL,            -- first path element ('' for files directly in the music directory)
	size INTEGER NOT NULL,
	mtime_ms INTEGER NOT NULL,
	title TEXT NOT NULL,
	artist TEXT NOT NULL,
	album TEXT NOT NULL,
	duration_ms INTEGER NOT NULL,  -- 0 = unknown
	format TEXT NOT NULL,          -- mp3, flac, ...
	sample_rate INTEGER NOT NULL,
	bits INTEGER NOT NULL,
	tagged INTEGER NOT NULL,       -- 0 = title/artist come from the file name
	scan_id INTEGER NOT NULL       -- last scan that saw the file
);
CREATE INDEX tracks_root ON tracks(root);

CREATE TABLE library_scans (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	started_ms INTEGER NOT NULL,
	finished_ms INTEGER NOT NULL DEFAULT 0,
	state TEXT NOT NULL,           -- running | done | failed
	report TEXT NOT NULL DEFAULT '{}'
);

-- How TrackFerry routed (or, on a watch-only speaker, would have routed) the command: JSON.
ALTER TABLE commands ADD COLUMN route TEXT NOT NULL DEFAULT '';

CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
