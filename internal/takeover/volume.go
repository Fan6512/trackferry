package takeover

import (
	"context"
	"time"

	"trackferry/internal/logx"
)

// A speaker's volume is remembered, so that muting need not wait for an answer about it.
// "Nothing" is not 0 on most models: told 0 they go to the least they have, 5 of 100 on OH2P,
// and say so. Such a reading is what our own mute leaves behind far more often than what the
// listener chose, so it only counts once it stays.

const (
	// DefaultFloor is taken for models whose least volume is not known.
	DefaultFloor = 12
	// lowStays is how long a reading at the floor must last to be the listener's choice.
	lowStays = 40 * time.Second
	// settleAfter is how long after the volume was looked after a reading is not trusted.
	settleAfter = 4 * time.Second
	// refreshEvery is between looks at a speaker nobody is playing on.
	refreshEvery = 30 * time.Second
)

// checkAfter is when a volume that was put back is looked at again.
var checkAfter = []time.Duration{1500 * time.Millisecond, 4 * time.Second, 10 * time.Second}

// known is what is remembered of a speaker's volume.
type known struct {
	v     int
	low   int // a reading at the floor that has yet to last
	since time.Time
}

func (e *Engine) volume(dev string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.volumes[dev].v
}

// floor is the least volume a speaker goes down to when it is told 0.
func (e *Engine) floor(dev string) int {
	if e.Floor == nil {
		return DefaultFloor
	}
	return e.Floor(dev)
}

// know remembers a volume that was set on purpose.
func (e *Engine) know(dev string, v int) {
	if v <= 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.volumes[dev] = known{v: v}
}

// learn takes a reading of a speaker's volume into account.
func (e *Engine) learn(dev string, v int) {
	if v <= 0 {
		return
	}
	floor := e.floor(dev)
	e.mu.Lock()
	defer e.mu.Unlock()
	k := e.volumes[dev]
	switch {
	case k.v == 0, v > floor, v >= k.v:
		k = known{v: v}
	case k.low != v:
		k.low, k.since = v, e.now()
	case e.now().Sub(k.since) >= lowStays:
		k = known{v: v}
	}
	e.volumes[dev] = k
}

// calm reports that nothing has been done to a speaker's volume lately.
func (e *Engine) calm(dev string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.devs[dev]
	return d == nil || d.hold == 0 && e.now().Sub(d.letGo) > settleAfter
}

// Prime asks a speaker for its volume ahead of the next request.
func (e *Engine) Prime(ctx context.Context, dev string) {
	if !e.calm(dev) {
		return
	}
	if _, pending, err := e.st.PendingVolume(ctx, dev); err != nil || pending {
		return
	}
	if p, err := e.sp.PlayStatus(ctx, dev); err == nil && e.calm(dev) {
		e.learn(dev, p.Volume)
	}
}

// Refresh keeps the remembered volumes current until ctx ends. devices lists the speakers.
func (e *Engine) Refresh(ctx context.Context, devices func() []string) {
	for {
		func() {
			defer logx.Recover(e.log, "音箱")
			for _, dev := range devices() {
				c, cancel := context.WithTimeout(ctx, 6*time.Second)
				e.Prime(c, dev)
				cancel()
			}
		}()
		if e.sleep(ctx, refreshEvery) != nil {
			return
		}
	}
}

// Turned says the listener told Xiao AI something about the volume: whatever it is in a
// moment is meant, and is not corrected.
func (e *Engine) Turned(ctx context.Context, dev string) {
	e.touch(dev)
	if e.sleep(ctx, settleAfter) != nil {
		return
	}
	p, err := e.sp.PlayStatus(ctx, dev)
	if err != nil || e.held(dev) {
		return
	}
	if _, pending, err := e.st.PendingVolume(ctx, dev); err == nil && !pending {
		e.know(dev, p.Volume)
	}
}

// touch ends the checks on a volume put back earlier.
func (e *Engine) touch(dev string) int {
	d := e.device(dev)
	e.mu.Lock()
	defer e.mu.Unlock()
	d.touched++
	return d.touched
}

// check sees that a volume that was put back arrived and stays: the call may get lost, and
// a speaker may come back to the volume it was muted at.
func (e *Engine) check(dev string, v int) {
	if v <= e.floor(dev) {
		return
	}
	d, mark := e.device(dev), e.touch(dev)
	go func() {
		defer logx.Recover(e.log, "接管")
		var at time.Duration
		for _, after := range e.checks {
			time.Sleep(after - at)
			at = after
			if e.stale(d, mark, dev) {
				return
			}
			c, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			p, err := e.sp.PlayStatus(c, dev)
			if err == nil && !e.held(dev) && !e.stale(d, mark, dev) && p.Volume <= e.floor(dev) {
				err := e.sp.SetVolume(c, dev, v)
				e.log.Warn("接管　音量没有回去，再设一次", "读到", p.Volume, "应为", v, "结果", describe(err))
			}
			cancel()
		}
	}()
}

func (e *Engine) stale(d *device, mark int, dev string) bool {
	if _, pending, err := e.st.PendingVolume(context.Background(), dev); err != nil || pending {
		return true
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return d.touched != mark
}
