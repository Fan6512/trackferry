package feiniu

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// shut puts a gate before a NAS. Its cookie ends when pass changes.
func shut(t *testing.T) (*httptest.Server, *atomic.Value, *atomic.Int32) {
	valid := true
	inner := nas(t, &valid)
	var pass atomic.Value
	pass.Store("one")
	var asked atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == gatePath {
			asked.Add(1)
			if r.Header.Get("x-access-code") != "MTIzNA==" { // 1234
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "os-access-code", Value: pass.Load().(string)})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if k, err := r.Cookie("os-access-code"); err != nil || k.Value != pass.Load() {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte("<title>飞牛 fnOS</title>"))
			return
		}
		inner.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)
	return s, &pass, &asked
}

func TestAccessCode(t *testing.T) {
	ctx := context.Background()
	s, pass, asked := shut(t)
	path := filepath.Join(t.TempDir(), "f.json")
	m, _ := NewManager(path)
	if err := m.Login(ctx, s.URL, "me", "pw", ""); !errors.Is(err, ErrGate) {
		t.Fatal(err)
	}
	if err := m.Login(ctx, s.URL, "me", "pw", "9999"); !errors.Is(err, ErrGateCode) {
		t.Fatal(err)
	}
	if err := m.Login(ctx, s.URL, "me", "pw", " 1234 "); err != nil {
		t.Fatal(err)
	}
	if !m.Status().Code {
		t.Fatal("code not kept")
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatal(fi.Mode())
	}
	before := asked.Load()
	if _, _, err := m.Songs(ctx, 1, 1); err != nil || asked.Load() != before {
		t.Fatal(err, "asked again")
	}
	pass.Store("two") // it ended
	resp, err := m.Open(ctx, "g0", "")
	if err != nil || asked.Load() != before+1 {
		t.Fatal(err, asked.Load())
	}
	resp.Body.Close()
	again, _ := NewManager(path) // a restart
	if _, _, err := again.Songs(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
}

func TestRefusedAccessCodeIsNotTriedOver(t *testing.T) {
	s, _, asked := shut(t)
	c := newClient(s.URL, "", "")
	c.gate = newGate("9999")
	for range 3 {
		if _, _, err := c.system(context.Background()); !errors.Is(err, ErrGateCode) {
			t.Fatal(err)
		}
	}
	if asked.Load() != 1 {
		t.Fatal(asked.Load())
	}
}

func TestAccessCodeGoesNowhereElse(t *testing.T) {
	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		leaked.Store(r.Header.Get("x-access-code") != "")
	}))
	defer other.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == gatePath {
			http.Redirect(w, r, other.URL, http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	}))
	defer s.Close()
	c := newClient(s.URL, "", "")
	c.gate = newGate("1234")
	if _, _, err := c.system(context.Background()); err == nil || !strings.Contains(err.Error(), "别处") || leaked.Load() {
		t.Fatal(err, leaked.Load())
	}
}
