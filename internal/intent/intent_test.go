package intent

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Intent
	}{
		{"播放晴天", Intent{Kind: KindSong, Title: "晴天"}},
		{"播放周杰伦的晴天", Intent{Kind: KindSong, Title: "晴天", Artist: "周杰伦"}},
		{"我想听周杰伦唱的稻香。", Intent{Kind: KindSong, Title: "稻香", Artist: "周杰伦"}},
		{"来一首后来", Intent{Kind: KindSong, Title: "后来"}},
		{"小爱同学，放一首七里香吧", Intent{Kind: KindSong, Title: "七里香"}},
		{"播放歌曲夜曲", Intent{Kind: KindSong, Title: "夜曲"}},
		{"播放晴天这首歌", Intent{Kind: KindSong, Title: "晴天"}},
		{"播放周杰伦的歌", Intent{Kind: KindArtist, Artist: "周杰伦"}},
		{"我要听陈奕迅的歌曲", Intent{Kind: KindArtist, Artist: "陈奕迅"}},
		{"放点音乐", Intent{Kind: KindRandom}},
		{"播放音乐", Intent{Kind: KindRandom}},
		{"来点好听的歌", Intent{Kind: KindRandom}},
		{"随便放一首歌", Intent{Kind: KindRandom}},
		{"暂停", Intent{Kind: KindControl, Control: "pause"}},
		{"下一首", Intent{Kind: KindControl, Control: "next"}},
		{"播放新闻", Intent{}},
		{"播放郭德纲的相声", Intent{}},
		{"放假安排", Intent{}},
		{"今天天气怎么样", Intent{}},
		{"给我讲个故事", Intent{}},
		{"", Intent{}},
	}
	for _, c := range cases {
		if got := Parse(c.in); got != c.want {
			t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}
