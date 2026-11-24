package xiaomi

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Password login states reported to the UI.
const (
	PwStateCaptcha = "need_captcha" // type the characters in the image
	PwStateVerify  = "need_verify"  // enter the SMS / email code Xiaomi sent
	PwStateLinked  = "linked"
)

// passwordTTL bounds how long an unfinished attempt (and the password hash) stays in memory.
const passwordTTL = 10 * time.Minute

// PasswordStatus is the password login in progress. It never includes credentials.
type PasswordStatus struct {
	ID         string    `json:"id"`
	State      string    `json:"state"`
	Account    string    `json:"account"` // masked
	Message    string    `json:"message,omitempty"`
	CaptchaRev int       `json:"captcha_rev,omitempty"` // changes whenever a new captcha image is ready
	VerifyURL  string    `json:"verify_url,omitempty"`
	VerifyType string    `json:"verify_type,omitempty"` // phone | email
	ExpiresAt  time.Time `json:"expires_at"`
	Sync       *SyncInfo `json:"sync,omitempty"`
}

type pwLogin struct {
	a      *passwordAttempt
	status PasswordStatus
}

// ErrNoPasswordLogin means no password login is in progress (or it expired).
var ErrNoPasswordLogin = &Error{Kind: KindExpired, Detail: "no password login is in progress"}

func maskAccount(user string) string {
	if at := strings.IndexByte(user, '@'); at > 0 {
		return maskUserID(user[:at]) + user[at:]
	}
	if len(user) > 7 {
		return user[:3] + "****" + user[len(user)-4:]
	}
	return maskUserID(user)
}

// StartPassword begins a password login. Depending on Xiaomi's risk control it links the
// account at once or asks for a captcha or an SMS / email code.
func (m *Manager) StartPassword(ctx context.Context, user, password string) (PasswordStatus, error) {
	m.pwMu.Lock()
	defer m.pwMu.Unlock()
	user = strings.TrimSpace(user)
	if user == "" || password == "" {
		return PasswordStatus{}, errf(KindAuth, "account and password are required")
	}
	a, err := m.client.newPasswordAttempt(user, password)
	if err != nil {
		return PasswordStatus{}, err
	}
	m.mu.Lock()
	m.pw = nil
	m.mu.Unlock()
	if err := a.begin(ctx); err != nil {
		return PasswordStatus{}, err
	}
	login := &pwLogin{a: a, status: PasswordStatus{ID: NewDeviceID()[:12], Account: maskAccount(user), ExpiresAt: time.Now().Add(passwordTTL)}}
	step, err := a.submit(ctx, "")
	return m.advance(ctx, login, step, err)
}

// SubmitCaptcha answers the image captcha.
func (m *Manager) SubmitCaptcha(ctx context.Context, answer string) (PasswordStatus, error) {
	m.pwMu.Lock()
	defer m.pwMu.Unlock()
	login, err := m.currentPassword(PwStateCaptcha)
	if err != nil {
		return PasswordStatus{}, err
	}
	step, err := login.a.submit(ctx, strings.TrimSpace(answer))
	return m.advance(ctx, login, step, err)
}

// SubmitVerifyCode answers the SMS / email identity check.
func (m *Manager) SubmitVerifyCode(ctx context.Context, ticket string) (PasswordStatus, error) {
	m.pwMu.Lock()
	defer m.pwMu.Unlock()
	login, err := m.currentPassword(PwStateVerify)
	if err != nil {
		return PasswordStatus{}, err
	}
	s, err := login.a.verify(ctx, strings.TrimSpace(ticket))
	if err != nil {
		if KindOf(err) == KindAuth {
			// Wrong or stale code: the user may try again with a new one.
			st := m.setPassword(login, func(st *PasswordStatus) { st.Message = "验证码不对或已过期，请重新获取后再填" })
			return st, err
		}
		return PasswordStatus{}, err
	}
	return m.finishPassword(ctx, login, s)
}

// Password reports the password login in progress.
func (m *Manager) Password() (PasswordStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pw == nil || time.Now().After(m.pw.status.ExpiresAt) {
		return PasswordStatus{}, false
	}
	return m.pw.status, true
}

// CancelPassword forgets the attempt, including the password hash.
func (m *Manager) CancelPassword() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pw = nil
}

// PasswordCaptcha returns the current captcha image.
func (m *Manager) PasswordCaptcha() ([]byte, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pw == nil || m.pw.status.State != PwStateCaptcha || len(m.pw.a.captcha) == 0 {
		return nil, "", ErrNoPasswordLogin
	}
	return m.pw.a.captcha, m.pw.a.captchaType, nil
}

// RefreshCaptcha fetches a new captcha image for the same attempt ("can't read it").
func (m *Manager) RefreshCaptcha(ctx context.Context) (PasswordStatus, error) {
	m.pwMu.Lock()
	defer m.pwMu.Unlock()
	login, err := m.currentPassword(PwStateCaptcha)
	if err != nil {
		return PasswordStatus{}, err
	}
	if err := login.a.fetchCaptcha(ctx, login.a.captchaURL); err != nil {
		return PasswordStatus{}, err
	}
	return m.setPassword(login, func(st *PasswordStatus) { st.CaptchaRev++; st.Message = "已换一张验证码" }), nil
}

func (m *Manager) currentPassword(want string) (*pwLogin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pw == nil || time.Now().After(m.pw.status.ExpiresAt) {
		m.pw = nil
		return nil, ErrNoPasswordLogin
	}
	if m.pw.status.State != want {
		return nil, errf(KindExpired, "password login is at step %s, not %s", m.pw.status.State, want)
	}
	return m.pw, nil
}

func (m *Manager) setPassword(login *pwLogin, f func(*PasswordStatus)) PasswordStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	f(&login.status)
	return login.status
}

// advance applies one serviceLoginAuth2 outcome.
func (m *Manager) advance(ctx context.Context, login *pwLogin, step passwordStep, err error) (PasswordStatus, error) {
	switch {
	case step.State == PwNeedCaptcha:
		msg := "小米要求输入图形验证码"
		if err != nil {
			msg = "验证码不对，已换一张新的"
		}
		st := m.setPassword(login, func(st *PasswordStatus) {
			st.State, st.Message, st.CaptchaRev = PwStateCaptcha, msg, st.CaptchaRev+1
		})
		m.mu.Lock()
		m.pw = login
		m.mu.Unlock()
		return st, nil
	case err != nil:
		m.CancelPassword()
		return PasswordStatus{}, err
	case step.State == PwNeedVerify:
		st := m.setPassword(login, func(st *PasswordStatus) {
			st.State, st.VerifyURL, st.VerifyType = PwStateVerify, login.a.verifyURL, login.a.verifyType
			st.Message = "小米要求验证身份"
		})
		m.mu.Lock()
		m.pw = login
		m.mu.Unlock()
		return st, nil
	case step.State == PwDone:
		return m.finishPassword(ctx, login, step.Session)
	}
	m.CancelPassword()
	return PasswordStatus{}, errors.New("unexpected password login step")
}

func (m *Manager) finishPassword(ctx context.Context, login *pwLogin, s Session) (PasswordStatus, error) {
	m.CancelPassword()
	login.a = nil // drop the password hash and cookies
	info, err := m.link(ctx, s)
	if err != nil {
		return PasswordStatus{}, errf(KindOf(err), "密码正确，但换取音箱服务令牌失败：%v", err)
	}
	login.status.State, login.status.Sync, login.status.Message = PwStateLinked, &info, ""
	login.status.VerifyURL = ""
	return login.status, nil
}
