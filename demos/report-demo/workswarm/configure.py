"""Install one report planner via public WorkSwarm APIs in an isolated workspace.

Adapted from W05 configure.py's assistant/skill/MCP assembly, not its private
directory probing or credit business. Model credentials are optional only for
configuration preparation; a true P0-WS run still requires a real model.
Model credentials are read from process environment only and never persisted.
"""
from __future__ import annotations

import argparse
import asyncio
import json
import os
import shutil
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--workspace", type=Path, required=True)
    parser.add_argument("--model-from-env", action="store_true")
    args = parser.parse_args()
    workspace = args.workspace.resolve()
    os.environ["JIUWENSWARM_DATA_DIR"] = str(workspace)
    config = json.loads(args.config.read_text(encoding="utf-8-sig"))

    # The init CLI derives the default HOME directly. Use the public workspace
    # preparation function with an explicit path to preserve the user's home.
    from jiuwenswarm.common.utils import get_agent_skills_dir, get_config_file, get_env_file, prepare_workspace
    prepare_workspace(overwrite=False, preferred_language="zh", workspace_dir=workspace)
    # The generated example .env is loaded with override=True by upstream.
    # Remove only its model stub assignments so the launching process owns all
    # model settings. Never replace them with actual model credentials.
    env_path = get_env_file()
    if env_path.exists():
        reserved = {"API_BASE", "API_KEY", "MODEL_NAME", "MODEL_PROVIDER", "ENDPOINT_PROFILE"}
        lines = env_path.read_text(encoding="utf-8").splitlines()
        env_path.write_text("\n".join(line for line in lines if line.split("=", 1)[0].strip().removeprefix("export ") not in reserved) + "\n", encoding="utf-8")

    from jiuwenswarm.server.runtime import extension_package_manager as epm
    from jiuwenswarm.server.runtime.mcp.credential import CredentialStore
    from jiuwenswarm.server.runtime.mcp.registry import connect_mcp, register_custom_mcp
    from ruamel.yaml import YAML

    name = "report-orchestration"
    CredentialStore().save_token(name, "REPORT_PLANNER_TOKEN", config["tokens"]["planner"])
    register_custom_mcp(name, {
        "transport": "streamable-http", "url": config["public_url"] + "/mcp",
        "headers": {"Authorization": "Bearer ${REPORT_PLANNER_TOKEN}"}, "timeout_s": 20,
    })
    connect_mcp(name)  # connecting means selectable, not a fabricated handshake pass

    # The current public connect API intentionally leaves custom connectors in
    # "connecting" until a handler verifies them. Do the actual handshake here
    # before recording connected and entering the assistant installation gate.
    from mcp import ClientSession
    from mcp.client.streamable_http import streamablehttp_client
    from jiuwenswarm.server.runtime.mcp.state_store import set_mcp_state

    async def verify_connector() -> None:
        async with streamablehttp_client(config["public_url"] + "/mcp",
                headers={"Authorization": "Bearer " + config["tokens"]["planner"]}) as transport:
            async with ClientSession(transport[0], transport[1]) as session:
                await session.initialize()
                tools = await session.list_tools()
                names = {tool.name for tool in tools.tools}
                expected = {"list_capabilities", "select_capabilities", "propose_plan", "get_plan", "run_status", "verify_run"}
                if names != expected:
                    raise RuntimeError("Planner connector exposes an unexpected tool surface")

    asyncio.run(verify_connector())
    set_mcp_state(name, state="connected")

    target = Path(get_agent_skills_dir()) / "annual-report"
    target.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(Path(__file__).parent / "skills/annual-report/SKILL.md", target / "SKILL.md")
    params = {
        "id": "report-planner", "name": "年报协作规划助手",
        "description": "消费获准能力目录，形成候选图，交用户确认，跟踪真实报告与核验。",
        "persona": "你是年报协作规划助手。必须调用目录与提交工具，按 annual-report 技能生成候选；你不执行专家工作，没有批准权。只根据工具事实报告状态。",
        "skills": ["annual-report"], "mcps": [name],
        "quickInputs": ["根据招商银行2025年报生成经营与资产质量简报，比较2024，说明八项指标、变化与来源。"],
        "tags": [{"zh": "年报", "en": "Annual report"}],
    }
    if epm.show_agent_template("report-planner") is None:
        epm.create_agent_template(params)
    else:
        epm.update_agent_template(params)
    ok, _payload = epm.install_equipment_gated("agent_templates", {"id": "report-planner"})
    if not ok:
        raise SystemExit("Report planner installation rejected; no readiness claimed")

    yaml = YAML()
    config_path = Path(get_config_file())
    data = yaml.load(config_path.read_text(encoding="utf-8"))
    data["progressive_tool_enabled"] = False
    with config_path.open("w", encoding="utf-8") as stream:
        yaml.dump(data, stream)

    from install_policy import install
    install(workspace)

    # Gateway's documented user Application Plugin directory. Only install our
    # W02 adapter and its prebuilt assets; do not overwrite any other plugin.
    plugin = Path(__file__).parent / "workbench-plugin"
    if not (plugin / "frontend/dist/index.html").is_file():
        raise SystemExit("Build workbench-plugin/app first; plugin installation incomplete")
    installed = workspace / "domain/application_plugins/credit-workbench"
    installed.mkdir(parents=True, exist_ok=True)
    for name in ("extension.py", "extension.yaml"):
        shutil.copyfile(plugin / name, installed / name)
    shutil.copytree(plugin / "frontend", installed / "frontend", dirs_exist_ok=True)

    if args.model_from_env:
        required = ("API_BASE", "API_KEY", "MODEL_NAME")
        if any(not os.environ.get(key) for key in required):
            raise SystemExit("Model environment is incomplete; no credentials persisted")
    print(json.dumps({"workspace": str(workspace), "assistant_installed": True, "connector_handshake_verified": True,
                      "model_environment_present": bool(args.model_from_env), "true_planning_verified": False}))


if __name__ == "__main__":
    main()
