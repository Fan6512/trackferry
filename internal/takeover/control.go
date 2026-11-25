package takeover

import (
	"context"
	"time"

	"trackferry/internal/logx"
)

// Control actions.
const (
	ControlPlay   = "play"
	ControlPause  = "pause"
	ControlStop   = "stop"
	ControlVolume = "volume"
	ControlWake   = "wake"

	ControlNext     = "next"
	ControlPrevious = "previous"
	ControlMode     = "mode"
)

// pressed is how long a speaker gets to do what a button asked before it is looked at.
const pressed = 350 * time.Millisecond

// Press does what a button asks for, and makes sure it was done: speakers confirm a pause
// they then ignore.
func (e *Engine) Press(ctx context.Context, dev, action string) error {
	d := e.device(dev)
	e.mu.Lock()
	on, watching, cut, gen := d.cancel != nil, d.watching, d.cut, d.gen
	e.mu.Unlock()
	switch {
	case action == ControlPlay && on && watching:
		e.Hear(dev, Heard{Control: "resume", Button: true})
		e.mu.Lock()
		d.cut = false
		e.mu.Unlock()
		if cut {
			return nil // the song is pushed again from where it was
		}
		return e.sp.PlayerOp(ctx, dev, ControlPlay)
	case action == ControlPlay:
		return e.sp.PlayerOp(ctx, dev, ControlPlay)
	case action == ControlPause && (!on || watching):
		return e.pause(ctx, dev, d, gen, on)
	}
	// Stop. A song that is only being started cannot be paused, so that stops it too.
	e.mu.Lock()
	if d.cancel != nil {
		d.cancel(errStopped)
	}
	e.mu.Unlock()
	err := e.halt(ctx, dev)
	if err != nil && ctx.Err() == nil {
		err = e.halt(ctx, dev)
	}
	// A push may have been on its way, and some models need to be told again.
	go func() {
		defer logx.Recover(e.log, "音箱")
		bg := context.WithoutCancel(ctx)
		for i, quiet := 0, 0; i < 6 && quiet < 2; i++ {
			e.sleep(bg, pressed)
			e.mu.Lock()
			newer := d.gen != gen
			e.mu.Unlock()
			if newer {
				return
			}
			if p, err := e.sp.PlayStatus(bg, dev); err == nil && !p.Playing() {
				quiet++
				continue
			}
			quiet = 0
			e.halt(bg, dev)
		}
	}()
	return err
}

// pause pauses, asks again, and at last stops: a song of ours can be continued from there.
func (e *Engine) pause(ctx context.Context, dev string, d *device, gen int, ours bool) error {
	e.Hear(dev, Heard{Control: "pause", Button: true})
	var err error
	for range 2 {
		if err = e.sp.PlayerOp(ctx, dev, ControlPause); err == nil {
			e.sleep(ctx, pressed)
			if p, serr := e.sp.PlayStatus(ctx, dev); serr == nil && !p.Playing() {
				return nil
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	if !ours || ctx.Err() != nil {
		return err
	}
	if err = e.halt(ctx, dev); err != nil {
		e.Hear(dev, Heard{Control: "resume", Button: true}) // it goes on
		return err
	}
	e.mu.Lock()
	if d.gen == gen {
		d.cut = true
	}
	e.mu.Unlock()
	e.Hear(dev, Heard{Control: "pause", Button: true, Cut: true})
	return nil
}

// SetVolume sets the volume. On a speaker that is muted for a moment it is the volume that
// comes back, so that Xiao AI's answer stays unheard.
func (e *Engine) SetVolume(ctx context.Context, dev string, volume int) error {
	if changed, err := e.st.ChangePendingVolume(ctx, dev, volume); err != nil || changed && e.held(dev) {
		return err
	}
	e.touch(dev)
	if err := e.sp.SetVolume(ctx, dev, volume); err != nil {
		return err
	}
	e.know(dev, volume)
	if !e.held(dev) {
		e.st.ClearPendingVolume(ctx, dev)
	}
	return nil
}

// Woke tells a song of ours that its speaker was woken.
func (e *Engine) Woke(dev string) {
	e.Release(dev)
	e.Hear(dev, Heard{Woke: true})
}

// Describe explains a failed control.
func Describe(err error) string { return describe(err) }
