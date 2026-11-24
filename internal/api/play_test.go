package api

import (
	"context"
	"strings"
	"testing"

	"trackferry/internal/takeover"
)

func TestPlayingSettings(t *testing.T) {
	s, tok := newServer(t)
	h := s.Handler()
	for _, c := range []struct {
		body string
		code int
		want takeover.Prefs
	}{
		{`{"after":true}`, 200, takeover.Prefs{Artist: true, After: true, Hours: 3}},
		{`{"artist":false,"hours":0}`, 200, takeover.Prefs{After: true}},
		{`{"hours":25}`, 400, takeover.Prefs{After: true}},
		{`{"hours":-1}`, 400, takeover.Prefs{After: true}},
	} {
		if rec := call(h, "PUT", "/api/playing", tok, c.body); rec.Code != c.code {
			t.Fatal(c.body, rec.Code, rec.Body)
		}
		if got := takeover.LoadPrefs(context.Background(), s.Store); got != c.want {
			t.Fatal(c.body, got)
		}
	}
	if c := call(h, "GET", "/api/playing", "", "").Code; c != 401 {
		t.Fatal(c)
	}
}

func TestDeviceHush(t *testing.T) {
	s, tok, _ := homeServer(t)
	h := s.Handler()
	for body, want := range map[string]int{`{"hush":"loud"}`: 400, `{"hush":""}`: 400, `{"hush":"strong"}`: 409, `{"hush":"normal"}`: 200} {
		if rec := call(h, "PATCH", "/api/devices/d1", tok, body); rec.Code != want {
			t.Fatal(body, rec.Code, rec.Body)
		}
	}
	if b := call(h, "GET", "/api/devices", tok, "").Body.String(); !strings.Contains(b, `"hush": "normal"`) {
		t.Fatal(b)
	}
}
