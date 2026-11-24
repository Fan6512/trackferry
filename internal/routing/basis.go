package routing

import (
	"fmt"
	"strings"

	"trackferry/internal/intent"
	"trackferry/internal/library"
	"trackferry/internal/names"
	"trackferry/internal/online"
)

// Fact is one thing a decision rests on, for people.
type Fact struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// Because records what the decision rests on.
func (r *Route) Because(name, format string, args ...any) {
	r.Basis = append(r.Basis, Fact{name, fmt.Sprintf(format, args...)})
}

// Heard says how an utterance was understood.
func Heard(query string, in intent.Intent) string {
	var parts []string
	e := intent.Explain(query)
	if e.Verb != "" && in.Kind != intent.KindControl && in.Kind != intent.KindFavorite {
		parts = append(parts, "开头是“"+e.Verb+"”")
	}
	switch in.Kind {
	case intent.KindSong:
		parts = append(parts, "点歌")
	case intent.KindArtist:
		parts = append(parts, "听一位歌手")
	case intent.KindRandom:
		parts = append(parts, "随便听")
	case intent.KindVersion:
		parts = append(parts, "换一位歌手唱")
	case intent.KindList:
		what := "飞牛音乐的收藏"
		if in.Control == intent.ListPlaylist {
			what = "飞牛音乐的歌单"
			if in.Title != "" {
				what += "“" + in.Title + "”"
			}
		}
		return strings.Join(append(parts, "随机连播"+what), "，")
	case intent.KindFavorite:
		switch in.Control {
		case intent.ListAdd:
			return "把正在放的歌加入歌单 " + in.Title
		case intent.ListRemove:
			return "把正在放的歌移出歌单 " + in.Title
		}
		if in.Control == "remove" {
			return "取消收藏正在放的歌"
		}
		return "收藏正在放的歌"
	case intent.KindControl:
		return "播放控制"
	default:
		if e.Word != "" {
			return "不是点歌：有“" + e.Word + "”"
		}
		if e.Verb == "" {
			return "不是点歌：开头没有“播放”“我想听”这类词"
		}
		return "不是点歌"
	}
	if in.Artist != "" {
		parts = append(parts, "歌手 "+in.Artist)
	}
	if in.Title != "" && in.Kind != intent.KindVersion {
		parts = append(parts, "歌名 "+in.Title)
	}
	return strings.Join(parts, "，")
}

// chose says why m was taken from several copies by the same singer.
func chose(m library.Match, all []library.Match, request string) string {
	if len(all) < 2 {
		if m.Album != "" {
			return "只有这一首对得上，在专辑《" + m.Album + "》"
		}
		return "只有这一首对得上"
	}
	s := fmt.Sprintf("有 %d 首对得上，放%s的这首", len(all), m.Artist)
	if m.Album != "" {
		s = fmt.Sprintf("有 %d 首对得上，放%s《%s》里的", len(all), m.Artist, m.Album)
	}
	mine := names.VersionPenalty(m.Title+" "+m.Album, request)
	worse := 0
	for _, o := range all {
		if names.VersionPenalty(o.Title+" "+o.Album, request) > mine {
			worse++
		}
	}
	switch {
	case m.Favorite:
		s += "：你收藏了它"
	case worse > 0:
		s += fmt.Sprintf("：另外 %d 首是现场、伴奏这类版本", worse)
	default:
		s += "：歌名和格式排在最前"
	}
	return s
}

// online says how the platforms came to the song.
func (r *Route) online(outs []online.Outcome) {
	asked := 0
	for _, o := range outs {
		if o.State != online.StateDisabled && o.State != online.StateOpen {
			asked++
		}
	}
	who := map[string]string{"kuwo": "酷我", "kugou": "酷狗", "migu": "咪咕", "netease": "网易云"}[r.Source]
	if who == "" {
		return
	}
	for _, o := range outs {
		if o.Source == r.Source && o.Detail != "" {
			r.Because("选它", "%s：%s", who, o.Detail)
			return
		}
	}
	if asked > 1 {
		r.Because("选它", "同时问了 %d 家，%s先给出完整版", asked, who)
	} else {
		r.Because("选它", "%s有免费的完整版", who)
	}
}
