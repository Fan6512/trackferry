package feiniu

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 飞牛 can ask for an access code before anything behind its address answers. The code is
// exchanged at /access_code_verify for cookies, which end on their own.

// Errors of the gate.
var (
	ErrGate     = errors.New("这台飞牛要先过访问码")
	ErrGateCode = errors.New("飞牛没有接受这个访问码")
)

const (
	gatePath = "/access_code_verify"
	// gateRest is how long a code that was refused is not tried again: 飞牛 locks out whoever
	// keeps trying.
	gateRest = 15 * time.Minute
	gateHops = 5
)

// gate is the access code of one NAS and the cookies it was exchanged for.
type gate struct {
	code string

	mu      sync.Mutex
	cookies map[string]string
	n       int // how often it was passed
	refused time.Time
}

func newGate(code string) *gate { return &gate{code: strings.TrimSpace(code)} }

// add puts the gate's cookies on a request and returns which pass they are of.
func (g *gate) add(req *http.Request) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	for k, v := range g.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	return g.n
}

// gated reports an answer of the gate in place of 飞牛音乐's.
func gated(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return strings.Contains(ct, "text/html") ||
		resp.StatusCode/100 == 3 && strings.Contains(resp.Header.Get("Location"), "access")
}

// do sends what build makes, passing the gate first when it turns out to be shut.
func (c *client) do(hc *http.Client, build func() (*http.Request, error)) (*http.Response, error) {
	for again := false; ; again = true {
		req, err := build()
		if err != nil {
			return nil, err
		}
		n := c.gate.add(req)
		resp, err := hc.Do(req)
		if err != nil {
			return nil, clean(err)
		}
		if c.seen != nil && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
			sum := sha256.Sum256(resp.TLS.PeerCertificates[0].Raw)
			*c.seen = hex.EncodeToString(sum[:])
		}
		shut := gated(resp)
		// With a code at hand it is worth a try on other refusals, too.
		maybe := resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode/100 == 3
		if !shut && (!maybe || c.gate.code == "" || again) {
			return resp, nil
		}
		resp.Body.Close()
		if c.gate.code == "" || again {
			return nil, ErrGate
		}
		if err := c.pass(req.Context(), n); err != nil {
			return nil, err
		}
	}
}

// pass exchanges the code for cookies, unless that was done since pass n.
func (c *client) pass(ctx context.Context, n int) error {
	g := c.gate
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.n != n {
		return nil
	}
	if time.Since(g.refused) < gateRest {
		return ErrGateCode
	}
	at, err := url.Parse(c.base + gatePath)
	if err != nil {
		return err
	}
	got := map[string]string{}
	for range gateHops {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, at.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("x-access-code", base64.StdEncoding.EncodeToString([]byte(g.code)))
		req.Header.Set("x-access-source", "web")
		for k, v := range got {
			req.AddCookie(&http.Cookie{Name: k, Value: v})
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return clean(err)
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		for _, k := range resp.Cookies() {
			if k.Value != "" && k.MaxAge >= 0 {
				got[k.Name] = k.Value
			}
		}
		switch s := resp.StatusCode; {
		case s/100 == 3:
			next, err := at.Parse(resp.Header.Get("Location"))
			// The code goes nowhere else, and not in the clear.
			if err != nil || next.Host != at.Host || next.Scheme != at.Scheme {
				return errors.New("飞牛把访问码的验证转到了别处，没有继续")
			}
			at = next
			continue
		case s == http.StatusTooManyRequests:
			g.refused = time.Now()
			return errors.New("访问码试得太多，飞牛暂时不让再试，过一会儿再连")
		case s == http.StatusUnauthorized, s == http.StatusForbidden:
			g.refused = time.Now()
			return ErrGateCode
		case s == http.StatusNotFound, s/100 == 2:
			g.cookies, g.n = got, g.n+1
			return nil
		default:
			return fmt.Errorf("验证访问码时飞牛返回 HTTP %d", s)
		}
	}
	return errors.New("验证访问码时飞牛一直在跳转")
}
