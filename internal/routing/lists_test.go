package routing

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"trackferry/internal/feiniu"
	"trackferry/internal/intent"
	"trackferry/internal/library"
	"trackferry/internal/store"
)

type nas struct {
	off   bool
	lists []feiniu.Playlist
	put   []string
}

func song(g string) feiniu.Song {
	s := feiniu.Song{GUID: g, Title: "歌" + g, Duration: 200000}
	s.Audio.Path = "/vol1/1000/音乐/" + g + ".mp3"
	return s
}

func (n *nas) Linked() bool { return true }
func (n *nas) Songs(context.Context, int, int) ([]feiniu.Song, int, error) {
	return []feiniu.Song{song("a"), song("b"), song("c")}, 3, nil
}
func (n *nas) Favorites(context.Context) ([]feiniu.Song, error) {
	if n.off {
		return nil, errors.New("连不上")
	}
	return []feiniu.Song{song("a"), song("c"), song("new")}, nil
}
func (n *nas) Playlists(context.Context) ([]feiniu.Playlist, error) { return n.lists, nil }
func (n *nas) PlaylistSongs(_ context.Context, g string) ([]feiniu.Song, error) {
	if g == "empty" {
		return nil, nil
	}
	return []feiniu.Song{song("a"), song("b")}, nil
}
func (n *nas) PutInPlaylist(_ context.Context, p, g string, on bool) (bool, error) {
	n.put = append(n.put, p+g)
	return true, nil
}

func TestLists(t *testing.T) {
	ctx, log := context.Background(), slog.New(slog.DiscardHandler)
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	n := &nas{lists: []feiniu.Playlist{{GUID: "p", Name: "测试"}, {GUID: "empty", Name: "空的"}, {GUID: "q", Name: "周末开车"}}}
	lib := library.New("", st, log)
	lib.Remote = n
	if err := lib.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	r := &Router{Library: lib, Lists: n, Log: log}
	ask := func(q string) *Route { return r.Decide(ctx, intent.Parse(q), q) }
	seen := map[string]bool{}
	for range 60 {
		rt := ask("播放测试歌单")
		if rt.Decision != Local || len(rt.Queue()) != 2 || rt.Queue()[0] != rt.TrackID || rt.List != "歌单“测试”" {
			t.Fatalf("%+v", rt)
		}
		seen[rt.Title] = true
	}
	if len(seen) != 2 {
		t.Error("not shuffled", seen)
	}
	if rt := ask("播放我的收藏"); rt.Decision != Local || len(rt.Queue()) != 2 || rt.List != "我的收藏" || !strings.Contains(rt.Layers[0].Detail, "另有 1 首") {
		t.Errorf("%+v", rt)
	}
	for q, why := range map[string]string{"播放空的歌单": "没有歌", "播放不存在歌单": "有测试、空的、周末开车", "播放我的歌单": "没说是哪个"} {
		if rt := ask(q); rt.Decision != Native || !strings.Contains(rt.Reason, why) {
			t.Errorf("%s: %s", q, rt.Reason)
		}
	}
	for _, q := range []string{"播放开车歌单", "播放周末开铺歌单"} {
		if rt := ask(q); rt.List != "歌单“周末开车”" {
			t.Errorf("%s: %+v", q, rt)
		}
	}
	id := ask("播放测试歌单").TrackID
	if rt, ok := r.Next(id, "x"); !ok || rt.Decision != Local || rt.Match() == nil || rt.List != "x" {
		t.Error("next")
	}
	if list, ok, err := r.Put(ctx, id, "测试", true); err != nil || !ok || list != "测试" || len(n.put) != 1 {
		t.Error(list, err, n.put)
	}
	if _, _, err := r.Put(ctx, id, "没有", true); err == nil || len(n.put) != 1 {
		t.Error("put into nothing")
	}
	n.off = true // the last sync's favourites are used
	if rt := ask("播放我的收藏"); !strings.Contains(rt.Reason, "没能读到") {
		t.Error(rt.Reason)
	}
	r.Openable = func(string) bool { return false }
	if rt := ask("播放测试歌单"); rt.Decision != Native || slices.Contains([]string{""}, rt.Reason) {
		t.Errorf("%+v", rt)
	}
}
