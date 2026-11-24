package takeover

import (
	"context"
	"time"

	"trackferry/internal/xiaomi"
)

// prompt bounds every call to a speaker: one answer lost on the way must not leave the
// room muted for as long as a connection takes to give up.
type prompt struct{ sp Speaker }

func (p prompt) PlayStatus(ctx context.Context, dev string) (xiaomi.PlayStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return p.sp.PlayStatus(ctx, dev)
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
