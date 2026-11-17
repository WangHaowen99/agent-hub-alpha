"""W02 Application Plugin adapted to W04's single Go/PG authority.

Source: WangHaowen99/PR30 41a3675 workbench-plugin/extension.py.
Keep local Gateway RPC + iframe contributions; replace credit DTO/credentials
with scoped read identity and Go-issued human session/CSRF. No agent dispatch.
"""
from __future__ import annotations

import asyncio
import base64
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import re
import urllib.error
import urllib.request
from typing import Any

from jiuwenswarm.extensions.sdk import ApplicationPluginExtension, FrontendContribution

PLUGIN_ID = "credit-workbench"  # same W02 application; annual-report adapter
TASK = re.compile(r"T-[0-9a-f]{32}")
ARTIFACT = re.compile(r"[A-Za-z0-9_-]{1,100}")


class OrchestrationError(Exception):
    def __init__(self, message: str, code: str = "UPSTREAM") -> None:
        super().__init__(message)
        self.code = code


class CreditWorkbench(ApplicationPluginExtension):
    plugin_id = PLUGIN_ID

    async def initialize(self, config: Any) -> None:
        del config
        # Private server config only. No credentials or identity come from model
        # arguments or the WebSocket's self-declared user ID.
        self.config = json.loads(Path(os.environ["REPORT_DEMO_CONFIG"]).read_text(encoding="utf-8-sig"))
        self.base = self.config["public_url"].rstrip("/")

    async def shutdown(self) -> None:
        pass

    def frontend_contributions(self) -> tuple[FrontendContribution, ...]:
        return (FrontendContribution(id=PLUGIN_ID, nav_key=f"app:{PLUGIN_ID}", title="编排台",
                                     render_mode="iframe", entrypoint="index.html", position=60),)

    def call(self, method: str, path: str, body: dict | None = None, *, password: str | None = None) -> Any:
        cookies = http.cookiejar.CookieJar()
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(cookies))

        def request(verb: str, target: str, value: dict | None, headers: dict) -> Any:
            data = json.dumps(value, ensure_ascii=False).encode("utf-8") if value is not None else None
            req = urllib.request.Request(self.base + target, method=verb, data=data,
                                         headers={"Content-Type": "application/json", **headers})
            try:
                with opener.open(req, timeout=15) as response:
                    raw=response.read()
                    if target.endswith('/download'):
                        return {'content_base64':base64.b64encode(raw).decode('ascii'),'digest':'sha256:'+hashlib.sha256(raw).hexdigest()}
                    return json.loads(raw or b"null")
            except urllib.error.HTTPError as exc:
                try:
                    error = json.loads(exc.read()).get("error", {})
                    code, message = error.get("code", "UPSTREAM"), error.get("message", "编排服务拒绝请求")
                except (ValueError, AttributeError):
                    code, message = "UPSTREAM", "编排服务拒绝请求"
                raise OrchestrationError(message, code) from None
            except (urllib.error.URLError, TimeoutError):
                raise OrchestrationError("编排服务暂时连不上；先核对原任务，不自动重发") from None

        if method == "GET" and password is None:
            return request(method, path, body, {"Authorization": "Bearer " + self.config["tokens"]["operator"]})
        if not isinstance(password, str) or not password:
            raise OrchestrationError("请输入演示账号密码", "UNAUTHORIZED")
        auth = request("POST", "/auth/login", {"user": self.config["user"], "password": password},
                       {"Origin": self.base})
        return request(method, path, body, {"X-CSRF-Token": auth["csrf"], "Origin": self.base})

    def dispatch(self, method: str, params: Any) -> Any:
        params = params if isinstance(params, dict) else {}
        if method == "plans":
            return self.call("GET", "/v1/workbench/plans")
        task = params.get("task") or params.get("run") or params.get("plan")
        if not isinstance(task, str) or not TASK.fullmatch(task):
            raise OrchestrationError("需要原任务ID", "BAD_REQUEST")
        if method == "graph":
            return self.call("GET", f"/v1/workbench/tasks/{task}" + ("?verify=1" if params.get("verify") else ""))
        password = params.get("password")
        if method == "approve":
            if params.get("version") != 1 or not isinstance(params.get("hash"), str):
                raise OrchestrationError("需要当前图版本与摘要", "BAD_REQUEST")
            return self.call("POST", f"/v1/tasks/{task}/admit", {"graph_digest": params["hash"]}, password=password)
        if method == "resume":
            return self.call("POST", f"/v1/tasks/{task}/resume", {}, password=password)
        if method == "command":
            action, artifact = params.get("action"), params.get("artifact")
            if action == "pause":
                return self.call("POST", f"/v1/tasks/{task}/pause", {}, password=password)
            if not isinstance(artifact, str) or not ARTIFACT.fullmatch(artifact):
                raise OrchestrationError("需要当前报告产物ID", "BAD_REQUEST")
            if action == "approve_report":
                return self.call("POST", f"/v1/tasks/{task}/artifacts/{artifact}/approve", {}, password=password)
            if action == "download_report":
                return self.call("GET", f"/v1/tasks/{task}/artifacts/{artifact}/download", password=password)
            if action == "deliver":
                key = params.get("command_id")
                if not isinstance(key, str) or not 1 <= len(key) <= 100:
                    raise OrchestrationError("需要稳定交付命令ID", "BAD_REQUEST")
                return self.call("POST", f"/v1/tasks/{task}/artifacts/{artifact}/deliver", {"delivery_id": key}, password=password)
        raise OrchestrationError("不支持的命令", "BAD_REQUEST")

    def bind_web_channel(self, channel: Any, services: Any) -> None:
        del services
        for name in ("plans", "graph", "approve", "resume", "command"):
            def handler_for(method):
                async def handler(ws, req_id, params, session_id):
                    del session_id
                    try:
                        payload = await asyncio.to_thread(self.dispatch, method, params)
                    except OrchestrationError as exc:
                        await channel.send_response(ws, req_id, ok=False, error=str(exc), code=exc.code)
                        return
                    except Exception:
                        await channel.send_response(ws, req_id, ok=False, error="插件暂不可用；保留原任务身份", code="UPSTREAM")
                        return
                    await channel.send_response(ws, req_id, ok=True, payload=payload)
                return handler
            channel.register_method(f"plugin.credit_workbench.{name}", handler_for(name), local_only=True)


async def register_extensions(registry: Any) -> list[CreditWorkbench]:
    plugin = CreditWorkbench()
    registry.register_application_plugin(plugin)
    return [plugin]
