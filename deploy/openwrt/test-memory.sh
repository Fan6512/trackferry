#!/bin/sh
set -eu
cd "$(dirname "$0")"
. ./memory.sh
fixture=$(mktemp)
trap 'rm -f "$fixture"' EXIT
case_plan() {
    printf 'MemTotal: %s kB\nMemAvailable: %s kB\n' "$1" "$2" > "$fixture"
    actual=$(tf_memory_plan "$fixture")
    [ "$actual" = "$3" ] || { printf 'expected: %s\nactual: %s\n' "$3" "$actual" >&2; exit 1; }
}
case_plan 131072 131072 '131072 131072 64 0 0 0 unsupported'
case_plan 131073 114688 '131073 114688 64 24 36 48 ok'
case_plan 262144 114687 '262144 114687 64 24 36 48 insufficient'
case_plan 262144 114688 '262144 114688 64 24 36 48 ok'
case_plan 262145 131072 '262145 131072 64 32 48 64 ok'
case_plan 495484 153600 '495484 153600 73 32 48 64 ok'
case_plan 495484 140288 '495484 140288 73 32 48 64 ok'
case_plan 495484 140287 '495484 140287 73 32 48 64 insufficient'
case_plan 524288 144384 '524288 144384 77 32 48 64 ok'
case_plan 524289 177151 '524289 177151 77 48 72 96 insufficient'
case_plan 524289 177152 '524289 177152 77 48 72 96 ok'
case_plan 1048576 256000 '1048576 256000 154 48 72 96 ok'
case_plan 1048577 288768 '1048577 288768 154 64 96 128 ok'
case_plan 2097152 0 '2097152 0 308 64 96 128 insufficient'
tf_memory_load "$fixture"
[ "$TF_MAX_MIB" = 128 ] && [ "$TF_MEMORY_STATE" = insufficient ]
for invalid in '' 'MemTotal: 500000 kB' 'MemAvailable: 100000 kB' 'MemTotal: 0 kB
MemAvailable: 0 kB' 'MemTotal: 500000 kB
MemAvailable: 500001 kB' 'MemTotal: 500000 kB
MemAvailable: -1 kB' 'MemTotal: 500000 MB
MemAvailable: 150000 kB' 'MemTotal: 500000 kB
MemAvailable: 150000 kB
MemAvailable: 160000 kB'; do
    printf '%s\n' "$invalid" > "$fixture"
    if tf_memory_plan "$fixture"; then echo 'Accepted invalid meminfo' >&2; exit 1; fi
done
echo 'Memory profile and admission-boundary tests passed (14 valid, 8 invalid fixtures).'
