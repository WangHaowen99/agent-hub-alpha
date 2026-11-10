#!/bin/bash
# 以企业版双令牌模式启动：首次运行生成测试凭据（只存 ~/.config/aha-swarm，不进仓库），
# 渲染 Keycloak realm，再叠加 compose.enterprise.yaml 启动。
set -euo pipefail
cd "$(dirname "$0")"
S=${AHA_SECRETS_DIR:-$HOME/.config/aha-swarm}
mkdir -p "$S/kc-import" && chmod 700 "$S"
if [ ! -f "$S/kc.env" ]; then
  umask 077
  {
    echo "AHA_KC_ADMIN_PASSWORD=$(openssl rand -hex 16)"
    echo "AHA_AGENT_CLIENT_SECRET=$(openssl rand -hex 24)"
    echo "AHA_USER_PASSWORD=$(openssl rand -hex 12)"
  } > "$S/kc.env"
fi
grep -q '^AHA_DB_PASSWORD=' "$S/kc.env" || (umask 077; echo "AHA_DB_PASSWORD=$(openssl rand -hex 16)" >> "$S/kc.env")
set -a; . "$S/kc.env"; set +a
python3 -I -c 'import os,string,sys; sys.stdout.write(string.Template(open(sys.argv[1]).read()).substitute(os.environ))' \
  keycloak/aha-realm.json.tmpl > "$S/kc-import/aha-realm.json"
chmod 600 "$S/kc-import/aha-realm.json"
docker compose -f compose.yaml -f compose.enterprise.yaml up -d "$@"
