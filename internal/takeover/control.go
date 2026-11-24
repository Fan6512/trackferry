package takeover

import "context"

// Control actions.
const (
	ControlPlay   = "play"
	ControlPause  = "pause"
	ControlStop   = "stop"
	ControlVolume = "volume"
	ControlWake   = "wake"
)

// Press does what a button asks for, and tells a song of ours that is on about it: it must
// not go on by itself after a pause.
func (e *Engine) Press(ctx context.Context, dev, action string) error {
	if action != ControlPlay {
		e.Release(dev) // a press outranks the guard
	}
	var err error
	switch action {
	case ControlStop:
		e.sp.PlayerOp(ctx, dev, "pause") // some models ignore a lone stop
		err = e.sp.PlayerOp(ctx, dev, "stop")
	default:
		err = e.sp.PlayerOp(ctx, dev, action)
	}
	if err != nil {
		return err
	}
	if action == ControlPlay {
		action = "resume"
	}
	e.Hear(dev, Heard{Control: action, Button: true})
	return nil
}

// SetVolume sets the volume. On a speaker that is muted for a moment it is the volume that
// comes back, so that Xiao AI's answer stays unheard.
func (e *Engine) SetVolume(ctx context.Context, dev string, volume int) error {
	if changed, err := e.st.ChangePendingVolume(ctx, dev, volume); err != nil || changed && e.held(dev) {
		return err
	}
	if err := e.sp.SetVolume(ctx, dev, volume); err != nil {
		return err
	}
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
