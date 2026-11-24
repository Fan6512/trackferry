#!/usr/bin/env bash
# Every place that names the version must agree, and the changelog must cover it.
#
#   scripts/check-release.sh            consistency only (every commit)
#   scripts/check-release.sh v1.2.3     also: this tag may be released now
set -euo pipefail
cd "$(dirname "$0")/.."

bad=0
fail() { echo "  ✗ $*" >&2; bad=1; }
v="$(tr -d '[:space:]' < VERSION)"
echo "VERSION $v"

[[ "$v" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || fail "VERSION 必须是 x.y.z，不带 v，不带后缀"
field() { node -p "const p=JSON.parse(require('fs').readFileSync('$1','utf8')); $2"; }
[ "$(field web/package.json p.version)" = "$v" ] || fail "web/package.json 的版本不是 $v"
[ "$(field web/package-lock.json p.version)" = "$v" ] || fail "web/package-lock.json 的版本不是 $v"
[ "$(field web/package-lock.json "p.packages[''].version")" = "$v" ] || fail "web/package-lock.json 根包的版本不是 $v"
images="$(sed -n 's/^ *image: *madrays\/trackferry://p' compose.yaml compose.bridge.yaml | sort -u)"
[ "$images" = latest ] || fail "compose.yaml 和 compose.bridge.yaml 的镜像标签必须全部是 latest（现在：${images//$'\n'/ }）"
grep -Eq "^## \[${v//./\\.}\] - [0-9]{4}-[0-9]{2}-[0-9]{2}$" CHANGELOG.md || fail "CHANGELOG.md 缺少一行：## [$v] - YYYY-MM-DD"
[ -n "$(scripts/release-notes.sh "$v" 2>/dev/null)" ] || fail "CHANGELOG.md 的 $v 没有内容"

if [ $# -gt 0 ]; then
  tag="$1"
  [ "$tag" = "v$v" ] || fail "标签 $tag 与 VERSION 不符，应为 v$v"
  [ -z "$(git status --porcelain)" ] || fail "工作区有未提交的改动"
  git merge-base --is-ancestor HEAD "${TRACKFERRY_MAIN:-main}" 2>/dev/null || fail "发版的提交必须已经在 main 上"
  last="$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' | { grep -vx "$tag" || true; } | sed 's/^v//' | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)"
  if [ -n "$last" ]; then
    top="$(printf '%s\n%s\n' "$last" "$v" | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)"
    [ "$top" = "$v" ] && [ "$last" != "$v" ] || fail "$v 必须大于已发布的 $last"
  fi
fi
[ $bad = 0 ] && echo "  ✓ 一致" || exit 1
