package api

import (
	"net/http"
	"time"
)

// liveFor bounds one connection; the browser reconnects, and its session is checked again.
const liveFor = 10 * time.Minute

// liveEvents streams what is happening on the speakers (server-sent events).
func (s *Server) liveEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Now().Add(liveFor + time.Minute))
	ch, stop := s.Live.Subscribe()
	defer stop()
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	end := time.NewTimer(liveFor)
	defer end.Stop()
	beat := time.NewTicker(20 * time.Second)
	defer beat.Stop()
	for {
		var msg []byte
		select {
		case <-r.Context().Done():
			return
		case <-end.C:
			return
		case <-beat.C:
			msg = []byte(": \n\n")
		case m, ok := <-ch:
			if !ok {
				return
			}
			msg = m
		}
		if _, err := w.Write(msg); err != nil || rc.Flush() != nil {
			return
		}
	}
}
