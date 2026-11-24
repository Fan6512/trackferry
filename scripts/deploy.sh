#!/usr/bin/env bash
# For development: build this tree on a NAS over SSH and restart the container there.
#
#   TRACKFERRY_SSH_HOST=<ssh alias> TRACKFERRY_DEPLOY_DIR=<dir on the NAS> scripts/deploy.sh
#
# The NAS keeps its own compose.yaml (image trackferry:local), .env and data/: none of them is
# ever written. The running image and a copy of the database are kept for going back.
set -euo pipefail
host="${TRACKFERRY_SSH_HOST:?set TRACKFERRY_SSH_HOST}"
dir="${TRACKFERRY_DEPLOY_DIR:?set TRACKFERRY_DEPLOY_DIR}"
cd "$(dirname "$0")/.."

commit="$(git rev-parse --short HEAD)"
[ -z "$(git status --porcelain -- go.mod go.sum cmd internal web Dockerfile VERSION)" ] || commit="$commit-dirty"
version="$(tr -d '[:space:]' < VERSION)"
echo "$version ($commit) → $host:$dir"

# Everything happens in one connection: a NAS may prune images nothing is using yet.
COPYFILE_DISABLE=1 tar --no-xattrs --exclude=web/node_modules --exclude=web/dist \
  --exclude=internal/webui/dist/assets --exclude=internal/webui/dist/index.html \
  -czf - VERSION go.mod go.sum cmd internal web Dockerfile .dockerignore |
  ssh -o BatchMode=yes "$host" "set -euo pipefail
    cd '$dir'
    test -f compose.yaml || { echo 'no compose.yaml in $dir' >&2; exit 2; }
    grep -q 'trackferry:local' compose.yaml || { echo 'compose.yaml in $dir does not use trackferry:local' >&2; exit 2; }
    rm -rf cmd internal web
    tar -xzf -
    ! docker image inspect trackferry:local >/dev/null 2>&1 || docker save trackferry:local | gzip -1 > rollback.tar.gz
    DOCKER_BUILDKIT=0 docker build -q --build-arg COMMIT='$commit' --build-arg VERSION='$version' -t trackferry:local .
    docker compose stop trackferry
    test ! -f data/trackferry.db || cp -p data/trackferry.db data/trackferry.db.before
    docker compose up -d --no-build --wait --wait-timeout 120
    echo '$version $commit' > DEPLOYED
    docker compose ps"
