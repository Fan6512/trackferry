package takeover

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"trackferry/internal/intent"
	"trackferry/internal/store"
)

type heardStarter struct {
	fakeStarter
	heard  []Heard
	spoken []string
	idle   bool
}

func (h *heardStarter) Hear(_ string, x Heard) { h.heard = append(h.heard, x) }

func (h *heardStarter) Playing(string) bool { return !h.idle }

func (h *heardStarter) Say(_ context.Context, _ string, work func() string) error {
	h.spoken = append(h.spoken, work())
	return nil
}

func TestFavoriteOfTheSongOnTheSpeaker(t *testing.T) {
	_, _, st := setup(t)
	sp := &heardStarter{}
	var got []int64
	d := &Dispatcher{Engine: sp, Store: st, Log: slog.New(slog.DiscardHandler), Allowed: func(string) bool { return true },
		Favorite: func(_ context.Context, id int64, on bool) error {
			if !on {
				id = -id
			}
			got = append(got, id)
			return nil
		}}
	say := func(control, mode string) {
		c := store.Command{DeviceID: "d1", RequestKey: control + mode, Query: "收藏", Answer: "好的", IntentKind: intent.KindFavorite, IntentControl: control}
		d.OnCommand(context.Background(), c, "X", mode)
	}
	say("add", store.ModeTakeover) // nothing was asked for
	d.Remember("d1", "晴天", "周杰伦", 0)
	say("add", store.ModeTakeover) // an online song
	if len(got) != 0 {
		t.Fatal(got)
	}
	d.Remember("d1", "晴天", "周杰伦", 7)
	say("add", store.ModeObserve) // only watching
	say("add", store.ModeTakeover)
	say("remove", store.ModeTakeover)
	if len(got) != 2 || got[0] != 7 || got[1] != -7 {
		t.Fatal(got)
	}
	want := []string{"没听出是哪首歌，没有收藏", "晴天不在曲库里，没法收藏", "已收藏晴天", "已取消收藏晴天"}
	if !slices.Equal(sp.spoken, want) || sp.released != 0 {
		t.Fatalf("%q", sp.spoken)
	}
	// Over music that is not ours, Xiao AI answers for herself.
	sp.idle = true
	d.Remember("d1", "晴天", "周杰伦", 0)
	say("add", store.ModeTakeover)
	if len(sp.spoken) != 4 {
		t.Fatalf("%q", sp.spoken)
	}
	d.Forget("d1")
	if s, _ := d.favorite(context.Background(), "d1", true); !strings.Contains(s, "不知道") {
		t.Fatal(s)
	}
}

func TestSaySilencesThenSpeaksThenPutsTheVolumeBack(t *testing.T) {
	e, sp, st := setup(t)
	ctx := context.Background()
	e.devs["d1"] = &device{cancel: func() {}, gen: 1, heard: make(chan Heard, 8)}
	err := e.Say(ctx, "d1", func() string { return "已收藏晴天" })
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(sp.calls, " ")
	if got != "status volume pause stop pause stop volume speak@40:已收藏晴天" {
		t.Fatal(got)
	}
	if _, ok, _ := st.PendingVolume(ctx, "d1"); ok || sp.volume != 40 {
		t.Fatal("volume not back")
	}
	if a, b := <-e.devs["d1"].heard, <-e.devs["d1"].heard; !a.Quiet || b.Quiet || b.Answer != 5 {
		t.Fatal(a, b)
	}

	// Something newer was said meanwhile: quiet, volume back.
	sp.calls = nil
	c, cancel := context.WithCancel(ctx)
	if err := e.Say(c, "d1", func() string { cancel(); return "x" }); err == nil || sp.volume != 40 {
		t.Fatal(err, sp.volume)
	}
	// A newer run owns the volume from here.
	sp.calls = nil
	e.Say(ctx, "d1", func() string { e.mu.Lock(); e.devs["d1"].gen++; e.mu.Unlock(); return "x" })
	st.ClearPendingVolume(ctx, "d1")

	// Nothing of ours on the speaker: only speak.
	sp.calls = nil
	if err := e.Say(ctx, "d2", func() string { return "好" }); err != nil || len(sp.calls) != 1 {
		t.Fatal(err, sp.calls)
	}
}
