package live

import (
	"context"
	"strings"
	"testing"
)

func TestFlowReachesWatchers(t *testing.T) {
	var none *Hub
	none.Pulse(Pulse{})
	none.Begin(Flow{}).Layer("library", 1)
	From(context.Background()).Done("")

	h := New()
	tr := h.Begin(Flow{Kind: "command", Said: "播放晴天"})
	ch, stop := h.Subscribe()
	if m := string(<-ch); !strings.HasPrefix(m, "event: hello") || !strings.Contains(m, "播放晴天") {
		t.Fatalf("a newcomer sees what is going on: %s", m)
	}
	tr.Layer("library", map[string]string{"state": "asking"})
	if m := string(<-ch); !strings.HasPrefix(m, "event: flow") || !strings.Contains(m, "asking") {
		t.Fatal(m)
	}
	if !h.Watching() {
		t.Fatal("watching")
	}
	stop()
	stop()
	if h.Watching() {
		t.Fatal("still watching")
	}
}

func TestSlowWatcherIsDropped(t *testing.T) {
	h := New()
	ch, stop := h.Subscribe()
	defer stop()
	for range 200 {
		h.Pulse(Pulse{DeviceID: "d"})
	}
	if h.Watching() {
		t.Fatal("a reader that stopped reading must not hold anything up")
	}
	for range ch {
	}
}
