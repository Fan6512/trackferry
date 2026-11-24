package library

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"trackferry/internal/feiniu"
)

type fakeRemote struct {
	linked bool
	songs  []feiniu.Song
	total  int
	err    error
}

func (f *fakeRemote) Linked() bool { return f.linked }
func (f *fakeRemote) Songs(_ context.Context, page, size int) ([]feiniu.Song, int, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	a := min((page-1)*size, len(f.songs))
	return f.songs[a:min(a+size, len(f.songs))], f.total, nil
}

func song(id int, title, artist, file string) feiniu.Song {
	var s feiniu.Song
	raw := fmt.Sprintf("g%d", id)
	s.GUID, s.Title, s.Audio.Path, s.CoverID = raw, title, file, "c"+raw
	s.Artists = append(s.Artists, struct {
		GUID    string `json:"guid"`
		Name    string `json:"name"`
		CoverID string `json:"coverId"`
	}{Name: artist})
	return s
}

func TestRemoteReplacesFiles(t *testing.T) {
	ctx := context.Background()
	l, _ := newLib(t, map[string][]byte{"a/甲 - 旧歌.mp3": id3("旧歌", "甲")})
	if err := l.Scan(ctx); err != nil || l.Size() != 1 {
		t.Fatal(err, l.Size())
	}
	f := &fakeRemote{linked: true}
	for i := range 450 {
		f.songs = append(f.songs, song(i, fmt.Sprint("歌", i), "甲", "/vol1/1000/音乐/x.flac"))
	}
	f.songs = append(f.songs, song(900, "不支持", "甲", "/vol1/1000/音乐/x.dsf"))
	f.total = len(f.songs)
	l.Remote = f
	f.err = errors.New("down")
	if err := l.Scan(ctx); err == nil || l.Size() != 1 {
		t.Fatalf("a failed sync must keep the index: %v %d", err, l.Size())
	}
	f.err = nil
	if err := l.Scan(ctx); err != nil || l.Size() != 450 {
		t.Fatal(err, l.Size())
	}
	st := l.Status().Last
	if st.Source != "feiniu" || st.Skipped != 1 || st.Added != 450 || st.Roots[0].Name != "音乐" {
		t.Errorf("%+v", st)
	}
	if err := l.Scan(ctx); err != nil || l.Status().Last.Added != 0 {
		t.Errorf("unchanged songs rewritten: %+v", l.Status().Last)
	}
	f.songs, f.total = f.songs[:449], 451 // a short read
	if err := l.Scan(ctx); err == nil || l.Size() != 450 {
		t.Errorf("short read: %v %d", err, l.Size())
	}
	f.linked = false
	if err := l.Scan(ctx); err != nil || l.Size() != 1 {
		t.Errorf("after unlinking: %v %d", err, l.Size())
	}
}
