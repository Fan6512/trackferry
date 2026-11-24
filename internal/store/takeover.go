package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Takeover outcomes.
const (
	OutcomeRunning    = "running"
	OutcomeConfirmed  = "confirmed"  // speaker fetched our stream and reports playing it
	OutcomeFetched    = "fetched"    // speaker fetched our stream; playing not confirmed
	OutcomeDispatched = "dispatched" // play call accepted; stream never fetched
	OutcomeFailed     = "failed"
	OutcomeCancelled  = "cancelled" // superseded by a newer command on the same speaker
)

// Step is one timed action within a takeover run.
type Step struct {
	Step   string `json:"step"`
	AtMS   int64  `json:"at_ms"` // since the run started
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// TakeoverRun is one takeover attempt.
type TakeoverRun struct {
	ID         int64  `json:"id"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	CommandID  int64  `json:"command_id,omitempty"`
	Kind       string `json:"kind"`
	Source     string `json:"source"`
	StartedMS  int64  `json:"started_ms"`
	UpstreamMS int64  `json:"upstream_ms,omitempty"`
	FinishedMS int64  `json:"finished_ms,omitempty"`
	Outcome    string `json:"outcome"`
	Detail     string `json:"detail,omitempty"`
	Steps      []Step `json:"steps"`
	Song       *Song  `json:"song,omitempty"` // sent from the library
}

// Song is what a run plays.
type Song struct {
	TrackID    int64  `json:"track_id"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album,omitempty"`
	Cover      string `json:"cover,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

// StartRun inserts a run in state running and sets r.ID.
func (s *Store) StartRun(ctx context.Context, r *TakeoverRun) error {
	var cmd any
	if r.CommandID != 0 {
		cmd = r.CommandID
	}
	song := ""
	if r.Song != nil {
		b, _ := json.Marshal(r.Song)
		song = string(b)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO takeover_runs(device_id, command_id, kind, source, started_ms, upstream_ms, outcome, song)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, r.DeviceID, cmd, r.Kind, r.Source, r.StartedMS, r.UpstreamMS, OutcomeRunning, song)
	if err != nil {
		return err
	}
	r.ID, err = res.LastInsertId()
	r.Outcome = OutcomeRunning
	return err
}

// SaveRun writes a run's progress (steps, outcome, finish time).
func (s *Store) SaveRun(ctx context.Context, r *TakeoverRun) error {
	steps, err := json.Marshal(r.Steps)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE takeover_runs SET finished_ms = ?, outcome = ?, detail = ?, steps = ? WHERE id = ?`,
		r.FinishedMS, r.Outcome, r.Detail, string(steps), r.ID)
	return err
}

const runCols = `r.id, r.device_id, COALESCE(NULLIF(d.alias, ''), d.name, ''), COALESCE(r.command_id, 0), r.kind, r.source,
	r.started_ms, r.upstream_ms, r.finished_ms, r.outcome, r.detail, r.steps, r.song
	FROM takeover_runs r LEFT JOIN devices d ON d.device_id = r.device_id`

// Runs lists recent takeover runs, newest first.
func (s *Store) Runs(ctx context.Context, limit int) ([]TakeoverRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 20
	}
	rows, err := s.rd.QueryContext(ctx, `SELECT `+runCols+` ORDER BY r.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TakeoverRun{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Run returns one takeover run.
func (s *Store) Run(ctx context.Context, id int64) (TakeoverRun, error) {
	r, err := scanRun(s.rd.QueryRowContext(ctx, `SELECT `+runCols+` WHERE r.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// FailRunningRuns marks runs left "running" by a previous process as failed.
func (s *Store) FailRunningRuns(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE takeover_runs SET outcome = ?, detail = 'TrackFerry 在接管途中重启', finished_ms = ? WHERE outcome = ?`,
		OutcomeFailed, now.UnixMilli(), OutcomeRunning)
	return err
}

func scanRun(r scanner) (TakeoverRun, error) {
	var t TakeoverRun
	var steps, song string
	if err := r.Scan(&t.ID, &t.DeviceID, &t.DeviceName, &t.CommandID, &t.Kind, &t.Source, &t.StartedMS, &t.UpstreamMS, &t.FinishedMS,
		&t.Outcome, &t.Detail, &steps, &song); err != nil {
		return t, err
	}
	t.Steps = []Step{}
	json.Unmarshal([]byte(steps), &t.Steps)
	if song != "" {
		json.Unmarshal([]byte(song), &t.Song)
	}
	return t, nil
}

// SetPendingVolume remembers the volume to restore on a speaker TrackFerry is muting.
// An existing entry is kept: it holds the user's real volume, not an intermediate 0.
func (s *Store) SetPendingVolume(ctx context.Context, deviceID string, volume int, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO pending_volume(device_id, volume, set_ms) VALUES (?, ?, ?) ON CONFLICT(device_id) DO NOTHING`,
		deviceID, volume, at.UnixMilli())
	return err
}

// PendingVolume returns the volume waiting to be restored on a speaker.
func (s *Store) PendingVolume(ctx context.Context, deviceID string) (int, bool, error) {
	var v int
	err := s.rd.QueryRowContext(ctx, "SELECT volume FROM pending_volume WHERE device_id = ?", deviceID).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return v, err == nil, err
}

// PendingVolumes lists every speaker still waiting for its volume back.
func (s *Store) PendingVolumes(ctx context.Context) (map[string]int, error) {
	rows, err := s.rd.QueryContext(ctx, "SELECT device_id, volume FROM pending_volume")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var v int
		if err := rows.Scan(&id, &v); err != nil {
			return nil, err
		}
		out[id] = v
	}
	return out, rows.Err()
}

// ClearPendingVolume forgets a restored volume.
func (s *Store) ClearPendingVolume(ctx context.Context, deviceID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM pending_volume WHERE device_id = ?", deviceID)
	return err
}

// Device returns one speaker.
func (s *Store) Device(ctx context.Context, id string) (Device, error) {
	d, err := scanDevice(s.rd.QueryRowContext(ctx, `SELECT device_id, name, alias, model, hardware, miot_did, presence, mode, first_seen_at, last_seen_at, in_account
		FROM devices WHERE device_id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

// SetCommandStage records what TrackFerry did with a command.
func (s *Store) SetCommandStage(ctx context.Context, id int64, stage string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE commands SET stage = ? WHERE id = ?", stage, id)
	return err
}
