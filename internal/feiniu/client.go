// Package feiniu talks to 飞牛音乐 (trim.music) on the NAS as an ordinary signed-in client:
// the track index, covers, lyrics and the audio itself. The only thing it changes there is the
// listener's favourites, when they ask for it.
package feiniu

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"trackferry/internal/relay"
)

// Errors callers tell apart.
var (
	ErrAuth      = errors.New("飞牛音乐登录已失效")
	ErrCert      = errors.New("飞牛的证书和登录时不一样")
	ErrNotLinked = errors.New("还没有连接飞牛音乐")
)

// APIError is a refusal reported by 飞牛音乐.
type APIError struct {
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("飞牛音乐拒绝了请求（%d %s）", e.Code, e.Msg)
}

// Session is what a login leaves behind. The password is not part of it.
type Session struct {
	Base     string    `json:"base"` // https://nas:5667
	User     string    `json:"user"`
	Token    string    `json:"token"`
	DeviceID string    `json:"device_id"`
	Cert     string    `json:"cert"` // SHA-256 of the NAS's certificate at login ("" = plain HTTP)
	LinkedAt time.Time `json:"linked_at"`
	Server   string    `json:"server"`
	Version  string    `json:"version"`
	// Code is the access code of the NAS, "" when it asks for none. What it is exchanged for
	// ends on its own, so it is needed again.
	Code string `json:"code,omitempty"`
}

// client is one NAS address. pin, when set, is the only certificate accepted; NAS
// certificates are self-signed, so the one seen at login is remembered instead of checked
// against public authorities.
type client struct {
	base  string
	token string
	gate  *gate
	http  *http.Client // JSON and images
	audio *http.Client // no overall timeout
	seen  *string      // when set, receives the certificate's fingerprint
}

func newClient(base, token, pin string) *client {
	tlsc := &tls.Config{
		InsecureSkipVerify: true, // replaced by the pin check below
		VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 {
				return ErrCert
			}
			sum := sha256.Sum256(raw[0])
			fp := hex.EncodeToString(sum[:])
			if pin != "" && fp != pin {
				return ErrCert
			}
			return nil
		},
		// Resumed sessions skip VerifyPeerCertificate, so the pin is checked here as well.
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return ErrCert
			}
			sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
			fp := hex.EncodeToString(sum[:])
			if pin != "" && fp != pin {
				return ErrCert
			}
			return nil
		},
	}
	tr := &http.Transport{
		TLSClientConfig: tlsc, DialContext: dial,
		TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 20 * time.Second,
		MaxIdleConnsPerHost: 8, IdleConnTimeout: 90 * time.Second,
	}
	noRedirect := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client{base: base, token: token, gate: newGate(""),
		http:  &http.Client{Transport: tr, Timeout: 30 * time.Second, CheckRedirect: noRedirect},
		audio: &http.Client{Transport: tr, CheckRedirect: noRedirect}}
}

// RelaySocket leads to this NAS's own 飞牛 when the relay container runs (see package relay).
var RelaySocket = relay.Socket

func relayed() bool {
	fi, err := os.Stat(RelaySocket)
	return err == nil && fi.Mode()&os.ModeSocket != 0
}

// dial reaches this NAS through the relay, past its firewall; everything else directly.
func dial(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	if addr == net.JoinHostPort("127.0.0.1", discoverPort) && relayed() {
		return d.DialContext(ctx, "unix", RelaySocket)
	}
	return d.DialContext(ctx, network, addr)
}

const apiPath = "/music/api/v1"

func (c *client) request(ctx context.Context, method, path string, q url.Values, body any) (*http.Request, error) {
	u := c.base + apiPath + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept-Encoding", "identity")
	if c.token != "" {
		req.AddCookie(&http.Cookie{Name: "music-token", Value: url.QueryEscape(c.token)})
	}
	return req, nil
}

// clean hides the address and certificate chain details of a transport error.
func clean(err error) error {
	if errors.Is(err, ErrCert) {
		return ErrCert
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		if ue.Timeout() {
			return errors.New("连接飞牛音乐超时")
		}
		return fmt.Errorf("连不上飞牛音乐：%w", ue.Err)
	}
	return err
}

type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

const maxJSON = 16 << 20

func decode(body io.Reader, out any) error {
	raw, err := io.ReadAll(io.LimitReader(body, maxJSON))
	if err != nil {
		return err
	}
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return errors.New("这个地址没有返回飞牛音乐的数据")
	}
	switch e.Code {
	case 0:
	case 99999, 120001:
		return ErrAuth
	default:
		return &APIError{Code: e.Code, Msg: e.Msg}
	}
	if out == nil || len(e.Data) == 0 {
		return nil
	}
	return json.Unmarshal(e.Data, out)
}

