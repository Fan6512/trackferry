package takeover

import (
	"fmt"
	"time"

	"trackferry/internal/xiaomi"
)

const (
	phasePlaying  = "playing"
	phasePaused   = "paused"
	phaseTalking  = "talking"
	phaseResuming = "resuming"
)

// maxPause is how long a paused song is waited for.
const maxPause = 30 * time.Minute

// startLag is from the speaker's first request to the first sound.
const startLag = 500 * time.Millisecond

// watch follows our song until it is over. It keeps the song's clock, because speakers do not
// report a position for pushed audio; it continues the song after someone talked to Xiao AI;
// and it stops the speaker once the song has played through, so it does not repeat. A newer
// run on the speaker cancels the watch (and owns the speaker).
func (r *runner) watch(fetchedAt time.Time, heard <-chan Heard) {
	e, t := r.e, r.req.Track
	tm := e.timing
	if tm.Look <= 0 {
		tm.Look, tm.LookClose = DefaultTiming.Look, DefaultTiming.LookClose
	}
	length := t.Length
	if length <= 0 {
		length = t.Duration
	}
	track := t
	var ended <-chan time.Time
	if t.Duration > 0 {
		x := time.NewTimer(time.Until(fetchedAt.Add(t.Duration + tm.EndSlack)))
		defer x.Stop()
		ended = x.C
	}
	limit := time.NewTimer(tm.MaxWatch)
	defer limit.Stop()

	base := time.Duration(0)          // played before anchor
	anchor := fetchedAt.Add(startLag) // sounding since
	sounding := true
	at := func() time.Duration {
		if !sounding {
			return base
		}
		return base + max(0, e.now().Sub(anchor))
	}
	var (
		talkedUntil time.Time // someone talked; Xiao AI is done about then
		silent      int       // status reads in a row without sound
		foreign     int
		byUser      bool // the listener wants quiet
		yield       bool // Xiao AI was asked to play something
		pausedAt    time.Time
		resumes     int
		lastOK      = e.now()
	)
	pause := func(when time.Time) {
		if !sounding {
			return
		}
		base = max(0, base+when.Sub(anchor))
		if length > 0 {
			base = min(base, length)
		}
		sounding, pausedAt = false, e.now()
	}
	play := func(from time.Duration, when time.Time) {
		base, anchor, sounding, pausedAt = from, when, true, time.Time{}
	}
	over := func(step, why string) { r.phase = ""; r.step(step, true, why); r.status() }
	finish := func(why string) {
		p, err := r.status()
		if err == nil && !p.Playing() {
			r.step(StepEnded, true, why+"，音箱已经停了")
			return
		}
		if err == nil && r.foreign(p) {
			r.step(StepEnded, true, why+"，音箱在放别的，不动它")
			return
		}
		// Left alone the speaker would repeat the song for good.
		for i := 0; i < 4; i++ {
			e.sp.PlayerOp(r.ctx, r.req.DeviceID, "pause")
			if err = e.sp.PlayerOp(r.ctx, r.req.DeviceID, "stop"); err == nil || e.sleep(r.ctx, time.Duration(i+1)*time.Second) != nil {
				break
			}
		}
		r.step(StepEnded, err == nil, why+"，停止循环 "+describe(err))
	}
	// again pushes the song from where it stopped.
	again := func() bool {
		if t.Resume == nil || resumes >= 20 {
			return false
		}
		resumes++
		from := max(time.Second, base-1500*time.Millisecond)
		r.phase = phaseResuming
		n, start, err := t.Resume(r.ctx, from)
		if err != nil { // a format that only starts at its beginning
			start = 0
			n = Track{URL: t.URL, Lost: t.Lost}
		}
		err = e.sp.PlayURL(r.ctx, r.req.DeviceID, r.req.Hardware, n.URL)
		for i := 0; err != nil && i < 3 && e.sleep(r.ctx, time.Duration(i+1)*400*time.Millisecond) == nil; i++ {
			err = e.sp.PlayURL(r.ctx, r.req.DeviceID, r.req.Hardware, n.URL)
		}
		if err != nil {
			r.step(StepResume, false, describe(err))
			return false
		}
		select {
		case <-n.Fetched: // nil when starting over: wait a moment instead
		case <-time.After(6 * time.Second):
			if n.Fetched != nil {
				r.step(StepResume, false, "音箱没有来取流")
				return false
			}
		case <-r.ctx.Done():
			return false
		}
		track = n
		play(start, e.now().Add(startLag))
		r.phase = phasePlaying
		if start == 0 {
			r.step(StepResume, true, "这个格式只能从头放")
		} else {
			r.step(StepResume, true, fmt.Sprintf("从 %d:%02d 接着放", int(start.Minutes()), int(start.Seconds())%60))
		}
		return true
	}

	timer := time.NewTimer(tm.Look)
	defer timer.Stop()
	r.phase = phasePlaying
	for {
		close := !talkedUntil.IsZero() || (!sounding && e.now().Sub(pausedAt) < 20*time.Second)
		if close {
			timer.Reset(tm.LookClose)
		} else {
			look := tm.Look
			if !sounding && e.now().Sub(pausedAt) > 2*time.Minute {
				look = max(look, min(10*time.Second, 4*look))
			}
			timer.Reset(look)
		}
		select {
		case <-r.ctx.Done():
			r.phase = ""
			r.step(StepCancelled, true, "")
			return
		case <-limit.C:
			return
		case <-ended:
			finish("播完了")
			return
		case <-track.Restarted:
			if length > 0 && at() < length*8/10 {
				track.Restarted = nil // it went back to the beginning; the song is not over
				play(0, e.now().Add(startLag))
				continue
			}
			finish("音箱又从头放了")
			return
		case h := <-heard:
			switch {
			case h.Control == "pause", h.Control == "stop", h.Stop:
				byUser, talkedUntil = true, time.Time{}
			case h.Control == "resume":
				byUser = false
				talkedUntil = e.now().Add(tm.Pause)
			case h.Control != "", h.Audio:
				yield, talkedUntil = true, time.Time{}
			default:
				byUser, yield = false, false
				if !h.Quiet {
					r.step(StepTalked, true, "")
				}
				talkedUntil = h.At.Add(tm.Pause + min(45*time.Second, time.Duration(h.Answer)*tm.Speak))
				if x := e.now().Add(tm.Pause); talkedUntil.Before(x) {
					talkedUntil = x
				}
			}
			continue
		case <-timer.C:
		}

		talking := !talkedUntil.IsZero() && e.now().Before(talkedUntil)
		switch {
		case talking:
			r.phase = phaseTalking
		case sounding:
			r.phase = phasePlaying
		default:
			r.phase = phasePaused
		}
		r.elapsed = at()
		p, err := r.status()
		if err != nil {
			continue
		}
		now := e.now()
		lost := track.Lost != nil && track.Lost()
		// Xiao AI's voice can read as "playing" too; a speaker that hung up is not playing ours.
		ours := p.Playing() && !r.foreign(p) && !(lost && (talking || !sounding))
		switch {
		case ours:
			silent, foreign = 0, 0
			if !sounding && !talking {
				play(base, lastOK.Add(now.Sub(lastOK)/2))
				r.phase = phasePlaying
				byUser = false
				r.step(StepContinued, true, "")
			}
			if !talkedUntil.IsZero() && now.After(talkedUntil.Add(2*time.Second)) {
				talkedUntil = time.Time{}
			}
			if length > 0 && at() > length+8*time.Second {
				finish("播完了")
				return
			}
		default:
			if p.Playing() {
				foreign++
			} else {
				foreign = 0
			}
			silent++
			if sounding {
				when := lastOK.Add(now.Sub(lastOK) / 2)
				if length > 0 && at() > length-6*time.Second {
					r.phase = ""
					finish("播完了")
					return
				}
				pause(when)
				r.phase = phasePaused
				if talking {
					r.phase = phaseTalking
				}
				r.elapsed = base
				r.step(StepPaused, true, "")
			}
			switch {
			case yield && foreign >= 2, yield && now.Sub(pausedAt) > 15*time.Second:
				over(StepStopped, "小爱在放别的")
				return
			case foreign >= 12:
				over(StepStopped, "音箱在放别的")
				return
			case byUser || talkedUntil.IsZero():
				if now.Sub(pausedAt) > maxPause {
					over(StepStopped, "停了很久，不再等")
					return
				}
			case !talking && silent >= 2 && !p.Playing():
				talkedUntil, silent = time.Time{}, 0
				if !again() && r.ctx.Err() == nil {
					over(StepStopped, "没能接着放")
					return
				}
			}
		}
		lastOK = now
	}
}

// foreign reports that the speaker plays something that is certainly not ours.
func (r *runner) foreign(p xiaomi.PlayStatus) bool {
	return p.Playing() && xiaomi.UsesMusicAPI(r.req.Hardware) && p.AudioID != "" && p.AudioID != xiaomi.OurAudioID
}
