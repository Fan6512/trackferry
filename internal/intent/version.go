package intent

import "strings"

var versionTails = []string{"唱的那个版本", "唱的版本", "的那个版本", "的版本", "唱的那首", "唱的", "那版", "的"}

// version recognises talk about another version of the song that was just asked for:
// "放等什么君的", "换成等什么君的版本", "换个版本". Such a sentence only means something after
// a song; follow says it means nothing at all otherwise.
func version(q string) (in Intent, ok bool) {
	for _, p := range []string{"不是这个版本", "不是这一版", "不要这个版本", "不要这个", "不是这个", "不是这首", "不要这首", "不对", "错了"} {
		rest, cut := strings.CutPrefix(q, p)
		if !cut {
			continue
		}
		rest = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rest), "我"))
		if rest == "" {
			if strings.HasSuffix(p, "版本") || strings.HasSuffix(p, "版") {
				return Intent{Kind: KindVersion, Follow: true}, true
			}
			return Intent{}, false
		}
		if in, ok = version(rest); ok {
			return in, true
		}
		if in, ok = version(strings.TrimSpace(strings.TrimPrefix(rest, "要"))); ok {
			return in, true
		}
		// "不是这个，放等什么君的", "不对，播放周杰伦的晴天": what follows is a request of its own.
		in = Parse(rest)
		return in, in.Kind == KindVersion || in.Kind == KindSong || in.Kind == KindArtist
	}
	switch q {
	case "换个版本", "换一个版本", "换版本", "换一版", "换个人唱的", "换个歌手", "换一个歌手", "放别的版本", "播放别的版本", "听别的版本", "还有别的版本", "放另一个版本", "换另一个版本":
		return Intent{Kind: KindVersion, Follow: true}, true
	}
	for _, v := range []string{"换成", "换到", "换一首", "换一个", "换个", "换", "改成", "改放", "我要", "要", "听"} {
		body, cut := strings.CutPrefix(q, v)
		if !cut {
			continue
		}
		for _, t := range versionTails {
			if a, cut := strings.CutSuffix(body, t); cut {
				if a = cleanName(a); a != "" && len([]rune(a)) <= 12 && !strings.Contains(a, "的") {
					return Intent{Kind: KindVersion, Artist: a, Follow: true}, true
				}
				return Intent{}, false
			}
		}
		return Intent{}, false
	}
	return Intent{}, false
}

// whose reads the body of a request that ends in 的: "放等什么君的".
func whose(body string) (Intent, bool) {
	for _, t := range versionTails {
		if a, cut := strings.CutSuffix(body, t); cut {
			if a = cleanName(a); a != "" && !strings.Contains(a, "的") {
				return Intent{Kind: KindVersion, Artist: a, Title: cleanName(body)}, true
			}
			return Intent{}, false
		}
	}
	return Intent{}, false
}