func (c *client) call(ctx context.Context, method, path string, q url.Values, body, out any) error {
	resp, err := c.do(c.http, func() (*http.Request, error) { return c.request(ctx, method, path, q, body) })
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrAuth
	}
	if resp.StatusCode/100 == 3 {
		return errors.New("这个地址会跳转，请填飞牛的 HTTPS 地址（默认端口 5667）")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("飞牛音乐返回 HTTP %d", resp.StatusCode)
	}
	return decode(resp.Body, out)
}

// media opens a binary endpoint. Refusals arrive as JSON with HTTP 200.
func (c *client) media(ctx context.Context, hc *http.Client, path string, q url.Values, rng string) (*http.Response, error) {
	resp, err := c.do(hc, func() (*http.Request, error) {
		req, err := c.request(ctx, http.MethodGet, path, q, nil)
		if err == nil && rng != "" {
			req.Header.Set("Range", rng)
		}
		return req, err
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return resp, nil
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		defer resp.Body.Close()
		if err := decode(resp.Body, nil); err != nil {
			return nil, err
		}
		return nil, errors.New("飞牛音乐没有返回内容")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrAuth
		}
		return nil, fmt.Errorf("飞牛音乐返回 HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

// --- shapes (only what is used) ---

type named struct {
	GUID    string `json:"guid"`
	Name    string `json:"name"`
	CoverID string `json:"coverId"`
}

// Song is one track of 飞牛音乐's library.
type Song struct {
	GUID     string  `json:"guid"`
	Title    string  `json:"title"`
	Artists  []named `json:"artists"`
	Album    *named  `json:"album"`
	CoverID  string  `json:"coverId"`
	Duration float64 `json:"duration"` // ms
	Favorite bool    `json:"isFavorite"`
	Cue      bool    `json:"isCue"`
	Created  int64   `json:"createdAt"` // s
	Audio    struct {
		Path       string `json:"path"`
		Format     string `json:"format"`
		Container  string `json:"container"`
		Codec      string `json:"codec"`
		Size       int64  `json:"size"`
		SampleRate int    `json:"sampleRate"`
		Bits       int    `json:"bitDepth"`
	} `json:"audioSpec"`
}

type songPage struct {
	List  []Song `json:"list"`
	Total int    `json:"total"`
}

func (c *client) songs(ctx context.Context, page, size int) (songPage, error) {
	var out songPage
	err := c.call(ctx, http.MethodGet, "/track/list", url.Values{"page": {strconv.Itoa(page)}, "size": {strconv.Itoa(size)}, "sort": {"createdAt,asc"}}, nil, &out)
	return out, err
}

func (c *client) lyrics(ctx context.Context, guid string) (string, error) {
	var out struct {
		List []struct {
			GUID    string `json:"guid"`
			Content string `json:"content"`
			LRC     bool   `json:"isLRC"`
		} `json:"list"`
		Preferred string `json:"preferred"`
	}
	if err := c.call(ctx, http.MethodGet, "/lyric/list", url.Values{"trackGUID": {guid}}, nil, &out); err != nil {
		return "", err
	}
	best := ""
	for _, l := range out.List {
		if l.GUID == out.Preferred && l.Content != "" {
			return l.Content, nil
		}
		if best == "" || l.LRC && !strings.Contains(best, "[0") {
			best = l.Content
		}
	}
	return best, nil
}

// login exchanges the account's password for a token. Like the official web client, only
// the password's SHA-256 leaves this process.
func (c *client) login(ctx context.Context, user, password, device string) (string, error) {
	sum := sha256.Sum256([]byte(password))
	var out struct {
		Token string `json:"userToken"`
	}
	err := c.call(ctx, http.MethodPost, "/user/password-login", nil,
		map[string]string{"username": user, "password": hex.EncodeToString(sum[:]), "deviceId": device}, &out)
	if err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("飞牛音乐没有返回登录凭证")
	}
	return out.Token, nil
}

func (c *client) system(ctx context.Context) (name, version string, err error) {
	var out struct {
		Name    string `json:"serverName"`
		Version string `json:"serverVersion"`
	}
	err = c.call(ctx, http.MethodGet, "/sys/config", nil, nil, &out)
	return out.Name, out.Version, err
}
