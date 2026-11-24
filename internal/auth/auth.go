// Package auth hashes the admin password and manages the first-run setup code.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Iterations is the PBKDF2-SHA256 work factor (OWASP 2023 guidance).
const Iterations = 600_000

const keyLen = 32

// ErrWeakPassword rejects passwords that are too short.
var ErrWeakPassword = errors.New("password must be at least 8 characters")

// Hash derives a key for password with a fresh salt.
func Hash(password string) (hash, salt []byte, err error) {
	if utf8.RuneCountInString(password) < 8 {
		return nil, nil, ErrWeakPassword
	}
	salt = make([]byte, 16)
	rand.Read(salt)
	hash, err = pbkdf2.Key(sha256.New, password, salt, Iterations, keyLen)
	return hash, salt, err
}

// Verify checks password against a stored hash.
func Verify(password string, hash, salt []byte, iterations int) bool {
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(hash))
	return err == nil && subtle.ConstantTimeCompare(got, hash) == 1
}

// NewToken returns a random URL-safe session token.
func NewToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return strings.ToLower(strings.TrimRight(base32.StdEncoding.EncodeToString(b), "="))
}

// LoadOrCreateSetupCode returns the one-time code that authorises setting the first admin
// password, so a stranger on the LAN cannot claim a fresh install. It is printed in the
// container log and stored under data/private.
func LoadOrCreateSetupCode(path string) (string, error) {
	if raw, err := os.ReadFile(path); err == nil {
		if c := strings.TrimSpace(string(raw)); len(c) >= 8 {
			return c, nil
		}
	}
	b := make([]byte, 5)
	rand.Read(b)
	code := base32.StdEncoding.EncodeToString(b) // 8 characters, A-Z2-7
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return code, os.WriteFile(path, []byte(code+"\n"), 0o600)
}

// SetupCodeMatches compares codes case-insensitively, ignoring spaces and dashes.
func SetupCodeMatches(want, got string) bool {
	norm := func(s string) []byte {
		return []byte(strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(s))))
	}
	w, g := norm(want), norm(got)
	return len(w) >= 8 && subtle.ConstantTimeCompare(w, g) == 1
}

// Limiter slows password guessing: after 5 failures within 15 minutes a client waits.
type Limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

// NewLimiter returns an empty Limiter.
func NewLimiter() *Limiter { return &Limiter{hits: map[string][]time.Time{}} }

const (
	limitWindow = 15 * time.Minute
	limitCount  = 5
)

func (l *Limiter) recent(key string, now time.Time) []time.Time {
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < limitWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.hits, key)
	} else {
		l.hits[key] = kept
	}
	return kept
}

// Allow reports whether key may try again, and if not, how long to wait.
func (l *Limiter) Allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r := l.recent(key, now)
	if len(r) < limitCount {
		return true, 0
	}
	return false, limitWindow - now.Sub(r[0])
}

// Fail records a failed attempt.
func (l *Limiter) Fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hits[key] = append(l.recent(key, now), now)
}

// Reset clears key after a success.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}
