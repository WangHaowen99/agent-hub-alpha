#!/bin/bash
# 从本机 jiuwenswarm 克隆的 origin/develop 导出源码并构建镜像。只读克隆（fetch + archive），不改工作区。
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="${SWARM_REPO:?set SWARM_REPO to a jiuwenswarm clone}"
REF="${SWARM_REF:-origin/develop}"
git -C "$REPO" fetch -q origin develop
COMMIT="$(git -C "$REPO" rev-parse --short=9 "$REF")"
SRC="$HERE/build/src-$COMMIT"
if [ ! -d "$SRC" ]; then
  mkdir -p "$SRC.tmp" && git -C "$REPO" archive "$COMMIT" | tar -x -C "$SRC.tmp" && mv "$SRC.tmp" "$SRC"
fi
docker build -f "$HERE/Dockerfile" --build-context swarm-src="$SRC" \
  --label aha.swarm-commit="$COMMIT" -t aha-swarm/workswarm:"$COMMIT" "$HERE"
echo "SWARM_COMMIT=$COMMIT" > "$HERE/.env"
echo "built aha-swarm/workswarm:$COMMIT"
