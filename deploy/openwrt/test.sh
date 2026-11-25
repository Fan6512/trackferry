#!/bin/sh
set -eu
cd "$(dirname "$0")"
for f in common.sh run.sh trackferry.init tf verify.sh install.sh uninstall.sh test.sh; do sh -n "$f"; done
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
echo 'Static shell and mount/IP input checks passed.'
