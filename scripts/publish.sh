#!/usr/bin/env bash
# Publishes the version in VERSION from this machine: every check, both images, the tag and the Release.
# Safe to run again after a failure: what is already published is left as it is.
#
#   scripts/publish.sh --build   checks and builds only, publishes nothing
#   scripts/publish.sh
set -euo pipefail
cd "$(dirname "$0")/.."
v="$(tr -d '[:space:]' < VERSION)"; tag="v$v"; image=madrays/trackferry; repo=madrays/trackferry
step() { printf '\n▸ %s\n' "$*"; }

step "核对 $tag"
[ -z "$(git status --porcelain)" ] || { echo "工作区有未提交的改动" >&2; exit 1; }
[ "$(git rev-parse --abbrev-ref HEAD)" = main ] || { echo "请在 main 上发版" >&2; exit 1; }
scripts/check-release.sh "$tag"
for f in compose.yaml compose.bridge.yaml; do docker compose -f "$f" config -q; done
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null && [ "$(git rev-parse "$tag^{commit}")" != "$(git rev-parse HEAD)" ]; then
  echo "$tag 已经指向别的提交，版本号不能重用" >&2; exit 1
fi

step 测试
(cd web && npm ci --no-audit --no-fund && npm test && npm run build)
[ -z "$(gofmt -l cmd internal)" ] || { echo "有文件没有 gofmt" >&2; exit 1; }
go vet ./...
go test -race ./...

step 构建
commit="$(git rev-parse HEAD)"
mkdir -p build/run dist
for ca in /etc/ssl/certs/ca-certificates.crt /etc/ssl/cert.pem; do [ ! -f "$ca" ] || { cp "$ca" dist/ca-certificates.crt; break; }; done
for arch in amd64 arm64; do
  CGO_ENABLED=0 GOOS=linux GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=$v -X main.commit=${commit:0:7} -X main.repo=$repo" \
    -o dist/$arch/trackferry ./cmd/trackferry
done
bx() {
  docker buildx build --platform linux/amd64,linux/arm64 -f build/Dockerfile.release --provenance=false \
    --label org.opencontainers.image.source="https://github.com/$repo" \
    --label org.opencontainers.image.version="$v" \
    --label org.opencontainers.image.revision="$commit" \
    --label org.opencontainers.image.licenses=MIT "$@" .
}
if [ "${1:-}" = --build ]; then bx -t "$image:$v"; exit; fi

step 镜像
git fetch -q origin main
git merge-base --is-ancestor HEAD origin/main || { echo "先推送 main：git push origin main" >&2; exit 1; }
if out="$(docker buildx imagetools inspect "$image:$v" --format '{{json .Image}}' 2>&1)"; then
  grep -q "$commit" <<< "$out" || { echo "$image:$v 已由别的提交发布，版本号不能重用" >&2; exit 1; }
  echo "$image:$v 已在 Docker Hub"
else
  grep -qiE "not found|does not exist|insufficient_scope|denied" <<< "$out" || { echo "$out" >&2; exit 1; }
  bx -t "$image:$v" -t "$image:${v%.*}" -t "$image:latest" --push
fi

step "标签与 Release"
git rev-parse -q --verify "refs/tags/$tag" >/dev/null || scripts/release-notes.sh "$v" | git tag -a "$tag" --cleanup=whitespace -F -
git push origin "$tag"
if gh release view "$tag" -R "$repo" >/dev/null 2>&1; then
  echo "Release 已存在"
else
  scripts/release-notes.sh "$v" | gh release create "$tag" -R "$repo" --verify-tag --title "TrackFerry $v" --notes-file - compose.yaml
fi
