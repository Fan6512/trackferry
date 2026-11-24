#!/usr/bin/env bash
# Turns on the checks in scripts/hooks for this clone.
set -euo pipefail
cd "$(dirname "$0")/.."
git config core.hooksPath scripts/hooks
git config commit.template scripts/hooks/message.txt
echo "已启用：提交前、提交说明、推送前三道检查"
