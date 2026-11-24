package xiaomi

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func TestPlayerCallsRelayThroughUbus(t *testing.T) {
	f := newFake(t)
	m, _ := newManager(t, f, filepath.Join(t.TempDir(), "xiaomi.json"))
	ctx := context.Background()
	if _, err := m.PlayStatus(ctx, "d1"); err != ErrNotLinked {
		t.Fatalf("unlinked: %v", err)
	}
	m.StartQR(ctx)
	close(f.scanned)
	waitState(t, m, QRLinked)
	f.acceptToken.Store("rotated-elsewhere") // first call must renew

	p, err := m.PlayStatus(ctx, "d1")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Playing() || p.Volume != 35 || p.AudioID != "987654321987654321" || p.Position != 4200 || p.Duration != 200000 {
		t.Fatalf("status %+v", p)
	}
	if err := m.PlayerOp(ctx, "d1", "stop"); err != nil {
		t.Fatal(err)
	}
	var de *DeviceError
	if err := m.SetVolume(ctx, "d1", 0); !errors.As(err, &de) || de.Code != 123 {
		t.Fatalf("refused volume: %v", err)
	}
	if err := m.PlayURL(ctx, "d1", "LX06", "http://nas/stream/abc"); err != nil {
		t.Fatal(err)
	}
	if err := m.PlayURL(ctx, "d1", "S12", "http://nas/stream/def"); err != nil {
		t.Fatal(err)
	}

	f.ubusMu.Lock()
	defer f.ubusMu.Unlock()
	var methods []string
	for _, c := range f.ubusCalls {
		methods = append(methods, c.Get("method"))
	}
	want := []string{"player_get_play_status", "player_play_operation", "player_set_volume", "player_play_music", "player_play_url"}
	if len(methods) != len(want) {
		t.Fatalf("methods %v", methods)
	}
	for i := range want {
		if methods[i] != want[i] {
			t.Fatalf("methods %v", methods)
		}
	}
	var music struct {
		Start string `json:"startaudioid"`
		Music string `json:"music"`
	}
	json.Unmarshal([]byte(f.ubusCalls[3].Get("message")), &music)
	var payload struct {
		Behavior string `json:"play_behavior"`
		Payload  struct {
			Items []struct {
				Stream struct {
					URL string `json:"url"`
				} `json:"stream"`
			} `json:"audio_items"`
		} `json:"payload"`
	}
	json.Unmarshal([]byte(music.Music), &payload)
	if music.Start != OurAudioID || payload.Behavior != "REPLACE_ALL" || payload.Payload.Items[0].Stream.URL != "http://nas/stream/abc" {
		t.Fatalf("play_music message %s", f.ubusCalls[3].Get("message"))
	}
	if f.ubusCalls[1].Get("deviceId") != "d1" || f.ubusCalls[1].Get("path") != "mediaplayer" {
		t.Fatalf("op form %v", f.ubusCalls[1])
	}
}

func TestParsePlayStatusRejectsMissingState(t *testing.T) {
	if _, err := parsePlayStatus(json.RawMessage(`{"code":0,"info":"{}"}`)); err == nil {
		t.Fatal("status without state accepted")
	}
	p, err := parsePlayStatus(json.RawMessage(`{"code":0,"info":"{\"status\":0}"}`))
	if err != nil || p.Volume != -1 || p.Position != -1 || p.Playing() {
		t.Fatalf("%+v %v", p, err)
	}
}
