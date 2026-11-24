package library

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"sort"
	"strings"

	"trackferry/internal/names"
	"trackferry/internal/store"
)

// Album is one album of the library.
type Album struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Artist  string `json:"artist"`
	Cover   string `json:"cover"`
	Tracks  int    `json:"tracks"`
	AddedMS int64  `json:"added_ms"`
	rows    []int
}

// Artist is one artist of the library.
type Artist struct {
	Name   string `json:"name"`
	Cover  string `json:"cover"`
	Tracks int    `json:"tracks"`
}

// group derives albums and artists. Only 飞牛音乐 identifies albums.
func (x *index) group() {
	at := map[string]int{}
	for i, t := range x.rows {
		if t.AlbumID == "" {
			continue
		}
		k, ok := at[t.AlbumID]
		if !ok {
			k = len(x.albums)
			at[t.AlbumID] = k
			x.albums = append(x.albums, Album{ID: t.AlbumID, Name: t.Album, Artist: t.Artist})
		}
		a := &x.albums[k]
		a.rows = append(a.rows, i)
		a.Tracks++
		a.AddedMS = max(a.AddedMS, t.AddedMS)
		if a.Cover == "" {
			a.Cover = t.Cover
		}
		if a.Artist != t.Artist && len(names.Artists(t.Artist)) < len(names.Artists(a.Artist)) {
			a.Artist = t.Artist // the shortest credit is the album's main artist
		}
	}
	sort.SliceStable(x.albums, func(i, j int) bool { return x.albums[i].AddedMS > x.albums[j].AddedMS })
	for _, ids := range x.byArtist {
		a := Artist{Tracks: len(ids)}
		key := ""
		for _, i := range ids {
			t := x.rows[i]
			// Show the credit as written, from a song this artist sings alone.
			if as := names.Artists(t.Artist); len(as) == 1 && (a.Name == "" || t.Cover != "" && a.Cover == "") {
				a.Name, key = t.Artist, as[0]
				if t.Cover != "" {
					a.Cover = t.Cover
				}
			}
		}
		if key != "" && a.Name != "" {
			x.artists = append(x.artists, a)
		}
	}
	sort.Slice(x.artists, func(i, j int) bool {
		if x.artists[i].Tracks != x.artists[j].Tracks {
			return x.artists[i].Tracks > x.artists[j].Tracks
		}
		return x.artists[i].Name < x.artists[j].Name
	})
}

func (l *Library) index() *index {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.idx
}

// Track finds an indexed song by ID.
func (l *Library) Track(id int64) (store.TrackRow, bool) {
	x := l.index()
	i, ok := x.byID[id]
	if !ok {
		return store.TrackRow{}, false
	}
	return x.rows[i], true
}

// ByPath finds an indexed song by its path.
func (l *Library) ByPath(p string) (store.TrackRow, bool) {
	x := l.index()
	i, ok := x.byPath[p]
	if !ok {
		return store.TrackRow{}, false
	}
	return x.rows[i], true
}

// Shelf is what the management page opens with.
type Shelf struct {
	Newest    []Album          `json:"newest"`
	Wall      []Album          `json:"wall"` // a random draw of albums with covers
	Artists   []Artist         `json:"artists"`
	Favorites []store.TrackRow `json:"favorites"`
	Picks     []store.TrackRow `json:"picks"`
	Albums    int              `json:"albums"`
}

// Shelf draws a selection.
func (l *Library) Shelf() Shelf {
	x := l.index()
	s := Shelf{Newest: []Album{}, Wall: []Album{}, Artists: []Artist{}, Favorites: []store.TrackRow{}, Picks: []store.TrackRow{}, Albums: len(x.albums)}
	var art []Album
	for _, a := range x.albums {
		if a.Cover != "" {
			art = append(art, a)
		}
	}
	s.Newest = append(s.Newest, art[:min(24, len(art))]...)
	for _, i := range rand.Perm(len(art))[:min(72, len(art))] {
		s.Wall = append(s.Wall, art[i])
	}
	// Singers with more songs are drawn more often, but not always the same ones.
	var singers []Artist
	for _, a := range x.artists {
		if a.Cover != "" && a.Tracks > 1 && len(singers) < 240 {
			singers = append(singers, a)
		}
	}
	for n := 4; len(s.Artists) < 24 && len(singers) > 0; n++ {
		i := rand.IntN(min(len(singers), 10*n))
		s.Artists = append(s.Artists, singers[i])
		singers = append(singers[:i], singers[i+1:]...)
	}
	var fav, all []store.TrackRow
	for _, t := range x.rows {
		if t.Favorite {
			fav = append(fav, t)
		}
	}
	for _, i := range rand.Perm(len(fav))[:min(24, len(fav))] {
		s.Favorites = append(s.Favorites, fav[i])
	}
	// Songs to come across: originals with a cover, one per singer.
	seen := map[string]bool{}
	for range 400 {
		if len(all) == 12 || len(x.rows) == 0 {
			break
		}
		t := x.rows[rand.IntN(len(x.rows))]
		if k := names.Norm(t.Artist); t.Cover != "" && k != "" && !seen[k] && names.VersionPenalty(t.Title, "") == 0 {
			seen[k] = true
			all = append(all, t)
		}
	}
	s.Picks = append(s.Picks, all...)
	return s
}

// ErrNotRemote is a song 飞牛音乐 does not hold.
var ErrNotRemote = errors.New("这首歌不在飞牛音乐里")

// SetFavorite adds a song to the favourites in 飞牛音乐, or takes it out.
func (l *Library) SetFavorite(ctx context.Context, id int64, on bool) (store.TrackRow, error) {
	t, ok := l.Track(id)
	f, can := l.Remote.(interface {
		Favorite(ctx context.Context, guid string, on bool) error
	})
	if !ok || t.GUID == "" || !can {
		return t, ErrNotRemote
	}
	if t.Favorite == on {
		return t, nil
	}
	if err := f.Favorite(ctx, t.GUID, on); err != nil {
		return t, err
	}
	t.Favorite = on
	l.mu.Lock()
	x := *l.idx
	if i, ok := x.byID[id]; ok {
		x.rows = slices.Clone(x.rows)
		x.rows[i].Favorite = on
		l.idx = &x
	}
	l.mu.Unlock()
	return t, l.st.SetFavorite(context.WithoutCancel(ctx), id, on)
}

// AlbumTracks lists an album's songs.
func (l *Library) AlbumTracks(id string) (Album, []store.TrackRow) {
	x := l.index()
	out := []store.TrackRow{}
	for _, a := range x.albums {
		if a.ID == id {
			for _, i := range a.rows {
				out = append(out, x.rows[i])
			}
			return a, out
		}
	}
	return Album{}, out
}

// ArtistTracks lists an artist's songs, favourites and originals first.
func (l *Library) ArtistTracks(name string, limit int) []store.TrackRow {
	x := l.index()
	out := []store.TrackRow{}
	for _, i := range x.byArtist[names.Norm(strings.TrimSpace(name))] {
		out = append(out, x.rows[i])
	}
	score := func(t store.TrackRow) int {
		n := names.VersionPenalty(t.Title, "")
		if t.Favorite {
			n -= 5
		}
		return n
	}
	sort.SliceStable(out, func(i, j int) bool { return score(out[i]) < score(out[j]) })
	return out[:min(limit, len(out))]
}
