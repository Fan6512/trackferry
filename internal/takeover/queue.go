package takeover

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	"trackferry/internal/live"
	"trackferry/internal/routing"
	"trackferry/internal/store"
)

// End is how a run ended.
type End int

const (
	EndOther    End = iota // stopped, replaced, or something else plays
	EndPlayed              // the song played through
	EndUnplayed            // the speaker did not play it
)

// queue is a list being played on a speaker, one song after another.
type queue struct {
	dev, hardware, list string
	ids                 []int64
	pos                 int
	turn                int  // which start is the current one
	fails               int  // songs in a row that did not play
	web                 bool // sent from the page, so watching is enough
	until               time.Time
}

// maxFails ends a queue whose songs do not play.
const maxFails = 3

// Drop ends the list a speaker plays; the song on it goes on.
func (d *Dispatcher) Drop(dev string) { d.unqueue(dev) }

func (d *Dispatcher) unqueue(dev string) *queue {
	d.mu.Lock()
	defer d.mu.Unlock()
	q := d.queues[dev]
	delete(d.queues, dev)
	return q
}

func (d *Dispatcher) enqueue(q *queue) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.queues == nil {
		d.queues = map[string]*queue{}
	}
	d.queues[q.dev] = q
}

func (d *Dispatcher) queued(dev string) *queue {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.queues[dev]
}

// Playing settings.
const (
	SettingArtist = "play.artist" // an artist's other songs follow: on (default) | off
	SettingAfter  = "play.after"  // random songs follow a single one: on | off (default)
	SettingHours  = "play.hours"  // how long songs follow on their own; 0: no limit
)

// DefaultHours ends a list nobody listens to any more.
const DefaultHours = 3

// Prefs is how songs follow one another.
type Prefs struct {
	Artist bool `json:"artist"`
	After  bool `json:"after"`
	Hours  int  `json:"hours"`
}

// LoadPrefs reads the settings.
func LoadPrefs(ctx context.Context, st *store.Store) Prefs {
	get := func(k string) string { v, _ := st.Setting(ctx, k); return v }
	p := Prefs{Artist: get(SettingArtist) != "off", After: get(SettingAfter) == "on", Hours: DefaultHours}
	if n, err := strconv.Atoi(get(SettingHours)); err == nil {
		p.Hours = n
	}
	return p
}

// SavePrefs stores the settings.
func SavePrefs(ctx context.Context, st *store.Store, p Prefs) error {
	sw := func(b bool) string {
		if b {
			return "on"
		}
		return "off"
	}
	return errors.Join(st.SetSetting(ctx, SettingArtist, sw(p.Artist)), st.SetSetting(ctx, SettingAfter, sw(p.After)),
		st.SetSetting(ctx, SettingHours, strconv.Itoa(p.Hours)))
}

// follow has ids play one after another on a speaker, the first of which is about to be
// started, and returns what the end of that one calls.
func (d *Dispatcher) follow(q *queue) func(End) {
	if h := LoadPrefs(context.Background(), d.Store).Hours; h > 0 {
		q.until = time.Now().Add(time.Duration(h) * time.Hour)
	}
	d.enqueue(q)
	return d.turn(q)
}

// afterwards is how many random songs are drawn to follow a single one.
const afterwards = 500

// After has random songs follow a single one, if that is wanted. track is 0 for a song that
// is not in the library.
func (d *Dispatcher) After(dev, hardware string, track int64, web bool) func(End) {
	if d.Random == nil || d.Next == nil || !LoadPrefs(context.Background(), d.Store).After {
		return nil
	}
	ids := []int64{track}
	for _, id := range d.Random(afterwards) {
		if id != track {
			ids = append(ids, id)
		}
	}
	if len(ids) < 2 {
		return nil
	}
	return d.follow(&queue{dev: dev, hardware: hardware, list: "接着随机放", ids: ids, web: web})
}

