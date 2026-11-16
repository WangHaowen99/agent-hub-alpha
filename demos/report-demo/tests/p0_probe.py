"""Real HTTP/MCP/PG technical smoke, explicitly not a real-model planning test."""
from __future__ import annotations

import argparse
import asyncio
import json
from pathlib import Path

import requests
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client


async def verify(config: dict) -> dict:
    base = config["public_url"]
    headers = {"Authorization": "Bearer " + config["tokens"]["planner"]}
    health = requests.get(base + "/healthz", timeout=10)
    health.raise_for_status()
    checks = {"control_health": True, "real_model_planning": False}
    async with streamablehttp_client(base + "/mcp", headers=headers) as transport:
        async with ClientSession(transport[0], transport[1]) as client:
            await client.initialize()
            tools = await client.list_tools()
            names = {t.name for t in tools.tools}
            assert names == {"list_capabilities", "propose_plan", "get_plan", "run_status", "verify_run"}
            checks["planner_has_no_control_write_tools"] = True
            resource = await client.read_resource("ui://report-demo/graph")
            assert "text/html" in resource.contents[0].mimeType
            checks["mcp_app_resource"] = True
            catalog_result = await client.call_tool("list_capabilities", {})
            catalog = catalog_result.structuredContent
            graph = {"goal": "[TECHNICAL FIXTURE] verify transport and confirmation only", "material_id": catalog["material_id"],
                     "focus": "business", "catalog_digest": catalog["catalog_digest"],
                     "policy_version": catalog["catalog"]["policy_version"], "nodes": [
                         {"id": "extract", "expert": "extractor@1", "role": "资料", "task": "从固定材料提取", "depends_on": []},
                         {"id": "business", "expert": "business@1", "role": "经营", "task": "消费提取产物", "depends_on": ["extract"]},
                         {"id": "draft", "expert": "writer@1", "role": "写作", "task": "消费经营分析", "depends_on": ["business"]},
                         {"id": "check", "expert": "checker@1", "role": "核验", "task": "检查同一报告", "depends_on": ["draft"]},
                         {"id": "select", "expert": "select@1", "role": "选择", "task": "选择已核验报告", "depends_on": ["check"]},
                     ]}
            for missing in ('check', 'select'):
                excluded = {'check', 'select'} if missing == 'check' else {'select'}
                invalid = dict(graph, nodes=[n for n in graph['nodes'] if n['id'] not in excluded])
                rejected = await client.call_tool('propose_plan', {'command_id': 'incomplete-report-' + missing, 'graph': invalid})
                assert rejected.isError, 'incomplete report was admitted through MCP'
                http_rejected = requests.post(base + '/v1/commands', headers=headers,
                                              json={'command_id': 'incomplete-report-' + missing, 'graph': invalid}, timeout=10)
                assert http_rejected.status_code == 422 and http_rejected.json()['error']['code'] == 'POLICY_REJECTED'
            checks['http_and_mcp_reject_incomplete_report'] = True
            import uuid
            command = "technical-probe-" + uuid.uuid4().hex
            result = await client.call_tool("propose_plan", {"command_id": command, "graph": graph})
            assert not result.isError, result.content
            task = result.structuredContent["task"]
            checks.update(task_id=task["task_id"], graph_digest=task["graph_digest"], java_compiler_digest=task["runtime_spec_digest"])
            duplicate = await client.call_tool("propose_plan", {"command_id": command, "graph": graph})
            assert duplicate.structuredContent["task"]["task_id"] == task["task_id"]
            checks["mcp_duplicate_command"] = True
            grant = result.meta["report_view_authorization"]
            assert requests.get(result.structuredContent["projection_url"], headers={"Authorization": "ReportView " + grant}, timeout=10).status_code == 200
            assert requests.get(result.structuredContent["projection_url"], timeout=10).status_code == 401
            checks["scoped_projection_read"] = True
            assert requests.post(base + "/v1/tasks/" + task["task_id"] + "/admit", headers=headers,
                                 json={"graph_digest": task["graph_digest"]}, timeout=10).status_code == 401
            session = requests.Session()
            login = session.post(base + "/auth/login", json={"user": config["user"], "password": config["user_password"]}, timeout=10)
            login.raise_for_status()
            user_headers = {"X-CSRF-Token": login.json()["csrf"]}
            # Pause before admission: an enabled live worker must never race
            # this transport fixture's confirmation and start a model node.
            stopped = session.post(base + "/v1/tasks/" + task["task_id"] + "/pause", headers=user_headers, json={}, timeout=10)
            stopped.raise_for_status()
            assert session.post(base + "/v1/tasks/" + task["task_id"] + "/admit",
                                json={"graph_digest": task["graph_digest"]}, timeout=10).status_code == 403
            approved = session.post(base + "/v1/tasks/" + task["task_id"] + "/admit", headers=user_headers,
                                    json={"graph_digest": task["graph_digest"]}, timeout=10)
            approved.raise_for_status()
            assert approved.json()["status"] == "QUEUED"
            paused = session.post(base + "/v1/tasks/" + task["task_id"] + "/pause", headers=user_headers, json={}, timeout=10)
            paused.raise_for_status()  # keep this technical fixture out of real worker execution
            final = session.get(base + "/v1/tasks/" + task["task_id"], timeout=10)
            final.raise_for_status()
            assert all(n["attempt"] == 0 and not n.get("result") for n in final.json()["nodes"].values())
            task_after = session.get(base + "/v1/tasks/" + task["task_id"], timeout=10).json()
            assert task_after["approved_by"] == config["user"] and task_after["pause_requested"]
            checks["trusted_confirmation_csrf_and_pg_readback"] = True
            checks["fixture_paused_without_expert_execution"] = True
    return checks


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--report", type=Path, required=True)
    args = parser.parse_args()
    config = json.loads(args.config.read_text(encoding="utf-8-sig"))
    report = asyncio.run(verify(config))
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report, ensure_ascii=True))


if __name__ == "__main__":
    main()
