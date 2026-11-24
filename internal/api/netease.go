package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"trackferry/internal/online"
)

func (s *Server) neteaseAccount(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Netease.Account())
}

func (s *Server) neteaseStart(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	u, err := s.Netease.StartQR(ctx)
	if err != nil {
		s.Log.Warn("网易云　二维码没有取到")
		writeError(w, http.StatusBadGateway, "netease", "连不上网易云，过一会儿再试")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": u})
}

func (s *Server) neteaseQR(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	state, err := s.Netease.QR(ctx)
	if errors.Is(err, online.ErrNoQR) {
		writeError(w, http.StatusNotFound, "no_qr", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "netease", online.Describe(err))
		return
	}
	if state == online.QRDone {
		s.Log.Info("网易云　已登录")
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": state, "account": s.Netease.Account()})
}

func (s *Server) neteaseCancel(w http.ResponseWriter, _ *http.Request) {
	s.Netease.CancelQR()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) neteaseLogout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if err := s.Netease.Logout(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "登录凭据没能删掉")
		return
	}
	s.Log.Info("网易云　已退出")
	writeJSON(w, http.StatusOK, s.Netease.Account())
}

func (s *Server) neteaseSignIn(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Cookie string `json:"cookie"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.Netease.SignIn(ctx, body.Cookie); err != nil {
		writeError(w, http.StatusBadGateway, "netease", online.Describe(err))
		return
	}
	s.Log.Info("网易云　已登录")
	writeJSON(w, http.StatusOK, s.Netease.Account())
}
