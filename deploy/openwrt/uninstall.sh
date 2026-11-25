#!/bin/sh
# Remove only this draft's management layer. Keep binaries, data, config, account and cgroup.
set -eu
[ "$(id -u)" = 0 ] && [ "$#" -eq 0 ] || exit 1
[ -r /usr/libexec/trackferry-openwrt/common.sh ] || exit 1
. /usr/libexec/trackferry-openwrt/common.sh
exec 9>"$TF_MANAGE_LOCK"
flock -n 9 || { tf_error "Another install, uninstall or start is running"; exit 1; }
tf_load || exit 1
[ -x "$TF_INIT" ] || exit 1
for p in "$TF_LIB" "$TF_INIT" /usr/bin/tf "$TF_LIB/init.installed" "$TF_LIB/tf.installed" "$TF_LIB/hotplug.installed" "$TF_MOUNT_HOOK" "$TF_IFACE_HOOK"; do
    tf_trusted "$p" || { tf_error "Untrusted management path: $p"; exit 1; }
done
cmp -s "$TF_INIT" "$TF_LIB/init.installed" && cmp -s /usr/bin/tf "$TF_LIB/tf.installed" &&
    cmp -s "$TF_MOUNT_HOOK" "$TF_LIB/hotplug.installed" && cmp -s "$TF_IFACE_HOOK" "$TF_LIB/hotplug.installed" ||
    { tf_error 'Management files have changed; refusing to stop or remove them'; exit 1; }
"$TF_INIT" stop
i=0
while ! tf_empty; do
    i=$((i+1)); [ "$i" -lt 30 ] || { tf_error 'Service still running; not removing files'; exit 1; }
    sleep 1
done
"$TF_INIT" disable
rm -f "$TF_MOUNT_HOOK" "$TF_IFACE_HOOK" "$TF_PENDING"
rm -f "$TF_INIT" /usr/bin/tf "$TF_SETTINGS" "$TF_LIB/run.sh" "$TF_LIB/common.sh" "$TF_LIB/memory.sh" "$TF_LIB/tf.installed" "$TF_LIB/init.installed" "$TF_LIB/hotplug.installed"
rmdir "$TF_LIB" 2>/dev/null || echo "TrackFerry: $TF_LIB not empty; remove it before reinstalling" >&2
echo "Management removed; preserved $TF_APP, account and cgroup."
