package intent

import "testing"

func TestGuideTellsTheTruth(t *testing.T) {
	for _, l := range [][]Example{yes, no} {
		for _, e := range l {
			if got := Parse(e.Say); got.Kind != e.Kind {
				t.Errorf("%s is %q, the guide says %q", e.Say, got.Kind, e.Kind)
			}
		}
	}
	for k := range TheGuide().Controls {
		if k == "" {
			t.Error("a listed control is not understood")
		}
	}
	if r := Explain("播放新闻"); r.Why != "not_music" || r.Word != "新闻" || r.Verb != "播放" {
		t.Errorf("%+v", r)
	}
	if r := Explain("今天天气怎么样"); r.Why != "no_verb" {
		t.Errorf("%+v", r)
	}
	if !Silences("别唱了") || Silences("今天几号") {
		t.Error("silences")
	}
}

func TestAnotherVersion(t *testing.T) {
	for q, artist := range map[string]string{"放等什么君的": "等什么君", "播放等什么君唱的": "等什么君", "换等什么君的": "等什么君", "换成等什么君的版本": "等什么君",
		"不是这个，放等什么君的": "等什么君", "不是这个版本，我要等什么君的": "等什么君", "我想听等什么君的": "等什么君", "换个版本": "", "不是这个版本": ""} {
		if in := Parse(q); in.Kind != KindVersion || in.Artist != artist {
			t.Errorf("%s: %+v", q, in)
		}
	}
	for _, q := range []string{"不是这个", "错了", "要不要出门", "听说今天下雨", "换一首", "我要睡觉了", "播放你的背包"} {
		if in := Parse(q); in.Kind == KindVersion {
			t.Errorf("%s: %+v", q, in)
		}
	}
}
