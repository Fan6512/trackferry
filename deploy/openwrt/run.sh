#!/bin/sh
set -eu
. /usr/libexec/trackferry-openwrt/common.sh
. /usr/libexec/trackferry-openwrt/memory.sh
# Check at the actual start, not while registering the procd service.
if ! tf_preflight; then
    logger -t trackferry 'Refusing launch: mount, identity, configuration; inspect before recovery'
    exit 1
fi
# A procd restart may overlap with the previous instance's graceful termination: wait first,
# so the old process is gone (and its lock released) before this one takes over.
tf_wait_empty || { logger -t trackferry 'Refusing launch: the previous instance is still running'; exit 1; }
# One launcher owns this data directory. The descriptor survives the execs below, so the
# server itself holds the lock until it exits; a second launcher stops here instead of
# sharing the database. Only a hand-run binary bypasses this and then loses the port.
exec 9>"$TF_APP/.instance.lock" || exit 1
flock -n 9 || { logger -t trackferry 'Refusing launch: another instance is starting or running'; exit 1; }
# Sample only after the previous instance exits; do not lower the profile on pressure.
tf_memory_admit || { logger -t trackferry 'Refusing launch: insufficient or unknown memory budget; use tf mem'; exit 1; }
tf_guard || { logger -t trackferry 'Refusing launch: resource guard failed'; exit 1; }
logger -t trackferry "Memory profile: Go=$TF_GO_MIB MiB high=$TF_HIGH_MIB MiB max=$TF_MAX_MIB MiB reserve=$TF_RESERVE_MIB MiB"
# Recheck the mount and all guards immediately before the non-root exec.
tf_preflight && tf_guard_values || exit 1
echo "$$" > "$TF_CG/cgroup.procs"
grep -qx '0::/trackferry' /proc/self/cgroup || exit 1
export TRACKFERRY_CONFIG="$TF_APP/config.yaml"
export TRACKFERRY_LISTEN="$TF_IP:18561" TRACKFERRY_URL="http://$TF_IP:18561"
export GOMEMLIMIT="${TF_GO_MIB}MiB" GOGC=75 GOMAXPROCS=1
exec /sbin/start-stop-daemon -S -c trackferry:trackferry -x "$TF_APP/trackferry-linux-arm64" -- serve
