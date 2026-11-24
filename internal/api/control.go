package api

import (
	"errors"
	"net/http"

	"trackferry/internal/home"
	"trackferry/internal/store"
	"trackferry/internal/takeover"
	"trackferry/internal/xiaomi"
)

// control presses a speaker's buttons.
func (s *Server) control(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"` // play | pause | stop | volume | wake
		Volume *int   `json:"volume"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	switch req.Action {
	case takeover.ControlPlay, takeover.ControlPause, takeover.ControlStop, takeover.ControlWake:
	case takeover.ControlVolume:
		if req.Volume == nil || *req.Volume < 0 || *req.Volume > 100 {
			writeError(w, http.StatusBadRequest, "bad_volume", "音量要在 0 到 100 之间")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "bad_action", "action 只能是 play、pause、stop、volume 或 wake")
		return
	}
	d, err := s.Store.Device(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no_device", "没有这台音箱")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !d.InAccount || d.Mode == store.ModeOff {
		writeError(w, http.StatusConflict, "device_off", "这台音箱没有接入。先在音箱页把它设为「观察」或「接管」")
		return
	}
	switch req.Action {
	case takeover.ControlVolume:
		err = s.Engine.SetVolume(r.Context(), d.DeviceID, *req.Volume)
	case takeover.ControlWake:
		if err = s.Xiaomi.Wake(r.Context(), d.Hardware, d.MiotDID); err == nil {
			s.Engine.Woke(d.DeviceID)
		}
	default:
		if req.Action == takeover.ControlStop && s.Dispatcher != nil {
			s.Dispatcher.Halt(d.DeviceID)
		}
		err = s.Engine.Press(r.Context(), d.DeviceID, req.Action)
	}
	if err != nil {
		s.Log.Warn("音箱　没有照做", "音箱", firstNonEmpty(d.Alias, d.Name), "操作", req.Action, "原因", takeover.Describe(err))
		if errors.Is(err, xiaomi.ErrCannotWake) {
			writeError(w, http.StatusConflict, "cannot_wake", "这个型号不支持唤醒")
			return
		}
		writeError(w, http.StatusBadGateway, "not_done", takeover.Describe(err))
		return
	}
	s.Home.Look(d.DeviceID)
	w.WriteHeader(http.StatusNoContent)
}

// controlState reads what the buttons need to know: playing, paused or idle, and the volume.
func (s *Server) controlState(w http.ResponseWriter, r *http.Request) {
	d, err := s.Store.Device(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "no_device", "没有这台音箱")
		return
	}
	if !d.InAccount || d.Mode == store.ModeOff {
		writeError(w, http.StatusConflict, "device_off", "这台音箱没有接入")
		return
	}
	p, err := s.Xiaomi.PlayStatus(r.Context(), d.DeviceID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "no_status", takeover.Describe(err))
		return
	}
	out := struct {
		State  string `json:"state"`
		Volume *int   `json:"volume"`
	}{State: home.Idle}
	switch p.Status {
	case xiaomi.PlayPlaying:
		out.State = home.Playing
	case xiaomi.PlayPaused:
		out.State = home.Paused
	}
	if p.Volume >= 0 {
		out.Volume = &p.Volume
	}
	if v, ok, err := s.Store.PendingVolume(r.Context(), d.DeviceID); err == nil && ok {
		out.Volume = &v
	}
	writeJSON(w, http.StatusOK, out)
}
