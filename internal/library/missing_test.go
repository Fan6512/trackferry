package library

import (
	"context"
	"testing"

	"trackferry/internal/store"
)

func TestMissing(t *testing.T) {
	l, _ := newLib(t, map[string][]byte{"a.mp3": id3("晴天", "周杰伦")})
	if err := l.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := l.Missing([]store.OnlinePlay{
		{Title: "晴天", Artist: "周杰伦/某人", AtMS: 9}, {Title: "晴天 (Live)", Artist: "周杰伦", AtMS: 8},
		{Title: "海阔天空", Artist: "Beyond", AtMS: 5}, {Title: "海阔天空", Artist: "信乐团", AtMS: 4},
		{Title: "海阔天空", Artist: "Beyond", AtMS: 7}, {Title: "", AtMS: 1},
	})
	if len(got) != 4 || got[0].Artist != "Beyond" || got[0].Plays != 2 || got[0].LastMS != 7 || got[1].Title != "晴天 (Live)" || got[1].Owned || !got[3].Owned {
		t.Fatalf("%+v", got)
	}
}
