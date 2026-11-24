package fscheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWritableMusicIsReported(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.mp3"), nil, 0o600)
	m := CheckMusic(dir)
	if !m.Readable || m.ReadOnly || m.OK() || m.Entries != 1 {
		t.Fatalf("got %+v", m)
	}
	if _, err := os.Stat(filepath.Join(dir, ".trackferry-write-probe")); !os.IsNotExist(err) {
		t.Fatal("probe file left behind")
	}
}

func TestMissingMusic(t *testing.T) {
	m := CheckMusic(filepath.Join(t.TempDir(), "missing"))
	if m.Readable || m.Error == "" {
		t.Fatalf("got %+v", m)
	}
}

func TestData(t *testing.T) {
	if d := CheckData(t.TempDir()); !d.Writable {
		t.Fatalf("got %+v", d)
	}
	if d := CheckData(filepath.Join(t.TempDir(), "missing")); d.Writable {
		t.Fatalf("got %+v", d)
	}
}
