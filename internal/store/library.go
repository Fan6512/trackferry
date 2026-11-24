package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// TrackRow is one indexed file of the local library.
type TrackRow struct {
	ID         int64  `json:"id"`
	Path       string `json:"path"`
	Root       string `json:"root"`
	Size       int64  `json:"size"`
	MTimeMS    int64  `json:"-"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	DurationMS int64  `json:"duration_ms"`
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
	Bits       int    `json:"bits"`
	Tagged     bool   `json:"tagged"`
	// Set for tracks of 飞牛音乐, whose Path is "fn:" + GUID.
	GUID     string `json:"guid,omitempty"`
	Cover    string `json:"cover,omitempty"`
	AlbumID  string `json:"album_id,omitempty"`
	Favorite bool   `json:"favorite,omitempty"`
	AddedMS  int64  `json:"added_ms,omitempty"`
}

const trackCols = `id, path, root, size, mtime_ms, title, artist, album, duration_ms, format, sample_rate, bits, tagged, guid, cover, album_id, favorite, added_ms FROM tracks`

// Tracks reads the whole index.
func (s *Store) Tracks(ctx context.Context) ([]TrackRow, error) {
	// Paged so a large library never holds the only connection for long.
	var out []TrackRow
	last := int64(0)
	for {
		n, err := s.tracksPage(ctx, last, &out)
		if err != nil || n == 0 {
			return out, err
		}
		last = out[len(out)-1].ID
	}
}

func (s *Store) tracksPage(ctx context.Context, after int64, out *[]TrackRow) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+trackCols+` WHERE id > ? ORDER BY id LIMIT 2000`, after)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var t TrackRow
		var tagged, fav int
		if err := rows.Scan(&t.ID, &t.Path, &t.Root, &t.Size, &t.MTimeMS, &t.Title, &t.Artist, &t.Album, &t.DurationMS,
			&t.Format, &t.SampleRate, &t.Bits, &tagged, &t.GUID, &t.Cover, &t.AlbumID, &fav, &t.AddedMS); err != nil {
			return n, err
		}
		t.Tagged, t.Favorite = tagged == 1, fav == 1
		*out = append(*out, t)
		n++
	}
	return n, rows.Err()
}

// SaveTracks upserts a batch (changed files) and marks the unchanged paths as seen, in one
// short transaction.
func (s *Store) SaveTracks(ctx context.Context, scanID int64, changed []TrackRow, seen []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range changed {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tracks(path, root, size, mtime_ms, title, artist, album, duration_ms, format, sample_rate, bits, tagged, scan_id, guid, cover, album_id, favorite, added_ms)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(path) DO UPDATE SET root=excluded.root, size=excluded.size, mtime_ms=excluded.mtime_ms, title=excluded.title,
			artist=excluded.artist, album=excluded.album, duration_ms=excluded.duration_ms, format=excluded.format,
			sample_rate=excluded.sample_rate, bits=excluded.bits, tagged=excluded.tagged, scan_id=excluded.scan_id, guid=excluded.guid,
			cover=excluded.cover, album_id=excluded.album_id, favorite=excluded.favorite, added_ms=excluded.added_ms`,
			t.Path, t.Root, t.Size, t.MTimeMS, t.Title, t.Artist, t.Album, t.DurationMS, t.Format, t.SampleRate, t.Bits, b2i(t.Tagged), scanID, t.GUID, t.Cover, t.AlbumID, b2i(t.Favorite), t.AddedMS); err != nil {
			return err
		}
	}
	for _, id := range seen {
		if _, err := tx.ExecContext(ctx, `UPDATE tracks SET scan_id = ? WHERE id = ?`, scanID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetFavorite marks a track.
func (s *Store) SetFavorite(ctx context.Context, id int64, on bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE tracks SET favorite = ? WHERE id = ?`, b2i(on), id)
	return err
}

// DropUnseen removes a root's tracks the scan did not see. Call it only for roots the scan
// read completely: an unreadable or unmounted folder must not empty the index.
func (s *Store) DropUnseen(ctx context.Context, scanID int64, root string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tracks WHERE root = ? AND scan_id <> ?`, root, scanID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DropRootsExcept removes the tracks of roots that no longer exist in the music directory.
func (s *Store) DropRootsExcept(ctx context.Context, roots []string) error {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT root FROM tracks`)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, r := range roots {
		keep[r] = true
	}
	var drop []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			rows.Close()
			return err
		}
		if !keep[r] {
			drop = append(drop, r)
		}
	}
	rows.Close()
	for _, r := range drop {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM tracks WHERE root = ?`, r); err != nil {
			return err
		}
	}
	return nil
}

// DropAllUnseen removes every track the scan did not see, whatever its root.
func (s *Store) DropAllUnseen(ctx context.Context, scanID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tracks WHERE scan_id <> ?`, scanID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DropRemote removes the tracks of 飞牛音乐.
func (s *Store) DropRemote(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tracks WHERE guid <> ''`)
	return err
}

// StartScan records a scan start and returns its ID; scans a previous process left running
// are marked failed.
func (s *Store) StartScan(ctx context.Context, now time.Time) (int64, error) {
	if _, err := s.db.ExecContext(ctx, `UPDATE library_scans SET state = 'failed' WHERE state = 'running'`); err != nil {
		return 0, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO library_scans(started_ms, state) VALUES (?, 'running')`, now.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishScan stores a scan's result.
func (s *Store) FinishScan(ctx context.Context, id int64, state, report string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE library_scans SET finished_ms = ?, state = ?, report = ? WHERE id = ?`, now.UnixMilli(), state, report, id)
	return err
}

// LastScan returns the newest finished scan's report JSON ("" when there is none).
func (s *Store) LastScan(ctx context.Context) (string, error) {
	var r string
	err := s.db.QueryRowContext(ctx, `SELECT report FROM library_scans WHERE state <> 'running' AND report <> '{}' ORDER BY id DESC LIMIT 1`).Scan(&r)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return r, err
}

// Setting reads a setting ("" when unset).
func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// SetSetting stores a setting.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
