package takeover

import (
	"context"
	"fmt"
	"time"

	"trackferry/internal/xiaomi"
)

// prompt bounds every call to a speaker: one answer lost on the way must not leave the
// room muted for as long as a connection takes to give up.
type prompt struct{ sp Speaker }

// askAgain is when a status that has not come is asked for once more: single calls were seen
// to take three seconds while others passed in a third of one.
const askAgain = 800 * time.Millisecond

// PlayStatus is the only call made twice. Reading changes nothing, but a late second stop or
// volume would land on the song.
func (p prompt) PlayStatus(ctx context.Context, dev string) (xiaomi.PlayStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	type answer struct {
		p   xiaomi.PlayStatus
		err error
	}
	got := make(chan answer, 2)
	ask := func() {
		defer func() {
			if v := recover(); v != nil {
				got <- answer{err: fmt.Errorf("程序出错：%v", v)}
			}
		}()
		st, err := p.sp.PlayStatus(ctx, dev)
		got <- answer{st, err}
	}
	go ask()
	again := time.NewTimer(askAgain)
	defer again.Stop()
	for left := 1; ; {
		select {
		case a := <-got:
			if left--; a.err == nil || left == 0 {
				return a.p, a.err
			}
		case <-again.C:
			left++
			go ask()
		}
	}
}

func (p prompt) PlayerOp(ctx context.Context, dev, action string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.sp.PlayerOp(ctx, dev, action)
}

func (p prompt) SetVolume(ctx context.Context, dev string, v int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.sp.SetVolume(ctx, dev, v)
}

func (p prompt) PlayURL(ctx context.Context, dev, hardware, url string) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return p.sp.PlayURL(ctx, dev, hardware, url)
}

func (p prompt) Speak(ctx context.Context, dev, text string) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return p.sp.Speak(ctx, dev, text)
}
