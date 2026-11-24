package online

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Netease is 网易云音乐. Signed out it plays what is free in full; signed in, what the
// owner's own account may play in full. Trial clips are refused either way.
type Netease struct {
	Client *http.Client
	File   string // where the session is kept, 0600; empty: nowhere
	Base   string // endpoint override for tests

	mu  sync.Mutex
	ses neteaseSession
	key string // the QR login in progress
}

type neteaseSession struct {
	Cookie   string `json:"cookie"`
	Name     string `json:"name"`
	VIP      bool   `json:"vip"`
	LinkedMS int64  `json:"linked_ms"`
	Expired  bool   `json:"expired"`
}

// NeteaseAccount is what may be shown of the signed-in account.
type NeteaseAccount struct {
	Linked   bool   `json:"linked"`
	Name     string `json:"name,omitempty"`
	VIP      bool   `json:"vip"`
	LinkedMS int64  `json:"linked_ms,omitempty"`
	Expired  bool   `json:"expired"`
}

// States of a QR login.
const (
	QRWaiting = "waiting"
	QRScanned = "scanned"
	QRExpired = "expired"
	QRDone    = "done"
)

// refused is an answer to a login, in words for people.
type refused string

func (r refused) Error() string { return string(r) }

// ErrNoQR is no login in progress.
var ErrNoQR = errors.New("没有进行中的扫码登录")

// Name implements Source.
func (*Netease) Name() string { return "netease" }

// Load reads the kept session.
func (n *Netease) Load() error {
	b, err := os.ReadFile(n.File)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return json.Unmarshal(b, &n.ses)
}

// save is called with mu held.
func (n *Netease) save() error {
	if n.File == "" {
		return nil
	}
	if n.ses.Cookie == "" {
		if err := os.Remove(n.File); !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	b, err := json.Marshal(n.ses)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(n.File), 0o700); err != nil {
		return err
	}
	tmp := n.File + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, n.File)
}

// Account tells who is signed in.
func (n *Netease) Account() NeteaseAccount {
	n.mu.Lock()
	defer n.mu.Unlock()
	s := n.ses
	return NeteaseAccount{Linked: s.Cookie != "", Name: s.Name, VIP: s.VIP, LinkedMS: s.LinkedMS, Expired: s.Expired}
}

func (n *Netease) cookie() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.ses.Expired {
		return ""
	}
	return n.ses.Cookie
}

func (n *Netease) post(ctx context.Context, path string, form url.Values, cookie string, v any) ([]*http.Cookie, error) {
	base := n.Base
	if base == "" {
		base = "https://music.163.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://music.163.com/")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	// The cookie goes nowhere else, and never over plain HTTP.
	client := *n.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var code struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(b, &code); err != nil {
		return nil, err
	}
	if code.Code == 405 || code.Code == -460 || code.Code == -462 {
		return nil, ErrLimited
	}
	return resp.Cookies(), json.Unmarshal(b, v)
}

// Search implements Source. Its answer says what the account at hand may play.
func (n *Netease) Search(ctx context.Context, q string) ([]Candidate, error) {
	var r struct {
		Result struct {
			Songs []struct {
				ID int64 `json:"id"`
				N  text  `json:"name"`
				Dt int64 `json:"dt"`
				Ar []struct {
					Name string `json:"name"`
				} `json:"ar"`
				Fee int `json:"fee"`
				P   struct {
					Pl int64 `json:"pl"`
					St int   `json:"st"`
				} `json:"privilege"`
			} `json:"songs"`
		} `json:"result"`
	}
	form := url.Values{"s": {q}, "type": {"1"}, "limit": {"15"}, "offset": {"0"}}
	if _, err := n.post(ctx, "/api/cloudsearch/pc", form, n.cookie(), &r); err != nil {
		return nil, err
	}
	var out []Candidate
	for _, s := range r.Result.Songs {
		var as []string
		for _, a := range s.Ar {
			as = append(as, a.Name)
		}
		c := Candidate{Source: "netease", ID: strconv.FormatInt(s.ID, 10), Title: unesc(s.N), Artist: strings.Join(as, "/"),
			Duration: time.Duration(s.Dt) * time.Millisecond, DurationMS: s.Dt, Restricted: s.P.Pl <= 0 || s.P.St < 0}
		c.Member = s.Fee != 0 && s.Fee != 8
		out = append(out, c)
	}
	return out, nil
}

