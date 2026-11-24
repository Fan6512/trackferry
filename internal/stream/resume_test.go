package stream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResumesAnMP3NearATime(t *testing.T) {
	dir := t.TempDir()
	// 128 kbit/s, 44.1 kHz: 417 bytes a frame, 26.12 ms each.
	var b []byte
	for i := 0; i < 2000; i++ {
		f := make([]byte, 417)
		copy(f, []byte{0xFF, 0xFB, 0x90, 0x00})
		f[4] = byte(i)
		b = append(b, f...)
	}
	os.WriteFile(filepath.Join(dir, "a.mp3"), b, 0o600)
	s := New("http://nas", dir)
	d := time.Duration(2000*26122) * time.Microsecond
	h, err := s.Song("a.mp3", "a", d)
	if err != nil {
		t.Fatal(err)
	}
	n, at, err := h.Resume(context.Background(), d/2)
	if err != nil || at != d/2 {
		t.Fatal(at, err)
	}
	srv := httptest.NewServer(s)
	defer srv.Close()
	get := func(rng string) (*http.Response, []byte) {
		q, _ := http.NewRequest("GET", srv.URL+n.URL[len("http://nas"):], nil)
		if rng != "" {
			q.Header.Set("Range", rng)
		}
		r, err := http.DefaultClient.Do(q)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		x, _ := io.ReadAll(r.Body)
		return r, x
	}
	r, x := get("")
	if r.StatusCode != 200 || len(x) < 416000 || len(x) > 418000 || x[0] != 0xFF {
		t.Fatal(r.StatusCode, len(x))
	}
	r, y := get("bytes=100-199")
	if r.StatusCode != 206 || string(y) != string(x[100:200]) {
		t.Fatal(r.StatusCode, len(y))
	}
}
