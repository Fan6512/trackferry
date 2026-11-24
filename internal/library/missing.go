package library

import (
	"slices"
	"sort"

	"trackferry/internal/names"
	"trackferry/internal/store"
)

// Missing is a song that was played from a platform.
type Missing struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Plays  int    `json:"plays"`
	LastMS int64  `json:"last_ms"`
	Owned  bool   `json:"owned"` // in the library by now
}

// Has reports whether a song is in the library: by title and any of its singers.
func (l *Library) Has(title, artist string) bool {
	plain, _ := names.SplitTitle(title)
	singers := names.Artists(artist)
	if len(singers) == 0 {
		singers = []string{""}
	}
	for _, t := range []string{title, plain} {
		for _, a := range singers {
			for _, m := range l.Song(t, a, title).Matches {
				if sameVersion(m.Title+" "+m.Album, title) {
					return true
				}
			}
		}
	}
	return false
}

// sameVersion: "晴天 (Live)" is not had by having "晴天".
func sameVersion(have, want string) bool {
	has := names.Versions(have)
	for _, v := range names.Versions(want) {
		if !slices.Contains(has, v) {
			return false
		}
	}
	return true
}

// Missing sums plays up by song, the most played first. Songs the library has come last.
func (l *Library) Missing(plays []store.OnlinePlay) []Missing {
	out := []Missing{}
	byTitle := map[string][]int{}
next:
	for _, p := range plays {
		k := names.Norm(p.Title)
		if k == "" {
			continue
		}
		for _, i := range byTitle[k] {
			if m := &out[i]; names.Norm(m.Artist) == names.Norm(p.Artist) || names.SameArtist(m.Artist, p.Artist) {
				m.Plays++
				m.LastMS = max(m.LastMS, p.AtMS)
				continue next
			}
		}
		byTitle[k] = append(byTitle[k], len(out))
		out = append(out, Missing{Title: p.Title, Artist: p.Artist, Plays: 1, LastMS: p.AtMS})
	}
	for i := range out {
		out[i].Owned = l.Has(out[i].Title, out[i].Artist)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Owned != b.Owned {
			return b.Owned
		}
		if a.Plays != b.Plays {
			return a.Plays > b.Plays
		}
		return a.LastMS > b.LastMS
	})
	return out
}
