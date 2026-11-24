// Package observe polls the Xiao AI conversation records of enabled speakers and records
// the music commands it recognises. It never controls a speaker itself; a registered
// OnCommand callback (the takeover dispatcher) decides what to do with each command.
//
// Each speaker gets a watermark taken from Xiaomi's own record timestamps. The first
// successful read after start (or after the speaker is enabled) only sets the watermark,
// so commands spoken before TrackFerry was watching are never replayed. A failed read
// never moves the watermark: "could not read" is not "nothing new".
package observe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"trackferry/internal/intent"
	"trackferry/internal/live"
	"trackferry/internal/logx"
	"trackferry/internal/store"
	"trackferry/internal/xiaomi"
)

// Source reads conversation records (the xiaomi.Manager).
type Source interface {
	Conversations(ctx context.Context, deviceID, hardware string, limit int) ([]xiaomi.Record, error)
	Status() xiaomi.AccountStatus
}

// Device watch states.
const (
	StateStarting    = "starting"    // waiting for the first read to set the watermark
	StateListening   = "listening"   // watermark set; new commands are recorded
	StateError       = "error"       // reads are failing; retrying with backoff
	StatePaused      = "paused"      // no usable Xiaomi login
	StateUnsupported = "unsupported" // model whose records need another API
)

// staleAfter marks records that became visible long after Xiaomi recognised them. They are
// recorded but must never be acted on once takeover exists.
const staleAfter = 2 * time.Minute

// modelsWithoutRecords read conversations through a different (ubus) API, see xiaomusic
// GET_ASK_BY_MINA; not supported yet.
var modelsWithoutRecords = map[string]bool{"M01": true}

// DeviceStatus is one watched speaker.
type DeviceStatus struct {
	DeviceID   string    `json:"device_id"`
	Name       string    `json:"name"`
	State      string    `json:"state"`
	LastPollAt time.Time `json:"last_poll_at,omitzero"`
	LastOKAt   time.Time `json:"last_ok_at,omitzero"`
	Error      string    `json:"error,omitempty"`
	ErrorKind  string    `json:"error_kind,omitempty"`
	Recorded   int       `json:"recorded"` // music commands recorded since the watch started
	Ignored    int       `json:"ignored"`  // other utterances, counted but not stored
	Since      time.Time `json:"since"`
}

// Status is the observer as a whole.
type Status struct {
	Paused     string         `json:"paused,omitempty"` // unlinked | relink_required
	IntervalMS int64          `json:"interval_ms"`
	Devices    []DeviceStatus `json:"devices"`
}

type watch struct {
	status    DeviceStatus
	hardware  string
	mode      string
	watermark int64
	baseline  bool
	failures  int
	next      time.Time
	inflight  bool
}

// Observer is safe for concurrent use.
type Observer struct {
	src      Source
	st       *store.Store
	log      *slog.Logger
	interval time.Duration
	now      func() time.Time

	// Live, when set, hears about every look at a speaker.
	Live *live.Hub

	// OnOther, when set, hears that someone said something that is not a request. Its words
	// are used at once and never kept.
	OnOther func(deviceID string, r xiaomi.Record)

	// Follows reports that a song was asked for a moment ago, so "换个版本" means something.
	Follows func(deviceID string) bool

	onCommand func(ctx context.Context, c store.Command, hardware, mode string)

	mu      sync.Mutex
	watches map[string]*watch
	paused  string
	wg      sync.WaitGroup
}

// New builds an Observer polling every interval.
func New(src Source, st *store.Store, log *slog.Logger, interval time.Duration) *Observer {
	return &Observer{src: src, st: st, log: log, interval: interval, now: time.Now, watches: map[string]*watch{}}
}

// OnCommand registers f to be called (in its own goroutine) for every newly recorded
// command, with the speaker's hardware model and mode. Call before Run.
func (o *Observer) OnCommand(f func(ctx context.Context, c store.Command, hardware, mode string)) {
	o.onCommand = f
}