// PlayList plays a list on a speaker, in random order, until something else is asked for.
func (d *Dispatcher) PlayList(ctx context.Context, dev store.Device, list string, ids []int64) (store.TakeoverRun, error) {
	ids = slices.Clone(ids)
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	for i, id := range ids[:min(10, len(ids))] {
		rt, ok := d.Next(id, list)
		if !ok {
			continue
		}
		q := &queue{dev: dev.DeviceID, hardware: dev.Hardware, list: list, ids: append(ids[i:len(ids):len(ids)], ids[:i]...), web: true}
		return d.push(ctx, dev, rt, func() func(End) { return d.follow(q) })
	}
	return store.TakeoverRun{}, errors.New(list + "里的歌暂时读不到")
}

// Push plays one song that was picked by hand.
func (d *Dispatcher) Push(ctx context.Context, dev store.Device, rt *routing.Route) (store.TakeoverRun, error) {
	return d.push(ctx, dev, rt, func() func(End) { return d.After(dev.DeviceID, dev.Hardware, rt.TrackID, true) })
}

func (d *Dispatcher) push(ctx context.Context, dev store.Device, rt *routing.Route, ended func() func(End)) (store.TakeoverRun, error) {
	d.Halt(dev.DeviceID) // it wins over what is being prepared
	tr := d.Live.Begin(live.Flow{Kind: "push", DeviceID: dev.DeviceID, Device: d.name(dev.DeviceID), Said: rt.Label()})
	t, ok, err := d.TrackOf(rt, tr)
	if err == nil && !ok {
		err = errors.New("没有可放的音频")
	}
	if err != nil {
		tr.Done("准备音频失败")
		return store.TakeoverRun{}, err
	}
	d.Remember(dev.DeviceID, rt.Title, rt.Artist, rt.TrackID)
	song := &store.Song{TrackID: rt.TrackID, Title: rt.Title, Artist: rt.Artist, Album: rt.Album, Cover: rt.Cover, DurationMS: rt.DurationMS, List: rt.List}
	run, err := d.Engine.Start(ctx, Request{DeviceID: dev.DeviceID, Hardware: dev.Hardware, Kind: "push", Track: t, Song: song, Trace: tr, Ended: ended()})
	if err != nil {
		d.Drop(dev.DeviceID)
		tr.Done("没能开始")
	}
	return run, err
}

// turn makes the next start the current one and returns what its end calls.
func (d *Dispatcher) turn(q *queue) func(End) {
	d.mu.Lock()
	defer d.mu.Unlock()
	q.turn++
	n := q.turn
	return func(how End) {
		d.mu.Lock()
		on := d.queues[q.dev] == q && q.turn == n
		if on {
			switch how {
			case EndPlayed:
				q.fails = 0
			case EndUnplayed:
				q.fails++
			}
			if on = how != EndOther && q.fails < maxFails; !on {
				delete(d.queues, q.dev)
			}
		}
		d.mu.Unlock()
		if on {
			d.advance(q, n)
		}
	}
}

// step moves the queue and returns a song that can be read now. The list starts over, in a
// new order, when it is through.
func (d *Dispatcher) step(q *queue, by int) (*routing.Route, bool) {
	back := by < 0
	d.mu.Lock()
	was := q.ids[q.pos]
	d.mu.Unlock()
	for i := range min(40, 2*len(q.ids)) {
		d.mu.Lock()
		q.pos += by
		if q.pos < 0 {
			q.pos = 0
			by = 1 // if the first song cannot be read, the one after it
		}
		if q.pos >= len(q.ids) {
			rand.Shuffle(len(q.ids), func(i, j int) { q.ids[i], q.ids[j] = q.ids[j], q.ids[i] })
			q.pos = 0
		}
		id := q.ids[q.pos]
		d.mu.Unlock()
		if id == was && !back && i < len(q.ids)-1 {
			continue // not twice in a row, unless it is all there is
		}
		if rt, ok := d.Next(id, q.list); ok {
			return rt, true
		}
	}
	return nil, false
}

