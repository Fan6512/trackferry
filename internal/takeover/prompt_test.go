package takeover

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"trackferry/internal/store"
	"trackferry/internal/xiaomi"
)

// stalling answers status reads late; which ones, slow decides by their number.
type stalling struct {
	Speaker
	n    atomic.Int32
	slow func(n int32) bool
}

func (s *stalling) PlayStatus(ctx context.Context, dev string) (xiaomi.PlayStatus, error) {
	if s.slow(s.n.Add(1)) {
		<-ctx.Done()
		return xiaomi.PlayStatus{}, ctx.Err()
	}
	return s.Speaker.PlayStatus(ctx, dev)
}

func TestALateStatusIsAskedForAgain(t *testing.T) {
	s := &stalling{Speaker: newSpeaker(), slow: func(n int32) bool { return n == 1 }}
	from := time.Now()
	p, err := prompt{s}.PlayStatus(context.Background(), "d")
	if took := time.Since(from); err != nil || p.Volume != 40 || took < askAgain || took > 2*askAgain {
		t.Fatal(p, err, took)
	}
	if s.n.Load() != 2 {
		t.Fatal(s.n.Load())
	}
	sp := newSpeaker()
	sp.statusOK = false
	q := &stalling{Speaker: sp, slow: func(int32) bool { return false }}
	if _, err := (prompt{q}).PlayStatus(context.Background(), "d"); err == nil || q.n.Load() != 1 {
		t.Fatal("a plain error is the answer", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (prompt{&stalling{slow: func(int32) bool { return true }}}).PlayStatus(ctx, "d"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestVolumeComesBackWhileTheStatusHangs(t *testing.T) {
	e, sp, _ := setup(t)
	var pushed atomic.Bool
	var at time.Time
	sp.playHook = func() { pushed.Store(true); at = time.Now() }
	var back time.Duration
	sp.volHook = func(v int) error {
		if v > 0 && back == 0 {
			back = time.Since(at)
		}
		return nil
	}
	e.sp = prompt{&stalling{Speaker: sp, slow: func(int32) bool { return pushed.Load() }}}
	run, err := e.Run(context.Background(), req(sp))
	if err != nil || run.Outcome != store.OutcomeFetched {
		t.Fatal(err, run.Outcome, steps(run))
	}
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if back == 0 || back > time.Second || sp.volume != 40 {
		t.Fatal("still muted after", back, sp.volume, steps(run))
	}
}
