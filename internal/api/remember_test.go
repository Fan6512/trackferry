package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionLasts(t *testing.T) {
	s, _ := newServer(t)
	for _, ttl := range []time.Duration{sessionTTL, keptTTL, visitTTL} {
		w := httptest.NewRecorder()
		if err := s.startSession(w, httptest.NewRequest(http.MethodPost, "/api/login", nil), ttl); err != nil {
			t.Fatal(err)
		}
		want := int(ttl.Seconds())
		if ttl == visitTTL {
			want = 0
		}
		if ck := w.Result().Cookies()[0]; ck.MaxAge != want || !ck.HttpOnly {
			t.Fatalf("%v %+v", ttl, ck)
		}
	}
}