// Resolve implements Source.
func (n *Netease) Resolve(ctx context.Context, c Candidate) (string, error) {
	var r struct {
		Data []struct {
			URL   string          `json:"url"`
			Code  int             `json:"code"`
			Trial json.RawMessage `json:"freeTrialInfo"`
		} `json:"data"`
	}
	form := url.Values{"ids": {"[" + c.ID + "]"}, "level": {"exhigh"}, "encodeType": {"mp3"}}
	if _, err := n.post(ctx, "/api/song/enhance/player/url/v1", form, n.cookie(), &r); err != nil {
		return "", err
	}
	if len(r.Data) == 0 {
		return "", ErrNoAudio
	}
	d := r.Data[0]
	switch {
	case len(d.Trial) > 0 && string(d.Trial) != "null":
		return "", ErrTrial
	case d.URL == "" && d.Code == 404:
		return "", ErrNoAudio
	case d.URL == "":
		return "", ErrRestricted
	}
	return d.URL, nil
}

// StartQR begins a login and returns what the code says.
func (n *Netease) StartQR(ctx context.Context) (string, error) {
	var r struct {
		Key string `json:"unikey"`
	}
	if _, err := n.post(ctx, "/api/login/qrcode/unikey", url.Values{"type": {"3"}}, "", &r); err != nil {
		return "", err
	}
	if r.Key == "" {
		return "", errors.New("网易云没有给出二维码")
	}
	n.mu.Lock()
	n.key = r.Key
	n.mu.Unlock()
	return "https://music.163.com/login?codekey=" + url.QueryEscape(r.Key), nil
}

// QR asks how the login in progress stands, and signs in when it is done.
func (n *Netease) QR(ctx context.Context) (string, error) {
	n.mu.Lock()
	key := n.key
	n.mu.Unlock()
	if key == "" {
		return "", ErrNoQR
	}
	var r struct {
		Code int `json:"code"`
	}
	got, err := n.post(ctx, "/api/login/qrcode/client/login", url.Values{"key": {key}, "type": {"3"}}, "", &r)
	if err != nil {
		return "", err
	}
	switch r.Code {
	case 801:
		return QRWaiting, nil
	case 802:
		return QRScanned, nil
	case 800:
		return QRExpired, nil
	case 803:
	default:
		return "", refused("网易云没有同意这次登录（" + strconv.Itoa(r.Code) + "），过一会儿再试")
	}
	var keep []string
	for _, c := range got {
		if (c.Name == "MUSIC_U" || c.Name == "__csrf") && c.Value != "" {
			keep = append(keep, c.Name+"="+c.Value)
		}
	}
	cookie := strings.Join(keep, "; ")
	if !strings.Contains(cookie, "MUSIC_U=") {
		return "", refused("网易云没有给出登录凭据，请重新扫码")
	}
	n.mu.Lock()
	if n.key != key { // cancelled, or started anew
		n.mu.Unlock()
		return "", ErrNoQR
	}
	n.key = ""
	n.ses = neteaseSession{Cookie: cookie, LinkedMS: time.Now().UnixMilli()}
	err = n.save()
	n.mu.Unlock()
	if err != nil {
		return "", refused("登录凭据没能存下来")
	}
	n.Check(ctx)
	return QRDone, nil
}

// CancelQR drops the login in progress.
func (n *Netease) CancelQR() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.key = ""
}

// Check asks whether the session still holds, and for whom.
func (n *Netease) Check(ctx context.Context) error {
	n.mu.Lock()
	cookie := n.ses.Cookie
	n.mu.Unlock()
	if cookie == "" {
		return nil
	}
	var r struct {
		Code    int             `json:"code"`
		Account json.RawMessage `json:"account"`
		Profile *struct {
			Name string `json:"nickname"`
			VIP  int    `json:"vipType"`
		} `json:"profile"`
	}
	if _, err := n.post(ctx, "/api/nuser/account/get", url.Values{}, cookie, &r); err != nil {
		return err
	}
	if r.Code != 200 && r.Code != 301 {
		return errors.New("网易云返回了看不懂的内容")
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.ses.Cookie != cookie {
		return nil
	}
	n.ses.Expired = r.Profile == nil
	if r.Profile != nil {
		n.ses.Name, n.ses.VIP = r.Profile.Name, r.Profile.VIP > 0
	}
	return n.save()
}

// Run checks the session now and twice a day.
func (n *Netease) Run(ctx context.Context) {
	for {
		c, cancel := context.WithTimeout(ctx, 15*time.Second)
		n.Check(c) // offline: as before
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-time.After(12 * time.Hour):
		}
	}
}

// Logout forgets the session here, and ends it there if that can be done.
func (n *Netease) Logout(ctx context.Context) error {
	n.mu.Lock()
	cookie := n.ses.Cookie
	n.ses, n.key = neteaseSession{}, ""
	err := n.save()
	n.mu.Unlock()
	if cookie != "" {
		var r struct{}
		n.post(ctx, "/api/logout", url.Values{}, cookie, &r)
	}
	return err
}
