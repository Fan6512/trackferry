#!/bin/sh
# Read Linux meminfo once. Values are KiB; limits/reserve are whole MiB.
# The profile is a proposal for the next start, not a runtime adjustment.
tf_memory_plan() {
    awk '
      $1=="MemTotal:" || $1=="MemAvailable:" {
        if (NF!=3 || $2 !~ /^[0-9]+$/ || $3!="kB") bad=1
        if ($1=="MemTotal:") { t=$2+0; nt++ }
        else { a=$2+0; na++ }
      }
      END {
        if (bad || nt!=1 || na!=1 || t<=0 || a<0 || a>t) exit 1
        reserve=int((t+8191)/8192)
        if (reserve<64) reserve=64
        g=h=m=0
        if (t<=128*1024) state="unsupported"
        else {
          if (t<=256*1024) {g=24; h=36; m=48}
          else if (t<=512*1024) {g=32; h=48; m=64}
          else if (t<=1024*1024) {g=48; h=72; m=96}
          else {g=64; h=96; m=128}
          state=(a>=(reserve+m)*1024 ? "ok" : "insufficient")
        }
        printf "%.0f %.0f %d %d %d %d %s\n", t,a,reserve,g,h,m,state
      }' "$1"
}
tf_memory_load() {
    plan=$(tf_memory_plan "$1") || return 1
    read -r TF_TOTAL_KIB TF_AVAILABLE_KIB TF_RESERVE_MIB TF_GO_MIB TF_HIGH_MIB TF_MAX_MIB TF_MEMORY_STATE <<EOF_PLAN
$plan
EOF_PLAN
}
tf_memory_report() {
    printf 'next_start: %s\nMemTotal_KiB: %s\nMemAvailable_KiB: %s\nreserve_MiB: %s\nGOMEMLIMIT_MiB: %s\nmemory.high_MiB: %s\nmemory.max_MiB: %s\n' \
        "$TF_MEMORY_STATE" "$TF_TOTAL_KIB" "$TF_AVAILABLE_KIB" "$TF_RESERVE_MIB" "$TF_GO_MIB" "$TF_HIGH_MIB" "$TF_MAX_MIB"
}
tf_memory_admit() {
    tf_memory_load /proc/meminfo || { echo 'TrackFerry: missing or invalid MemTotal/MemAvailable' >&2; return 1; }
    if [ "$TF_MEMORY_STATE" != ok ]; then
        tf_memory_report >&2
        echo 'TrackFerry: insufficient memory budget or unsupported device; not starting' >&2
        return 1
    fi
}
