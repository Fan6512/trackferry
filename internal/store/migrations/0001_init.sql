-- Process starts. Lets restarts and volume recovery be checked from the API.
CREATE TABLE boots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	started_at TEXT NOT NULL,
	version TEXT NOT NULL,
	git_commit TEXT NOT NULL
);
