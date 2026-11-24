package xiaomi

// Speaker playback control over the Mina "remote ubus" relay: play status, pause / stop,
// volume and URL playback. Request shapes follow songloft-plugin-miot (Apache-2.0)
// src/mina/client.ts and xiaomusic (MIT).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ubusTimeout bounds one relayed call; the cloud answers within a second when the speaker
// is online.
const ubusTimeout = 5 * time.Second

// OurAudioID is the cover id TrackFerry sends with player_play_music. The play status
// echoes the audio id, which is how a guard tells our stream from Xiao AI's own.
const OurAudioID = "1732418460076477549"

const musicCPID = "355454500"

// musicAPIModels play pushed URLs through player_play_music (with REPLACE_ALL); other
// models use player_play_url. List from songloft-plugin-miot DEFAULT_MUSIC_API_MODELS.
var musicAPIModels = map[string]bool{
	"X08C": true, "X08E": true, "X8S": true, "X8F": true, "X4B": true, "LX05": true, "OH11": true,
	"OH2": true, "OH2P": true, "X6A": true, "LX04": true, "L05B": true, "L05C": true, "LX06": true,
	"L06A": true, "X08A": true, "X10A": true, "L15A": true, "L16A": true, "L17A": true,
}

// UsesMusicAPI reports whether a hardware model plays URLs through player_play_music.
func UsesMusicAPI(hardware string) bool { return musicAPIModels[strings.ToUpper(hardware)] }

// Play states reported by player_get_play_status.
const (
	PlayStopped = 0
	PlayPlaying = 1
	PlayPaused  = 2
	PlayIdle3   = 3 // some models report 3 after a pushed stream was interrupted
)

// PlayStatus is one player_get_play_status reading. Firmware is known to misreport the
// state; treat it as a hint and prefer evidence such as advancing position or our stream
// being fetched.
type PlayStatus struct {
	Status   int    `json:"status"`
	Volume   int    `json:"volume"` // -1 when not reported
	AudioID  string `json:"audio_id,omitempty"`
	Position int64  `json:"position"` // ms, -1 when not reported
	Duration int64  `json:"duration"` // ms
}

// Playing reports whether the speaker says it is playing.
func (p PlayStatus) Playing() bool { return p.Status == PlayPlaying }

// DeviceError means the cloud relayed the call but the speaker refused it.
type DeviceError struct{ Code int }

func (e *DeviceError) Error() string { return fmt.Sprintf("speaker refused: code %d", e.Code) }

