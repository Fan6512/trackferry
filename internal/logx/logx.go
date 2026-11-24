// Package logx writes the container log: one readable line per event, and a few blocks that
// are meant to be found at a glance.
package logx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// New returns the logger for a format: "json" for machines, anything else for people.
func New(w io.Writer, format string, debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	if format == "json" {
		return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
	}
	return slog.New(&handler{w: w, mu: &sync.Mutex{}, level: level})
}

type handler struct {
	w     io.Writer
	mu    *sync.Mutex
	level slog.Level
	attrs []slog.Attr
}

func (h *handler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }
func (h *handler) WithGroup(string) slog.Handler                { return h }
func (h *handler) WithAttrs(a []slog.Attr) slog.Handler {
	n := *h
	n.attrs = append(append([]slog.Attr{}, h.attrs...), a...)
	return &n
}

var levels = map[slog.Level]string{slog.LevelDebug: "调试", slog.LevelInfo: "    ", slog.LevelWarn: "注意", slog.LevelError: "错误"}

// Handle writes "11-24 09:08:12  注意  主题　内容  键=值".
func (h *handler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Time.Format("01-02 15:04:05"))
	b.WriteString("  ")
	b.WriteString(levels[r.Level])
	b.WriteString("  ")
	b.WriteString(clean(r.Message))
	put := func(a slog.Attr) bool {
		if v := text(a.Value); v != "" {
			fmt.Fprintf(&b, "  %s=%s", a.Key, v)
		}
		return true
	}
	for _, a := range h.attrs {
		put(a)
	}
	r.Attrs(put)
	b.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func text(v slog.Value) string {
	v = v.Resolve()
	if v.Kind() == slog.KindDuration {
		return v.Duration().Round(time.Millisecond).String()
	}
	s := clean(v.String())
	if strings.ContainsAny(s, " \"=") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// clean keeps one event on one line, whatever a song or a server is called.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

const rule = "────────────────────────────────────────────────────"

// Row is one line of a block.
type Row struct{ Name, Value string }

// Banner is the summary printed once the service is up.
func Banner(w io.Writer, title string, rows ...Row) {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s\n  %s\n\n", rule, title)
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s%s%s\n", r.Name, strings.Repeat("　", max(1, 6-len([]rune(r.Name)))), r.Value)
	}
	fmt.Fprintf(&b, "%s\n\n", rule)
	io.WriteString(w, b.String())
}

// SetupCode prints the code that sets the admin password, so that it cannot be overlooked.
func SetupCode(w io.Writer, code, url string) {
	if url == "" {
		url = "http://<NAS 地址>:18561"
	}
	fmt.Fprintf(w, "\n%s\n\n  首次使用，设置管理密码\n\n  初始化码　　%s\n  打开　　　　%s\n\n  只能用一次，设好密码后失效。\n  再看一次：docker exec trackferry /trackferry setup-code\n\n%s\n\n",
		strings.Repeat("━", 52), code, url, strings.Repeat("━", 52))
}
