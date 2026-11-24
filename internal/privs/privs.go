// Package privs lets the container start as root only to hand its own folders to an ordinary
// identity, which everything else then runs as. A container told to run as some user skips
// all of it.
package privs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// DefaultID is an identity no account on the NAS uses.
const DefaultID = 10001

// Identity reads PUID and PGID. Root is not accepted: not running as root is the point.
func Identity(get func(string) string) (uid, gid int, err error) {
	if uid, err = id("PUID", get("PUID")); err != nil {
		return 0, 0, err
	}
	gid, err = id("PGID", get("PGID"))
	return uid, gid, err
}

func id(name, v string) (int, error) {
	if v == "" {
		return DefaultID, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 || n > 1<<31-2 {
		return 0, fmt.Errorf("%s=%q 不对，要写大于 0 的数字，例如 1000", name, v)
	}
	return n, nil
}

// ErrCannotDrop means the container is root without the means to stop being it.
var ErrCannotDrop = errors.New("以 root 启动，但不能切换到普通身份")

// Drop, when root, gives dirs to the identity and becomes it for good. Call it before any
// file those dirs hold is read or run. What cannot be given away is reported by whoever needs
// it: some mounts have no owners, yet can be written.
func Drop(dirs ...string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	uid, gid, err := Identity(os.Getenv)
	if err != nil {
		return err
	}
	for _, d := range dirs {
		give(d, uid, gid)
	}
	groups := []int{}
	if have, err := os.Getgroups(); err == nil {
		for _, g := range have { // group_add in the compose file
			if g != 0 && g != gid {
				groups = append(groups, g)
			}
		}
	}
	for _, e := range []error{syscall.Setgroups(groups), syscall.Setgid(gid), syscall.Setuid(uid)} {
		if e != nil {
			return fmt.Errorf("%w：%v", ErrCannotDrop, e)
		}
	}
	// No way back, or we are not who we say.
	if os.Geteuid() != uid || os.Getegid() != gid || syscall.Setuid(0) == nil {
		return ErrCannotDrop
	}
	for _, d := range dirs {
		if fi, err := os.Stat(d); err == nil && fi.Mode().Perm()&0o700 != 0o700 {
			os.Chmod(d, fi.Mode().Perm()|0o700)
		}
	}
	return nil
}

// give changes the owner of dir and what it holds. It follows no links and stays on dir's
// own filesystem, so nothing outside of it, or mounted into it, is touched.
func give(dir string, uid, gid int) {
	top, err := os.Lstat(dir)
	if err != nil || !top.IsDir() {
		return
	}
	dev := top.Sys().(*syscall.Stat_t).Dev
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok || st.Dev != dev {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if int(st.Uid) != uid || int(st.Gid) != gid {
			os.Lchown(p, uid, gid)
		}
		return nil
	})
}
