package xiaomi

// Waking a speaker. The Mina service has no call for it; the Mi Home service does, as an
// action of the public MIoT specification. The request signature follows MiService (MIT).

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const miotSID = "xiaomiio"

type wakeAction struct {
	siid, aiid int
	text       bool // takes a text, which may be empty
}

// wakeActions by model, from the MIoT specification of each.
var wakeActions = map[string]wakeAction{
	"07G": {5, 1, false}, "16B": {7, 1, false}, "L04M": {5, 2, true}, "L05B": {5, 1, false}, "L05C": {5, 1, false},
	"L05G": {5, 1, false}, "L06A": {5, 2, true}, "L09A": {3, 2, true}, "L09B": {7, 1, false}, "L15A": {7, 1, false},
	"L16A": {7, 1, false}, "L17A": {7, 1, false}, "L7A": {5, 2, false}, "LX01": {5, 2, false}, "LX04": {5, 2, true},
	"LX05": {5, 3, false}, "LX06": {5, 3, false}, "LX5A": {5, 3, false}, "M03A": {7, 1, false}, "OH11": {7, 1, false},
	"OH1M": {7, 1, false}, "OH1P": {7, 1, false}, "OH2": {5, 1, false}, "OH27": {7, 1, false}, "OH2P": {7, 1, false},
	"OH8": {7, 1, false}, "S12": {5, 3, false}, "X08A": {5, 2, true}, "X08C": {3, 2, false}, "X08E": {7, 1, false},
	"X10A": {7, 1, false}, "X4B": {5, 1, false}, "X6A": {7, 1, false}, "X8F": {7, 1, false}, "X8S": {7, 1, false},
}

// CanWake reports whether a speaker can be woken from here.
func CanWake(hardware, did string) bool {
	_, ok := wakeActions[strings.ToUpper(hardware)]
	return ok && did != ""
}

// ErrCannotWake means the model has no such action.
var ErrCannotWake = errors.New("this model cannot be woken")

// sign prepares a Mi Home request.
func sign(uri, data, ssecurity string, nonce []byte) (url.Values, error) {
	key, err := base64.StdEncoding.DecodeString(ssecurity)
	if err != nil {
		return nil, errf(KindProtocol, "ssecurity unreadable")
	}
	n := base64.StdEncoding.EncodeToString(nonce)
	sum := sha256.Sum256(append(key, nonce...))
	mac := hmac.New(sha256.New, sum[:])
	mac.Write([]byte(uri + "&" + base64.StdEncoding.EncodeToString(sum[:]) + "&" + n + "&data=" + data))
	return url.Values{"_nonce": {n}, "data": {data}, "signature": {base64.StdEncoding.EncodeToString(mac.Sum(nil))}}, nil
}

// miotAction runs one action. s carries a token for the Mi Home service.
func (c *Client) miotAction(ctx context.Context, s Session, did string, a wakeAction) error {
	in := []any{}
	if a.text {
		in = append(in, "")
	}
	res, err := c.miotCall(ctx, s, "/miotspec/action", map[string]any{"did": did, "siid": a.siid, "aiid": a.aiid, "in": in})
	if err != nil {
		return err
	}
	var out struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(res, &out) == nil && out.Code != 0 {
		return &DeviceError{Code: out.Code}
	}
	return nil
}

// miotCall makes one call and returns its result.
func (c *Client) miotCall(ctx context.Context, s Session, uri string, params any) (json.RawMessage, error) {
	raw, err := json.Marshal(map[string]any{"params": params})
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, 12)
	rand.Read(nonce[:8])
	binary.BigEndian.PutUint32(nonce[8:], uint32(time.Now().Unix()/60))
	form, err := sign(uri, string(raw), s.SSecurity, nonce)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.ep.Miot+uri, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent(s.DeviceID))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Xiaomi-Protocal-Flag-Cli", "PROTOCAL-HTTP2")
	req.Header.Set("Cookie", "userId="+s.UserID+"; serviceToken="+s.ServiceToken+"; PassportDeviceId="+s.DeviceID)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errf(KindNetwork, "miot: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, errf(KindNetwork, "miot: %v", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	var out struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Result  json.RawMessage `json:"result"`
	}
	if resp.StatusCode >= 400 || json.Unmarshal(body, &out) != nil {
		return nil, errf(KindUpstream, "miot: HTTP %d", resp.StatusCode)
	}
	if out.Code != 0 {
		if strings.Contains(strings.ToLower(out.Message), "auth") {
			return nil, ErrUnauthorized
		}
		return nil, errf(KindUpstream, "miot: code %d", out.Code)
	}
	return out.Result, nil
}

// Wake has a speaker listen, as if its name had been called.
func (m *Manager) Wake(ctx context.Context, hardware, did string) error {
	a, ok := wakeActions[strings.ToUpper(hardware)]
	if !ok || did == "" {
		return ErrCannotWake
	}
	s, err := m.miotSession(ctx, "")
	if err != nil {
		return err
	}
	err = m.client.miotAction(ctx, *s, did, a)
	if !errors.Is(err, ErrUnauthorized) {
		return err
	}
	if s, err = m.miotSession(ctx, s.ServiceToken); err != nil {
		return err
	}
	return m.client.miotAction(ctx, *s, did, a)
}

// miotSession returns a Mi Home token of the linked account, other than the refused one. It
// is kept in memory only, and its failure says nothing about the login itself.
func (m *Manager) miotSession(ctx context.Context, refused string) (*Session, error) {
	m.syncMu.Lock()
	defer m.syncMu.Unlock()
	m.mu.Lock()
	relink := m.relink
	m.mu.Unlock()
	s := m.session()
	if s == nil || relink {
		return nil, ErrNotLinked
	}
	if k := m.miot; k != nil && k.UserID == s.UserID && k.PassToken == s.PassToken && k.ServiceToken != refused && time.Since(k.ObtainedAt) < renewAfter {
		return k, nil
	}
	// The exchange may rotate the passToken, so it is not left half done.
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 25*time.Second)
	defer cancel()
	n, err := m.client.exchange(c, *s, miotSID)
	if err != nil {
		return nil, err
	}
	if n.PassToken != s.PassToken {
		s.PassToken = n.PassToken
		m.mu.Lock()
		m.sess = s
		m.mu.Unlock()
		if err := m.save(s); err != nil {
			m.log.Warn("小米　新的登录没有存上", "原因", err)
		}
	}
	m.miot = &n
	return &n, nil
}
