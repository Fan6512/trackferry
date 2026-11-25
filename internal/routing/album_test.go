package routing

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"trackferry/internal/intent"
	"trackferry/internal/library"
	"trackferry/internal/store"
)

func TestAlbums(t *testing.T) {
	ctx, log := context.Background(), slog.New(slog.DiscardHandler)
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	row := func(p, title, artist, album, id string) store.TrackRow {
		return store.TrackRow{Path: p, Title: title, Artist: artist, Album: album, AlbumID: id, GUID: p, DurationMS: 200000, Tagged: true}
	}
	if err := st.SaveTracks(ctx, 1, []store.TrackRow{
		row("1", "爱在西元前", "周杰伦", "范特西", "f"), row("2", "简单爱", "周杰伦", "范特西", "f"), row("3", "双截棍", "周杰伦", "范特西 (豪华版)", "g"),
		row("4", "翻唱", "别人", "范特西", "h"), row("5", "叶子", "阿桑", "受了点伤", "i"), row("6", "散歌", "阿桑", "", ""),
	}, nil); err != nil {
		t.Fatal(err)
	}
	lib := library.New("", st, log)
	if err := lib.Load(ctx); err != nil {
		t.Fatal(err)
	}
	r := &Router{Library: lib, Log: log}
	ask := func(q string, track int64) *Route {
		c := ctx
		if track != 0 {
			c = After(ctx, Previous{Title: "x", Track: track})
		}
		return r.Decide(c, intent.Parse(q), q)
	}
	for q, want := range map[string]string{"播放专辑范特西": "范特西", "播放周杰伦的专辑范特西": "范特西", "播放受了点伤这张专辑": "受了点伤", "播放受了点上专辑": "受了点伤", "播放范特西的专辑": "范特西"} {
		if rt := ask(q, 0); rt.Decision != Local || rt.List != "专辑“"+want+"”" || len(rt.Queue()) == 0 || rt.Queue()[0] != rt.TrackID {
			t.Errorf("%s: %+v", q, rt)
		}
	}
	if rt := ask("播放周杰伦的范特西专辑", 0); len(rt.Queue()) != 2 || rt.Artist != "周杰伦" {
		t.Errorf("%+v", rt)
	}
	loose := lib.Song("散歌", "", "散歌").Matches[0].ID
	leaf := lib.Song("叶子", "", "叶子").Matches[0].ID
	if rt := ask("播放这张专辑", leaf); rt.List != "专辑“受了点伤”" {
		t.Errorf("%+v", rt)
	}
	for q, c := range map[string]struct {
		track int64
		why   string
	}{"播放这张专辑": {0, "不知道"}, "播放这首歌的专辑": {loose, "没有专辑信息"}, "播放不存在的东西专辑": {0, "曲库里没有专辑"}} {
		if rt := ask(q, c.track); rt.Decision != Native || !strings.Contains(rt.Reason, c.why) {
			t.Errorf("%s: %+v", q, rt)
		}
	}
}
