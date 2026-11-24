// Package intent recognises song requests in Xiao AI speech-recognition text.
//
// It is deliberately rule based: every decision can be unit tested and explained in the
// command log, and anything that is not clearly about music is left to Xiao AI.
package intent

import (
	"strings"
	"unicode"
)

// Kinds of recognised commands.
const (
	KindSong     = "song"     // a specific song, optionally with the artist
	KindArtist   = "artist"   // songs by an artist
	KindRandom   = "random"   // "play some music"
	KindControl  = "control"  // pause, next, ...
	KindVersion  = "version"  // the song just asked for, by someone else
	KindFavorite = "favorite" // the song just asked for goes into the favourites or a playlist; Control says how
	KindList     = "list"     // a list of 飞牛音乐, one song after another; Control says which
	KindNone     = ""         // not a music command: handed to Xiao AI, not stored
)

// Intent is the parse result.
type Intent struct {
	Kind    string `json:"kind"`
	Title   string `json:"title,omitempty"`
	Artist  string `json:"artist,omitempty"`
	Control string `json:"control,omitempty"` // pause | resume | next | previous | stop; add | remove | list_add | list_remove; favorites | playlist
	// Follow: only means something right after a song.
	Follow bool `json:"follow,omitempty"`
}

// Music reports whether the command is something TrackFerry handles.
func (i Intent) Music() bool { return i.Kind != KindNone }

var controls = map[string]string{
	"暂停": "pause", "暂停播放": "pause", "暂停一下": "pause", "先暂停": "pause",
	"继续": "resume", "继续播放": "resume", "接着放": "resume", "接着播": "resume",
	"下一首": "next", "下一曲": "next", "切歌": "next", "换一首": "next", "换首歌": "next", "换一首歌": "next", "播放下一首": "next",
	"上一首": "previous", "上一曲": "previous", "播放上一首": "previous",
	"停止": "stop", "停止播放": "stop", "别放了": "stop", "关闭音乐": "stop", "停止音乐": "stop",
}

// Request verbs, longest first so "给我放" wins over "放".
var verbs = []string{
	"随便放", "随便来", "随机播放", "我想听一下", "我想听", "我要听", "给我放", "帮我放", "给我播放", "帮我播放", "给我唱", "我想听听",
	"播放一下", "播放", "放一下", "来一个", "来一首", "来首", "来点", "想听", "唱一首", "唱首", "点一首", "放",
}

// Counters and fillers that may follow the verb.
var fillers = []string{"一首", "首", "一个", "一下", "一些", "点", "些"}

// Things people "play" that are not music; these stay with Xiao AI.
var nonMusic = []string{
	"新闻", "故事", "相声", "评书", "小说", "电台", "广播", "有声书", "白噪音", "天气", "英语", "单词",
	"笑话", "播客", "节目", "电影", "电视", "视频", "动画片", "闹钟", "雨声", "海浪声", "睡前", "古诗", "课文",
}

// Generic "some music" bodies.
var generic = map[string]bool{
	"": true, "歌": true, "音乐": true, "歌曲": true, "首歌": true, "一首歌": true, "好听的": true, "好听的歌": true,
	"随便": true, "随机": true, "随便一首": true, "随机一首": true, "随便放": true, "流行歌": true, "流行音乐": true, "我的歌": true,
}

// Parse classifies one recognised utterance.
func Parse(query string) Intent {
	q := normalize(query)
	if q == "" {
		return Intent{}
	}
	if c, ok := controls[q]; ok {
		return Intent{Kind: KindControl, Control: c}
	}
	if in, ok := favorite(q); ok {
		return in
	}
	if in, ok := list(q); ok {
		return in
	}
	if v, ok := version(q); ok {
		return v
	}
	body, ok := stripVerb(q)
	if !ok {
		return Intent{}
	}
	for _, w := range nonMusic {
		if strings.Contains(body, w) {
			return Intent{}
		}
	}
	body = trimAffixes(body)
	if generic[body] {
		return Intent{Kind: KindRandom}
	}
	// "周杰伦的歌" / "周杰伦的音乐": the artist only.
	for _, suf := range []string{"的歌曲", "的歌", "的音乐", "的专辑", "唱的歌"} {
		if a, found := strings.CutSuffix(body, suf); found && a != "" {
			return Intent{Kind: KindArtist, Artist: cleanName(a)}
		}
	}
	if v, ok := whose(body); ok {
		return v
	}
	// "周杰伦唱的晴天" / "周杰伦的晴天".
	for _, sep := range []string{"唱的", "的"} {
		if i := strings.LastIndex(body, sep); i > 0 && i+len(sep) < len(body) {
			artist, title := cleanName(body[:i]), cleanName(body[i+len(sep):])
			if artist != "" && title != "" && !strings.Contains(artist, "的") {
				return Intent{Kind: KindSong, Title: title, Artist: artist}
			}
		}
	}
	// "晴天 周杰伦" is ambiguous ordering; keep the whole text as the title and let search
	// decide later.
	return Intent{Kind: KindSong, Title: cleanName(body)}
}

func normalize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	for _, p := range []string{"小爱同学", "小爱"} {
		s = strings.TrimSpace(strings.TrimPrefix(s, p))
	}
	for _, p := range []string{"请", "帮我", "麻烦"} {
		if strings.HasPrefix(s, p) && !strings.HasPrefix(s, "帮我放") && !strings.HasPrefix(s, "帮我播放") {
			s = strings.TrimSpace(strings.TrimPrefix(s, p))
		}
	}
	for _, suf := range []string{"吧", "呗", "好吗", "好不好", "可以吗", "啊", "呀"} {
		s = strings.TrimSpace(strings.TrimSuffix(s, suf))
	}
	return s
}

func stripVerb(q string) (string, bool) {
	for _, v := range verbs {
		if rest, ok := strings.CutPrefix(q, v); ok {
			// "放假" / "放心" are not requests; a bare "放" must be followed by a filler or a
			// recognisable body, which is checked by requiring the remainder not to start a
			// common word.
			if v == "放" && startsAny(rest, "假", "心", "弃", "学", "下", "大", "松", "开") {
				return "", false
			}
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

func trimAffixes(b string) string {
	for changed := true; changed; {
		changed = false
		for _, f := range fillers {
			if rest, ok := strings.CutPrefix(b, f); ok && rest != b {
				// Keep "点" when it is part of a title such as "点歌的人".
				if f == "点" && strings.HasPrefix(rest, "歌的") {
					continue
				}
				b, changed = strings.TrimSpace(rest), true
				break
			}
		}
	}
	for _, p := range []string{"歌曲", "单曲", "歌名叫", "一首叫", "叫"} {
		if rest, ok := strings.CutPrefix(b, p); ok && rest != "" {
			b = strings.TrimSpace(rest)
		}
	}
	for _, s := range []string{"这首歌曲", "这首歌", "这首", "这个歌", "的音乐吧"} {
		if rest, ok := strings.CutSuffix(b, s); ok && rest != "" {
			b = strings.TrimSpace(rest)
		}
	}
	if rest, ok := strings.CutSuffix(b, "歌曲"); ok && rest != "" && !strings.HasSuffix(rest, "的") {
		b = strings.TrimSpace(rest)
	}
	return b
}

func cleanName(s string) string {
	s = strings.TrimSpace(s)
	for _, w := range []string{"歌手", "歌曲", "一首"} {
		s = strings.TrimSpace(strings.TrimPrefix(s, w))
	}
	return s
}

func startsAny(s string, ps ...string) bool {
	for _, p := range ps {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
