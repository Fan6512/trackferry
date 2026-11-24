package xiaomi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWakeRequest(t *testing.T) {
	var form map[string][]string
	code := `{"code":0,"result":{"code":0}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		form = r.PostForm
		if r.URL.Path != "/miotspec/action" {
			t.Error(r.URL.Path)
		}
		w.Write([]byte(code))
	}))
	defer srv.Close()
	c := NewClient(Endpoints{Miot: srv.URL})
	s := Session{UserID: "1", ServiceToken: "T", SSecurity: "AAECAwQFBgcICQoLDA0ODw=="}
	if err := c.miotAction(context.Background(), s, "9", wakeActions["LX04"]); err != nil {
		t.Fatal(err)
	}
	var data struct {
		Params struct {
			DID        string
			Siid, Aiid int
			In         []string
		}
	}
	json.Unmarshal([]byte(form["data"][0]), &data)
	if p := data.Params; p.DID != "9" || p.Siid != 5 || p.Aiid != 2 || len(p.In) != 1 || form["_nonce"] == nil || form["signature"] == nil {
		t.Fatalf("%+v", form)
	}
	code = `{"code":0,"result":{"code":-704042011}}`
	var de *DeviceError
	if err := c.miotAction(context.Background(), s, "9", wakeActions["LX06"]); !errors.As(err, &de) {
		t.Fatal(err)
	}
	code = `{"code":3,"message":"auth err"}`
	if err := c.miotAction(context.Background(), s, "9", wakeActions["LX06"]); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if CanWake("LX06", "") || CanWake("ZZZ", "9") || !CanWake("lx06", "9") {
		t.Fatal("CanWake")
	}
}
