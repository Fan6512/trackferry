#!/usr/bin/env bash
# Prepares a release: writes the version everywhere, dates the changelog and runs every check.
# It never commits, tags or pushes; the commands for that are printed at the end.
#
#   scripts/release.sh 1.2.3
set -euo pipefail
cd "$(dirname "$0")/.."
v="${1:?用法：scripts/release.sh x.y.z}"
[[ "$v" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || { echo "版本号必须是 x.y.z" >&2; exit 2; }
if git rev-parse -q --verify "refs/tags/v$v" >/dev/null; then echo "v$v 已经发布过，版本号不能重用" >&2; exit 2; fi

grep -Eq "^## \[${v//./\\.}\] " CHANGELOG.md || {
  [ -n "$(scripts/release-notes.sh 未发布)" ] || { echo "CHANGELOG.md 的“未发布”是空的，先写清楚这一版改了什么" >&2; exit 2; }
  awk -v v="$v" -v d="$(date +%Y-%m-%d)" '{ print } /^## \[未发布\]$/ { print ""; print "## [" v "] - " d }' CHANGELOG.md > CHANGELOG.md.new
  mv CHANGELOG.md.new CHANGELOG.md
}
printf '%s\n' "$v" > VERSION
(cd web && npm version "$v" --no-git-tag-version --allow-same-version >/dev/null)

scripts/check-release.sh
(cd web && npm ci --no-audit --no-fund && npm test && npm run build)
go vet ./...
go test -race ./...

msg="$(git rev-parse --git-path RELEASE_MSG)"
cat > "$msg" <<MSG
发布：$v

版本号和更新记录统一为 $v。
这一版的改动见 CHANGELOG.md 的对应小节。
MSG

cat <<MSG

$v 准备好了。看过 git diff 后：

  git add -A && git commit -F $msg
  git push origin main
  scripts/publish.sh            测试、构建并推送镜像、打标签、创建 Release
MSG
