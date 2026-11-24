-- The song a run plays, when it was sent from the library instead of asked for.
ALTER TABLE takeover_runs ADD COLUMN song TEXT NOT NULL DEFAULT '';
