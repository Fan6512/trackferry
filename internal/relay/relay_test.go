package relay

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCarriesBothWaysAndReplacesAStaleSocket(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	dir, err := os.MkdirTemp("", "tf") // Unix socket paths must be short
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "s")
	os.WriteFile(sock, nil, 0o600)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, sock, ln.Addr().String(), slog.New(slog.DiscardHandler)) }()
	var c net.Conn
	for range 50 {
		if c, err = net.Dial("unix", sock); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	c.Write([]byte("hello"))
	c.(*net.UnixConn).CloseWrite()
	if b, _ := io.ReadAll(c); string(b) != "hello" {
		t.Fatalf("got %q", b)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
