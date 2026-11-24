package xiaomi

// Password login: serviceLogin → serviceLoginAuth2, with the image captcha and the SMS /
// email identity check Xiaomi's risk control adds. The sequence follows
// songloft-plugin-miot (Apache-2.0) src/mina/auth.ts.
//
// The plaintext password never leaves this file: only its MD5 (what Xiaomi's form expects)
// is kept, in memory, for the few minutes an attempt may need captcha retries.

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// Password step results.
const (
	PwNeedCaptcha = "need_captcha"
	PwNeedVerify  = "need_verify"
	PwDone        = "done"
)

// passwordAttempt is one password login with its own cookie jar.
type passwordAttempt struct {
	c        *Client
	hc       *http.Client
	jar      http.CookieJar
	deviceID string
	user     string
	hash     string
	params   map[string]string // _sign, qs, callback, sid from serviceLogin

	captcha     []byte
	captchaType string
	captchaURL  string
	ick         string
	verifyURL   string
	verifyType  string // phone | email
}

// passwordStep is what one round trip produced.
type passwordStep struct {
	State   string
	Session Session // when State == PwDone (without serviceToken)
}

func (c *Client) newPasswordAttempt(user, password string) (*passwordAttempt, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	sum := md5.Sum([]byte(password))
	a := &passwordAttempt{
		c: c, jar: jar, deviceID: NewDeviceID(), user: user,
		hash: strings.ToUpper(hex.EncodeToString(sum[:])),
	}
	a.hc = &http.Client{
		Timeout:       c.http.Timeout,
		Transport:     c.http.Transport,
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	u, err := url.Parse(c.ep.Account)
	if err != nil {
		return nil, err
	}
	jar.SetCookies(u, []*http.Cookie{{Name: "sdkVersion", Value: sdkVersion, Path: "/"}, {Name: "deviceId", Value: a.deviceID, Path: "/"}})
	return a, nil
}

func (a *passwordAttempt) do(ctx context.Context, method, rawURL string, form url.Values, extraCookie string) (*http.Response, []byte, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", userAgent(a.deviceID))
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if extraCookie != "" {
		req.Header.Set("Cookie", extraCookie)
	}
	resp, err := a.hc.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp, raw, err
}

func (a *passwordAttempt) abs(u string) string {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return a.c.ep.Account + u
}

func (a *passwordAttempt) cookie(name string) string {
	u, _ := url.Parse(a.c.ep.Account)
	for _, ck := range a.jar.Cookies(u) {
		if ck.Name == name && ck.Value != "" && ck.Value != "EXPIRED" {
			return ck.Value
		}
	}
	return ""
}

// begin fetches the login form parameters.
func (a *passwordAttempt) begin(ctx context.Context) error {
	_, body, err := a.do(ctx, http.MethodGet, a.c.ep.Account+"/pass/serviceLogin?sid="+minaSID+"&_json=true", nil, "")
	if err != nil {
		return errf(KindNetwork, "serviceLogin: %v", err)
	}
	m, err := decodeAccountJSON(body)
	if err != nil {
		return err
	}
	a.params = map[string]string{"_sign": str(m, "_sign"), "qs": str(m, "qs"), "callback": str(m, "callback"), "sid": str(m, "sid")}
	if a.params["sid"] == "" {
		a.params["sid"] = minaSID
	}
	if a.params["_sign"] == "" || a.params["callback"] == "" {
		return errf(KindProtocol, "serviceLogin returned no login parameters (code %d)", code(m))
	}
	return nil
}

// submit posts the password (and captcha answer, if any).
func (a *passwordAttempt) submit(ctx context.Context, captcha string) (passwordStep, error) {
	form := url.Values{}
	form.Set("user", a.user)
	form.Set("hash", a.hash)
	for k, v := range a.params {
		form.Set(k, v)
	}
	if captcha != "" {
		form.Set("captCode", captcha)
	}
	extra := ""
	if captcha != "" && a.ick != "" && a.cookie("ick") == "" {
		// The jar already carries ick when Xiaomi set it on our host; sending it explicitly
		// covers captcha images served from another domain.
		extra = "ick=" + a.ick
	}
	resp, body, err := a.do(ctx, http.MethodPost, a.c.ep.Account+"/pass/serviceLoginAuth2?_json=true", form, extra)
	if err != nil {
		return passwordStep{}, errf(KindNetwork, "serviceLoginAuth2: %v", err)
	}
	m, err := decodeAccountJSON(body)
	if err != nil {
		return passwordStep{}, err
	}
	if n := str(m, "notificationUrl"); n != "" {
		a.verifyURL = a.abs(n)
		a.verifyType = "phone"
		if strings.Contains(strings.ToLower(a.verifyURL), "email") {
			a.verifyType = "email"
		}
		return passwordStep{State: PwNeedVerify}, nil
	}
	for _, f := range []string{"captchaUrl", "captchaURL", "captcha_url", "captcha", "imgUrl"} {
		if v := strings.TrimSpace(str(m, f)); v != "" && v != "null" {
			if err := a.fetchCaptcha(ctx, a.abs(v)); err != nil {
				return passwordStep{}, err
			}
			st := passwordStep{State: PwNeedCaptcha}
			if captcha != "" {
				return st, errf(KindAuth, "captcha rejected")
			}
			return st, nil
		}
	}
	if location := str(m, "location"); location != "" && code(m) == 0 {
		s := Session{
			UserID:    firstNonEmpty(str(m, "userId"), a.cookie("userId"), cookieValue(resp, "userId")),
			CUserID:   firstNonEmpty(str(m, "cUserId"), a.cookie("cUserId")),
			PassToken: firstNonEmpty(str(m, "passToken"), a.cookie("passToken"), cookieValue(resp, "passToken")),
			DeviceID:  a.deviceID,
		}
		if s.UserID == "" || s.PassToken == "" {
			return passwordStep{}, errf(KindProtocol, "password accepted but no passToken returned")
		}
		return passwordStep{State: PwDone, Session: s}, nil
	}
	return passwordStep{}, passwordRefusal(code(m), firstNonEmpty(str(m, "description"), str(m, "desc")))
}

// passwordRefusal turns serviceLoginAuth2 refusals into errors the UI can explain.
func passwordRefusal(n int, desc string) error {
	switch n {
	case 70016:
		return errf(KindAuth, "wrong account or password (code 70016)")
	case 70002:
		return errf(KindAuth, "account does not exist (code 70002)")
	case 87001:
		return errf(KindAuth, "captcha rejected (code 87001)")
	}
	return errf(KindUpstream, "password login refused: code %d %s", n, desc)
}

func (a *passwordAttempt) fetchCaptcha(ctx context.Context, src string) error {
	resp, body, err := a.do(ctx, http.MethodGet, src, nil, "")
	if err != nil {
		return errf(KindNetwork, "captcha image: %v", err)
	}
	if resp.StatusCode != http.StatusOK || len(body) == 0 {
		return errf(KindUpstream, "captcha image: HTTP %d", resp.StatusCode)
	}
	a.captcha, a.captchaType, a.captchaURL = body, resp.Header.Get("Content-Type"), src
	if a.captchaType == "" {
		a.captchaType = "image/jpeg"
	}
	a.ick = firstNonEmpty(cookieValue(resp, "ick"), a.cookie("ick"))
	return nil
}

// verify submits the SMS / email code and collects the passToken the redirect chain sets.
func (a *passwordAttempt) verify(ctx context.Context, ticket string) (Session, error) {
	if a.verifyURL == "" {
		return Session{}, errf(KindExpired, "no verification is pending")
	}
	if strings.Contains(a.verifyURL, "/fe/service/identity/authStart") {
		// Sets identity_session, which the verify call requires.
		a.do(ctx, http.MethodGet, strings.Replace(a.verifyURL, "/fe/service/identity/authStart", "/identity/list", 1), nil, "")
	}
	api, flag := "/identity/auth/verifyPhone", "4"
	if a.verifyType == "email" {
		api, flag = "/identity/auth/verifyEmail", "8"
	}
	form := url.Values{"ticket": {ticket}, "trust": {"true"}, "_json": {"true"}, "_flag": {flag}}
	_, body, err := a.do(ctx, http.MethodPost, fmt.Sprintf("%s%s?_dc=%d", a.c.ep.Account, api, time.Now().UnixMilli()), form, "")
	if err != nil {
		return Session{}, errf(KindNetwork, "verify: %v", err)
	}
	m, err := decodeAccountJSON(body)
	if err != nil {
		return Session{}, err
	}
	location := str(m, "location")
	if n := code(m); n != 0 || location == "" {
		return Session{}, errf(KindAuth, "verification code rejected: code %d %s", n, firstNonEmpty(str(m, "tips"), str(m, "desc")))
	}
	s := Session{DeviceID: a.deviceID}
	next := a.abs(location)
	for hop := 0; hop < 10 && next != ""; hop++ {
		r, _, err := a.do(ctx, http.MethodGet, next, nil, "")
		if err != nil {
			return Session{}, errf(KindNetwork, "verify redirect: %v", err)
		}
		s.PassToken = firstNonEmpty(cookieValue(r, "passToken"), s.PassToken)
		s.UserID = firstNonEmpty(cookieValue(r, "userId"), s.UserID)
		s.CUserID = firstNonEmpty(cookieValue(r, "cUserId"), s.CUserID)
		next = ""
		if loc, err := r.Location(); err == nil {
			next = loc.String()
		}
	}
	s.PassToken = firstNonEmpty(s.PassToken, a.cookie("passToken"))
	s.UserID = firstNonEmpty(s.UserID, a.cookie("userId"))
	s.CUserID = firstNonEmpty(s.CUserID, a.cookie("cUserId"))
	if s.PassToken == "" || s.UserID == "" {
		return Session{}, errf(KindProtocol, "verification accepted but no passToken returned")
	}
	return s, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
