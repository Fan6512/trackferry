package feiniu

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"
)

// Server is a 飞牛音乐 that answered.
type Server struct {
	Base    string `json:"base"`
	Name    string `json:"name"`
	Version string `json:"version"`
	guid    string
}

// Discovery is what looking around found. Blocked lists addresses that should be this NAS
// but gave no answer at all, the way a firewall drops packets.
type Discovery struct {
	Servers []Server `json:"servers"`
	Blocked []string `json:"blocked"`
	Relay   bool     `json:"relay"` // the relay container is there
}

// Discover looks for 飞牛音乐: on this machine, at the hosts it is known by, at the container's
// gateway, and only when none of those answer, on the known hosts' own private /24 networks.
// It reads one public page and sends no credentials.
func Discover(ctx context.Context, hosts []string) Discovery {
	ctx, cancel := context.WithTimeout(ctx, 9*time.Second)
	defer cancel()
	known := append([]string{"127.0.0.1"}, hosts...)
	if gw := gateway(); gw != "" {
		known = append(known, gw)
	}
	known = uniq(known)
	d := Discovery{Servers: []Server{}, Blocked: []string{}, Relay: relayed()}
	found, timedOut := probeAll(ctx, known, 2500*time.Millisecond, len(known))
	if len(found) == 0 {
		for _, h := range hosts {
			if timedOut[h] {
				d.Blocked = append(d.Blocked, h)
			}
		}
		var near []string
		skip := map[string]bool{}
		for _, h := range known {
			skip[h] = true
		}
		for _, h := range hosts {
			for _, ip := range subnet(h) {
				if !skip[ip] {
					skip[ip] = true
					near = append(near, ip)
				}
			}
		}
		if len(near) <= 1024 {
			found, _ = probeAll(ctx, near, 1200*time.Millisecond, 64)
		}
	}
	seen := map[string]bool{}
	for _, s := range found { // in order of preference
		if s.guid != "" && seen[s.guid] {
			continue
		}
		seen[s.guid] = true
		d.Servers = append(d.Servers, s)
	}
	return d
}

// probeAll keeps the order of hosts.
func probeAll(ctx context.Context, hosts []string, each time.Duration, workers int) ([]Server, map[string]bool) {
	res := make([]*Server, len(hosts))
	late := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(1, workers))
	for i, h := range hosts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			s, timeout := probe(ctx, h, each)
			mu.Lock()
			res[i] = s
			if timeout {
				late[h] = true
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	out := []Server{}
	for _, s := range res {
		if s != nil {
			out = append(out, *s)
		}
	}
	return out, late
}

// discoverPort is 飞牛's HTTPS port; its plain port only redirects there.
var discoverPort = "5667"

func probe(ctx context.Context, host string, limit time.Duration) (*Server, bool) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	base := "https://" + net.JoinHostPort(host, discoverPort)
	c := newClient(base, "", "")
	defer c.http.CloseIdleConnections()
	var out struct {
		GUID    string `json:"serverGUID"`
		Name    string `json:"serverName"`
		Version string `json:"serverVersion"`
	}
	if err := c.call(ctx, http.MethodGet, "/sys/config", nil, nil, &out); err != nil {
		return nil, ctx.Err() != nil
	}
	if out.GUID == "" && out.Version == "" {
		return nil, false
	}
	return &Server{Base: base, Name: out.Name, Version: out.Version, guid: out.GUID}, false
}

// subnet lists the other hosts of a private IPv4 address's /24.
func subnet(host string) []string {
	a, err := netip.ParseAddr(host)
	if err != nil || !a.Is4() || !a.IsPrivate() {
		return nil
	}
	b := a.As4()
	out := make([]string, 0, 253)
	for i := 1; i < 255; i++ {
		if byte(i) != b[3] {
			out = append(out, netip.AddrFrom4([4]byte{b[0], b[1], b[2], byte(i)}).String())
		}
	}
	return out
}

// gateway is the default route's next hop: the NAS itself when running in a bridged container.
func gateway() string {
	raw, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	return parseGateway(string(raw))
}

func parseGateway(table string) string {
	for _, line := range strings.Split(table, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || f[1] != "00000000" {
			continue
		}
		b, err := hex.DecodeString(f[2])
		if err != nil || len(b) != 4 {
			continue
		}
		v := binary.LittleEndian.Uint32(b)
		ip := netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
		if !ip.IsUnspecified() {
			return ip.String()
		}
	}
	return ""
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