// Status reports every watched speaker.
func (o *Observer) Status() Status {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := Status{Paused: o.paused, IntervalMS: o.interval.Milliseconds(), Devices: []DeviceStatus{}}
	for _, w := range o.watches {
		out.Devices = append(out.Devices, w.status)
	}
	sort.Slice(out.Devices, func(i, j int) bool { return out.Devices[i].Name < out.Devices[j].Name })
	return out
}

// maxOffline bounds the wait while Xiaomi cannot be reached: once the line is back, the next
// request should be heard at once.
const maxOffline = 5 * time.Second

// pollTimeout bounds one read; a record read later than that is hardly worth waiting for.
const pollTimeout = 6 * time.Second

// maxRefused bounds the wait after Xiaomi refuses reads.
const maxRefused = 5 * time.Minute

// Run polls until ctx ends.
func (o *Observer) Run(ctx context.Context) {
	t := time.NewTicker(o.interval)
	defer t.Stop()
	var pruned time.Time
	for {
		if now := o.now(); now.Sub(pruned) > 24*time.Hour {
			pruned = now
			if n, err := o.st.Prune(ctx, now.AddDate(0, 0, -180)); err != nil {
				o.log.Warn("记录　清理没有完成", "原因", err)
			} else if n > 0 {
				o.log.Info("记录　清理过期", "条数", n)
			}
		}
		o.Tick(ctx)
		select {
		case <-ctx.Done():
			o.wg.Wait()
			return
		case <-t.C:
		}
	}
}

// Tick starts a read for every speaker that is due. Exposed for tests.
func (o *Observer) Tick(ctx context.Context) {
	devs, err := o.st.Devices(ctx)
	if err != nil {
		o.log.Warn("听歌　读不到音箱列表", "原因", err)
		return
	}
	acct := o.src.Status()
	paused := ""
	switch {
	case acct.RelinkRequired:
		paused = xiaomi.SyncRelinkRequired
	case !acct.Linked:
		paused = "unlinked"
	}
	now := o.now()
	o.mu.Lock()
	defer o.mu.Unlock()
	o.paused = paused
	active := map[string]bool{}
	for _, d := range devs {
		if d.Mode == store.ModeOff || !d.InAccount {
			continue
		}
		active[d.DeviceID] = true
		w := o.watches[d.DeviceID]
		if w == nil {
			w = &watch{status: DeviceStatus{DeviceID: d.DeviceID, State: StateStarting, Since: now}}
			o.watches[d.DeviceID] = w
		}
		w.status.Name = firstNonEmpty(d.Alias, d.Name, d.DeviceID)
		w.hardware, w.mode = d.Hardware, d.Mode
		if modelsWithoutRecords[strings.ToUpper(d.Hardware)] {
			w.status.State, w.status.Error = StateUnsupported, "这个型号的对话记录要走另一种接口，暂不支持"
			continue
		}
		if paused != "" {
			// A watch that resumes after a pause starts from a fresh watermark.
			w.status.State, w.baseline = StatePaused, false
			continue
		}
		if w.inflight || now.Before(w.next) {
			continue
		}
		w.inflight = true
		o.wg.Add(1)
		go o.poll(ctx, d.DeviceID, w)
	}
	// Speakers switched off (or gone) lose their watch; re-enabling sets a new watermark.
	for id := range o.watches {
		if !active[id] {
			delete(o.watches, id)
		}
	}
}

