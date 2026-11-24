package routing

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"

	"trackferry/internal/intent"
	"trackferry/internal/library"
	"trackferry/internal/online"
	"trackferry/internal/store"
)

type silent struct{}

func (silent) Name() string                                               { return "kuwo" }
func (silent) Search(context.Context, string) ([]online.Candidate, error) { return nil, nil }
func (silent) Resolve(context.Context, online.Candidate) (string, error) {
	return "", online.ErrNoAudio
}

func TestCachedSongNeedsNoPlatform(t *testing.T) {
	ctx, log := context.Background(), slog.New(slog.DiscardHandler)
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sources := online.New(log, http.DefaultClient, silent{})
	var asked string
	r := &Router{Library: library.New(t.TempDir(), st, log), Online: sources, Log: log, Cached: func(a string) *online.Found {
		if asked = a; a != "晴天|周杰伦|" {
			return nil
		}
		return online.Saved{Source: "kuwo", ID: "1", Title: "晴天", Artist: "周杰伦", DurationMS: 9000}.Found()
	}}
	rt := r.Decide(ctx, intent.Parse("播放周杰伦的晴天"), "播放周杰伦的晴天")
	if rt.Decision != Online || rt.Found() == nil || rt.DurationMS != 9000 || rt.Asked() != asked {
		t.Fatalf("%s %+v", asked, rt)
	}
	if rt := r.Decide(ctx, intent.Parse("播放晴天"), "播放晴天"); rt.Decision != Native {
		t.Fatalf("no singer asked for: %+v", rt)
	}
	sources.SetEnabled("kuwo", false)
	if rt := r.Decide(ctx, intent.Parse("播放周杰伦的晴天"), "播放周杰伦的晴天"); rt.Decision != Native {
		t.Fatalf("switched off: %+v", rt)
	}
	after := After(ctx, Previous{Title: "晴天", Artist: "周杰伦", Tried: []string{"周杰伦"}})
	if rt := r.Decide(after, intent.Parse("换个版本"), "换个版本"); rt.Asked() != "" {
		t.Fatalf("another version: %+v", rt)
	}
}
