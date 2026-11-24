package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"trackferry/internal/feiniu"
	"trackferry/internal/online"
	"trackferry/internal/routing"
	"trackferry/internal/store"
	"trackferry/internal/takeover"
	"trackferry/internal/xiaomi"
)

// target is the speaker of a request that plays something on it.
func (s *Server) target(w http.ResponseWriter, r *http.Request) (store.Device, bool) {
	if !s.Xiaomi.Status().Linked {
		writeError(w, http.StatusConflict, "not_linked", "还没有关联小米账号")
		return store.Device{}, false
	}
	d, err := s.Store.Device(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no_device", "没有这台音箱")
		return d, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return d, false
	}
	if !d.InAccount || d.Mode == store.ModeOff {
		writeError(w, http.StatusConflict, "device_off", "这台音箱没有接入。先在音箱页把它设为「观察」")
		return d, false
	}
	if err := s.Stream.Ready(); err != nil {
		writeError(w, http.StatusConflict, "no_public_url", "音箱不知道从哪里取音频：请在 config.yaml 里设置 public_base_url（音箱能访问到的 NAS 地址，例如 http://192.168.1.10:18561）后重启")
		return d, false
	}
	return d, true
}

// play sends a list, or an online song, to a speaker.
func (s *Server) play(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source string `json:"source"` // favorites | playlist | artist | album | online
		ID     string `json:"id"`     // the playlist, album or online song
		Name   string `json:"name"`   // the artist
	}
	if !readJSON(w, r, &req) {
		return
	}
	d, ok := s.target(w, r)
	if !ok {
		return
	}
	var list string
	var ids []int64
	switch req.Source {
	case "favorites":
		list, ids = "我的收藏", s.Library.FavoriteIDs()
	case "playlist":
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		name, got, err := s.Router.Collect(ctx, req.ID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "feiniu", err.Error())
			return
		}
		list, ids = "歌单“"+name+"”", got
	case "artist":
		list, ids = req.Name+"的歌", s.Library.ArtistIDs(req.Name)
	case "album":
		a, ts := s.Library.AlbumTracks(req.ID)
		list = "专辑“" + a.Name + "”"
		for _, t := range ts {
			ids = append(ids, t.ID)
		}
	case "online":
		found, err := s.Router.Online.Take(r.Context(), req.ID)
		if errors.Is(err, online.ErrGone) {
			writeError(w, http.StatusNotFound, "gone", err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusBadGateway, "no_audio", "这首放不了："+err.Error())
			return
		}
		s.answerRun(w, d)(s.Dispatcher.Push(r.Context(), d, routing.FromOnline(found)))
		return
	default:
		writeError(w, http.StatusBadRequest, "bad_source", "source 只能是 favorites、playlist、artist、album 或 online")
		return
	}
	if len(ids) == 0 {
		writeError(w, http.StatusNotFound, "empty", list+"里没有可放的歌")
		return
	}
	s.answerRun(w, d)(s.Dispatcher.PlayList(r.Context(), d, list, ids))
}

func (s *Server) answerRun(w http.ResponseWriter, d store.Device) func(store.TakeoverRun, error) {
	return func(run store.TakeoverRun, err error) {
		if err != nil {
			writeError(w, http.StatusConflict, "no_audio", err.Error())
			return
		}
		run.DeviceName = firstNonEmpty(d.Alias, d.Name)
		writeJSON(w, http.StatusAccepted, run)
	}
}

func (s *Server) playing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, takeover.LoadPrefs(r.Context(), s.Store))
}

func (s *Server) playingUpdate(w http.ResponseWriter, r *http.Request) {
	p := takeover.LoadPrefs(r.Context(), s.Store) // what is left out stays
	if !readJSON(w, r, &p) {
		return
	}
	if p.Hours < 0 || p.Hours > 24 {
		writeError(w, http.StatusBadRequest, "bad_hours", "时间要在 0 到 24 小时之间，0 是不限")
		return
	}
	if err := takeover.SavePrefs(r.Context(), s.Store, p); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deviceHush(w http.ResponseWriter, r *http.Request, hush string) {
	d, err := s.Store.Device(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "no_device", "没有这台音箱")
		return
	}
	switch {
	case hush != takeover.HushChosenNormal && hush != takeover.HushStrong && hush != takeover.HushWait:
		writeError(w, http.StatusBadRequest, "bad_hush", "hush 只能是 normal、strong 或 wait")
		return
	case hush == takeover.HushStrong && !xiaomi.CanHush(d.Hardware, d.MiotDID):
		writeError(w, http.StatusConflict, "cannot_hush", "这个型号不能通过米家静音")
		return
	}
	if err := s.Store.SetDeviceHush(r.Context(), d.DeviceID, hush); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if hush != takeover.HushChosenNormal && xiaomi.CanHush(d.Hardware, d.MiotDID) {
		go s.Xiaomi.WarmHome(context.WithoutCancel(r.Context()))
	}
	d.Hush = hush
	s.Log.Info("音箱　改了静音方式", "音箱", d.Name, "方式", hush)
	writeJSON(w, http.StatusOK, s.deviceView(d))
}

func (s *Server) libraryFavorites(w http.ResponseWriter, r *http.Request) {
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	ts, n := s.Library.Favorites(from, 60)
	writeJSON(w, http.StatusOK, map[string]any{"tracks": ts, "total": n})
}

func (s *Server) libraryPlaylists(w http.ResponseWriter, r *http.Request) {
	out := []feiniu.Playlist{}
	if s.Router.Lists != nil && s.Router.Lists.Linked() {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		all, err := s.Router.Lists.Playlists(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, "feiniu", "没能读到飞牛音乐的歌单："+err.Error())
			return
		}
		out = append(out, all...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"playlists": out})
}

func (s *Server) libraryPlaylist(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	name, ids, err := s.Router.Collect(ctx, r.PathValue("guid"))
	if err != nil {
		writeError(w, http.StatusBadGateway, "feiniu", err.Error())
		return
	}
	ts := []store.TrackRow{}
	for _, id := range ids[:min(500, len(ids))] {
		if t, ok := s.Library.Track(id); ok {
			ts = append(ts, t)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "tracks": ts, "total": len(ids)})
}

func (s *Server) librarySearchOnline(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len([]rune(q)) > 100 {
		writeError(w, http.StatusBadRequest, "bad_text", "搜索词要在 1 到 100 字之间")
		return
	}
	songs, outs := s.Router.Online.Search(r.Context(), q)
	writeJSON(w, http.StatusOK, map[string]any{"songs": songs, "sources": outs})
}
