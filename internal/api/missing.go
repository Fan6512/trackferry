package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"trackferry/internal/cache"
	"trackferry/internal/library"
	"trackferry/internal/names"
)

func (s *Server) missing(w http.ResponseWriter, r *http.Request) ([]library.Missing, bool) {
	plays, err := s.Store.OnlinePlays(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return nil, false
	}
	gone := s.dismissed(r)
	out := []library.Missing{}
	for _, m := range s.Library.Missing(plays) {
		if !m.Owned && !slices.ContainsFunc(gone, song{m.Title, m.Artist}.same) {
			out = append(out, m)
		}
	}
	return out, true
}

// settingDismissed lists the songs taken off the list by hand.
const settingDismissed = "missing.dismissed"

const maxDismissed = 2000

// song names a song by words; singers may be credited differently from play to play.
type song struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

func (a song) same(b song) bool {
	return names.Norm(a.Title) == names.Norm(b.Title) && (names.Norm(a.Artist) == names.Norm(b.Artist) || names.SameArtist(a.Artist, b.Artist))
}

func (s *Server) dismissed(r *http.Request) (out []song) {
	raw, _ := s.Store.Setting(r.Context(), settingDismissed)
	json.Unmarshal([]byte(raw), &out)
	return out
}

// libraryMissingDismiss takes a song off the list for good, or puts it back.
func (s *Server) libraryMissingDismiss(w http.ResponseWriter, r *http.Request) {
	var req struct {
		song
		Back bool `json:"back"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if names.Norm(req.Title) == "" || len(req.Title)+len(req.Artist) > 600 {
		writeError(w, http.StatusBadRequest, "bad_song", "没说是哪首歌")
		return
	}
	s.missMu.Lock()
	defer s.missMu.Unlock()
	gone := slices.DeleteFunc(s.dismissed(r), req.song.same)
	if !req.Back {
		gone = append(gone, req.song)
	}
	if len(gone) > maxDismissed {
		gone = gone[len(gone)-maxDismissed:]
	}
	raw, _ := json.Marshal(gone)
	if err := s.Store.SetSetting(r.Context(), settingDismissed, string(raw)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.libraryMissing(w, r)
}

func (s *Server) libraryMissing(w http.ResponseWriter, r *http.Request) {
	if songs, ok := s.missing(w, r); ok {
		writeJSON(w, http.StatusOK, map[string]any{"songs": songs})
	}
}

// libraryMissingCSV exports what the library still lacks.
func (s *Server) libraryMissingCSV(w http.ResponseWriter, r *http.Request) {
	songs, ok := s.missing(w, r)
	if !ok {
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("Content-Disposition", `attachment; filename="trackferry-missing.csv"`)
	h.Set("Cache-Control", "no-store")
	w.Write([]byte{0xEF, 0xBB, 0xBF}) // a BOM: spreadsheets take it for another encoding otherwise
	c := csv.NewWriter(w)
	c.Write([]string{"歌名", "歌手", "次数", "最近一次"})
	for _, m := range songs {
		c.Write([]string{cell(m.Title), cell(m.Artist), strconv.Itoa(m.Plays), time.UnixMilli(m.LastMS).Format("2006-01-02 15:04")})
	}
	c.Flush()
}

// cell keeps a spreadsheet from taking a platform's text for a formula.
func cell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func (s *Server) cacheStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Cache.Status())
}

// SettingCache is the cache's limit in MB, or cache.NoLimit.
const SettingCache = "cache.limit_mb"

func (s *Server) cacheLimit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LimitMB *int `json:"limit_mb"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.LimitMB == nil || !cache.Valid(*req.LimitMB) {
		writeError(w, http.StatusBadRequest, "bad_limit", "上限要在 0 到 100 GB 之间，或者不限")
		return
	}
	if err := s.Store.SetSetting(r.Context(), SettingCache, strconv.Itoa(*req.LimitMB)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.Cache.SetLimit(*req.LimitMB)
	s.cacheStatus(w, r)
}

func (s *Server) cacheClear(w http.ResponseWriter, r *http.Request) {
	s.Cache.Clear()
	s.cacheStatus(w, r)
}
