package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"trackferry/internal/home"
	"trackferry/internal/live"
	"trackferry/internal/store"
	"trackferry/internal/takeover"
	"trackferry/internal/xiaomi"
)

type quietSpeaker struct {
	takeover.Speaker
	ops    []string
	volume int
	err    error
}

func (q *quietSpeaker) PlayerOp(_ context.Context, _, op string) error {
	q.ops = append(q.ops, op)
	return q.err
}

func (q *quietSpeaker) SetVolume(_ context.Context, _ string, v int) error {
	q.volume = v
	return q.err
}

func homeServer(t *testing.T) (*Server, string, *quietSpeaker) {
	s, tok := newServer(t)
	sp := &quietSpeaker{}
	log := slog.New(slog.DiscardHandler)
	s.Engine = takeover.New(sp, s.Store, log, takeover.DefaultTiming)
	s.Home = home.New(home.Source{Hub: live.New(), Devices: s.Store.Devices, Pending: s.Store.PendingVolumes,
		Status: func(context.Context, string) (xiaomi.PlayStatus, error) { return xiaomi.PlayStatus{Volume: 25}, nil }}, log, home.DefaultCadence)
	if _, err := s.Store.SetDeviceMode(context.Background(), "d1", store.ModeObserve); err != nil {
		t.Fatal(err)
	}
	return s, tok, sp
}

func call(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHomeEndpointIsOffAndHasItsOwnToken(t *testing.T) {
	s, admin, _ := homeServer(t)
	h := s.Handler()
	if c := call(h, "GET", "/api/ha/state", admin, "").Code; c != 404 {
		t.Fatal("off by default", c)
	}
	if c := call(h, "GET", "/api/home", "", "").Code; c != 401 {
		t.Fatal(c)
	}
	call(h, "PUT", "/api/home", admin, `{"enabled":true}`)
	if c := call(h, "GET", "/api/ha/state", admin, "").Code; c != 401 {
		t.Fatal("the admin token opens it", c)
	}
	token := func() string {
		var out struct{ Token string }
		json.Unmarshal(call(h, "POST", "/api/home/token", admin, "").Body.Bytes(), &out)
		return out.Token
	}
	tok := token()
	rec := call(h, "GET", "/api/ha/state", tok, "")
	var snap home.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil || rec.Code != 200 || len(snap.Speakers) != 1 || *snap.Speakers[0].Volume != 25 {
		t.Fatal(rec.Code, rec.Body)
	}
	for _, p := range []string{"/api/status", "/api/home", "/api/devices"} {
		if c := call(h, "GET", p, tok, "").Code; c != 401 {
			t.Fatal(p, c)
		}
	}
	if v, _ := s.Store.Setting(context.Background(), haToken); v == "" || strings.Contains(v, tok) {
		t.Fatal("stored in the clear")
	}
	again := token()
	if c := call(h, "GET", "/api/ha/state", tok, "").Code; c != 401 {
		t.Fatal("old token", c)
	}
	call(h, "PUT", "/api/home", admin, `{"enabled":false}`)
	if c := call(h, "GET", "/api/ha/state", again, "").Code; c != 404 {
		t.Fatal(c)
	}
}

func TestHomeEventsEndWhenSwitchedOff(t *testing.T) {
	s, admin, _ := homeServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	h := s.Handler()
	call(h, "PUT", "/api/home", admin, `{"enabled":true}`)
	var out struct{ Token string }
	json.Unmarshal(call(h, "POST", "/api/home/token", admin, "").Body.Bytes(), &out)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/ha/events", nil)
	req.Header.Set("Authorization", "Bearer "+out.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatal(err, resp)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	first := ""
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "event: "); ok && first == "" {
			first = v
			call(h, "PUT", "/api/home", admin, `{"enabled":false}`)
		}
	}
	if sc.Err() != nil || first != "snapshot" {
		t.Fatal(sc.Err(), first)
	}
}

func TestControl(t *testing.T) {
	s, admin, sp := homeServer(t)
	h := s.Handler()
	press := func(id, body string, want int, kind string) {
		t.Helper()
		rec := call(h, "POST", "/api/devices/"+id+"/control", admin, body)
		if rec.Code != want || !strings.Contains(rec.Body.String(), kind) {
			t.Fatal(body, rec.Code, rec.Body)
		}
	}
	press("d1", `{"action":"pause"}`, 204, "")
	press("d1", `{"action":"volume","volume":33}`, 204, "")
	press("d1", `{"action":"volume"}`, 400, "bad_volume")
	press("d1", `{"action":"volume","volume":101}`, 400, "bad_volume")
	press("d1", `{"action":"dance"}`, 400, "bad_action")
	press("nope", `{"action":"play"}`, 404, "no_device")
	press("d1", `{"action":"wake"}`, 409, "cannot_wake") // no did known
	sp.err = errors.New("x")
	press("d1", `{"action":"play"}`, 502, "not_done")
	s.Store.SetDeviceMode(context.Background(), "d1", store.ModeOff)
	press("d1", `{"action":"play"}`, 409, "device_off")
	if len(sp.ops) != 2 || sp.ops[0] != "pause" || sp.volume != 33 {
		t.Fatal(sp.ops, sp.volume)
	}
	if c := call(h, "POST", "/api/devices/d1/control", "", `{"action":"play"}`).Code; c != 401 {
		t.Fatal(c)
	}
}
