package intent

import "strings"

var favoriteSongs = []string{"", "这首歌", "这首歌曲", "这首", "这个歌", "这歌", "当前歌曲", "当前这首", "当前这首歌", "现在这首歌", "现在这首", "正在放的歌", "正在播放的歌", "正在放的这首歌", "它"}

// favorite recognises "收藏这首歌", "把这首歌加入收藏", "取消收藏". The order of the words
// varies, the words themselves hardly do.
func favorite(q string) (Intent, bool) {
	c, ok := kept(&q)
	if ok {
		return Intent{Kind: KindFavorite, Control: c, Follow: true}, true
	}
	return put(q)
}

// kept reads q without its polite words, which it removes.
func kept(text *string) (string, bool) {
	q := *text
	q = strings.NewReplacer(" ", "", "一下", "").Replace(q)
	for _, p := range []string{"给我", "帮我", "我要", "我想", "把", "将", "给"} {
		q = strings.TrimPrefix(q, p)
	}
	for _, s := range []string{"起来", "下来", "了", "下"} {
		q = strings.TrimSuffix(q, s)
	}
	*text = q
	is := func(s string) bool {
		for _, w := range favoriteSongs {
			if s == w {
				return true
			}
		}
		return false
	}
	for _, v := range []string{"取消收藏", "不收藏", "移出收藏", "从收藏里删掉", "从收藏里移除", "从收藏中移除", "从收藏移除", "移出我的收藏", "别收藏"} {
		if rest, ok := strings.CutPrefix(q, v); ok && is(rest) {
			return "remove", true
		}
		if rest, ok := strings.CutSuffix(q, v); ok && is(rest) {
			return "remove", true
		}
	}
	for _, v := range []string{"加入到我的收藏", "加入我的收藏", "添加到我的收藏", "放进我的收藏", "加入到收藏", "加入收藏夹", "添加到收藏夹", "加入收藏", "添加到收藏", "添加收藏", "加到收藏里", "加到收藏", "加进收藏", "放进收藏", "存到收藏", "收藏下", "收藏"} {
		if rest, ok := strings.CutPrefix(q, v); ok && is(rest) {
			return "add", true
		}
		if rest, ok := strings.CutSuffix(q, v); ok && is(rest) {
			return "add", true
		}
	}
	switch q {
	case "喜欢这首歌", "喜欢这首", "我喜欢这首歌", "我喜欢这首", "我很喜欢这首歌", "很喜欢这首歌":
		return "add", true
	}
	return "", false
}
