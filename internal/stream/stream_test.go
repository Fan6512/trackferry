package stream

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToneIsServedAndFetchRecorded(t *testing.T) {
	s := New("http://nas:18561/", t.TempDir())
	h, err := s.Tone()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h.URL, "http://nas:18561/stream/") || !strings.HasSuffix(h.URL, ".wav") {
		t.Fatal(h.URL)
	}
	select {
	case <-h.Fetched():
		t.Fatal("fetched before any request")
	default:
	}
	path := strings.TrimPrefix(h.URL, "http://nas:18561")
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Range", "bytes=0-99")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != 100 || !strings.HasPrefix(rec.Body.String(), "RIFF") {
		t.Fatalf("code %d len %d", rec.Code, rec.Body.Len())
	}
	select {
	case <-h.Fetched():
	default:
		t.Fatal("fetch not recorded")
	}
	if f, n := h.First(); n != 1 || f.Range != "bytes=0-99" {
		t.Fatal(f, n)
	}
	// An immediate second request from the start is the player probing, not a replay.
	s.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	select {
	case <-h.Restarted():
		t.Fatal("probe counted as a replay")
	default:
	}
	h.mu.Lock()
	h.first.At = h.first.At.Add(-ToneLength) // the track has played through
	h.mu.Unlock()
	s.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	select {
	case <-h.Restarted():
	default:
		t.Fatal("replay not detected")
	}

	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream/deadbeef.wav", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatal("unknown token served")
	}
}

func TestFileStaysInsideMusicDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a"), 0o755)
	os.WriteFile(filepath.Join(dir, "a", "song.mp3"), []byte("ID3fake"), 0o644)
	os.WriteFile(filepath.Join(dir, "cover.jpg"), []byte("x"), 0o644)
	s := New("http://nas", dir)
	if _, err := s.File("../etc/passwd", ""); err == nil {
		t.Fatal("escaped the music directory")
	}
	rel, err := s.RandomFile()
	if err != nil || rel != "a/song.mp3" {
		t.Fatal(rel, err)
	}
	h, err := s.File(rel, "")
	if err != nil || !strings.HasSuffix(h.URL, ".mp3") || h.Label != "song.mp3" {
		t.Fatal(h, err)
	}
	srv := httptest.NewServer(s)
	defer srv.Close()
	resp, err := http.Get(srv.URL + strings.TrimPrefix(h.URL, "http://nas"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "ID3fake" || resp.Header.Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("%q %s", body, resp.Header.Get("Content-Type"))
	}
}

func TestNoBaseURL(t *testing.T) {
	if _, err := New("", t.TempDir()).Tone(); err != ErrNoBaseURL {
		t.Fatal(err)
	}
}
