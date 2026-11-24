package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadOrCreateToken returns the admin token stored at path, generating a 256-bit token
// with 0600 permissions on first start. The token never appears in logs or API output.
func LoadOrCreateToken(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		tok := strings.TrimSpace(string(raw))
		if len(tok) < 32 {
			return "", fmt.Errorf("admin token in %s is too short", path)
		}
		return tok, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read admin token: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create private dir: %w", err)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(buf)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create admin token: %w", err)
	}
	if _, err := f.WriteString(tok + "\n"); err != nil {
		f.Close()
		return "", err
	}
	return tok, f.Close()
}
