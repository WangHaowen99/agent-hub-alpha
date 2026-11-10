"""每次启动幂等执行：首次 init；模型配置写进工作区 config/.env；
注册授信试用 MCP 连接器，并建一个绑定该连接器的专家（选中专家即挂载工具）。"""
from __future__ import annotations

import json
import logging
import os
import shutil
import subprocess
from pathlib import Path

HOME = Path(os.environ.get("JIUWENSWARM_DATA_DIR") or Path.home() / ".jiuwenswarm")
CONFIG = HOME / "config" / "config.yaml"
DOTENV = HOME / "config" / ".env"


def ensure_init() -> None:
    if not CONFIG.exists():
        subprocess.run([shutil.which("jiuwenswarm-init") or "jiuwenswarm-init"], check=True)


def set_dotenv(values: dict[str, str]) -> None:
    lines = DOTENV.read_text(encoding="utf-8").splitlines() if DOTENV.exists() else []
    seen, out = set(), []
    for line in lines:
        key = line.split("=", 1)[0].strip() if "=" in line and not line.lstrip().startswith("#") else None
        if key in values:
            out.append(f'{key}="{values[key]}"')
            seen.add(key)
        else:
            out.append(line)
    out.extend(f'{k}="{v}"' for k, v in values.items() if k not in seen)
    DOTENV.write_text("\n".join(out) + "\n", encoding="utf-8")
    DOTENV.chmod(0o600)


MCP_NAME = "credit-pilot"
EXPERT_ID = "credit-pilot"
EXPERT_PERSONA = """你是授信试用助手，通过 credit-pilot 服务办理授信申请。

1. 用户要申请授信时，调用 pilot_submit_application 提交；用户要看界面时调用 pilot_open_console。
2. 工具返回的是命令回执，不是业务结果。业务方是否批准，只能以 pilot_get_task 读到的 provider_status 为准，不要自己推断或宣称已批准、已放款。
3. control_state 为 unresolved 或 provider_status 为空时，如实说明结果未知，不要重新提交新的申请。
4. 取消只能用 pilot_request_cancel 申请，以业务方确认为准。"""


def register_connector() -> None:
    from jiuwenswarm.common.utils import get_workspace_dir

    path = Path(get_workspace_dir()) / "mcp" / "state.json"
    path.parent.mkdir(parents=True, exist_ok=True)
    try:
        state = json.loads(path.read_text(encoding="utf-8-sig")) if path.exists() else {}
    except ValueError:
        state = {}
    state.setdefault("version", 1)
    state.setdefault("mounts", {})
    connectors = state.setdefault("mcp", {})
    previous = connectors.get(MCP_NAME) or {}
    headers = {}
    static_user = os.environ.get("PILOT_STATIC_USER", "")
    if static_user:
        from jiuwenswarm.server.runtime.mcp.credential import CredentialStore

        CredentialStore().save_token(MCP_NAME, "PILOT_STATIC_USER", static_user)
        headers["X-Pilot-User"] = "${PILOT_STATIC_USER}"
    connectors[MCP_NAME] = {
        "transport": "streamable-http",
        "url": os.environ["PILOT_MCP_URL"],
        "headers": headers,
        "timeout_s": 60,
        "integration_type": "remote-mcp",
        "state": "connected",
        "enabled": bool(previous.get("enabled", True)),
        "server_id_scope": f"mcp:{MCP_NAME}",
        "description": "授信试用业务服务：提交申请、查询权威状态、申请取消",
    }
    token_url = os.environ.get("PILOT_DYNAMIC_TOKEN_URL", "")
    if token_url:
        # 双令牌网关的 dynamic_token_url 只在 config.yaml mcp.servers 条目里保留
        # （state.json 记录转条目时会丢掉它，且同名时 state.json 优先），
        # 企业版里这一条由 Manager 下发，这里直接写入。
        from jiuwenswarm.common.config import upsert_mcp_server_in_config

        entry = {"name": MCP_NAME, **connectors.pop(MCP_NAME)}
        for k in ("integration_type", "state", "description"):
            entry.pop(k, None)
        entry["enabled"] = True
        entry["headers"] = {"Authorization": "Bearer template-static"}
        entry["dynamic_token_url"] = token_url
        upsert_mcp_server_in_config(entry)
    else:
        from jiuwenswarm.common.config import remove_mcp_server_in_config

        try:
            remove_mcp_server_in_config(MCP_NAME)
        except KeyError:
            pass
    tmp = path.with_suffix(".tmp")
    tmp.write_text(json.dumps(state, ensure_ascii=False, indent=2, sort_keys=True), encoding="utf-8")
    os.replace(tmp, path)


