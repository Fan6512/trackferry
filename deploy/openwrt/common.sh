#!/bin/sh
# Sourced by root-owned management scripts, never from the music/data directory.
TF_SETTINGS=/etc/trackferry-openwrt.conf
TF_CG=/sys/fs/cgroup/trackferry
TF_INIT=/etc/init.d/trackferry
TF_LIB=/usr/libexec/trackferry-openwrt
TF_PENDING=/var/run/trackferry.boot-pending
TF_MOUNT_HOOK=/etc/hotplug.d/mount/95-trackferry
TF_IFACE_HOOK=/etc/hotplug.d/iface/95-trackferry
TF_MANAGE_LOCK=/var/lock/trackferry-openwrt-install.lock
tf_error() { printf '%s\n' "TrackFerry: $*" >&2; return 1; }
# The flag lives in tmpfs, where any local user may plant a symlink. Remove whatever
# is there, then create with O_EXCL so a symlink raced in afterwards is never followed.
tf_pending_set() {
    rm -f "$TF_PENDING" 2>/dev/null
    (set -C; umask 077; : > "$TF_PENDING") 2>/dev/null
}
tf_mount_path() {
    case "$1" in /mnt/*) ;; *) return 1;; esac
    name=$(printf '%s' "$1" | cut -c6-)
    case "$name" in ''|.|..|*[!a-zA-Z0-9_.-]*) return 1;; esac
}
tf_ip() {
    printf '%s\n' "$1" | awk -F. '
      NR!=1 || NF!=4 {good=0; exit 1}
      {for(i=1;i<=4;i++) if($i !~ /^[0-9]+$/ || length($i)>3 || $i+0>255 || (length($i)>1 && substr($i,1,1)=="0")) exit 1
      if($1==0 || $1==127 || $1>=224 || ($1==169 && $2==254)) exit 1
      good=1} END {if(!good) exit 1}'
}
tf_trusted() {
    [ ! -L "$1" ] || return 1
    [ "$(stat -c %u "$1" 2>/dev/null)" = 0 ] || return 1
    mode=$(stat -c %a "$1") || return 1
    case "$mode" in ''|*[!0-7]*) return 1;; esac
    [ "$((0$mode & 022))" -eq 0 ]
}
tf_device() {
    awk -v p="$1" '$2==p && $3=="ext4" && $1 ~ /^\/dev\// {n++; if($4 ~ /(^|,)noexec(,|$)/ || $4 !~ /(^|,)rw(,|$)/) bad=1; else print $1} END {if(n!=1 || bad) exit 1}' "$2"
}
tf_local_ip() {
    tf_ip "$1" && ip -4 addr show | awk -v ip="$1" '$1=="inet" {split($2,a,"/"); if(a[1]==ip) ok=1} END {exit !ok}'
}
tf_identity() {
    [ "$(id -u trackferry 2>/dev/null)" = 10001 ] &&
    [ "$(id -g trackferry 2>/dev/null)" = 10001 ] &&
    [ "$(id -G trackferry 2>/dev/null)" = 10001 ]
}
tf_load() {
    tf_trusted "$TF_SETTINGS" || return 1
    # read reports EOF without a trailing newline, but still sets the variable.
    { IFS= read -r TF_MOUNT && IFS= read -r TF_DEVICE && { IFS= read -r TF_IP || [ -n "$TF_IP" ]; } && { extra=; ! IFS= read -r extra && [ -z "$extra" ]; }; } < "$TF_SETTINGS" || return 1
    tf_mount_path "$TF_MOUNT" && tf_ip "$TF_IP" || return 1
    TF_APP="$TF_MOUNT/trackferry"
}
tf_mounted() {
    tf_trusted /mnt && tf_trusted "$TF_MOUNT" &&
    device=$(tf_device "$TF_MOUNT" /proc/mounts) && [ -n "$device" ] && [ "$device" = "$TF_DEVICE" ]
}
tf_preflight() {
    tf_load && tf_mounted && tf_local_ip "$TF_IP" && tf_identity || return 1
    for p in "$TF_APP" "$TF_APP/trackferry-linux-arm64" "$TF_APP/config.yaml" "$TF_APP/music"; do
        tf_trusted "$p" || return 1
    done
    [ -x "$TF_APP/trackferry-linux-arm64" ] &&
    [ -f "$TF_APP/config.yaml" ] &&
    [ -d "$TF_APP/data" ] && [ ! -L "$TF_APP/data" ] &&
    [ "$(stat -c %u:%g:%a "$TF_APP/data")" = 10001:10001:700 ] &&
    [ -d "$TF_APP/music" ]
}
tf_empty() {
    [ ! -d "$TF_CG" ] || {
        members=$(cat "$TF_CG/cgroup.procs") && [ -z "$members" ]
    }
}
tf_cgroup2() {
    awk '$2=="/sys/fs/cgroup" && $3=="cgroup2" {ok=1} END {exit !ok}' "$1"
}
tf_guard() {
    [ "${TF_MEMORY_STATE:-}" = ok ] || return 1
    tf_cgroup2 /proc/mounts || return 1
    grep -qw memory /sys/fs/cgroup/cgroup.controllers || return 1
    tf_empty || return 1
    if ! grep -qw memory /sys/fs/cgroup/cgroup.subtree_control; then
        echo +memory > /sys/fs/cgroup/cgroup.subtree_control || return 1
    fi
    mkdir -p "$TF_CG" || return 1
    echo "$((TF_HIGH_MIB * 1048576))" > "$TF_CG/memory.high" &&
    echo "$((TF_MAX_MIB * 1048576))" > "$TF_CG/memory.max" &&
    echo 0 > "$TF_CG/memory.swap.max" &&
    tf_guard_values
}
tf_guard_values() {
    [ "${TF_MEMORY_STATE:-}" = ok ] &&
    [ "$(cat "$TF_CG/memory.high")" = "$((TF_HIGH_MIB * 1048576))" ] &&
    [ "$(cat "$TF_CG/memory.max")" = "$((TF_MAX_MIB * 1048576))" ] &&
    [ "$(cat "$TF_CG/memory.swap.max")" = 0 ]
}

# A signed upstream update execs in-place under data/update. This is a status
# check, not a signature verifier: upstream Launch verifies its own signed record.
tf_exe_path() {
    case "$1" in
        "$TF_APP/trackferry-linux-arm64") return 0;;
        "$TF_APP/data/update/trackferry-"*)
            v=${1#"$TF_APP/data/update/trackferry-"}
            printf '%s\n' "$v" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$';;
        *) return 1;;
    esac
}
tf_process() {
    tf_load || return 1
    members=$(cat "$TF_CG/cgroup.procs" 2>/dev/null) || return 1
    # The updater may briefly run a second version-probe child in the same cgroup.
    # Use the procd main PID, rather than guessing from all cgroup members.
    pid=$(ubus call service list '{"name":"trackferry"}' | jsonfilter -e '@.trackferry.instances.main.pid') || return 1
    case "$pid" in ''|*[!0-9]*) return 1;; esac
    printf '%s\n' "$members" | grep -qx "$pid" &&
    tf_exe_path "$(readlink "/proc/$pid/exe" 2>/dev/null)" &&
    awk '$1=="Uid:" {u=($2==10001 && $3==10001 && $4==10001 && $5==10001)}
         $1=="Gid:" {g=($2==10001 && $3==10001 && $4==10001 && $5==10001)} END {exit !(u && g)}' "/proc/$pid/status" 2>/dev/null &&
    grep -qx '0::/trackferry' "/proc/$pid/cgroup" 2>/dev/null
}
tf_wait_empty() {
    i=0
    while ! tf_empty; do
        i=$((i+1)); [ "$i" -lt 30 ] || return 1
        sleep 1
    done
}
tf_wait_running() {
    i=0
    while ! tf_process; do
        i=$((i+1)); [ "$i" -lt 15 ] || return 1
        sleep 1
    done
}
