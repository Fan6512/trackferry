package xiaomi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

type hushSpec struct{ muteS, muteP, pauseS, pauseA int }

// hushSpecs by model, from the MIoT specification of each: the mute property and the pause
// action. Models without a mute property are not here. Nor is
// LX06: it obeys the volume, and drops the stream it plays once its mute is lifted.
var hushSpecs = map[string]hushSpec{
	"OH2P": {2, 2, 3, 3}, "OH2": {2, 2, 3, 3},
	"L05B": {2, 2, 3, 3}, "L05C": {2, 2, 3, 3}, "L15A": {2, 2, 3, 3}, "L16A": {2, 2, 3, 3}, "L17A": {2, 2, 3, 3},
	"LX04": {2, 2, 3, 2}, "LX05": {2, 2, 3, 2}, "LX5A": {2, 2, 3, 2}, "L04M": {2, 2, 3, 2},
	"L06A": {2, 2, 3, 2}, "X08A": {2, 2, 4, 2}, "S12": {2, 2, 4, 1}, "LX01": {2, 2, 4, 1}, "L7A": {2, 2, 4, 1},
}

// floors by model, from the MIoT specification: the least volume, which 0 leads to.
var floors = map[string]int{
	"OH2P": 5, "OH2": 4, "LX04": 5, "LX05": 5, "LX06": 6, "LX5A": 12, "L06A": 6, "X08A": 5, "L7A": 3, "LX01": 1, "S12": 1,
	"L05B": 0, "L05C": 0, "L15A": 0, "L16A": 0, "L17A": 0,
}

// VolumeFloor is the least volume a model goes down to. Readings may be one off.
func VolumeFloor(hardware string) (int, bool) {
	n, ok := floors[strings.ToUpper(hardware)]
	return n + 1, ok
}

// CanHush reports whether a speaker can be muted through Mi Home.
func CanHush(hardware, did string) bool {
	_, ok := hushSpecs[strings.ToUpper(hardware)]
	return ok && did != ""
}

// Stubborn reports a model that goes on talking at full volume when its volume is set to
// nothing and it is told to stop.
func Stubborn(hardware string) bool { return strings.EqualFold(hardware, "OH2P") }

// Hush mutes a speaker through Mi Home and pauses what it plays, or lifts the mute.
func (m *Manager) Hush(ctx context.Context, hardware, did string, on bool) error {
	p, ok := hushSpecs[strings.ToUpper(hardware)]
	if !ok || did == "" {
		return ErrCannotWake
	}
	s, err := m.miotSession(ctx, "")
	if err != nil {
		return err
	}
	both := func() error {
		var wg sync.WaitGroup
		if on {
			wg.Go(func() { m.client.miotAction(ctx, *s, did, wakeAction{siid: p.pauseS, aiid: p.pauseA}) })
		}
		err := m.client.miotSet(ctx, *s, did, p.muteS, p.muteP, on)
		wg.Wait()
		return err
	}
	if err = both(); !errors.Is(err, ErrUnauthorized) {
		return err
	}
	if s, err = m.miotSession(ctx, s.ServiceToken); err != nil {
		return err
	}
	return both()
}

// WarmHome gets the Mi Home token ahead of its first use.
func (m *Manager) WarmHome(ctx context.Context) error {
	_, err := m.miotSession(ctx, "")
	return err
}

// miotSet writes one property.
func (c *Client) miotSet(ctx context.Context, s Session, did string, siid, piid int, value any) error {
	res, err := c.miotCall(ctx, s, "/miotspec/prop/set", []map[string]any{{"did": did, "siid": siid, "piid": piid, "value": value}})
	if err != nil {
		return err
	}
	var out []struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(res, &out) != nil || len(out) != 1 {
		return errf(KindProtocol, "prop/set: unexpected answer")
	}
	if n := out[0].Code; n != 0 && n != 1 { // 1: accepted, not confirmed
		return &DeviceError{Code: n}
	}
	return nil
}
