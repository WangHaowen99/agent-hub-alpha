"""每次启动幂等执行：首次 init；模型配置写进工作区 config/.env；
注册编排服务的 MCP 连接器、写入编排技能，并建一个绑定两者的"授信编排助手"专家。"""
from __future__ import annotations

import json
import logging
import os
import re
import shutil
import subprocess
from pathlib import Path

HOME = Path(os.environ.get("JIUWENSWARM_DATA_DIR") or Path.home() / ".jiuwenswarm")
CONFIG = HOME / "config" / "config.yaml"
DOTENV = HOME / "config" / ".env"
MCP_NAME = "credit-orchestration"
SKILL_NAME = "credit-orchestration"
EXPERT_ID = "credit-orchestrator"

SKILL_MD = """---
name: credit-orchestration
description: 把一项多步任务编排成由已登记专家组成的编排图，先显性化给用户审批，批准后再执行并验证。用户要做授信尽调、要组队完成多步任务、要看编排进度或验证结果时使用。
---

# 编排技能

你负责"找出 f"：从能力目录里挑专家组队，交出一张编排图。你不执行任何专家的工作，也没有批准权。

## 步骤

1. 调用 `list_capabilities`，只能从返回的 experts 里选专家（expert 字段填目录 id），不要编造专家，也不要自己替专家干活。
   返回的 `plan_rules` 是尽调规约，编排单必须逐条满足，预检会按它检查。
2. 组队：每个成员写
   - `id`：简短英文，如 intake、financial、bureau、judge、report、submit；
   - `expert`：目录 id；`role`：角色；`task`：这一步的子任务（写清要交付什么）；
   - `depends_on`：需要哪些上游步骤的产物。
   关联方情况事先拆不清，交给黑盒专家 related-party（kind=blackbox，由 Loop Forest 运行时自主分叉）。
   规矩：
   - 调查步骤（财务、行业、征信、关联方）都依赖材料受理，彼此并行；
   - 风险判断（kind=judge）依赖所有调查步骤和黑盒步骤；报告撰写依赖风险判断；
   - 用户要求提交申请时，加有不可逆效果的专家（effects.level=irreversible），它依赖风险判断，
     并设 `run_if: {"step": "<判断步骤 id>", "contains": "DECISION=GRANT"}`，只在判断同意时执行。
3. 调用 `propose_plan`（case_id、goal、agents）。编排图显示在 WorkSwarm 左侧导航的「编排协作台」里：不要自己画图（mermaid、ASCII 都不要），不要列表复述成员；只用一两句话说明为什么这样组队。
4. 规约预检不通过（precheck.ok=false）：把 `not_held` 和 `errors` 如实告诉用户，换成权限范围内的专家，带上同一个 plan_id 再次 `propose_plan`。不要为了过预检删掉成员；改不了就说明原因，请用户决定。
5. 预检通过：请用户到「编排协作台」选中这版编排单（如 P3 v2），点「批准」并输入密码确认。你没有批准或执行的工具，批准前不要说"已开始执行"。批准后协作台会实时显示执行到哪一步。
6. 用户问进度时用 `run_status`（plan_id）读；状态、判断结论、银行回执只按工具返回说，不要推测。
7. 执行结束（status=done）后调用 `verify_run`。检查结果也显示在协作台的执行图下方，你只需一句话总结。
   有未通过的项：按该项的 `items` 如实说明是哪一步、缺了什么，不要推测原因，也不要提出工具没有给出的修复方案。
"""

PERSONA = ("你是授信编排助手。用 credit-orchestration 技能为任务组队、给出编排图、请用户审批，批准后跟踪执行并做验证。只依据工具返回说话。"
           "编排图显示在「编排协作台」里，确认也只在那里做：你绝不自己画图（包括 mermaid、ASCII 图），也不逐个列出成员。")


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


def set_top_level(key: str, value: str) -> None:
    text = CONFIG.read_text(encoding="utf-8")
    new, n = re.subn(rf"(?m)^{key}:.*$", f"{key}: {value}", text)
    if n == 0:
        new = text.rstrip("\n") + f"\n{key}: {value}\n"
    if new != text:
        CONFIG.write_text(new, encoding="utf-8")


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
    state.setdefault("mcp", {})[MCP_NAME] = {
        "transport": "streamable-http",
        "url": os.environ["CREDIT_MCP_URL"],
        "headers": {},
        "timeout_s": 120,
        "integration_type": "remote-mcp",
        "state": "connected",
        "enabled": True,
        "server_id_scope": f"mcp:{MCP_NAME}",
        "description": "授信编排服务：能力目录、编排单与规约预检、执行状态、验证",
    }
    tmp = path.with_suffix(".tmp")
    tmp.write_text(json.dumps(state, ensure_ascii=False, indent=2, sort_keys=True), encoding="utf-8")
    os.replace(tmp, path)


def write_skill() -> None:
    from jiuwenswarm.common.utils import get_agent_skills_dir

    d = Path(get_agent_skills_dir()) / SKILL_NAME
    d.mkdir(parents=True, exist_ok=True)
    (d / "SKILL.md").write_text(SKILL_MD, encoding="utf-8")


def ensure_expert() -> None:
    from jiuwenswarm.server.runtime import extension_package_manager as epm

    params = {
        "id": EXPERT_ID,
        "name": "授信编排助手",
        "description": "从能力目录里挑专家组队，给出可审批的编排图；批准后跟踪执行并做规约验证。",
        "persona": PERSONA,
        "skills": [SKILL_NAME],
        "mcps": [MCP_NAME],
        "quickInputs": ["为客户 C006 做授信尽调，要查完整征信，结论同意就提交授信申请", "我已批准，看一下执行进度", "验证这次执行"],
        "tags": [{"zh": "授信", "en": "Credit"}],
    }
    if (epm._local_root("agent_templates") / EXPERT_ID).is_dir():  # noqa: SLF001 - no public probe
        epm.update_agent_template(params)
    else:
        epm.create_agent_template(params)
    ok, payload = epm.install_equipment_gated("agent_templates", {"id": EXPERT_ID})
    if not ok:
        logging.warning("expert %s not installed: %s", EXPERT_ID, payload)


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="[credit-configure] %(message)s")
    ensure_init()
    set_dotenv({
        "API_BASE": os.environ["SWARM_API_BASE"],
        "API_KEY": os.environ["SWARM_API_KEY"],
        "MODEL_NAME": os.environ.get("SWARM_MODEL_NAME", "qwen-plus"),
        "MODEL_PROVIDER": "OpenAI",
        "ENDPOINT_PROFILE": os.environ.get("SWARM_ENDPOINT_PROFILE", "dashscope"),
    })
    # 编排工具要直接可调：渐进加载下 qwen-plus 有时跳过 tool_search 直接 tool_call，调用失败后就绕开工具自己编方案。
    # 代价是每轮提示词带上全部工具定义；这是演示专用实例，可以接受。
    set_top_level("progressive_tool_enabled", "false")
    register_connector()
    write_skill()
    ensure_expert()
    logging.info("workspace ready")


if __name__ == "__main__":
    main()
