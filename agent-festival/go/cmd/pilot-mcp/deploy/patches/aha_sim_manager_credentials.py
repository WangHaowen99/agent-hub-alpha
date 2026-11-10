"""验证部署用：代替 Manager 下发 MCP 动态 token 的实例凭据。

企业版里 Manager 把 client_id / client_secret 挂在 Agent 模板 data.custom_config，
企业配置合并时经 apply_dispatched_credentials 交给 mcp_dynamic_token_patch。
这里不接 Manager（配置源为 YAML），在该模块首次导入后用 AHA_SIM_CLIENT_ID /
AHA_SIM_CLIENT_SECRET 调同一个入口，其余链路不变。

另补两处上游缺口，都是双令牌在 Web 路径上走不通的原因：
- AHA_SIM_KEEP_TOKEN_URL：Web 专家只能挂 mcp/state.json 里已连接的连接器，而
  state_store.record_to_mcp_entry 转条目时丢掉 dynamic_token_url。
- AHA_SIM_MARK_ON_PROBE：probe_mcp_live_connection（启动预热、MCP App 取连接都走它）
  建连前不做动态 token 标记；连接进进程级缓存后，之后注册时虽然标记了，也直接复用
  这条只带静态头的连接。这里让探活在建连前先标记（候选修法）。
"""
import importlib.abc
import importlib.util
import os
import sys
import types

class _AfterImport(importlib.abc.MetaPathFinder):
    """首次导入 ``target`` 后对模块执行 ``hook``。"""

    def __init__(self, target, hook):
        self.target, self.hook = target, hook

    def find_spec(self, name, path, target=None):
        if name != self.target:
            return None
        sys.meta_path.remove(self)
        spec = importlib.util.find_spec(name)
        if spec is None or spec.loader is None:
            return spec
        original = spec.loader.exec_module

        def exec_module(module):
            original(module)
            self.hook(module)

        spec.loader.exec_module = exec_module
        return spec


def _debug(module):
    """AHA_SIM_DEBUG：记录标记了哪些网关、每个请求是否带到本轮用户令牌（只记有无，不记值）。"""
    mark = module.mark_dynamic_token_server
    user = module._current_user_authorization

    def mark_dynamic_token_server(server_id, token_url=None, *a, **kw):
        ok = mark(server_id, token_url, *a, **kw)
        print(f"[aha-sim] mark server_id={server_id!r} token_url={bool(token_url)} -> {ok}", file=sys.stderr, flush=True)
        return ok

    def _current_user_authorization():
        v = user()
        print(f"[aha-sim] user token present={bool(v)}", file=sys.stderr, flush=True)
        return v

    module.mark_dynamic_token_server = mark_dynamic_token_server
    module._current_user_authorization = _current_user_authorization

    strategy = module.DynamicTokenAuthStrategy
    authenticate = strategy.authenticate

    async def traced(self, auth_config, **kw):
        tool_id = str(getattr(auth_config, "tool_id", "") or "").strip()
        print(f"[aha-sim] authenticate tool_id={tool_id!r} creds={module.read_instance_credentials() is not None} "
              f"token_url={bool(module._registry_token_url(tool_id)) if tool_id else False}", file=sys.stderr, flush=True)
        return await authenticate(self, auth_config, **kw)

    strategy.authenticate = traced


def _dispatch_credentials(module):
    if os.getenv("AHA_SIM_DEBUG"):
        _debug(module)
    module.apply_dispatched_credentials(types.SimpleNamespace(
        agent_template={"data": {"custom_config": [{
            "client_id": os.environ["AHA_SIM_CLIENT_ID"],
            "client_secret": os.environ["AHA_SIM_CLIENT_SECRET"],
        }]}},
        resource_id="aha-sim", ref_template_id="aha-sim"))


def _keep_token_url(module):
    original = module.record_to_mcp_entry

    def record_to_mcp_entry(name, record):
        entry = original(name, record)
        if entry is not None and record.get("dynamic_token_url"):
            entry["dynamic_token_url"] = record["dynamic_token_url"]
        return entry

    module.record_to_mcp_entry = record_to_mcp_entry


def _mark_on_probe(module):
    original = module.probe_mcp_live_connection

    async def probe_mcp_live_connection(name):
        entry = module.get_mcp_server_config(str(name or "").strip()) or {}
        token_url = str(entry.get("dynamic_token_url") or "").strip()
        if token_url:
            cfg = module.build_mcp_server_config(entry, server_id_scope="jiuwenswarm", resolve_node_launcher=True)
            if cfg is not None:
                patch = importlib.import_module("jiuwenswarm.common.mcp_dynamic_token_patch")
                patch.apply_mcp_dynamic_token_patch()
                patch.mark_dynamic_token_server(cfg.server_id, token_url)
        return await original(name)

    module.probe_mcp_live_connection = probe_mcp_live_connection


if os.getenv("AHA_SIM_CLIENT_ID") and os.getenv("AHA_SIM_CLIENT_SECRET"):
    sys.meta_path.insert(0, _AfterImport("jiuwenswarm.common.mcp_dynamic_token_patch", _dispatch_credentials))
if os.getenv("AHA_SIM_KEEP_TOKEN_URL"):
    sys.meta_path.insert(0, _AfterImport("jiuwenswarm.server.runtime.mcp.state_store", _keep_token_url))
if os.getenv("AHA_SIM_MARK_ON_PROBE"):
    sys.meta_path.insert(0, _AfterImport("jiuwenswarm.common.mcp_config", _mark_on_probe))
