package takeover

import (
	"context"
	"unicode/utf8"

	"trackferry/internal/logx"
)

// Playing reports that a song of ours is on the speaker, sounding or paused.
func (e *Engine) Playing(deviceID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.devs[deviceID]
	return d != nil && d.cancel != nil
}

// Say speaks the outcome of work on the speaker. Over a song of ours it first silences what
// Xiao AI is saying, and the song goes on afterwards. ctx ends when something newer was said:
// the work is still finished, but nothing is spoken any more.
func (e *Engine) Say(ctx context.Context, deviceID string, work func() string) error {
	text := make(chan string, 1)
	go func() {
		said := ""
		defer func() { text <- said }() // a fault in the work must not leave the speaker hushed
		defer logx.Recover(e.log, "接管")
		said = work()
	}()
	e.mu.Lock()
	gen, ours := 0, false
	if d := e.devs[deviceID]; d != nil {
		gen, ours = d.gen, d.cancel != nil
	}
	e.mu.Unlock()
	restore := func() {}
	if ours {
		// The watch must wait for us, however long the work takes.
		e.Hear(deviceID, Heard{Answer: 60, Quiet: true})
		letGo := e.hold(deviceID)
		defer letGo()
		restore = e.hush(ctx, deviceID, gen)
	}
	said := <-text
	restore()
	err := ctx.Err()
	if err == nil && said != "" {
		err = e.sp.Speak(ctx, deviceID, said)
	}
	n := 0
	if err == nil {
		n = utf8.RuneCountInString(said)
	}
	e.Hear(deviceID, Heard{Answer: n})
	return err
}

// hush mutes and stops the speaker; the returned function puts the volume back.
func (e *Engine) hush(ctx context.Context, dev string, gen int) func() {
	bg := context.WithoutCancel(ctx)
	if _, pending, err := e.st.PendingVolume(bg, dev); err != nil || pending {
		return func() {} // a starting run is holding the volume down already
	}
	restore := func() {}
	if p, err := e.sp.PlayStatus(ctx, dev); err == nil && p.Volume > 0 && e.st.SetPendingVolume(bg, dev, p.Volume, e.now()) == nil {
		restore = func() {
			e.mu.Lock()
			newer := e.devs[dev] != nil && e.devs[dev].gen != gen
			e.mu.Unlock()
			if newer {
				return // that run found the pending volume and restores it
			}
			e.putBack(dev, p.Volume)
		}
		e.sp.SetVolume(ctx, dev, 0) // even if this fails it may have arrived: restored either way
	}
	for i := 0; i < 2 && ctx.Err() == nil; i++ {
		if i > 0 && e.sleep(ctx, e.timing.Settle) != nil {
			break
		}
		e.sp.PlayerOp(ctx, dev, "pause")
		e.sp.PlayerOp(ctx, dev, "stop")
	}
	return restore
}
