// Package home tells a home automation system what each speaker is doing: what TrackFerry
// plays and how far it is, and as much as the speaker itself reports otherwise. What is not
// known is null; nothing is guessed. Speakers are asked only while somebody listens.
package home

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"time"

	"trackferry/internal/live"
	"trackferry/internal/logx"
	"trackferry/internal/routing"
	"trackferry/internal/store"
	"trackferry/internal/xiaomi"
)

// SchemaVersion is raised when a field changes its meaning. New fields do not raise it.
const SchemaVersion = 1

// States of a speaker.
const (
	Playing = "playing"
	Paused  = "paused"
	Idle    = "idle"
	Off     = "off" // TrackFerry leaves this speaker alone
)

// Kinds of source.
const (
	Ours    = "trackferry"
	XiaoAI  = "xiaoai"
	Unknown = "unknown"
)

// Request is the last thing somebody asked a speaker to play.
type Request struct {
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// Speaker is one speaker. A null field is not known, or does not apply.
type Speaker struct {
	ID       string   `json:"speaker_id"`
	DID      string   `json:"miot_did,omitempty"`
	Name     string   `json:"name"`
	Model    string   `json:"model"`
	Mode     string   `json:"mode"`
	Online   bool     `json:"online"`
	Controls []string `json:"controls"` // what may be sent to .../control

	State      string  `json:"state"`
	Activity   *string `json:"activity"` // starting | talking | resuming
	SourceKind *string `json:"source_kind"`
	Source     *string `json:"source"` // library | kuwo | kugou | migu | netease

	PlayID       *int64   `json:"play_id"` // another number means another play, even of the same song
	TrackID      *int64   `json:"track_id"`
	Title        *string  `json:"title"`
	Artist       *string  `json:"artist"`
	Album        *string  `json:"album"`
	DurationMS   *int64   `json:"duration_ms"`
	PositionMS   *int64   `json:"position_ms"`
	PositionAt   *string  `json:"position_updated_at"`
	Rate         float64  `json:"playback_rate"`
	Volume       *int     `json:"volume"` // the listener's, also while TrackFerry mutes for a moment
	Muted        *bool    `json:"muted"`
	Cover        *string  `json:"cover"` // path below the service address
	CoverVersion *string  `json:"cover_version"`
	CoverColors  []string `json:"cover_colors"` // "#rrggbb", the most telling first
	Lyrics       *string  `json:"lyrics"`       // path below the service address
	Request      *Request `json:"last_request"`
	UpdatedAt    string   `json:"updated_at"`
	Stale        bool     `json:"stale"` // the speaker has not answered for a while
}

// Snapshot is everything.
type Snapshot struct {
	Schema     int       `json:"schema_version"`
	ServerTime string    `json:"server_time"`
	Instance   string    `json:"instance_id"` // another one after a restart; seq starts over
	Seq        int64     `json:"seq"`
	Speakers   []Speaker `json:"speakers"`
}

// Source is where the facts come from.
type Source struct {
	Hub     *live.Hub
	Devices func(context.Context) ([]store.Device, error)
	Pending func(context.Context) (map[string]int, error)
	Status  func(ctx context.Context, id string) (xiaomi.PlayStatus, error)
	Colors  func(ctx context.Context, cover string) []string // optional
}

// Cadence of asking speakers that do not play ours. Tests shorten it.
type Cadence struct {
	Tick, Merge, Busy, Idle time.Duration
}

// DefaultCadence keeps well below what the observer already asks.
var DefaultCadence = Cadence{Tick: time.Second, Merge: 150 * time.Millisecond, Busy: 5 * time.Second, Idle: 10 * time.Second}

const (
	staleAfter = 30 * time.Second
	drift      = 1500 * time.Millisecond // a position this far off the expected one is news
	startLag   = 500 * time.Millisecond  // from the first fetch to the first sound
)

type reading struct {
	p      xiaomi.PlayStatus
	ok     bool // ever answered
	at     time.Time
	due    time.Time
	again  time.Time // and once more then: what was asked for takes effect late
	busy   bool
	failed bool
}

// Tracker is safe for concurrent use. A nil Tracker does nothing.
type Tracker struct {
	src      Source
	log      *slog.Logger
	cad      Cadence
	now      func() time.Time
	instance string
	wake     chan struct{}

	updating sync.Mutex // one update at a time, taken before mu

	mu     sync.Mutex
	seq    int64
	last   map[string]Speaker
	order  []string
	reads  map[string]*reading
	said   map[string]Request
	colors map[string][]string
	subs   map[chan []byte]struct{}
}

// New returns a Tracker; Run makes it work.
func New(src Source, log *slog.Logger, cad Cadence) *Tracker {
	id := make([]byte, 6)
	rand.Read(id)
	return &Tracker{src: src, log: log, cad: cad, now: time.Now, instance: hex.EncodeToString(id), wake: make(chan struct{}, 1),
		last: map[string]Speaker{}, reads: map[string]*reading{}, said: map[string]Request{}, colors: map[string][]string{}, subs: map[chan []byte]struct{}{}}
}

// Run works until ctx ends. Without listeners it only remembers what was asked for.
func (t *Tracker) Run(ctx context.Context) {
	changed, stop := t.src.Hub.Changes()
	defer stop()
	tick := time.NewTicker(t.cad.Tick)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-t.wake:
		case <-changed:
			// What belongs together arrives together: one message for it.
			select {
			case <-ctx.Done():
				return
			case <-time.After(t.cad.Merge):
			}
		}
		t.round(ctx)
	}
}

