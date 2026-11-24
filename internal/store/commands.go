package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Command stages.
const (
	StageObserved  = "observed"   // watch-only speaker: recorded, Xiao AI handles it
	StageNative    = "native"     // takeover speaker, but nothing to play: left to Xiao AI
	StageTakenOver = "taken_over" // TrackFerry interrupted Xiao AI and played its own audio
	StageSkipped   = "skipped"    // takeover speaker, not acted on (stale, control, superseded)
)

// Native notes, recorded by hand while measuring the native-first window.
var NativeNotes = map[string]bool{"": true, "silent": true, "spoke": true, "playing": true}

// Command is one music command heard on a speaker.
type Command struct {
	ID            int64    `json:"id"`
	DeviceID      string   `json:"device_id"`
	DeviceName    string   `json:"device_name"`
	RequestKey    string   `json:"-"`
	Query         string   `json:"query"`
	Answer        string   `json:"answer"`
	AnswerTypes   []string `json:"answer_types"`
	UpstreamMS    int64    `json:"upstream_ms"`
	PrevPollMS    int64    `json:"prev_poll_ms"`
	SeenMS        int64    `json:"seen_ms"`
	Stale         bool     `json:"stale"`
	IntentKind    string   `json:"intent_kind"`
	IntentTitle   string   `json:"intent_title,omitempty"`
	IntentArtist  string   `json:"intent_artist,omitempty"`
	IntentControl string   `json:"intent_control,omitempty"`
	Stage         string   `json:"stage"`
	NativeNote    string   `json:"native_note"`
	// Route is how TrackFerry routed the command (routing.Route as JSON); null until routed.
	Route json.RawMessage `json:"route"`
}

// AddCommand stores a command. It reports false when the same upstream request was
// already recorded (duplicate poll result).
func (s *Store) AddCommand(ctx context.Context, c *Command) (bool, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO commands(device_id, request_key, query, answer, answer_types, upstream_ms, prev_poll_ms, seen_ms, stale,
			intent_kind, intent_title, intent_artist, intent_control, stage)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(device_id, request_key) DO NOTHING`,
		c.DeviceID, c.RequestKey, c.Query, c.Answer, strings.Join(c.AnswerTypes, ","), c.UpstreamMS, c.PrevPollMS, c.SeenMS, b2i(c.Stale),
		c.IntentKind, c.IntentTitle, c.IntentArtist, c.IntentControl, c.Stage)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	c.ID, _ = res.LastInsertId()
	return true, nil
}

const commandCols = `c.id, c.device_id, COALESCE(NULLIF(d.alias, ''), d.name, ''), c.request_key, c.query, c.answer, c.answer_types,
	c.upstream_ms, c.prev_poll_ms, c.seen_ms, c.stale, c.intent_kind, c.intent_title, c.intent_artist, c.intent_control, c.stage, c.native_note, c.route
	FROM commands c LEFT JOIN devices d ON d.device_id = c.device_id`

// Commands lists recent commands, newest first. before (an id) pages backwards; 0 = newest.
func (s *Store) Commands(ctx context.Context, deviceID string, before int64, limit int) ([]Command, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT ` + commandCols + ` WHERE (? = '' OR c.device_id = ?) AND (? = 0 OR c.id < ?) ORDER BY c.id DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, deviceID, deviceID, before, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Command{}
	for rows.Next() {
		c, err := scanCommand(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetNativeNote records what Xiao AI had already done when the command appeared.
func (s *Store) SetNativeNote(ctx context.Context, id int64, note string) (Command, error) {
	res, err := s.db.ExecContext(ctx, "UPDATE commands SET native_note = ? WHERE id = ?", note, id)
	if err != nil {
		return Command{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Command{}, ErrNotFound
	}
	c, err := scanCommand(s.db.QueryRowContext(ctx, `SELECT `+commandCols+` WHERE c.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Command{}, ErrNotFound
	}
	return c, err
}

// SetCommandRoute stores how a command was routed (JSON).
func (s *Store) SetCommandRoute(ctx context.Context, id int64, route []byte) error {
	_, err := s.db.ExecContext(ctx, "UPDATE commands SET route = ? WHERE id = ?", string(route), id)
	return err
}

// PruneCommands deletes commands seen before cutoff.
func (s *Store) PruneCommands(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM commands WHERE seen_ms < ?", cutoff.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func scanCommand(r scanner) (Command, error) {
	var c Command
	var types, route string
	var stale int
	err := r.Scan(&c.ID, &c.DeviceID, &c.DeviceName, &c.RequestKey, &c.Query, &c.Answer, &types, &c.UpstreamMS, &c.PrevPollMS, &c.SeenMS, &stale,
		&c.IntentKind, &c.IntentTitle, &c.IntentArtist, &c.IntentControl, &c.Stage, &c.NativeNote, &route)
	c.Stale = stale == 1
	if route != "" {
		c.Route = json.RawMessage(route)
	}
	c.AnswerTypes = []string{}
	if types != "" {
		c.AnswerTypes = strings.Split(types, ",")
	}
	return c, err
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
