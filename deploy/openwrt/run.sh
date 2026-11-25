#!/bin/sh
set -eu
. /usr/libexec/trackferry-openwrt/common.sh
tf_preflight && tf_guard_values && tf_empty || exit 1
echo "$$" > "$TF_CG/cgroup.procs"
grep -qx '0::/trackferry' /proc/self/cgroup || exit 1
export TRACKFERRY_CONFIG="$TF_APP/config.yaml" TRACKFERRY_UPDATE_CHECK=off
export TRACKFERRY_LISTEN="$TF_IP:18561" TRACKFERRY_URL="http://$TF_IP:18561"
export GOMEMLIMIT=32MiB GOGC=75 GOMAXPROCS=1
exec /sbin/start-stop-daemon -S -c trackferry:trackferry -x "$TF_APP/trackferry-linux-arm64" -- serve
