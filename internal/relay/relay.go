// Package relay carries connections from a Unix socket to one fixed TCP address. It runs in
// its own small container on the host's network, next to a main container that stays on a
// private one: the host's firewall has no say in a socket on a shared volume, so 飞牛音乐 is
// reachable without opening anything.
package relay

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Socket is where the relay listens, on a volume both containers mount.
const Socket = "/run/trackferry/feiniu.sock"

// Serve accepts on socket until ctx ends.
func Serve(ctx context.Context, socket, target string, log *slog.Logger) error {
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		return err
	}
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		ln.Close()
		return err
	}
	stop := context.AfterFunc(ctx, func() { ln.Close() })
	defer stop()
	log.Info("中转　已启动", "套接字", socket, "目标", target)
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go carry(ctx, c, target)
	}
}

func carry(ctx context.Context, in net.Conn, target string) {
	defer in.Close()
	out, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", target)
	if err != nil {
		return
	}
	defer out.Close()
	stop := context.AfterFunc(ctx, func() { in.Close(); out.Close() })
	defer stop()
	done := make(chan struct{}, 2)
	pipe := func(dst, src net.Conn) {
		io.Copy(dst, src)
		if c, ok := dst.(interface{ CloseWrite() error }); ok {
			c.CloseWrite()
		}
		done <- struct{}{}
	}
	go pipe(out, in)
	go pipe(in, out)
	<-done
	<-done
}
