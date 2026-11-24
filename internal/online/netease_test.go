package online

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeteasePlaysWhatTheAccountMay(t *testing.T) {
	stage := 801
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		in := strings.Contains(r.Header.Get("Cookie"), "MUSIC_U=secret")
		r.ParseForm()
		switch r.URL.Path {
		case "/api/cloudsearch/pc":
			pl := "0"
			if in {
				pl = "320000"
			}
			w.Write([]byte(`{"code":200,"result":{"songs":[{"id":1,"name":"甲","dt":200000,"fee":1,"ar":[{"name":"人"}],"privilege":{"pl":` + pl + `,"st":0}},
				{"id":2,"name":"乙","dt":200000,"fee":8,"ar":[{"name":"人"}],"privilege":{"pl":320000,"st":0}}]}}`))
		case "/api/song/enhance/player/url/v1":
			switch r.Form.Get("ids") {
			case "[1]":
				w.Write([]byte(`{"code":200,"data":[{"url":"http://m/1.mp3","code":200,"freeTrialInfo":{"start":0,"end":30}}]}`))
			case "[2]":
				w.Write([]byte(`{"code":200,"data":[{"url":"http://m/2.mp3","code":200,"freeTrialInfo":null}]}`))
			default:
				w.Write([]byte(`{"code":200,"data":[{"url":null,"code":-110}]}`))
			}
		case "/api/login/qrcode/unikey":
			w.Write([]byte(`{"code":200,"unikey":"k"}`))
		case "/api/login/qrcode/client/login":
			if stage == 803 {
				http.SetCookie(w, &http.Cookie{Name: "MUSIC_U", Value: "secret"})
				w.Write([]byte(`{"code":803}`))
				return
			}
			w.Write([]byte(`{"code":801}`))
		case "/api/nuser/account/get":
			if in && stage == 803 {
				w.Write([]byte(`{"code":200,"account":{},"profile":{"nickname":"某人","vipType":11}}`))
				return
			}
			w.Write([]byte(`{"code":200,"account":null,"profile":null}`))
		default:
			w.Write([]byte(`{"code":200}`))
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "private", "netease.json")
	n := &Netease{Client: srv.Client(), Base: srv.URL, File: file}

	got, err := n.Search(ctx, "甲")
	if err != nil || len(got) != 2 || !got[0].Restricted || got[1].Restricted || got[1].Member {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := n.Resolve(ctx, Candidate{ID: "1"}); !errors.Is(err, ErrTrial) {
		t.Fatal(err)
	}
	if u, err := n.Resolve(ctx, Candidate{ID: "2"}); err != nil || u != "http://m/2.mp3" {
		t.Fatal(u, err)
	}
	if _, err := n.Resolve(ctx, Candidate{ID: "3"}); !errors.Is(err, ErrRestricted) {
		t.Fatal(err)
	}
	if _, err := n.QR(ctx); !errors.Is(err, ErrNoQR) {
		t.Fatal(err)
	}
	if u, err := n.StartQR(ctx); err != nil || !strings.HasSuffix(u, "codekey=k") {
		t.Fatal(u, err)
	}
	if s, _ := n.QR(ctx); s != QRWaiting {
		t.Fatal(s)
	}
	stage = 803
	if s, err := n.QR(ctx); s != QRDone || err != nil {
		t.Fatal(s, err)
	}
	if a := n.Account(); !a.Linked || a.Name != "某人" || !a.VIP || a.Expired {
		t.Fatalf("%+v", a)
	}
	if fi, err := os.Stat(file); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatal(err)
	}
	if got, _ := n.Search(ctx, "甲"); got[0].Restricted || !got[0].Member {
		t.Fatalf("%+v", got[0])
	}
	// Read back; then the session ends elsewhere: free songs only, and the owner is told.
	m := &Netease{Client: srv.Client(), Base: srv.URL, File: file}
	if err := m.Load(); err != nil || !m.Account().Linked {
		t.Fatal(err)
	}
	stage = 0
	if m.Check(ctx); !m.Account().Expired {
		t.Fatal("not noticed")
	}
	if got, _ := m.Search(ctx, "甲"); !got[0].Restricted {
		t.Fatal("still used")
	}
	if err := m.Logout(ctx); err != nil || m.Account().Linked {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("still there")
	}
}
