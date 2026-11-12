"""编排台：WorkSwarm 应用插件。

页面在 WorkSwarm 里打开，经 WorkSwarm 自己的 WebSocket 调这里注册的 RPC；
这里再从服务端转给编排服务（Go）。浏览器不直接访问编排服务，编排服务也不用
对外开放。确认时把这条连接上的身份一并交给编排服务留档：declared 是客户端
自报的用户名，authenticated 是 Gateway 握手时认证过的身份（开源版为空）。
"""

from __future__ import annotations

import asyncio
import json
import os
import re
import urllib.error
import urllib.parse
import urllib.request
from typing import Any

from jiuwenswarm.extensions.sdk import ApplicationPluginExtension, FrontendContribution

PLUGIN_ID = "credit-workbench"
_PLAN = re.compile(r"^P\d{1,6}$")
_RUN = re.compile(r"^R\d{1,6}$")
_NO_PROXY = urllib.request.build_opener(urllib.request.ProxyHandler({}))


class OrchestrationError(Exception):
    def __init__(self, message: str, code: str = "UPSTREAM") -> None:
        super().__init__(message)
        self.code = code


def _call(base: str, method: str, path: str, body: dict | None = None) -> Any:
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(base + path, data=data, method=method, headers={"Content-Type": "application/json"})
    try:
        with _NO_PROXY.open(req, timeout=15) as resp:
            return json.loads(resp.read() or b"null")
    except urllib.error.HTTPError as exc:
        try:
            msg = json.loads(exc.read()).get("error") or f"编排服务返回 {exc.code}"
        except Exception:  # noqa: BLE001 - keep the upstream status when the body is not JSON
            msg = f"编排服务返回 {exc.code}"
        raise OrchestrationError(msg, "REJECTED" if exc.code in (400, 401, 403, 409) else "UPSTREAM") from exc
    except (urllib.error.URLError, TimeoutError) as exc:
        raise OrchestrationError("编排服务暂时连不上") from exc


class CreditWorkbench(ApplicationPluginExtension):
    plugin_id = PLUGIN_ID

    def __init__(self) -> None:
        self._base = os.getenv("CREDIT_INTERNAL_URL", "http://credit:8091").rstrip("/")

    async def initialize(self, config: Any) -> None:
        del config

    async def shutdown(self) -> None:
        return None

    def frontend_contributions(self) -> tuple[FrontendContribution, ...]:
        return (
            FrontendContribution(
                id="credit-workbench",
                nav_key=f"app:{PLUGIN_ID}",
                title="编排台",
                render_mode="iframe",
                entrypoint="index.html",
                position=60,
            ),
        )

    def bind_web_channel(self, channel: Any, services: Any) -> None:
        del services

        async def upstream(ws, req_id, fn):  # noqa: ANN001
            try:
                payload = await asyncio.to_thread(fn)
            except OrchestrationError as exc:
                await channel.send_response(ws, req_id, ok=False, error=str(exc), code=exc.code)
                return
            await channel.send_response(ws, req_id, ok=True, payload=payload)

        async def plans(ws, req_id, params, session_id):  # noqa: ANN001
            del params, session_id
            await upstream(ws, req_id, lambda: _call(self._base, "GET", "/api/plans"))

        async def graph(ws, req_id, params, session_id):  # noqa: ANN001
            del session_id
            params = params if isinstance(params, dict) else {}
            if _RUN.match(str(params.get("run", ""))):
                query = {"run": params["run"]}
            elif _PLAN.match(str(params.get("plan", ""))) and str(params.get("v", "")).isdigit():
                query = {"plan": params["plan"], "v": params["v"]}
            else:
                await channel.send_response(ws, req_id, ok=False, error="需要 plan 和 v，或 run", code="BAD_REQUEST")
                return
            if params.get("verify"):
                query["verify"] = "1"
            path = "/api/graph?" + urllib.parse.urlencode(query)
            await upstream(ws, req_id, lambda: _call(self._base, "GET", path))

        async def approve(ws, req_id, params, session_id):  # noqa: ANN001
            del session_id
            params = params if isinstance(params, dict) else {}
            plan, version, digest, password = (params.get(k) for k in ("plan", "version", "hash", "password"))
            if not (_PLAN.match(str(plan)) and isinstance(version, int) and isinstance(digest, str) and isinstance(password, str)):
                await channel.send_response(ws, req_id, ok=False, error="需要 plan、version、hash 和密码", code="BAD_REQUEST")
                return
            declared = getattr(channel, "connection_user_id", lambda _ws: None)(ws)
            body = {
                "plan": plan, "version": version, "hash": digest, "user": "alice", "password": password,
                "via": {"channel": "workswarm-plugin", "declared_user": declared or "",
                        "authenticated_user": getattr(ws, "authenticated_user_id", None) or ""},
            }
            await upstream(ws, req_id, lambda: _call(self._base, "POST", "/api/approve", body))

        # local_only: handled by the Gateway itself, not forwarded to the agent server.
        channel.register_method("plugin.credit_workbench.plans", plans, local_only=True)
        channel.register_method("plugin.credit_workbench.graph", graph, local_only=True)
        channel.register_method("plugin.credit_workbench.approve", approve, local_only=True)


async def register_extensions(registry: Any) -> list[CreditWorkbench]:
    plugin = CreditWorkbench()
    registry.register_application_plugin(plugin)
    return [plugin]
