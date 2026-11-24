package intent

import "strings"

// Reading explains how an utterance is understood.
type Reading struct {
	Intent
	Verb string `json:"verb,omitempty"` // the words that made it a request
	Word string `json:"word,omitempty"` // the word that made it something else
	Why  string `json:"why"`            // song | artist | random | control | no_verb | not_music | empty
}

// Explain parses an utterance and says why it came out that way.
func Explain(query string) Reading {
	r := Reading{Intent: Parse(query)}
	q := normalize(query)
	body, ok := "", false
	for _, v := range verbs {
		if strings.HasPrefix(q, v) {
			if body, ok = stripVerb(q); ok {
				r.Verb = v
				break
			}
		}
	}
	switch {
	case q == "":
		r.Why = "empty"
	case r.Kind != KindNone:
		r.Why = r.Kind
	case ok:
		r.Why = "not_music"
		for _, w := range nonMusic {
			if strings.Contains(body, w) {
				r.Word = w
				break
			}
		}
	default:
		r.Why = "no_verb"
	}
	return r
}

// Example is an utterance with what happens, for the guide.
type Example struct {
	Say  string `json:"say"`
	Then string `json:"then"`
	Kind string `json:"kind"`
}

// Guide is everything the recogniser knows, for people.
type Guide struct {
	Verbs    []string            `json:"verbs"`
	Polite   []string            `json:"polite"`
	Tails    []string            `json:"tails"`
	NotMusic []string            `json:"not_music"`
	Controls map[string][]string `json:"controls"`
	Yes      []Example           `json:"yes"`
	No       []Example           `json:"no"`
}

var yes = []Example{
	{"播放周杰伦的晴天", "放周杰伦的《晴天》", KindSong},
	{"我想听周杰伦唱的稻香", "放周杰伦的《稻香》", KindSong},
	{"来一首后来", "放《后来》", KindSong},
	{"小爱同学，放一首七里香吧", "称呼和语气词会去掉", KindSong},
	{"播放晴天这首歌", "放《晴天》", KindSong},
	{"点一首夜曲", "放《夜曲》", KindSong},
	{"播放晴天现场版", "说了版本，就找这个版本", KindSong},
	{"我要听陈奕迅的歌", "从曲库里挑一首陈奕迅", KindArtist},
	{"放等什么君的", "接着上一首：换成等什么君唱的", KindVersion},
	{"换成周杰伦的版本", "接着上一首：换成周杰伦唱的", KindVersion},
	{"不是这个，放邓紫棋的", "接着上一首：换成邓紫棋唱的", KindVersion},
	{"换个版本", "接着上一首：换一位歌手唱的", KindVersion},
	{"收藏这首歌", "放歌时说：加入飞牛音乐的收藏，歌接着放", KindFavorite},
	{"把这首歌加入收藏", "同上", KindFavorite},
	{"取消收藏", "放歌时说：从飞牛音乐的收藏里拿掉", KindFavorite},
	{"把这首歌加入测试歌单", "放歌时说：加进飞牛音乐里叫“测试”的歌单", KindFavorite},
	{"从测试歌单里删掉这首歌", "放歌时说：从歌单里拿掉，歌曲文件不动", KindFavorite},
	{"播放我的收藏", "随机连播飞牛音乐的收藏，放完一首接下一首", KindList},
	{"随机播放我喜欢的歌", "同上", KindList},
	{"播放测试歌单", "随机连播飞牛音乐里叫“测试”的歌单", KindList},
	{"我想听我的歌单测试", "同上", KindList},
	{"放点音乐", "从曲库里随机一首", KindRandom},
	{"随便放一首歌", "从曲库里随机一首", KindRandom},
}

var no = []Example{
	{"晴天", "没有点歌的开头", KindNone},
	{"周杰伦的晴天", "没有点歌的开头", KindNone},
	{"晴天怎么唱", "这是在问问题", KindNone},
	{"这首歌叫什么", "这是在问问题", KindNone},
	{"播放新闻", "新闻归小爱", KindNone},
	{"播放郭德纲的相声", "相声归小爱", KindNone},
	{"我想听故事", "故事归小爱", KindNone},
	{"放一下雨声", "白噪音归小爱", KindNone},
	{"今天天气怎么样", "闲聊归小爱", KindNone},
	{"打开客厅的灯", "家电归小爱", KindNone},
}

// TheGuide lists what is understood.
func TheGuide() Guide {
	g := Guide{Verbs: verbs, Polite: []string{"小爱同学", "请", "帮我", "麻烦"}, Tails: []string{"吧", "呗", "好吗", "好不好", "可以吗", "啊", "呀"},
		NotMusic: nonMusic, Controls: map[string][]string{}, Yes: yes, No: no}
	for _, p := range []string{"暂停", "暂停播放", "继续", "继续播放", "下一首", "换一首", "切歌", "上一首", "停止", "停止播放", "别放了"} {
		g.Controls[controls[p]] = append(g.Controls[controls[p]], p)
	}
	return g
}

// Silences reports that the listener wants quiet, in words that are not a playback control.
func Silences(query string) bool {
	q := normalize(query)
	for _, w := range []string{"停", "闭嘴", "安静", "别唱", "别放", "别播", "不要放", "不听了", "不想听", "关掉音乐", "关音乐", "关闭音乐", "关掉歌", "关机", "晚安", "退出"} {
		if strings.Contains(q, w) {
			return true
		}
	}
	return false
}
