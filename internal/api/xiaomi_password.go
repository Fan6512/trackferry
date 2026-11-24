package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"trackferry/internal/xiaomi"
)

// passwordError explains a failed password-login step. The Xiaomi password itself is never
// logged or echoed.
func passwordError(w http.ResponseWriter, err error) {
	kind := xiaomi.KindOf(err)
	detail := err.Error()
	status := http.StatusBadGateway
	var msg string
	switch {
	case strings.Contains(detail, "70016"):
		status, msg = http.StatusUnauthorized, "账号或密码不对。小米 ID、手机号和邮箱都可以作为账号"
	case strings.Contains(detail, "70002"):
		status, msg = http.StatusUnauthorized, "小米没有找到这个账号"
	case err == xiaomi.ErrNoPasswordLogin || kind == xiaomi.KindExpired:
		status, msg = http.StatusConflict, "这次登录已经过期，请重新输入账号密码"
	case kind == xiaomi.KindAuth && strings.Contains(detail, "verification"):
		status, msg = http.StatusUnauthorized, "验证码不对或已过期，请重新获取后再填"
	case kind == xiaomi.KindAuth:
		status, msg = http.StatusUnauthorized, "小米拒绝了这次登录"
	case kind == xiaomi.KindNetwork:
		msg = "连不上小米服务器，请检查 NAS 的外网连接"
	case kind == xiaomi.KindProtocol:
		msg = "小米接口返回了看不懂的内容，可能接口有变化。可以改用扫码登录"
	default:
		msg = "小米服务器拒绝了这次登录，可以稍后再试或改用扫码登录"
	}
	writeJSON(w, status, map[string]string{"error": "xiaomi_" + kind, "message": msg, "detail": detail})
}

func (s *Server) passwordStart(w http.ResponseWriter, r *http.Request) {
	key := "xiaomi:" + clientKey(r)
	if ok, wait := s.xmLimiter.Allow(key, time.Now()); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "throttled", "密码登录失败次数太多，请 15 分钟后再试，或改用扫码登录。频繁试错可能让小米锁定账号")
		return
	}
	var in struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	st, err := s.Xiaomi.StartPassword(ctx, in.Account, in.Password)
	in.Password = ""
	if err != nil {
		if xiaomi.KindOf(err) == xiaomi.KindAuth {
			s.xmLimiter.Fail(key, time.Now())
		}
		s.Log.Info("小米　密码登录被拒", "类型", xiaomi.KindOf(err))
		passwordError(w, err)
		return
	}
	if st.State == xiaomi.PwStateLinked {
		s.xmLimiter.Reset(key)
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) passwordCaptcha(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	st, err := s.Xiaomi.SubmitCaptcha(ctx, in.Code)
	if err != nil {
		passwordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) passwordCaptchaRefresh(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	st, err := s.Xiaomi.RefreshCaptcha(ctx)
	if err != nil {
		passwordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) passwordVerify(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	st, err := s.Xiaomi.SubmitVerifyCode(ctx, in.Code)
	if err != nil {
		passwordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) passwordStatus(w http.ResponseWriter, _ *http.Request) {
	st, ok := s.Xiaomi.Password()
	if !ok {
		writeError(w, http.StatusNotFound, "no_password_login", "没有进行中的密码登录")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) passwordCancel(w http.ResponseWriter, _ *http.Request) {
	s.Xiaomi.CancelPassword()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) passwordCaptchaImage(w http.ResponseWriter, _ *http.Request) {
	img, ct, err := s.Xiaomi.PasswordCaptcha()
	if err != nil {
		writeError(w, http.StatusNotFound, "no_captcha", "没有待输入的验证码")
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(img)
}
