package takeover

import (
	"context"
	"testing"
	"time"
)

func (f *fakeSpeaker) lastVolume() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.volume
}

func TestVolumeWhileMutedIsTheOneThatComesBack(t *testing.T) {
	e, sp, st := setup(t)
	if err := e.SetVolume(context.Background(), "d1", 12); err != nil || sp.lastVolume() != 12 {
		t.Fatal(err, sp.lastVolume())
	}
	e.Start(context.Background(), req(sp))
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end); time.Sleep(time.Millisecond) {
		if _, ok, _ := st.PendingVolume(context.Background(), "d1"); ok && e.held("d1") && sp.lastVolume() == 0 {
			break
		}
	}
	if err := e.SetVolume(context.Background(), "d1", 55); err != nil {
		t.Fatal(err)
	}
	if v := sp.lastVolume(); v != 0 && v != 55 {
		t.Fatal(v)
	}
	for end := time.Now().Add(2 * time.Second); sp.lastVolume() != 55; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal(sp.lastVolume())
		}
	}
}

func TestPressedPauseStaysPaused(t *testing.T) {
	e, sp, _ := setup(t)
	e.timing.Look, e.timing.LookClose, e.timing.Pause = 10*time.Millisecond, 10*time.Millisecond, 30*time.Millisecond
	r := req(sp)
	r.Track.Length = time.Hour
	r.Track.Resume = func(context.Context, time.Duration) (Track, time.Duration, error) {
		t.Error("continued by itself")
		return Track{}, 0, context.Canceled
	}
	e.Start(context.Background(), r)
	time.Sleep(700 * time.Millisecond)
	if err := e.Press(context.Background(), "d1", ControlPause); err != nil {
		t.Fatal(err)
	}
	sp.mu.Lock()
	sp.playing = ""
	sp.mu.Unlock()
	e.Woke("d1") // and waking it does not bring the song back
	time.Sleep(300 * time.Millisecond)
}
