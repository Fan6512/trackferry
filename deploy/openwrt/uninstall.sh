#!/bin/sh
# Remove only this draft's management layer. Keep binaries, data, config, account and cgroup.
set -eu
[ "$(id -u)" = 0 ] && [ "$#" -eq 0 ] || exit 1
[ -r /usr/libexec/trackferry-openwrt/common.sh ] || exit 1
. /usr/libexec/trackferry-openwrt/common.sh
tf_load || exit 1
[ -x "$TF_INIT" ] || exit 1
"$TF_INIT" stop
i=0
while ! tf_empty; do
    i=$((i+1)); [ "$i" -lt 30 ] || { tf_error 'Service still running; not removing files'; exit 1; }
    sleep 1
done
"$TF_INIT" disable
rm -f "$TF_INIT" /usr/bin/tf "$TF_SETTINGS" "$TF_LIB/run.sh" "$TF_LIB/common.sh"
rmdir "$TF_LIB"
echo "Management removed; preserved $TF_APP, account and cgroup."
