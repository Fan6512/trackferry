package logx

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLine(t *testing.T) {
	var b bytes.Buffer
	New(&b, "", false).With("音箱", "书房").Warn("点歌　取流\n失败", "歌", "a b", "空", "")
	got := b.String()
	if strings.Count(got, "\n") != 1 || !strings.HasSuffix(got, "注意  点歌　取流 失败  音箱=书房  歌=\"a b\"\n") {
		t.Fatalf("%q", got)
	}
	b.Reset()
	New(&b, "", false).Debug("x")
	if b.Len() != 0 {
		t.Fatal("debug line written")
	}
	New(&b, "json", false).Info("x", slog.Int("n", 1))
	if !strings.HasPrefix(b.String(), "{") {
		t.Fatal(b.String())
	}
}

func TestSetupCode(t *testing.T) {
	var b bytes.Buffer
	SetupCode(&b, "AB12-CD34", "")
	if !strings.Contains(b.String(), "初始化码　　AB12-CD34\n") {
		t.Fatal(b.String())
	}
}
