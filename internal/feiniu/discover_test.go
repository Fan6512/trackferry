package feiniu

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestParseGateway(t *testing.T) {
	table := "Iface Destination Gateway Flags\neth0 000018AC 00000000 0001\neth0 00000000 010018AC 0003\n"
	if g := parseGateway(table); g != "172.24.0.1" {
		t.Fatalf("got %q", g)
	}
	if g := parseGateway("nonsense"); g != "" {
		t.Fatalf("got %q", g)
	}
}

func TestSubnetOnlyPrivate(t *testing.T) {
	if n := len(subnet("192.168.1.10")); n != 253 {
		t.Fatalf("private: %d", n)
	}
	for _, h := range []string{"8.8.8.8", "nas.local", "127.0.0.1", "fd00::1"} {
		if subnet(h) != nil {
			t.Errorf("%s must not be scanned", h)
		}
	}
}

func TestDiscoverFindsThisMachine(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/music/api/v1/sys/config" || len(r.Cookies()) != 0 {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"code":0,"data":{"serverGUID":"g1","serverName":"NAS","serverVersion":"1.1.19"}}`))
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	old := discoverPort
	discoverPort = u.Port()
	defer func() { discoverPort = old }()
	d := Discover(context.Background(), []string{"localhost"})
	if len(d.Servers) != 1 || d.Servers[0].Name != "NAS" || d.Servers[0].Base != "https://127.0.0.1:"+u.Port() {
		t.Fatalf("one server, reached on loopback, even when two names lead to it: %+v", d)
	}
}

func TestProbeTellsSilenceFromRefusal(t *testing.T) {
	// Accepts and never answers, as good as a dropped packet.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	old := discoverPort
	discoverPort = port
	defer func() { discoverPort = old }()
	if s, late := probe(context.Background(), "127.0.0.1", 150*time.Millisecond); s != nil || !late {
		t.Fatalf("silence: %v %v", s, late)
	}
	ln.Close()
	if s, late := probe(context.Background(), "127.0.0.1", time.Second); s != nil || late {
		t.Fatalf("refusal: %v %v", s, late)
	}
}