func (t *Tracker) round(ctx context.Context) {
	defer logx.Recover(t.log, "智能家居")
	t.remember()
	if t.Listeners() > 0 {
		t.update(ctx, t.ask)
	}
}

// Listeners counts the open connections.
func (t *Tracker) Listeners() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.subs)
}

// Look has a speaker asked soon: somebody pressed a button.
func (t *Tracker) Look(id string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	if r := t.reads[id]; r != nil {
		r.due, r.again = t.now().Add(t.cad.Merge*2), t.now().Add(1500*time.Millisecond)
	}
	t.mu.Unlock()
	t.poke()
}

func (t *Tracker) poke() {
	select {
	case t.wake <- struct{}{}:
	default:
	}
}

// remember keeps the last request of each speaker; the hub forgets.
func (t *Tracker) remember() {
	flows := t.src.Hub.Flows()
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, f := range flows { // oldest first
		if f.Kind == "command" && f.DeviceID != "" && f.Said != "" {
			at := f.UpstreamMS
			if at == 0 {
				at = f.SeenMS
			}
			t.said[f.DeviceID] = Request{f.Said, time.UnixMilli(at).UTC()}
		}
	}
}

// ours is the flow whose song is on each speaker.
func ours(flows []live.Flow) map[string]live.Flow {
	out := map[string]live.Flow{}
	for _, f := range flows { // oldest first; the newest run is the one on
		if f.Done || f.DeviceID == "" {
			continue
		}
		if _, ok := f.Run.(store.TakeoverRun); ok {
			out[f.DeviceID] = f
		}
	}
	return out
}

// ask reads the speakers that are due, in the background.
func (t *Tracker) ask(ctx context.Context, id string) {
	t.mu.Lock()
	r := t.reads[id]
	if r == nil {
		r = &reading{}
		t.reads[id] = r
	}
	if r.busy || t.now().Before(r.due) {
		t.mu.Unlock()
		return
	}
	r.busy = true
	t.mu.Unlock()
	go t.read(ctx, id, r)
}

func (t *Tracker) read(ctx context.Context, id string, r *reading) {
	defer logx.Recover(t.log, "智能家居")
	c, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	p, err := t.src.Status(c, id)
	t.mu.Lock()
	r.busy = false
	r.failed = err != nil
	every := t.cad.Idle
	if err == nil {
		r.p, r.ok, r.at = p, true, t.now()
		if p.Status == xiaomi.PlayPlaying || p.Status == xiaomi.PlayPaused {
			every = t.cad.Busy
		}
	}
	r.due = t.now().Add(every)
	if r.again.After(t.now()) && r.again.Before(r.due) {
		r.due = r.again
	}
	t.mu.Unlock()
	t.poke()
}

