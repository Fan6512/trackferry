package home

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Line is one line of lyrics. T is when it is sung; null when the lyrics carry no times.
type Line struct {
	T    *int64 `json:"t"`
	Text string `json:"text"`
}

var (
	lrcTag    = regexp.MustCompile(`\[[^\]]*\]`)
	lrcTime   = regexp.MustCompile(`\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]`)
	lrcOffset = regexp.MustCompile(`(?i)^\[offset:\s*([+-]?\d{1,7})\]`)
)

// Lyrics reads LRC, with its offset already applied, or plain words.
func Lyrics(text string) (synced bool, lines []Line) {
	lines = []Line{}
	plain := []Line{}
	var shift int64
	for _, raw := range strings.Split(text, "\n") {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		if m := lrcOffset.FindStringSubmatch(raw); m != nil {
			shift, _ = strconv.ParseInt(m[1], 10, 64)
			continue
		}
		words := strings.TrimSpace(lrcTag.ReplaceAllString(raw, ""))
		if words == "" {
			continue
		}
		plain = append(plain, Line{Text: words})
		for _, m := range lrcTime.FindAllStringSubmatch(raw, -1) {
			min, _ := strconv.ParseInt(m[1], 10, 64)
			sec, _ := strconv.ParseInt(m[2], 10, 64)
			ms, _ := strconv.ParseInt((m[3] + "000")[:3], 10, 64)
			t := max(0, min*60000+sec*1000+ms-shift)
			lines = append(lines, Line{&t, words})
		}
	}
	if len(lines) == 0 {
		return false, plain
	}
	slices.SortStableFunc(lines, func(a, b Line) int { return cmp.Compare(*a.T, *b.T) })
	return true, lines
}
