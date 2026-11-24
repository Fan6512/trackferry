package intent

import "testing"

func TestFavorite(t *testing.T) {
	for q, want := range map[string]string{
		"收藏": "add", "收藏这首歌": "add", "加入收藏": "add", "把这首歌加入收藏": "add", "小爱同学，帮我收藏一下这首歌": "add", "这首歌收藏一下": "add",
		"收藏一下": "add", "收藏起来": "add", "我喜欢这首歌": "add", "把它加到收藏里": "add", "收藏了": "add",
		"取消收藏": "remove", "把这首歌取消收藏": "remove", "取消收藏这首歌": "remove",
		"播放我的收藏": "", "打开收藏夹": "", "收藏家": "", "喜欢你": "", "我喜欢你": "", "播放收藏": "",
	} {
		in := Parse(q)
		if want == "" {
			if in.Kind == KindFavorite {
				t.Errorf("%s: %+v", q, in)
			}
		} else if in.Kind != KindFavorite || in.Control != want || !in.Follow {
			t.Errorf("%s: %+v", q, in)
		}
	}
}