func (o *Observer) poll(ctx context.Context, id string, w *watch) {
	defer o.wg.Done()
	defer logx.Recover(o.log, "听歌")
	defer func() { // also after a fault: the speaker must be listened to again
		o.mu.Lock()
		w.inflight = false
		o.mu.Unlock()
	}()
	o.mu.Lock()
	hardware := w.hardware
	o.mu.Unlock()
	started := o.now()
	rctx, cancel := context.WithTimeout(ctx, pollTimeout)
	recs, err := o.src.Conversations(rctx, id, hardware, 5)
	cancel()

	o.mu.Lock()
	defer o.mu.Unlock()
	w.status.LastPollAt = started
	took := o.now().Sub(started).Milliseconds()
	defer func() {
		if o.watches[id] == w {
			o.Live.Pulse(live.Pulse{DeviceID: id, Name: w.status.Name, State: w.status.State, OK: err == nil, MS: took,
				Ignored: w.status.Ignored, Recorded: w.status.Recorded, Every: o.interval.Milliseconds()})
		}
	}()
	if o.watches[id] != w {
		return // switched off meanwhile
	}
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		w.failures++
		backoff := o.interval << min(w.failures, 5)
		if k := xiaomi.KindOf(err); k == xiaomi.KindNetwork {
			backoff = min(backoff, maxOffline)
		} else if w.failures > 3 { // refused: leave Xiaomi alone for a while
			backoff = min(o.interval<<min(w.failures, 10), maxRefused)
		}
		w.next = started.Add(backoff)
		w.status.Error, w.status.ErrorKind = explain(err), xiaomi.KindOf(err)
		if w.failures >= 2 {
			w.status.State = StateError
		}
		if w.failures == 2 || w.failures%20 == 0 {
			o.log.Warn("听歌　读取失败", "音箱", w.status.Name, "连续", w.failures, "类型", xiaomi.KindOf(err), "原因", err)
		}
		return
	}
	prevOK := w.status.LastOKAt
	w.failures, w.next = 0, time.Time{}
	w.status.LastOKAt, w.status.Error, w.status.ErrorKind = started, "", ""
	sort.Slice(recs, func(i, j int) bool { return recs[i].Time < recs[j].Time })
	if !w.baseline {
		for _, r := range recs {
			w.watermark = max(w.watermark, r.Time)
		}
		w.baseline = true
		w.status.State = StateListening
		o.log.Info("听歌　开始", "音箱", w.status.Name, "跳过旧记录", len(recs))
		return
	}
	w.status.State = StateListening
	for _, r := range recs {
		if r.Time <= w.watermark {
			continue
		}
		w.watermark = r.Time
		o.handle(ctx, id, w, r, prevOK, started)
	}
}

// handle records one new utterance. Called with o.mu held.
func (o *Observer) handle(ctx context.Context, id string, w *watch, r xiaomi.Record, prevOK, seen time.Time) {
	in := intent.Parse(r.Query)
	if !in.Music() || in.Follow && (o.Follows == nil || !o.Follows(id)) {
		w.status.Ignored++
		if o.OnOther != nil {
			o.OnOther(id, r)
		}
		return
	}
	key := r.RequestID
	if key == "" {
		key = fmt.Sprintf("t%d|%s", r.Time, r.Query)
	}
	prev := prevOK.UnixMilli()
	if prevOK.IsZero() {
		prev = seen.UnixMilli()
	}
	lag := seen.UnixMilli() - r.Time
	c := store.Command{
		DeviceID: id, RequestKey: key, Query: r.Query, Answer: r.Answer, AnswerTypes: r.AnswerTypes,
		UpstreamMS: r.Time, PrevPollMS: prev, SeenMS: seen.UnixMilli(), Stale: lag > staleAfter.Milliseconds(),
		IntentKind: in.Kind, IntentTitle: in.Title, IntentArtist: in.Artist, IntentControl: in.Control,
		Stage: store.StageObserved,
	}
	ok, err := o.st.AddCommand(ctx, &c)
	if err != nil {
		o.log.Warn("听歌　记录没有存上", "原因", err)
		return
	}
	if ok {
		w.status.Recorded++
		o.log.Info("点歌　听到", "音箱", w.status.Name, "种类", in.Kind, "延迟ms", lag, "过时", c.Stale)
		if o.onCommand != nil {
			o.wg.Add(1)
			go func(hardware, mode string) {
				defer o.wg.Done()
				defer logx.Recover(o.log, "点歌")
				o.onCommand(ctx, c, hardware, mode)
			}(w.hardware, w.mode)
		}
	}
}

func explain(err error) string {
	if errors.Is(err, xiaomi.ErrNotLinked) {
		return "没有可用的小米登录"
	}
	switch xiaomi.KindOf(err) {
	case xiaomi.KindNetwork:
		return "连不上小米服务器"
	case xiaomi.KindAuth:
		return "小米登录已失效，需要重新登录"
	case xiaomi.KindProtocol:
		return "小米返回了看不懂的对话记录"
	default:
		return "小米拒绝了读取对话记录的请求"
	}
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
