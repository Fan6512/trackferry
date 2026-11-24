package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHashVerify(t *testing.T) {
	if _, _, err := Hash("short"); err != ErrWeakPassword {
		t.Fatalf("weak: %v", err)
	}
	h, salt, err := Hash("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify("correct horse", h, salt, Iterations) || Verify("correct hors", h, salt, Iterations) {
		t.Fatal("verify mismatch")
	}
}

func TestSetupCode(t *testing.T) {
	p := filepath.Join(t.TempDir(), "private", "setup_code")
	a, err := LoadOrCreateSetupCode(p)
	if err != nil || len(a) != 8 {
		t.Fatalf("code %q %v", a, err)
	}
	b, _ := LoadOrCreateSetupCode(p)
	if a != b {
		t.Fatal("code changed between loads")
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("perm %v", fi.Mode().Perm())
	}
	if !SetupCodeMatches(a, " "+a[:4]+"-"+a[4:]+" ") {
		t.Fatal("formatted code rejected")
	}
	if SetupCodeMatches("", "") || SetupCodeMatches("short", "short") {
		t.Fatal("empty or short code accepted")
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter()
	now := time.Now()
	for range limitCount {
		l.Fail("ip", now)
	}
	if ok, wait := l.Allow("ip", now); ok || wait <= 0 {
		t.Fatal("should be throttled")
	}
	if ok, _ := l.Allow("ip", now.Add(limitWindow+time.Second)); !ok {
		t.Fatal("window should expire")
	}
	if ok, _ := l.Allow("other", now); !ok {
		t.Fatal("other client throttled")
	}
}
