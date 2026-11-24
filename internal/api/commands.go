package api

import (
	"errors"
	"net/http"
	"strconv"

	"trackferry/internal/observe"
	"trackferry/internal/store"
)

func (s *Server) observerStatus(w http.ResponseWriter, _ *http.Request) {
	if s.Observer == nil {
		writeJSON(w, http.StatusOK, observe.Status{Devices: []observe.DeviceStatus{}})
		return
	}
	writeJSON(w, http.StatusOK, s.Observer.Status())
}

func (s *Server) commands(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	cmds, err := s.Store.Commands(r.Context(), q.Get("device"), before, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commands": cmds})
}

func (s *Server) commandUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "no_command", "没有这条记录")
		return
	}
	var req struct {
		NativeNote string `json:"native_note"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !store.NativeNotes[req.NativeNote] {
		writeError(w, http.StatusBadRequest, "bad_note", "native_note 只能是空、silent、spoke 或 playing")
		return
	}
	c, err := s.Store.SetNativeNote(r.Context(), id, req.NativeNote)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no_command", "没有这条记录")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}
