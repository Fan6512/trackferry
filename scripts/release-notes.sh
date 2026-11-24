#!/usr/bin/env bash
# Prints the changelog section of a version: the text of its GitHub Release.
set -euo pipefail
cd "$(dirname "$0")/.."
awk -v head="## [${1:?version}]" '
  index($0, head) == 1 { on = 1; next }
  on && /^## \[/ { exit }
  on && /^\[[^]]+\]: / { exit }
  on { lines[++n] = $0 }
  END {
    for (a = 1; a <= n && lines[a] == ""; a++);
    for (b = n; b >= a && lines[b] == ""; b--);
    for (i = a; i <= b; i++) print lines[i]
  }' CHANGELOG.md
