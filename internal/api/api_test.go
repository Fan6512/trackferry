package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"trackferry/internal/config"
	"trackferry/internal/store"
	"trackferry/internal/stream"
	"trackferry/internal/takeover"
	"trackferry/internal/xiaomi"
)

func newServer(t *testing.T) (*Server, string) {
	t.Helper()
	ctx := context.Background()
	data := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(data, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	boot, err := st.RecordBoot(ctx, "test", "abc", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	tok, err := LoadOrCreateToken(filepath.Join(data, "private", "admin_token"))
	if err != nil {
		t.Fatal(err)
	}
	codePath := filepath.Join(data, "private", "setup_code")
	if err := os.WriteFile(codePath, []byte("ABCD2345\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.DataDir, cfg.MusicDir = data, t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sink := func(ctx context.Context, d []xiaomi.Device) error { return nil }
	xm, err := xiaomi.NewManager(xiaomi.NewClient(xiaomi.Endpoints{Account: "http://127.0.0.1:1", Mina: "http://127.0.0.1:1"}),
		filepath.Join(data, "private", "xiaomi.json"), sink, log)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SyncDevices(ctx, []store.DiscoveredDevice{{DeviceID: "d1", Name: "书房", Hardware: "LX06"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	return New(Options{Config: cfg, Store: st, Token: tok, SetupCodePath: codePath, Build: Build{Version: "test", Commit: "abc"},
		Boot: boot, Xiaomi: xm, Log: log}), tok
}

func do(t *testing.T, h http.Handler, path, token string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: invalid json %q", path, rec.Body.String())
	}
	return rec, body
}

func TestLiveNeedsNoAuth(t *testing.T) {
	s, _ := newServer(t)
	if rec, _ := do(t, s.Handler(), "/healthz/live", ""); rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestReadyReflectsInitAndWritableMusic(t *testing.T) {
	s, _ := newServer(t)
	h := s.Handler()
	rec, _ := do(t, h, "/healthz/ready", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("before MarkReady: code %d", rec.Code)
	}
	s.MarkReady()
	// The test music dir is writable, so readiness must still fail on music_read_only.
	rec, body := do(t, h, "/healthz/ready", "")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "music directory is writable") {
		t.Fatalf("code %d body %v", rec.Code, body)
	}
}

func TestAPIRequiresToken(t *testing.T) {
	s, tok := newServer(t)
	h := s.Handler()
	for _, bad := range []string{"", "wrong", tok + "x"} {
		if rec, _ := do(t, h, "/api/status", bad); rec.Code != http.StatusUnauthorized {
			t.Fatalf("token %q: code %d", bad, rec.Code)
		}
	}
	rec, body := do(t, h, "/api/status", tok)
	if rec.Code != http.StatusOK || body["boot_count"].(float64) != 1 {
		t.Fatalf("code %d body %v", rec.Code, body)
	}
	if strings.Contains(rec.Body.String(), tok) {
		t.Fatal("status leaks the admin token")
	}
	rec, body = do(t, h, "/api/devices", tok)
	if rec.Code != http.StatusOK || len(body["devices"].([]any)) != 1 {
		t.Fatalf("devices: %d %v", rec.Code, body)
	}
}

func TestTokenPersistsWithPrivatePermissions(t *testing.T) {
	p := filepath.Join(t.TempDir(), "private", "admin_token")
	a, err := LoadOrCreateToken(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadOrCreateToken(p)
	if err != nil || a != b || len(a) != 64 {
		t.Fatalf("a=%d b=%d err=%v", len(a), len(b), err)
	}
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("perm %v err %v", fi.Mode().Perm(), err)
	}
}

// browser drives the API the way the web UI does: cookies plus the CSRF header.
type browser struct {
	t      *testing.T
	h      http.Handler
	cookie *http.Cookie
}

func (b *browser) call(method, path, body string, csrf bool) (*httptest.ResponseRecorder, map[string]any) {
	b.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.7:5555"
	if csrf {
		req.Header.Set("X-Requested-With", "trackferry")
	}
	if b.cookie != nil {
		req.AddCookie(b.cookie)
	}
	rec := httptest.NewRecorder()
	b.h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == "tf_session" {
			if c.MaxAge < 0 {
				b.cookie = nil
			} else {
				b.cookie = c
			}
		}
	}
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestSetupLoginAndDeviceModes(t *testing.T) {
	s, _ := newServer(t)
	b := &browser{t: t, h: s.Handler()}

	_, body := b.call("GET", "/api/session", "", false)
	if body["setup_required"] != true || body["authenticated"] != false {
		t.Fatalf("session = %v", body)
	}
	if rec, _ := b.call("POST", "/api/setup", `{"setup_code":"ABCD2345","password":"longenough"}`, false); rec.Code != http.StatusForbidden {
		t.Fatalf("setup without csrf header: %d", rec.Code)
	}
	if rec, _ := b.call("POST", "/api/setup", `{"setup_code":"WRONG234","password":"longenough"}`, true); rec.Code != http.StatusForbidden {
		t.Fatalf("bad code: %d", rec.Code)
	}
	if rec, _ := b.call("POST", "/api/setup", `{"setup_code":"abcd-2345","password":"short"}`, true); rec.Code != http.StatusBadRequest {
		t.Fatalf("weak password: %d", rec.Code)
	}
	if rec, _ := b.call("POST", "/api/setup", `{"setup_code":"abcd-2345","password":"longenough"}`, true); rec.Code != http.StatusOK || b.cookie == nil {
		t.Fatalf("setup: %d", rec.Code)
	}
	if b.cookie.HttpOnly != true || b.cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie flags: %+v", b.cookie)
	}
	if rec, _ := b.call("POST", "/api/setup", `{"setup_code":"abcd-2345","password":"another-one"}`, true); rec.Code != http.StatusConflict {
		t.Fatalf("second setup: %d", rec.Code)
	}

	_, body = b.call("GET", "/api/devices", "", false)
	devs := body["devices"].([]any)
	if len(devs) != 1 || devs[0].(map[string]any)["mode"] != "off" {
		t.Fatalf("devices = %v", body)
	}
	if rec, _ := b.call("PATCH", "/api/devices/d1", `{"mode":"observe"}`, false); rec.Code != http.StatusForbidden {
		t.Fatalf("patch without csrf: %d", rec.Code)
	}
	if rec, body := b.call("PATCH", "/api/devices/d1", `{"mode":"observe"}`, true); rec.Code != http.StatusOK || body["mode"] != "observe" {
		t.Fatalf("patch: %d %v", rec.Code, body)
	}
	if rec, _ := b.call("PATCH", "/api/devices/d1", `{"mode":"takeover"}`, true); rec.Code != http.StatusConflict {
		t.Fatalf("takeover: %d", rec.Code)
	}
	if rec, _ := b.call("PATCH", "/api/devices/nope", `{"mode":"off"}`, true); rec.Code != http.StatusNotFound {
		t.Fatalf("missing device: %d", rec.Code)
	}
	if rec, _ := b.call("POST", "/api/devices/sync", ``, true); rec.Code != http.StatusConflict {
		t.Fatalf("sync without account: %d", rec.Code)
	}
	_, body = b.call("GET", "/api/xiaomi", "", false)
	if body["account"].(map[string]any)["linked"] != false {
		t.Fatalf("xiaomi = %v", body)
	}

	b.call("POST", "/api/logout", "", true)
	if rec, _ := b.call("GET", "/api/devices", "", false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: %d", rec.Code)
	}
	if rec, _ := b.call("POST", "/api/login", `{"password":"wrong-pass"}`, true); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: %d", rec.Code)
	}
	if rec, _ := b.call("POST", "/api/login", `{"password":"longenough"}`, true); rec.Code != http.StatusOK {
		t.Fatalf("login: %d", rec.Code)
	}
	if rec, _ := b.call("GET", "/api/devices", "", false); rec.Code != http.StatusOK {
		t.Fatalf("after login: %d", rec.Code)
	}
}

func TestLoginThrottled(t *testing.T) {
	s, _ := newServer(t)
	b := &browser{t: t, h: s.Handler()}
	b.call("POST", "/api/setup", `{"setup_code":"ABCD2345","password":"longenough"}`, true)
	b.cookie = nil
	for range 5 {
		b.call("POST", "/api/login", `{"password":"wrong-pass"}`, true)
	}
	if rec, _ := b.call("POST", "/api/login", `{"password":"longenough"}`, true); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt: %d", rec.Code)
	}
}

func TestWebUIMountedBesideAPI(t *testing.T) {
	s, _ := newServer(t)
	s.Web = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ui")) })
	h := s.Handler()
	for path, want := range map[string]string{"/": "ui", "/speakers": "ui"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Body.String() != want {
			t.Fatalf("%s = %q", path, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/nope", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "not_found") {
		t.Fatalf("/api/nope = %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST / = %d", rec.Code)
	}
}

func TestCommandsAndPasswordEndpoints(t *testing.T) {
	s, _ := newServer(t)
	b := &browser{t: t, h: s.Handler()}
	if rec, _ := b.call("POST", "/api/setup", `{"setup_code":"ABCD2345","password":"longenough"}`, true); rec.Code != http.StatusOK {
		t.Fatalf("setup: %d", rec.Code)
	}
	c := store.Command{DeviceID: "d1", RequestKey: "r1", Query: "播放晴天", UpstreamMS: 1, PrevPollMS: 1, SeenMS: time.Now().UnixMilli(), IntentKind: "song", IntentTitle: "晴天", Stage: store.StageObserved}
	if _, err := s.Store.AddCommand(context.Background(), &c); err != nil {
		t.Fatal(err)
	}
	_, body := b.call("GET", "/api/commands?limit=5", "", false)
	cmds := body["commands"].([]any)
	if len(cmds) != 1 || cmds[0].(map[string]any)["device_name"] != "书房" {
		t.Fatalf("commands = %v", body)
	}
	if _, ok := cmds[0].(map[string]any)["request_key"]; ok {
		t.Fatal("request key leaked")
	}
	if rec, body := b.call("PATCH", "/api/commands/"+strconv.FormatInt(c.ID, 10), `{"native_note":"spoke"}`, true); rec.Code != http.StatusOK || body["native_note"] != "spoke" {
		t.Fatalf("note: %d %v", rec.Code, body)
	}
	if rec, _ := b.call("PATCH", "/api/commands/"+strconv.FormatInt(c.ID, 10), `{"native_note":"loud"}`, true); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad note: %d", rec.Code)
	}
	if rec, body := b.call("GET", "/api/observer", "", false); rec.Code != http.StatusOK || body["devices"] == nil {
		t.Fatalf("observer: %d %v", rec.Code, body)
	}
	if rec, _ := b.call("POST", "/api/xiaomi/password/verify", `{"code":"123456"}`, true); rec.Code != http.StatusConflict {
		t.Fatalf("verify without attempt: %d", rec.Code)
	}
	// Xiaomi is unreachable in this test: a clear 502, never a 401 that would log the admin out.
	if rec, body := b.call("POST", "/api/xiaomi/password", `{"account":"a@b.c","password":"pw"}`, true); rec.Code != http.StatusBadGateway || body["error"] != "xiaomi_network" {
		t.Fatalf("password start: %d %v", rec.Code, body)
	}
}

func TestTakeoverEndpoints(t *testing.T) {
	s, _ := newServer(t)
	s.Stream = stream.New("", s.Config.MusicDir)
	s.Engine = takeover.New(s.Xiaomi, s.Store, s.Log, takeover.DefaultTiming)
	b := &browser{t: t, h: s.Handler()}
	if rec, _ := b.call("POST", "/api/setup", `{"setup_code":"ABCD2345","password":"longenough"}`, true); rec.Code != http.StatusOK {
		t.Fatalf("setup: %d", rec.Code)
	}
	if rec, body := b.call("GET", "/api/takeover/runs", "", false); rec.Code != http.StatusOK || body["stream_ready"] != false || len(body["runs"].([]any)) != 0 {
		t.Fatalf("runs: %d %v", rec.Code, body)
	}
	if rec, _ := b.call("POST", "/api/devices/d1/takeover-test", `{"source":"loud"}`, true); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad source: %d", rec.Code)
	}
	if rec, body := b.call("POST", "/api/devices/d1/takeover-test", `{"source":"tone"}`, true); rec.Code != http.StatusConflict || body["error"] != "not_linked" {
		t.Fatalf("unlinked: %d %v", rec.Code, body)
	}
	// Unknown stream tokens are a plain 404, with no login demanded.
	if rec, _ := b.call("GET", "/stream/0123456789abcdef.wav", "", false); rec.Code != http.StatusNotFound {
		t.Fatalf("stream: %d", rec.Code)
	}
}
