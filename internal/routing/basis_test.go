package routing

import (
	"strings"
	"testing"

	"trackferry/internal/intent"
)

func TestHeardSaysWhy(t *testing.T) {
	for q, want := range map[string]string{
		"播放周杰伦的晴天": "歌手 周杰伦，歌名 晴天",
		"播放新闻":     "有“新闻”",
		"晴天怎么唱":    "开头没有",
		"收藏这首歌":    "收藏正在放的歌",
		"暂停":       "播放控制",
	} {
		if got := Heard(q, intent.Parse(q)); !strings.Contains(got, want) {
			t.Errorf("%s: %s", q, got)
		}
	}
}
