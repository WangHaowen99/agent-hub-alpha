#!/bin/bash
# 交叉编译 Go 桩服务并组装镜像上下文（pilot/）：静态二进制 + adopt 用到的 schema。
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
GO_DIR="$(cd "$HERE/../../.." && pwd)"   # agent-festival/go
docker run --rm -v "$GO_DIR/..":/src -w /src/go/cmd/pilot-mcp \
  -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 -e GOPROXY="${GOPROXY:-https://goproxy.cn,direct}" \
  golang:1.25 go build -trimpath -ldflags="-s -w" -o deploy/pilot/pilot-mcp .
mkdir -p "$HERE/pilot/schema" && cp "$GO_DIR"/../schema/*.schema.json "$HERE/pilot/schema/"
docker build -q -t aha-swarm/pilot-mcp:dev "$HERE/pilot"