// update rebuilds every speaker, tells the listeners what changed, and asks those that do not
// play ours.
func (t *Tracker) update(ctx context.Context, ask func(context.Context, string)) {
	t.updating.Lock()
	defer t.updating.Unlock()
	devs, err := t.src.Devices(ctx)
	if err != nil {
		return
	}
	pending, _ := t.src.Pending(ctx)
	mine := ours(t.src.Hub.Flows())
	var order []string
	fresh := map[string]Speaker{}
	for _, d := range devs {
		if !d.InAccount {
			continue
		}
		f, on := mine[d.DeviceID]
		if on || d.Mode == store.ModeOff { // what was read before says nothing about after
			t.mu.Lock()
			delete(t.reads, d.DeviceID)
			t.mu.Unlock()
		}
		if !on && d.Mode != store.ModeOff && ask != nil {
			ask(ctx, d.DeviceID)
		}
		var vol *int
		if v, ok := pending[d.DeviceID]; ok {
			vol = &v
		}
		order = append(order, d.DeviceID)
		fresh[d.DeviceID] = t.build(ctx, d, f, on, vol)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	if !slices.Equal(order, t.order) { // speakers came or went
		t.order, t.last = order, fresh
		t.seq++
		t.send("snapshot", t.snapshot(now))
		return
	}
	for _, id := range order {
		if !t.news(t.last[id], fresh[id], now) {
			continue
		}
		t.last[id] = fresh[id]
		t.seq++
		t.send("speaker", map[string]any{"schema_version": SchemaVersion, "server_time": stamp(now), "instance_id": t.instance, "seq": t.seq, "speaker": fresh[id]})
	}
}

// news reports a difference worth telling. A position that simply moved on is none.
func (t *Tracker) news(a, b Speaker, now time.Time) bool {
	if far(a, b, now) {
		return true
	}
	for _, s := range []*Speaker{&a, &b} {
		s.PositionMS, s.PositionAt, s.UpdatedAt = nil, nil, ""
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return !bytes.Equal(x, y)
}

func far(a, b Speaker, now time.Time) bool {
	if a.PositionMS == nil || b.PositionMS == nil {
		return a.PositionMS != b.PositionMS
	}
	d := a.at(now) - b.at(now)
	return d > drift.Milliseconds() || d < -drift.Milliseconds()
}

// at is the position at a moment.
func (s Speaker) at(now time.Time) int64 {
	p := *s.PositionMS
	if s.Rate > 0 && s.PositionAt != nil {
		if when, err := time.Parse(time.RFC3339Nano, *s.PositionAt); err == nil {
			p += now.Sub(when).Milliseconds()
		}
	}
	return p
}

func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

func ptr[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

func (t *Tracker) build(ctx context.Context, d store.Device, f live.Flow, on bool, pending *int) Speaker {
	now := t.now()
	s := Speaker{ID: d.DeviceID, DID: d.MiotDID, Name: d.Alias, Model: d.Hardware, Mode: d.Mode, Online: d.Presence == "online",
		Controls: []string{}, State: Off, CoverColors: []string{}, UpdatedAt: stamp(now)}
	if s.Name == "" {
		s.Name = d.Name
	}
	if d.Mode == store.ModeOff {
		return s
	}
	s.Controls = []string{"play", "pause", "stop", "volume"}
	if xiaomi.CanWake(d.Hardware, d.MiotDID) {
		s.Controls = append(s.Controls, "wake")
	}
	t.mu.Lock()
	if r, ok := t.said[d.DeviceID]; ok {
		s.Request = &r
	}
	var r reading
	if p := t.reads[d.DeviceID]; p != nil {
		r = *p
	}
	t.mu.Unlock()
	if on {
		t.song(ctx, &s, f, now)
	} else {
		other(&s, r, now)
	}
	if pending != nil {
		s.Volume = pending
	}
	if s.Volume != nil {
		m := *s.Volume == 0
		s.Muted = &m
	}
	return s
}

// song is a speaker that plays ours.
func (t *Tracker) song(ctx context.Context, s *Speaker, f live.Flow, now time.Time) {
	run := f.Run.(store.TakeoverRun)
	kind := Ours
	s.SourceKind, s.PlayID = &kind, ptr(run.ID)
	switch rt, ok := f.Route.(routing.Route); {
	case run.Song != nil:
		g := run.Song
		lib := "library"
		s.Source, s.TrackID, s.Title, s.Artist, s.Album, s.DurationMS = &lib, ptr(g.TrackID), ptr(g.Title), ptr(g.Artist), ptr(g.Album), ptr(g.DurationMS)
		t.cover(ctx, s, g.Cover)
	case ok:
		s.Source, s.TrackID, s.Title, s.Artist, s.Album, s.DurationMS = ptr(rt.Source), ptr(rt.TrackID), ptr(rt.Title), ptr(rt.Artist), ptr(rt.Album), ptr(rt.DurationMS)
		t.cover(ctx, s, rt.Cover)
	default: // a test sound
		s.Title = ptr(run.Source)
	}
	if s.TrackID != nil {
		l := "/api/ha/lyrics/" + strconv.FormatInt(*s.TrackID, 10)
		s.Lyrics = &l
	}
	s.State = Idle
	phase := ""
	if p := f.Player; p != nil {
		phase = p.Phase
		if p.Volume >= 0 {
			v := p.Volume
			s.Volume = &v
		}
		s.Stale = now.Sub(time.UnixMilli(p.AtMS)) > staleAfter
	}
	position := func(ms, at int64) {
		if s.DurationMS != nil {
			ms = min(ms, *s.DurationMS)
		}
		ms = max(0, ms)
		w := stamp(time.UnixMilli(at))
		s.PositionMS, s.PositionAt = &ms, &w
	}
	switch phase {
	case "": // being started
		s.Activity = ptr("starting")
		for _, x := range run.Steps {
			switch x.Step {
			case "fetched":
				at := run.StartedMS + x.AtMS + startLag.Milliseconds()
				if n := now.UnixMilli(); n >= at {
					position(n-at, n)
				}
			case "confirmed":
				s.State, s.Activity = Playing, nil
			}
		}
		if s.State != Playing {
			s.PositionMS, s.PositionAt = nil, nil
		}
	case "talking", "resuming":
		s.State, s.Activity = Paused, ptr(phase)
		if f.Player.Ours && phase == "talking" && !f.Player.Held {
			s.State = Playing
		}
		position(f.Player.Elapsed, f.Player.AtMS)
	default:
		s.State = phase
		position(f.Player.Elapsed, f.Player.AtMS)
	}
	if s.State == Playing {
		s.Rate = 1
	}
}

// other is a speaker that plays something else, or nothing.
func other(s *Speaker, r reading, now time.Time) {
	s.State = Idle
	if !r.ok {
		s.Stale = r.failed
		return
	}
	s.Stale = now.Sub(r.at) > staleAfter
	if r.p.Volume >= 0 {
		v := r.p.Volume
		s.Volume = &v
	}
	switch r.p.Status {
	case xiaomi.PlayPlaying:
		s.State, s.Rate = Playing, 1
	case xiaomi.PlayPaused:
		s.State = Paused
	default:
		return
	}
	kind := Unknown
	if r.p.AudioID != "" && r.p.AudioID != xiaomi.OurAudioID {
		kind = XiaoAI
	}
	s.SourceKind = &kind
	if r.p.Duration > 0 && r.p.Position >= 0 {
		pos, at := r.p.Position, stamp(r.at)
		s.DurationMS, s.PositionMS, s.PositionAt = ptr(r.p.Duration), &pos, &at
	}
}

// cover names the picture, and its colours once they are known.
func (t *Tracker) cover(ctx context.Context, s *Speaker, id string) {
	if id == "" {
		return
	}
	p := "/api/ha/cover/" + url.PathEscape(id)
	s.Cover, s.CoverVersion = &p, &id
	if t.src.Colors == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	got, asked := t.colors[id]
	if asked {
		s.CoverColors = append(s.CoverColors, got...)
		return
	}
	if len(t.colors) > 500 {
		clear(t.colors)
	}
	t.colors[id] = nil
	go func() {
		defer logx.Recover(t.log, "智能家居")
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if v := t.src.Colors(c, id); len(v) > 0 {
			t.mu.Lock()
			t.colors[id] = v
			t.mu.Unlock()
			t.poke()
		}
	}()
}

// snapshot needs t.mu.
func (t *Tracker) snapshot(now time.Time) Snapshot {
	out := Snapshot{Schema: SchemaVersion, ServerTime: stamp(now), Instance: t.instance, Seq: t.seq, Speakers: []Speaker{}}
	for _, id := range t.order {
		out.Speakers = append(out.Speakers, t.last[id])
	}
	return out
}

// Snapshot reads everything now. Speakers nobody has looked at lately are asked first.
func (t *Tracker) Snapshot(ctx context.Context) Snapshot {
	t.remember()
	var wg sync.WaitGroup
	t.update(ctx, func(ctx context.Context, id string) {
		t.mu.Lock()
		r := t.reads[id]
		if r == nil {
			r = &reading{}
			t.reads[id] = r
		}
		old := !r.busy && !t.now().Before(r.due)
		r.busy = r.busy || old
		t.mu.Unlock()
		if old {
			wg.Add(1)
			go func() { defer wg.Done(); t.read(ctx, id, r) }()
		}
	})
	wg.Wait()
	t.update(ctx, nil)
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snapshot(t.now())
}

// Subscribe returns messages ready to send, beginning with everything. The channel is closed
// when the reader is too slow, or when the token changed.
func (t *Tracker) Subscribe(ctx context.Context) (<-chan []byte, func()) {
	t.Snapshot(ctx)
	t.mu.Lock()
	ch := make(chan []byte, 64)
	ch <- encode("snapshot", t.instance, t.seq, t.snapshot(t.now()))
	t.subs[ch] = struct{}{}
	t.mu.Unlock()
	t.poke()
	return ch, func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.close(ch)
	}
}

// Drop ends every connection.
func (t *Tracker) Drop() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for ch := range t.subs {
		t.close(ch)
	}
}

func (t *Tracker) close(ch chan []byte) {
	if _, ok := t.subs[ch]; ok {
		delete(t.subs, ch)
		close(ch)
	}
}

// send needs t.mu.
func (t *Tracker) send(event string, v any) {
	if len(t.subs) == 0 {
		return
	}
	msg := encode(event, t.instance, t.seq, v)
	for ch := range t.subs {
		select {
		case ch <- msg:
		default:
			t.close(ch)
		}
	}
}

func encode(event, instance string, seq int64, v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	head := "id: " + instance + "-" + strconv.FormatInt(seq, 10) + "\nevent: " + event + "\ndata: "
	return append(append([]byte(head), b...), "\n\n"...)
}
