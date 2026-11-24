package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":18561" || cfg.DataDir != "/data" || cfg.MusicDir != "/music" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLegacyDevicesListIsIgnored(t *testing.T) {
	cfg, err := Load(write(t, "devices:\n  - alias: study\n    mode: takeover\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Legacy) != 1 {
		t.Fatalf("legacy = %v", cfg.Legacy)
	}
}

func TestValidationErrors(t *testing.T) {
	cases := map[string]string{
		"bad listen":     "listen: localhost\n",
		"port range":     "listen: ':70000'\n",
		"empty data dir": "data_dir: ''\n",
		"unknown yaml":   "listen: [\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(write(t, body)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestListenPort(t *testing.T) {
	if p, err := ListenPort("0.0.0.0:8080"); err != nil || p != 8080 {
		t.Fatalf("got %d, %v", p, err)
	}
}

func TestEnvironmentWins(t *testing.T) {
	t.Setenv("TRACKFERRY_URL", "http://192.0.2.1:18561/")
	t.Setenv("TRACKFERRY_TAKEOVER", "书房，客厅, ")
	t.Setenv("TRACKFERRY_UPDATE_CHECK", "off")
	cfg, err := Load(write(t, "public_base_url: http://192.0.2.9:1\ntakeover_speakers: [\"*\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicBaseURL != "http://192.0.2.1:18561" || len(cfg.TakeoverSpeakers) != 2 || cfg.TakeoverAllowed("主卧") || !cfg.TakeoverAllowed("书房音箱") || cfg.UpdateCheck {
		t.Fatalf("%+v", cfg)
	}
	t.Setenv("TRACKFERRY_URL", "192.0.2.1:18561")
	if _, err := Load(write(t, "")); err == nil {
		t.Fatal("an address without http:// was accepted")
	}
}

func TestLocalURL(t *testing.T) {
	for listen, want := range map[string]string{
		":18561":             "http://127.0.0.1:18561",
		"0.0.0.0:18561":      "http://127.0.0.1:18561",
		"[::]:18561":         "http://127.0.0.1:18561",
		"192.168.1.10:18561": "http://192.168.1.10:18561",
		"[fd00::1]:18561":    "http://[fd00::1]:18561",
	} {
		if got, err := (Config{Listen: listen}).LocalURL(); err != nil || got != want {
			t.Errorf("%s: %s %v", listen, got, err)
		}
	}
}
