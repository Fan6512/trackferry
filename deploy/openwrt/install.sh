#!/bin/sh
# Fresh installations only. Existing experimental installations need a separate migration.
set -eu
[ "$(id -u)" = 0 ] || exit 1
[ "$#" -eq 4 ] || { echo 'Usage: sh install.sh /mnt/MOUNT LAN_IPV4 BINARY SIGNATURE' >&2; exit 2; }
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$here/common.sh"
. "$here/memory.sh"
mountpoint=$1; lan=$2; binary=$3; signature=$4
[ "$(uname -m)" = aarch64 ] || { tf_error 'ARM64 only in this draft'; exit 1; }
for cmd in openssl sha256sum ip stat ubus jsonfilter awk cmp readlink flock; do command -v "$cmd" >/dev/null || exit 1; done
[ -x /sbin/start-stop-daemon ] && [ -x /sbin/procd ] && [ -r /etc/rc.common ] || exit 1
tf_mount_path "$mountpoint" && tf_trusted /mnt && tf_trusted "$mountpoint" && tf_local_ip "$lan" && tf_identity || { tf_error 'Invalid mount, LAN IP or service account'; exit 1; }
tf_memory_load /proc/meminfo || { tf_error "Invalid memory statistics"; exit 1; }
tf_memory_report
[ "$TF_MEMORY_STATE" != unsupported ] || { tf_error "Device below minimum memory profile"; exit 1; }
if [ "$TF_MEMORY_STATE" = insufficient ]; then echo "Warning: current memory budget insufficient to start; installation stays disabled" >&2; fi
device=$(tf_device "$mountpoint" /proc/mounts) || exit 1
[ -n "$device" ] || exit 1
tf_cgroup2 /proc/mounts && grep -qw memory /sys/fs/cgroup/cgroup.controllers && tf_empty || exit 1
exec 9>"$TF_MANAGE_LOCK"
flock -n 9 || { tf_error "Another install/uninstall is running"; exit 1; }
app="$mountpoint/trackferry"
made_lib=0
for p in "$app" "$TF_INIT" "$TF_SETTINGS" "$TF_LIB" /usr/bin/tf "$TF_MOUNT_HOOK" "$TF_IFACE_HOOK"; do
    [ ! -e "$p" ] && [ ! -L "$p" ] || { tf_error "Existing path: $p (not modified)"; exit 1; }
done
[ -f "$binary" ] && [ -f "$signature" ] || exit 1
sh "$here/verify.sh" "$binary" "$signature"
umask 077
mkdir "$app"
# Until installation succeeds, remove only newly created files; never start the service here.
# The global paths below belong to whichever run created $TF_LIB; a second install on
# another mount must not remove them.
cleanup() {
    if [ "$made_lib" = 1 ]; then
        rm -f "$TF_INIT" "$TF_SETTINGS" /usr/bin/tf "$TF_MOUNT_HOOK" "$TF_IFACE_HOOK"
        rm -f "$TF_LIB/common.sh" "$TF_LIB/memory.sh" "$TF_LIB/run.sh" "$TF_LIB/tf.installed" "$TF_LIB/init.installed" "$TF_LIB/hotplug.installed"
        rmdir "$TF_LIB" 2>/dev/null || :
    fi
    rm -f "$app/config.yaml" "$app/trackferry-linux-arm64" "$app/trackferry-linux-arm64.sig"
    rmdir "$app/data" "$app/music" "$app" 2>/dev/null || :
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
cp "$binary" "$app/trackferry-linux-arm64"
cp "$signature" "$app/trackferry-linux-arm64.sig"
# Verify the actual installed bytes again before setting executable permissions.
sh "$here/verify.sh" "$app/trackferry-linux-arm64" "$app/trackferry-linux-arm64.sig"
mkdir "$app/data" "$app/music"
chown 10001:10001 "$app/data"
chmod 700 "$app/data"
chmod 755 "$app" "$app/music" "$app/trackferry-linux-arm64"
chmod 644 "$app/trackferry-linux-arm64.sig"
cat > "$app/config.yaml" <<EOF
listen: "$lan:18561"
public_base_url: "http://$lan:18561"
data_dir: "$app/data"
music_dir: "$app/music"
takeover_speakers: []
update_check: true
EOF
chmod 644 "$app/config.yaml"
mkdir -p /usr/libexec
mkdir "$TF_LIB"
made_lib=1
cp "$here/common.sh" "$TF_LIB/common.sh"
cp "$here/memory.sh" "$TF_LIB/memory.sh"
cp "$here/run.sh" "$TF_LIB/run.sh"
chmod 755 "$TF_LIB" "$TF_LIB/run.sh"
chmod 644 "$TF_LIB/common.sh" "$TF_LIB/memory.sh"
printf '%s\n' "$mountpoint" "$device" "$lan" > "$TF_SETTINGS"
cp "$here/trackferry.init" "$TF_INIT"
cp "$here/tf" /usr/bin/tf
chmod 755 "$TF_INIT" /usr/bin/tf
# Preserve root-owned exact copies to detect replaced management entry points on uninstall.
cp "$TF_INIT" "$TF_LIB/init.installed"
cp /usr/bin/tf "$TF_LIB/tf.installed"
mkdir -p /etc/hotplug.d/mount /etc/hotplug.d/iface
cp "$here/hotplug.sh" "$TF_MOUNT_HOOK"
cp "$here/hotplug.sh" "$TF_IFACE_HOOK"
chmod 644 "$TF_MOUNT_HOOK" "$TF_IFACE_HOOK"
cp "$here/hotplug.sh" "$TF_LIB/hotplug.installed"
chmod 600 "$TF_LIB/init.installed" "$TF_LIB/tf.installed" "$TF_LIB/hotplug.installed"
tf_preflight || exit 1
trap - EXIT HUP INT TERM
echo 'Installed (not started or enabled). Start with tf start after reviewing config.'
