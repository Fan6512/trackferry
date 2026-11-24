package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"trackferry/internal/cache"
	"trackferry/internal/library"
)

func (s *Server) missing(w http.ResponseWriter, r *http.Request) ([]library.Missing, bool) {
	plays, err := s.Store.OnlinePlays(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return nil, false
	}
	return s.Library.Missing(plays), true
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
		if !m.Owned {
			c.Write([]string{cell(m.Title), cell(m.Artist), strconv.Itoa(m.Plays), time.UnixMilli(m.LastMS).Format("2006-01-02 15:04")})
		}
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

// SettingCache is the cache's limit in MB.
const SettingCache = "cache.limit_mb"

func (s *Server) cacheLimit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LimitMB *int `json:"limit_mb"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.LimitMB == nil || *req.LimitMB < 0 || *req.LimitMB > cache.MaxMB {
		writeError(w, http.StatusBadRequest, "bad_limit", "上限要在 0 到 100 GB 之间")
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
