#!/bin/sh
set -eu
cd "$(dirname "$0")"
for f in common.sh run.sh trackferry.init tf verify.sh install.sh uninstall.sh test.sh hotplug.sh; do sh -n "$f"; done
. ./common.sh
tf_mount_path /mnt/usb-1
for p in / /mnt /mnt/../etc /mnt/a/b /mnt/.. '/mnt/a b'; do
    if tf_mount_path "$p"; then echo "Accepted invalid mount: $p" >&2; exit 1; fi
done
tf_ip 192.168.1.1
tf_ip 172.16.0.1
for ip in 127.0.0.1 0.0.0.0 224.0.0.1 169.254.1.1 192.168.01.1 192.168.1.256 localhost '1.2.3.4;id'; do
    if tf_ip "$ip"; then echo "Accepted invalid IP: $ip" >&2; exit 1; fi
done
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
printf '/dev/sda2 /mnt/usb ext4 rw 0 0\n/dev/sda1 /mnt/other ext4 rw 0 0\n' > "$tmp"
[ "$(tf_device /mnt/usb "$tmp")" = /dev/sda2 ]
if tf_device /mnt/missing "$tmp"; then exit 1; fi
printf '/dev/sdb1 /mnt/usb ext4 rw 0 0\n' >> "$tmp"
if tf_device /mnt/usb "$tmp"; then exit 1; fi
printf 'cgroup2 /sys/fs/cgroup cgroup2 rw 0 0\n' > "$tmp"
tf_cgroup2 "$tmp"
printf 'cgroup /sys/fs/cgroup cgroup rw 0 0\n' > "$tmp"
if tf_cgroup2 "$tmp"; then exit 1; fi

# One name under /mnt, no traversal, separators or trailing slash.
for p in /mnt/ /mnt//etc /mnt/usb/ /mnt/a:b; do
    if tf_mount_path "$p"; then echo "Accepted invalid mount: $p" >&2; exit 1; fi
done
tf_mount_path /mnt/.hidden

# No whitespace, suffix, sign or second line; 223/8 is still unicast.
multiline='1.2.3.4
5.6.7.8'
for ip in '1.2.3.4 ' ' 1.2.3.4' '1.2.3.4rm' '1.2.3.+4' "$multiline" ''; do
    if tf_ip "$ip"; then echo "Accepted invalid IP: $ip" >&2; exit 1; fi
done
tf_ip 223.255.255.254

# ext4 only, and two entries for one path leave the device unknown.
printf '/dev/sda1 /mnt/usb vfat rw 0 0\n' > "$tmp"
if tf_device /mnt/usb "$tmp" >/dev/null; then echo 'Accepted a vfat mount' >&2; exit 1; fi
printf '/dev/sda1 /mnt/usb ext4 rw 0 0\n/dev/sda2 /mnt/usb ext4 rw 0 0\n' > "$tmp"
if tf_device /mnt/usb "$tmp" >/dev/null; then echo 'Accepted an ambiguous mount' >&2; exit 1; fi
printf '/dev/sda1 /mnt/other ext4 rw 0 0\n' > "$tmp"
if tf_device /mnt/usb "$tmp" >/dev/null; then echo 'Accepted a missing mount' >&2; exit 1; fi

# Reject noexec and read-only mounts before writing or launching.
for opts in rw,noexec ro rw,nosuid,noexec; do
    printf '/dev/sda1 /mnt/usb ext4 %s 0 0\n' "$opts" > "$tmp"
    if tf_device /mnt/usb "$tmp" >/dev/null; then echo "Accepted $opts mount" >&2; exit 1; fi
done

# The unified mount must be exactly /sys/fs/cgroup.
printf 'cgroup2 /sys/fs/cgroup cgroup2 rw,nosuid 0 0\n' > "$tmp"
tf_cgroup2 "$tmp"
printf 'cgroup2 /sys/fs/cgroup/unified cgroup2 rw 0 0\n' > "$tmp"
if tf_cgroup2 "$tmp"; then echo 'Accepted cgroup2 on another path' >&2; exit 1; fi

# Never a symlink, never group- or world-writable, never another owner.
printf 'x\n' > "$tmp"
chmod 666 "$tmp"
if tf_trusted "$tmp"; then echo 'Trusted a world-writable file' >&2; exit 1; fi
chmod 644 "$tmp"
if [ "$(id -u)" != 0 ] && tf_trusted "$tmp"; then
    echo 'Trusted a file owned by another user' >&2; exit 1
fi
if ln -s "$tmp" "$tmp.link" 2>/dev/null; then
    if tf_trusted "$tmp.link"; then echo 'Trusted a symlink' >&2; exit 1; fi
    rm -f "$tmp.link"
fi

# Three lines; the last newline is optional, a fourth line is not.
( tf_trusted() { return 0; }
  TF_SETTINGS=$tmp
  printf '/mnt/usb\n/dev/sda1\n192.168.1.5' > "$TF_SETTINGS"
  tf_load || { echo 'Rejected settings without a final newline' >&2; exit 1; }
  [ "$TF_APP" = /mnt/usb/trackferry ] || { echo 'Wrong app path from tf_load' >&2; exit 1; }
  printf '/mnt/usb\n/dev/sda1\n192.168.1.5\nx\n' > "$TF_SETTINGS"
  if tf_load; then echo 'Accepted a fourth settings line' >&2; exit 1; fi
  printf '/mnt/usb\n/dev/sda1\n192.168.1.5\nx' > "$TF_SETTINGS"
  if tf_load; then echo 'Accepted a fourth settings line without final newline' >&2; exit 1; fi
  printf '/mnt/usb\n/dev/sda1\n' > "$TF_SETTINGS"
  if tf_load; then echo 'Accepted settings without an address' >&2; exit 1; fi
  printf '/mnt\n/dev/sda1\n192.168.1.5\n' > "$TF_SETTINGS"
  if tf_load; then echo 'Accepted an install path outside /mnt/*' >&2; exit 1; fi
)

# A directory that is not a cgroup must never look empty.
( TF_CG=/nonexistent/trackferry
  tf_empty || { echo 'Rejected a missing cgroup directory' >&2; exit 1; }
  TF_CG=$tmp.d
  mkdir "$TF_CG"
  if tf_empty 2>/dev/null; then echo 'Treated a plain directory as an empty cgroup' >&2; exit 1; fi
  rmdir "$TF_CG"
)
echo 'Static shell, mount, IP, device, cgroup, identity-file and settings checks passed.'

# Only the pinned executable and upstream's exact versioned update path count.
( TF_APP=/mnt/usb/trackferry
  tf_exe_path "$TF_APP/trackferry-linux-arm64"
  tf_exe_path "$TF_APP/data/update/trackferry-1.6.0"
  for p in "$TF_APP/data/update/download" "$TF_APP/data/update/trackferry-1.6.0/evil" "$TF_APP/data/update/trackferry-1.6.0 (deleted)" /tmp/trackferry-1.6.0; do
      if tf_exe_path "$p"; then echo "Accepted wrong executable: $p" >&2; exit 1; fi
  done
)
