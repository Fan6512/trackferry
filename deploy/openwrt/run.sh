#!/bin/sh
set -eu
. /usr/libexec/trackferry-openwrt/common.sh
# Check at the actual start, not while registering the procd service.
if ! tf_preflight; then
    logger -t trackferry 'Refusing launch: mount, identity, configuration; inspect before recovery'
    exit 1
fi
# A procd restart may overlap with the previous instance's graceful termination.
tf_wait_empty && tf_guard || { logger -t trackferry 'Refusing launch: cgroup busy or resource guard failed'; exit 1; }
# Recheck the mount and all guards immediately before the non-root exec.
tf_preflight && tf_guard_values || exit 1
echo "$$" > "$TF_CG/cgroup.procs"
grep -qx '0::/trackferry' /proc/self/cgroup || exit 1
export TRACKFERRY_CONFIG="$TF_APP/config.yaml"
export TRACKFERRY_LISTEN="$TF_IP:18561" TRACKFERRY_URL="http://$TF_IP:18561"
export GOMEMLIMIT=32MiB GOGC=75 GOMAXPROCS=1
exec /sbin/start-stop-daemon -S -c trackferry:trackferry -x "$TF_APP/trackferry-linux-arm64" -- serve
