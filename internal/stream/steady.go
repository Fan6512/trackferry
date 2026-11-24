package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// How a source that breaks off or stops sending is picked up again. Tests shorten these.
var (
	stallAfter = 12 * time.Second
	reopenWait = []time.Duration{100 * time.Millisecond, 400 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second}
)

// steady reads bytes off … end-1 of remote audio. When the source breaks off it goes on
// where it stopped, so a hiccup upstream is not heard as the song's end.
type steady struct {
	ctx    context.Context
	open   Opener
	body   io.ReadCloser
	cancel context.CancelFunc
	off    int64
	end    int64
	total  int64 // whole length, when known: a different one is a different recording
	fails  int
	since  int64 // read since the last reopening
	note   func(off int64)
}

func (s *steady) Read(p []byte) (int, error) {
	for {
		if s.off >= s.end {
			return 0, io.EOF
		}
		n, err := s.read(p[:min(int64(len(p)), s.end-s.off)])
		s.off += int64(n)
		if s.since += int64(n); s.since >= 256<<10 {
			s.fails = 0
		}
		if n > 0 {
			return n, nil
		}
		if err == nil {
			continue
		}
		if err := s.reopen(err); err != nil {
			return 0, err
		}
	}
}

func (s *steady) read(p []byte) (int, error) {
	if s.body == nil {
		return 0, io.ErrUnexpectedEOF
	}
	t := time.AfterFunc(stallAfter, s.cancel)
	defer t.Stop()
	return s.body.Read(p)
}

func (s *steady) reopen(cause error) error {
	s.Close()
	for {
		if s.ctx.Err() != nil {
			return s.ctx.Err()
		}
		if s.fails >= len(reopenWait) {
			return cause
		}
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-time.After(reopenWait[s.fails]):
		}
		s.fails++
		body, cancel, err := s.at(s.off)
		if err != nil {
			if errors.Is(err, ErrNoSeek) {
				return cause
			}
			cause = err
			continue
		}
		s.body, s.cancel, s.since = body, cancel, 0
		if s.note != nil {
			s.note(s.off)
		}
		return nil
	}
}

func (s *steady) at(from int64) (io.ReadCloser, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(s.ctx)
	t := time.AfterFunc(stallAfter, cancel)
	resp, err := s.open(ctx, fmt.Sprintf("bytes=%d-%d", from, s.end-1))
	t.Stop()
	if err != nil {
		cancel()
		return nil, nil, err
	}
	var a, b, total int64
	ok := resp.StatusCode == http.StatusPartialContent
	if ok {
		_, err := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%d", &a, &b, &total)
		ok = err == nil && a == from && (s.total <= 0 || total == s.total)
	}
	if !ok {
		resp.Body.Close()
		cancel()
		return nil, nil, ErrNoSeek
	}
	return resp.Body, cancel, nil
}

func (s *steady) Close() error {
	if s.body != nil {
		s.body.Close()
		s.cancel()
		s.body = nil
	}
	return nil
}

// openAgain opens remote audio, trying once more when the first try fails.
func openAgain(ctx context.Context, open Opener, rng string) (*http.Response, error) {
	resp, err := open(ctx, rng)
	if err == nil || ctx.Err() != nil {
		return resp, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(400 * time.Millisecond):
	}
	return open(ctx, rng)
}
