#!/bin/bash
# 用法：./kc-token.sh <alice|bob>  — 模拟 Manager 登录，向 Keycloak 取该用户的访问令牌，输出到 stdout。
set -euo pipefail
cd "$(dirname "$0")"
S=${AHA_SECRETS_DIR:-$HOME/.config/aha-swarm}
set -a; . "$S/kc.env"; set +a
docker compose -f compose.yaml -f compose.enterprise.yaml exec -T -e U="$1" -e P="$AHA_USER_PASSWORD" swarm python -c '
import json, os, urllib.parse, urllib.request
body = urllib.parse.urlencode({"grant_type": "password", "client_id": "workswarm-web",
    "username": os.environ["U"], "password": os.environ["P"], "scope": "openid"}).encode()
req = urllib.request.Request("http://keycloak:8080/realms/aha/protocol/openid-connect/token", body)
print(json.load(urllib.request.urlopen(req, timeout=10))["access_token"])'