// ubus relays one call to a speaker. It returns the speaker's own reply (data).
func (c *Client) ubus(ctx context.Context, s Session, deviceID, path, method string, message any) (json.RawMessage, error) {
	msg, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	form := url.Values{
		"deviceId":  {deviceID},
		"path":      {path},
		"method":    {method},
		"message":   {string(msg)},
		"requestId": {"app_ios_" + NewDeviceID()[:30]},
	}
	ctx, cancel := context.WithTimeout(ctx, ubusTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.ep.Mina+"/remote/ubus", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent(s.DeviceID))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "userId="+s.UserID+"; serviceToken="+s.ServiceToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errf(KindNetwork, "%s: %v", method, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, errf(KindNetwork, "%s: %v", method, err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	if resp.StatusCode >= 400 {
		return nil, errf(KindUpstream, "%s: HTTP %d", method, resp.StatusCode)
	}
	var out struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, errf(KindProtocol, "%s: unreadable reply", method)
	}
	if out.Code != 0 {
		// e.g. the speaker is offline; the cloud did not deliver the call.
		return nil, errf(KindUpstream, "%s: code %d %s", method, out.Code, out.Message)
	}
	// A non-zero code inside data means the speaker received the call and refused it.
	var dev struct {
		Code *int `json:"code"`
	}
	if json.Unmarshal(out.Data, &dev) == nil && dev.Code != nil && *dev.Code != 0 {
		return out.Data, &DeviceError{Code: *dev.Code}
	}
	return out.Data, nil
}

// PlayStatus reads the speaker's player state.
func (c *Client) PlayStatus(ctx context.Context, s Session, deviceID string) (PlayStatus, error) {
	data, err := c.ubus(ctx, s, deviceID, "mediaplayer", "player_get_play_status", map[string]any{})
	if err != nil {
		return PlayStatus{}, err
	}
	return parsePlayStatus(data)
}

func parsePlayStatus(data json.RawMessage) (PlayStatus, error) {
	var wrap struct {
		Info string `json:"info"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil || wrap.Info == "" {
		return PlayStatus{}, errf(KindProtocol, "play status without info")
	}
	var info struct {
		Status *int `json:"status"`
		Volume *int `json:"volume"`
		Detail *struct {
			AudioID  json.Number `json:"audio_id"`
			Position *int64      `json:"position"`
			Duration int64       `json:"duration"`
		} `json:"play_song_detail"`
	}
	dec := json.NewDecoder(strings.NewReader(wrap.Info))
	dec.UseNumber()
	if err := dec.Decode(&info); err != nil || info.Status == nil || *info.Status < 0 {
		return PlayStatus{}, errf(KindProtocol, "play status unreadable")
	}
	p := PlayStatus{Status: *info.Status, Volume: -1, Position: -1}
	if info.Volume != nil {
		p.Volume = *info.Volume
	}
	if d := info.Detail; d != nil {
		p.AudioID = d.AudioID.String()
		if d.Position != nil {
			p.Position = *d.Position
		}
		p.Duration = d.Duration
	}
	return p, nil
}

// PlayerOp sends play / pause / stop.
func (c *Client) PlayerOp(ctx context.Context, s Session, deviceID, action string) error {
	_, err := c.ubus(ctx, s, deviceID, "mediaplayer", "player_play_operation", map[string]any{"action": action, "media": "app_ios"})
	return err
}

// SetVolume sets the speaker volume (0-100).
func (c *Client) SetVolume(ctx context.Context, s Session, deviceID string, volume int) error {
	volume = max(0, min(100, volume))
	_, err := c.ubus(ctx, s, deviceID, "mediaplayer", "player_set_volume", map[string]any{"volume": volume, "media": "app_ios"})
	return err
}

// Speak has the speaker say text.
func (c *Client) Speak(ctx context.Context, s Session, deviceID, text string) error {
	_, err := c.ubus(ctx, s, deviceID, "mibrain", "text_to_speech", map[string]any{"text": text, "save": 0})
	return err
}

// PlayURL pushes an audio URL, choosing the API the model needs. Music API models replace
// the whole play queue (REPLACE_ALL), so Xiao AI's own playlist does not continue after it.
func (c *Client) PlayURL(ctx context.Context, s Session, deviceID, hardware, audioURL string) error {
	if !UsesMusicAPI(hardware) {
		_, err := c.ubus(ctx, s, deviceID, "mediaplayer", "player_play_url", map[string]any{"url": audioURL, "type": 2, "media": "app_ios"})
		return err
	}
	music := map[string]any{
		"payload": map[string]any{
			"audio_type": "",
			"audio_items": []any{map[string]any{
				"item_id": map[string]any{
					"audio_id": OurAudioID,
					"cp":       map[string]any{"album_id": "-1", "episode_index": 0, "id": musicCPID, "name": "xiaowei"},
				},
				"stream": map[string]any{"url": audioURL},
			}},
			"list_params": map[string]any{"listId": "-1", "loadmore_offset": 0, "origin": "xiaowei", "type": "MUSIC"},
		},
		"play_behavior": "REPLACE_ALL",
	}
	raw, err := json.Marshal(music)
	if err != nil {
		return err
	}
	_, err = c.ubus(ctx, s, deviceID, "mediaplayer", "player_play_music", map[string]any{"startaudioid": OurAudioID, "music": string(raw)})
	return err
}

// --- Manager wrappers: renew the serviceToken once on 401 ---

// withSession runs f with the current session, renewing the serviceToken once if Xiaomi
// rejects it.
func (m *Manager) withSession(ctx context.Context, f func(Session) error) error {
	m.mu.Lock()
	relink := m.relink
	m.mu.Unlock()
	s := m.session()
	if s == nil || relink {
		return ErrNotLinked
	}
	err := f(*s)
	if !errors.Is(err, ErrUnauthorized) {
		return err
	}
	ns, rerr := m.fresher(ctx, s)
	if rerr != nil {
		return rerr
	}
	if ns == nil {
		return err // just renewed: another token would be refused as well
	}
	return f(*ns)
}

// fresher returns a session newer than the refused s, or nil when s is itself brand new.
func (m *Manager) fresher(ctx context.Context, s *Session) (*Session, error) {
	m.syncMu.Lock()
	defer m.syncMu.Unlock()
	if cur := m.session(); cur == nil {
		return nil, ErrNotLinked
	} else if cur.ServiceToken != s.ServiceToken {
		return cur, nil
	}
	if time.Since(m.renewed) < renewPause {
		return nil, nil
	}
	// The renewal serves every speaker, so it outlives the one impatient read that began it.
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 25*time.Second)
	defer cancel()
	return m.renew(c, s)
}

// PlayStatus reads a speaker's player state.
func (m *Manager) PlayStatus(ctx context.Context, deviceID string) (PlayStatus, error) {
	var p PlayStatus
	err := m.withSession(ctx, func(s Session) (err error) {
		p, err = m.client.PlayStatus(ctx, s, deviceID)
		return err
	})
	return p, err
}

// PlayerOp sends play / pause / stop.
func (m *Manager) PlayerOp(ctx context.Context, deviceID, action string) error {
	return m.withSession(ctx, func(s Session) error { return m.client.PlayerOp(ctx, s, deviceID, action) })
}

// SetVolume sets the speaker volume.
func (m *Manager) SetVolume(ctx context.Context, deviceID string, volume int) error {
	return m.withSession(ctx, func(s Session) error { return m.client.SetVolume(ctx, s, deviceID, volume) })
}

// Speak has a speaker say text.
func (m *Manager) Speak(ctx context.Context, deviceID, text string) error {
	return m.withSession(ctx, func(s Session) error { return m.client.Speak(ctx, s, deviceID, text) })
}

// PlayURL pushes an audio URL to a speaker.
func (m *Manager) PlayURL(ctx context.Context, deviceID, hardware, audioURL string) error {
	return m.withSession(ctx, func(s Session) error { return m.client.PlayURL(ctx, s, deviceID, hardware, audioURL) })
}