def ensure_expert() -> None:
    from jiuwenswarm.server.runtime import extension_package_manager as epm

    params = {
        "id": EXPERT_ID,
        "name": "授信试用",
        "description": "通过 credit-pilot 服务提交授信申请并查询业务方结果。",
        "persona": EXPERT_PERSONA,
        "skills": [],
        "mcps": [MCP_NAME],
        "quickInputs": ["打开授信控制台", "客户 C001 资产 3500 万，近两年无逾期，帮我申请授信"],
        "tags": [{"zh": "授信", "en": "Credit"}],
    }
    if (epm._local_root("agent_templates") / EXPERT_ID).is_dir():  # noqa: SLF001 - no public probe
        epm.update_agent_template(params)
    else:
        epm.create_agent_template(params)
    ok, payload = epm.install_equipment_gated("agent_templates", {"id": EXPERT_ID})
    if not ok:
        logging.warning("expert %s not installed: %s", EXPERT_ID, payload)


def ensure_tenant_experts(users: list[str]) -> None:
    """企业版按 (group, bot, user) 租户隔离专家；正常由 Manager 下发，这里给每个测试用户装一份。

    专家卡片和安装闸门只认租户 mcp/state.json 里 state=connected 的连接器，所以在每个租户
    写一条带 dynamic_token_url 的已连接记录（转条目时保留该字段靠 AHA_SIM_KEEP_TOKEN_URL 补丁）。
    """
    from jiuwenswarm.common.utils import get_workspace_dir
    from jiuwenswarm.enterprise.tenant.tenant_context import TenantContext
    from jiuwenswarm.enterprise.tenant.tenant_paths import build_tenant_key

    for user in users:
        with TenantContext.scoped(build_tenant_key("default", "default", user)):
            path = Path(get_workspace_dir()) / "mcp" / "state.json"
            path.parent.mkdir(parents=True, exist_ok=True)
            state = json.loads(path.read_text(encoding="utf-8-sig")) if path.exists() else {}
            state.setdefault("version", 1)
            state.setdefault("mounts", {})
            # 企业模板带静态 Authorization 兜底：有本轮用户令牌时被它覆盖，没有时原样发出。
            # 双令牌补丁替换的是"请求头鉴权"策略，连接器没有任何静态头时根本不走这条路。
            state.setdefault("mcp", {})[MCP_NAME] = {
                "transport": "streamable-http", "url": os.environ["PILOT_MCP_URL"],
                "headers": {"Authorization": "Bearer template-static"},
                "integration_type": "remote-mcp", "state": "connected", "enabled": True,
                "server_id_scope": f"mcp:{MCP_NAME}", "timeout_s": 60,
                "dynamic_token_url": os.environ.get("PILOT_DYNAMIC_TOKEN_URL", "")}
            path.write_text(json.dumps(state, ensure_ascii=False, indent=2), encoding="utf-8")
            ensure_expert()
            logging.info("tenant %s: expert ready", user)


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="[aha-configure] %(message)s")
    ensure_init()
    set_dotenv({
        "API_BASE": os.environ["SWARM_API_BASE"],
        "API_KEY": os.environ["SWARM_API_KEY"],
        "MODEL_NAME": os.environ.get("SWARM_MODEL_NAME", "qwen-plus"),
        "MODEL_PROVIDER": "OpenAI",
        "ENDPOINT_PROFILE": os.environ.get("SWARM_ENDPOINT_PROFILE", "dashscope"),
    })
    if os.environ.get("PILOT_MCP_URL"):
        register_connector()
        ensure_expert()
        users = [u for u in os.environ.get("AHA_TENANT_USERS", "").split(",") if u.strip()]
        if users:
            ensure_tenant_experts(users)
    logging.info("workspace ready")


if __name__ == "__main__":
    main()
