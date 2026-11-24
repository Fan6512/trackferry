package online

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestKugou(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Query().Get("keyword") != "":
			w.Write([]byte(`{"data":{"info":[
				{"hash":"A","songname":"小幸运","singername":"田馥甄","duration":263,"privilege":10},
				{"hash":"B","songname":"小幸运","singername":"田馥甄","duration":178,"privilege":0},
				{"hash":"C","songname":"学猫叫","singername":"小潘潘、小峰峰","duration":209,"privilege":8}]}}`))
		case r.URL.Query().Get("hash") == "A":
			w.Write([]byte(`{"url":"","error":"需要付费","privilege":10}`))
		case r.URL.Query().Get("hash") == "T":
			w.Write([]byte(`{"url":"http://x.kugou.com/t.mp3","timeLength":60}`))
		default:
			w.Write([]byte(`{"url":"http://x.kugou.com/a.mp3","timeLength":209}`))
		}
	}))
	defer srv.Close()
	k := &Kugou{Client: srv.Client(), SearchURL: srv.URL, PlayURL: srv.URL}
	cs, err := k.Search(context.Background(), "小幸运")
	if err != nil || len(cs) != 3 || !cs[0].Restricted || !cs[1].Restricted || cs[2].Restricted {
		t.Fatalf("%+v %v", cs, err)
	}
	if _, err := k.Resolve(context.Background(), cs[0]); !errors.Is(err, ErrRestricted) {
		t.Fatal(err)
	}
	if u, err := k.Resolve(context.Background(), cs[2]); err != nil || u == "" {
		t.Fatal(err)
	}
	if _, err := k.Resolve(context.Background(), Candidate{ID: "T", Duration: 200 * time.Second}); !errors.Is(err, ErrTrial) {
		t.Fatal(err)
	}
}