// advance plays the next song of a queue after turn n ended.
func (d *Dispatcher) advance(q *queue, n int) {
	ctx := context.Background()
	end := func(why string) {
		d.mu.Lock()
		if d.queues[q.dev] == q && q.turn == n {
			delete(d.queues, q.dev)
		}
		d.mu.Unlock()
		d.Log.Info("连播　结束", "说明", why)
	}
	dev, err := d.Store.Device(ctx, q.dev)
	if err != nil || dev.Mode == store.ModeOff || !dev.InAccount || !q.web && (dev.Mode != store.ModeTakeover || !d.Allowed(q.dev)) {
		end("音箱不再接管")
		return
	}
	if !q.until.IsZero() && time.Now().After(q.until) {
		end("放够了设定的最长时间")
		return
	}
	rt, ok := d.step(q, 1)
	if !ok {
		end("后面的歌暂时读不到")
		return
	}
	tr := d.Live.Begin(live.Flow{Kind: "push", DeviceID: q.dev, Device: d.name(q.dev), Said: rt.Label()})
	t, _, err := d.TrackOf(rt, tr)
	if err != nil {
		tr.Done("准备音频失败")
		end("准备音频失败：" + err.Error())
		return
	}
	if d.queued(q.dev) != q {
		tr.Done("有了更新的口令")
		return
	}
	ended := d.turn(q)
	d.Remember(q.dev, rt.Title, rt.Artist, rt.TrackID)
	song := &store.Song{TrackID: rt.TrackID, Title: rt.Title, Artist: rt.Artist, Album: rt.Album, Cover: rt.Cover, DurationMS: rt.DurationMS, List: q.list}
	if _, err := d.Engine.Start(ctx, Request{DeviceID: q.dev, Hardware: q.hardware, Kind: "push", Track: t, Song: song, Trace: tr, Follows: true, Ended: ended}); err != nil {
		tr.Done("没能开始")
		end("没能开始：" + err.Error())
	}
}

func (d *Dispatcher) name(dev string) string {
	if d.Name == nil {
		return ""
	}
	return d.Name(dev)
}

// skip answers "下一首" and "上一首" while a list plays.
func (d *Dispatcher) skip(q *queue, c store.Command, hardware string, tr *live.Trace) (*routing.Route, Request, bool) {
	by := 1
	if c.IntentControl == "previous" {
		by = -1
	}
	start := time.Now()
	rt, ok := d.step(q, by)
	if !ok {
		return nil, Request{}, false
	}
	t, _, err := d.TrackOf(rt, tr)
	if err != nil {
		return nil, Request{}, false
	}
	rt.MS = time.Since(start).Milliseconds()
	return rt, Request{DeviceID: q.dev, Hardware: hardware, Kind: "command", CommandID: c.ID, UpstreamMS: c.UpstreamMS, Track: t, Trace: tr}, true
}

// put adds the song on the speaker to a playlist, or takes it out, and says how it went.
func (d *Dispatcher) put(ctx context.Context, dev, name string, on bool) (string, string) {
	d.mu.Lock()
	l, ok := d.last[dev]
	d.mu.Unlock()
	song := "《" + l.Title + "》"
	switch {
	case !ok:
		return "不知道说的是哪首歌", "没听出是哪首歌"
	case l.track == 0:
		return song + "不在曲库里，加不进歌单", l.Title + "不在曲库里，没法加进歌单"
	case d.Put == nil:
		return "没有连接飞牛音乐", "还没连接飞牛音乐"
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	list, changed, err := d.Put(ctx, l.track, name, on)
	switch {
	case err != nil:
		d.Log.Warn("歌单　没有成功", "原因", err)
		said := l.Title + "没能加进歌单"
		if !on {
			said = l.Title + "没能移出歌单"
		}
		if list == "" { // which playlist: worth hearing, and it holds no addresses
			said = strings.TrimSpace(sayable(err))
		}
		return song + "：" + err.Error(), said
	case on && changed:
		return song + "已加入歌单“" + list + "”", "已加入" + list + "歌单"
	case on:
		return song + "已经在歌单“" + list + "”里", "已经在" + list + "歌单里了"
	case changed:
		return song + "已移出歌单“" + list + "”", "已移出" + list + "歌单"
	}
	return song + "不在歌单“" + list + "”里", "它不在" + list + "歌单里"
}

// sayable shortens an error for the speaker.
func sayable(err error) string {
	s, _, cut := strings.Cut(err.Error(), "：")
	if cut && !strings.HasPrefix(s, "有几个") {
		return s
	}
	return err.Error()
}
