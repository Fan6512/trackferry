package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"trackferry/internal/cache"
	"trackferry/internal/library"
)

func TestCacheAndMissing(t *testing.T) {
	s, _ := newServer(t)
	log := slog.New(slog.DiscardHandler)
	c, err := cache.Open(t.TempDir(), "", 1, log)
	if err != nil {
		t.Fatal(err)
	}
	s.Cache, s.Library = c, library.New(t.TempDir(), s.Store, log)
	put := func(body string) int {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/api/cache", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		s.cacheLimit(w, r)
		return w.Code
	}
	if put(`{"limit_mb":-2}`) != 400 || put(`{"limit_mb":-1}`) != 200 || c.Status().LimitMB != cache.NoLimit || put(`{}`) != 400 || put(`{"limit_mb":2048}`) != 200 || c.Status().LimitMB != 2048 {
		t.Fatal(c.Status())
	}
	if v, _ := s.Store.Setting(t.Context(), SettingCache); v != "2048" {
		t.Fatal(v)
	}
	w := httptest.NewRecorder()
	s.libraryMissingCSV(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.HasPrefix(w.Body.String(), "\xef\xbb\xbf歌名,") || cell("=1+1") != "'=1+1" || cell("晴天") != "晴天" {
		t.Fatal(w.Body.String())
	}
}

func TestDismissMissing(t *testing.T) {
	s, _ := newServer(t)
	s.Library = library.New(t.TempDir(), s.Store, slog.New(slog.DiscardHandler))
	ask := func(body string) int {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		s.libraryMissingDismiss(w, r)
		return w.Code
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if ask(`{"title":""}`) != 400 || ask(`{"title":"海阔天空","artist":"Beyond/黄家驹"}`) != 200 || ask(`{"title":"海阔天空","artist":"Beyond"}`) != 200 {
		t.Fatal("codes")
	}
	if g := s.dismissed(r); len(g) != 1 || !g[0].same(song{"海阔天空", "beyond"}) || g[0].same(song{"海阔天空", "信乐团"}) {
		t.Fatal(g)
	}
	if ask(`{"title":"海阔天空","artist":"Beyond","back":true}`) != 200 || len(s.dismissed(r)) != 0 {
		t.Fatal("not put back")
	}
}
