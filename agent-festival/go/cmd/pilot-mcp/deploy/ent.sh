#!/bin/bash
# 用法：./ent.sh <docker compose 子命令…> — 带上测试凭据环境，对企业版模拟栈执行 docker compose。
set -euo pipefail
cd "$(dirname "$0")"
set -a; . "${AHA_SECRETS_DIR:-$HOME/.config/aha-swarm}/kc.env"; set +a
exec docker compose -f compose.yaml -f compose.enterprise.yaml "$@"
