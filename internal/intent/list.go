package intent

import (
	"regexp"
	"strings"
)

// What a list request means, in Control.
const (
	ListFavorites = "favorites"
	ListPlaylist  = "playlist" // Title is its name; "" when none was said
)

// What a favourite request means, in Control, when it is about a playlist: Title names it.
const (
	ListAdd    = "list_add"
	ListRemove = "list_remove"
)

// Verbs that only make a request of a list: "听我的收藏" is one, "听海" is not.
var listVerbs = []string{"循环播放", "连续播放", "顺序播放", "开始播放", "打开", "听听", "听一下", "听"}

const (
	listOwner = `^(?:一下)?(?:飞牛音乐|飞牛)?(?:里面|里|中)?的?(?:我的|我)?`
	listSongs = `(?:里面|里|中)?的?(?:所有|全部)?(?:歌曲|歌|音乐|单曲)?`
)

var (
	favoritesRe = regexp.MustCompile(listOwner + `(?:所有|全部)?的?(?:收藏(?:过的|夹|列表|歌单)?` + listSongs + `|(?:喜欢|红心)的?(?:歌曲|歌单|歌|音乐))$`)
	namedRe     = regexp.MustCompile(listOwner + `(?:名字叫|名叫|名为|叫做|叫)?(.*?)(?:的|这个|那个)?歌单` + listSongs + `$`)
	afterRe     = regexp.MustCompile(listOwner + `歌单(?:名字叫|名叫|名为|叫做|叫)?(.+)$`)
)

// list recognises "播放我的收藏", "播放测试歌单", "随机播放歌单测试".
func list(q string) (Intent, bool) {
	body, ok := "", false
	for _, v := range listVerbs {
		if body, ok = strings.CutPrefix(q, v); ok {
			break
		}
	}
	if !ok {
		if body, ok = stripVerb(q); !ok {
			return Intent{}, false
		}
	}
	body = strings.TrimSpace(body)
	for _, p := range []string{"随机播放", "播放", "随机"} {
		body = strings.TrimPrefix(body, p)
	}
	tight := strings.ReplaceAll(body, " ", "")
	if favoritesRe.MatchString(tight) {
		return Intent{Kind: KindList, Control: ListFavorites}, true
	}
	for _, re := range []*regexp.Regexp{namedRe, afterRe} {
		if m := re.FindStringSubmatch(body); m != nil {
			return Intent{Kind: KindList, Control: ListPlaylist, Title: strings.TrimSpace(m[1])}, true
		}
	}
	return Intent{}, false
}

var (
	theSong    = `(?:这首歌曲|这首歌|这首|这个歌|这歌|当前歌曲|当前这首歌|当前这首|现在这首歌|现在这首|正在放的歌|正在播放的歌|正在放的这首歌|它)`
	putIn      = `(?:添加到|添加进|添加入|加入到|加入|加到|加进|放入|放进|放到|存到|存进|存入|收藏到|收藏进|收进|收到)`
	listEnd    = `歌单(?:里面|里|中)?去?$`
	addRe      = regexp.MustCompile(`^` + theSong + `?` + putIn + `(.*?)` + listEnd)
	addAfterRe = regexp.MustCompile(`^` + theSong + `?` + putIn + `歌单(.+?)(?:里面|里|中)?去?$`)
	addHereRe  = regexp.MustCompile(`^(?:在|往|向)?(.*?)歌单(?:里面|里|中)?(?:添加|加入|加上|放入|加)` + theSong + `$`)
	outRe      = regexp.MustCompile(`^` + theSong + `?(?:从|在)(.*?)歌单(?:里面|里|中)?(?:把|将)?` + theSong + `?(?:删除|删掉|删了|移除|移出|去掉|拿掉)` + theSong + `?$`)
	outOfRe    = regexp.MustCompile(`^` + theSong + `?(?:移出|移除出)(.*?)歌单$`)
)

// put recognises "把这首歌加入测试歌单" and its opposite. q is as favorite trims it.
func put(q string) (Intent, bool) {
	for i, re := range []*regexp.Regexp{addRe, addAfterRe, addHereRe, outRe, outOfRe} {
		m := re.FindStringSubmatch(q)
		if m == nil {
			continue
		}
		name := m[1]
		for _, p := range []string{"飞牛音乐的", "飞牛的", "我的", "名字叫", "名叫", "名为", "叫做", "叫"} {
			name = strings.TrimPrefix(name, p)
		}
		for _, s := range []string{"的", "这个", "那个"} {
			name = strings.TrimSuffix(name, s)
		}
		in := Intent{Kind: KindFavorite, Control: ListAdd, Title: name, Follow: true}
		if i > 2 {
			in.Control = ListRemove
		}
		if name == "收藏" || name == "喜欢" {
			in.Title, in.Control = "", strings.TrimPrefix(in.Control, "list_")
		}
		return in, true
	}
	return Intent{}, false
}
