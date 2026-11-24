package intent

import "testing"

func TestLists(t *testing.T) {
	fav := Intent{Kind: KindList, Control: ListFavorites}
	named := func(n string) Intent { return Intent{Kind: KindList, Control: ListPlaylist, Title: n} }
	put := func(c, n string) Intent { return Intent{Kind: KindFavorite, Control: c, Title: n, Follow: true} }
	for q, want := range map[string]Intent{
		"播放我的收藏": fav, "随机播放我的收藏": fav, "播放飞牛音乐的收藏": fav, "小爱同学，我想听我收藏的歌": fav, "播放收藏夹": fav,
		"播放收藏里的歌": fav, "播放我喜欢的歌": fav, "放一下我的收藏吧": fav, "听我的收藏": fav, "播放所有收藏的歌曲": fav, "播放收藏歌单": fav,
		"播放飞牛音乐里的收藏": fav, "播放飞牛里我的收藏": fav,
		"播放测试歌单": named("测试"), "播放歌单测试": named("测试"), "随机播放测试歌单里的歌": named("测试"), "我想听我的测试歌单": named("测试"),
		"播放名叫测试的歌单": named("测试"), "听测试歌单": named("测试"), "播放我的歌单": named(""), "播放飞牛音乐的测试歌单": named("测试"),
		"播放测试这个歌单": named("测试"), "循环播放开车歌单": named("开车"), "播放睡前歌单": named("睡前"), "播放my list歌单": named("my list"),
		"把这首歌加入测试歌单": put(ListAdd, "测试"), "加入测试歌单": put(ListAdd, "测试"), "将这首歌添加到我的测试歌单里": put(ListAdd, "测试"),
		"把这首歌加入歌单测试": put(ListAdd, "测试"), "收藏到测试歌单": put(ListAdd, "测试"), "给测试歌单加这首歌": put(ListAdd, "测试"),
		"在测试歌单里加入这首歌": put(ListAdd, "测试"), "把这首歌加入歌单": put(ListAdd, ""), "帮我把这首歌放进测试的歌单": put(ListAdd, "测试"),
		"从测试歌单里删掉这首歌": put(ListRemove, "测试"), "把这首歌从测试歌单移除": put(ListRemove, "测试"), "把这首歌移出测试歌单": put(ListRemove, "测试"),
		"收藏这首歌": put("add", ""), "把这首歌加入收藏": put("add", ""), "取消收藏": put("remove", ""), "加入我的收藏": put("add", ""),
		"播放喜欢": {Kind: KindSong, Title: "喜欢"}, "播放周杰伦的晴天": {Kind: KindSong, Title: "晴天", Artist: "周杰伦"}, "歌单有哪些": {}, "听海": {},
	} {
		if got := Parse(q); got != want {
			t.Errorf("%s: %+v", q, got)
		}
	}
}
