// Package fscheck verifies the container's mounts: the music library must be readable and
// read-only, the data directory must be writable.
package fscheck

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Music describes the library mount.
type Music struct {
	Path     string `json:"path"`
	Readable bool   `json:"readable"`
	ReadOnly bool   `json:"read_only"`
	Entries  int    `json:"top_level_entries"`
	Note     string `json:"note,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Data describes the persistent data mount.
type Data struct {
	Path     string `json:"path"`
	Writable bool   `json:"writable"`
	Error    string `json:"error,omitempty"`
}

// OK reports whether the library is usable as TrackFerry expects.
func (m Music) OK() bool { return m.Readable && m.ReadOnly }

// CheckMusic lists the top level of dir and confirms it rejects writes.
// The write probe uses O_EXCL on a name that should never exist, and removes it if the mount
// turns out to be writable so a misconfiguration leaves no trace.
func CheckMusic(dir string) Music {
	m := Music{Path: dir}
	entries, err := os.ReadDir(dir)
	if err != nil {
		m.Error = err.Error()
		return m
	}
	m.Readable, m.Entries = true, len(entries)
	probe := filepath.Join(dir, ".trackferry-write-probe")
	f, err := os.OpenFile(probe, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	switch {
	case err == nil:
		f.Close()
		os.Remove(probe)
		m.Error = "music directory is writable; mount it read-only"
	case errors.Is(err, syscall.EROFS):
		m.ReadOnly = true
	default:
		// Permission denied also keeps the library safe, but report it so a missing :ro is noticed.
		m.ReadOnly = errors.Is(err, os.ErrPermission)
		if m.ReadOnly {
			m.Note = "write blocked by permissions, not by a read-only mount"
		} else {
			m.Error = fmt.Sprintf("write probe: %v", err)
		}
	}
	return m
}

// CheckData confirms dir accepts writes.
func CheckData(dir string) Data {
	d := Data{Path: dir}
	f, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		d.Error = err.Error()
		return d
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	d.Writable = true
	return d
}
