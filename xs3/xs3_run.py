#!/usr/bin/env python3
"""X-S3 共译差分·单臂单发启动器（用户本人执行——真 LLM 纪律）。

用法（钥匙经环境变量注入，不落盘不回显；本役实际执行时两臂统一走 XS3_KEY=volcano 网关钥匙）:
  export XS3_KEY=...

  python3 -I xs3_run.py --arm glm   # 我方弱形态臂（glm-5.3-flash）
  python3 -I xs3_run.py --arm qwen  # 第四臂（qwen3.8-flash）

行为: 单次调用（零上下文）· sampling 参数从 ARMS 单源（temperature=0/max_tokens=32768，v1.1 起 body 与台账同源）· 无自动重试。
首译绑定（86号 §四.2 拟定）: 首个完整呈回件=五格行齐全且每行含「格ID｜判定｜依据」；
不完整中间态不算呈回——本器只标记 COMPLETE/INCOMPLETE，是否重跑由你决定，
每次尝试（含失败）都写入 attempt 台账，如实留痕。
"""
import argparse, json, os, sys, time, pathlib

def _mint_attempt_id():
    """One ID per request lifecycle (PENDING row and terminal row share it
    when the caller pre-minted; the ledger stays append-only — two rows,
    one lifecycle)."""
    import uuid as _uuid
    return _uuid.uuid4().hex[:8]


def _prompt_digest(text):
    """Prompt content digest (sha256 12-hex) — acceptance 2: one row can
    identify which input file ran."""
    import hashlib
    return hashlib.sha256(text.encode()).hexdigest()[:12]


def _cells_used(text):
    """The expected-cell string actually derived from this prompt."""
    return cells_from_prompt(text)


ARMS = {
    "glm":  dict(endpoint="https://st8tp3ajl0df3n8b8l8qu.apigateway-cn-beijing.volceapi.com/compatible/chat/completions",
                 env="XS3_KEY", model="glm-5.3-flash",
                 proto="anthropic(volcano-compatible 形)", temperature=0, max_tokens=32768,
                 version_pin="GLM-5.3-Flash 上线 2026-08-26 ⇒ 训练截止≤2026-08-26 < 2026-11-10 08:12 ✓（GLM 家族=我方弱形态臂）；端点=volcano 网关（用户钥匙基建；86号「bigmodel 网关」措辞更正；56pp 代理前科披露 XM2 档案）"),
    "qwen": dict(endpoint="https://st8tp3ajl0df3n8b8l8qu.apigateway-cn-beijing.volceapi.com/compatible/chat/completions",
                 env="XS3_KEY", model="qwen3.8-flash",
                 proto="anthropic(volcano-compatible 形)", temperature=0, max_tokens=32768,
                 version_pin="qwen3.8-flash 产品线至迟 2026-09-19 在馆——上线日期非实际调用权重快照/训练截止的证明（保守披露口径）；同 id 静默更新盲区+同网关相关性披露在计分附注 v3"),
}
HERE = pathlib.Path(__file__).parent
OUT = HERE / "results"
PROMPT_FILE = HERE / "arm-input.md"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--arm", choices=ARMS, required=True)
    ap.add_argument("--prompt", default=None, help="输入件路径（默认 arm-input.md）；扰动臂用补正层冻结的对照/扰动件")
    a = ap.parse_args()
    conf = ARMS[a.arm]
    key = os.environ.get(conf["env"])
    if not key:
        sys.exit(f"缺 {conf['env']}——先在你的 shell 里 export（钥匙不经任何文件）")
    prompt_path = PROMPT_FILE if a.prompt is None else __import__("pathlib").Path(a.prompt)
    prompt = prompt_path.read_text(encoding="utf-8")
    # R6 F2 fix: sampling params single-sourced from ARMS — body and ledger
    # can no longer disagree ("one row reconstructs the run conditions" holds
    # by construction, not by coincidence of two hardcodes staying in sync).
    body = {
        "model": conf["model"],
        "temperature": conf["temperature"] if conf["temperature"] is not None else 0,
        "max_tokens": conf["max_tokens"],
        "messages": [{"role": "user", "content": prompt}],
    }
    import urllib.request
    req = urllib.request.Request(
        conf["endpoint"], data=json.dumps(body).encode(),
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"})
    t0 = time.time()
    # R10 acceptance-2 completion (125号 B 级): the attempt ID is minted and a
    # PENDING row is written BEFORE the request leaves — a hard-kill window
    # (SIGKILL between urlopen and the old log-at-completion call) used to
    # leave zero rows. The PENDING row shares the schema; every terminal path
    # (parse-fail exit / HTTP-fail exit / COMPLETE / INCOMPLETE) appends its
    # own new-ID row alongside (append-only stays true — PENDING is a
    # lifecycle record, not an overwrite).
    _aid = _mint_attempt_id()
    log_attempt(a.arm, conf, None, "PENDING", t0,
                prompt_digest=_prompt_digest(prompt), cells=_cells_used(prompt),
                proto=ARMS[a.arm].get("proto", ""),
                temperature=ARMS[a.arm].get("temperature"),
                max_tokens=ARMS[a.arm].get("max_tokens"),
                attempt_id=_aid)
    try:
        with urllib.request.urlopen(req, timeout=600) as r:
            raw = json.loads(r.read())
    except Exception as e:
        log_attempt(a.arm, conf, None, f"FAILED: {type(e).__name__}: {e}", t0,
                    prompt_digest=_prompt_digest(prompt), cells=_cells_used(prompt),
                    proto=ARMS[a.arm].get("proto", ""),
                    temperature=ARMS[a.arm].get("temperature"),
                    max_tokens=ARMS[a.arm].get("max_tokens"))
        sys.exit(f"调用失败（已入 attempt 台账，不算呈回）: {e}")
    # 双形解析: OpenAI(choices.message) 或 Anthropic(content blocks)——volcano /compatible 系 Claude 兼容层
    # RV25-03 fix: EVERY parse path from response-read to verdict lands in
    # the ledger — malformed shapes (null choices, null message, non-list
    # content, missing keys) used to crash with zero rows. One try/except
    # around the whole extraction block replaces the top-level-only guard.
    try:
        if not isinstance(raw, dict):
            raise ValueError(f"non-dict response body: {type(raw).__name__}")
        if raw.get("choices"):
            msg = (raw.get("choices") or [{}])[0].get("message") or {}
            content = msg.get("content", "") or ""
            reasoning = msg.get("reasoning_content", "") or ""
        else:
            blocks = raw.get("content") or []
            content = "".join(b.get("text", "") for b in blocks if b.get("type") == "text")
            reasoning = "".join(b.get("thinking", "") for b in blocks if b.get("type") == "thinking")
        # R8 M2 fix: non-str content/reasoning shapes (int, list, None from a
        # malformed message) normalise INSIDE the try — the R6 RV25-03 fix
        # claimed "EVERY parse path" but validate/reasoning_len sat outside
        # the block; content=12345 crashed with zero rows. Normalisation makes
        # the downstream non-str-unsafe expression unrepresentable.
        content = content if isinstance(content, str) else ""
        reasoning = reasoning if isinstance(reasoning, str) else ""
    except (ValueError, TypeError, AttributeError, IndexError, KeyError) as e:
        log_attempt(a.arm, conf, None,
                    f"FAILED: parse: {type(e).__name__}: {e}", t0,
                    prompt_digest=_prompt_digest(prompt), cells=_cells_used(prompt),
                    proto=ARMS[a.arm].get("proto", ""),
                    temperature=ARMS[a.arm].get("temperature"),
                    max_tokens=ARMS[a.arm].get("max_tokens"))
        sys.exit(f"响应解析失败（已入 attempt 台账，不算呈回）: {type(e).__name__}: {e}")
    complete = validate(content, cells=cells_from_prompt(prompt))
    log_attempt(a.arm, conf, raw, "COMPLETE" if complete else "INCOMPLETE", t0,
                content=content, reasoning_len=len(reasoning),
                prompt_digest=_prompt_digest(prompt), cells=_cells_used(prompt),
                proto=ARMS[a.arm].get("proto", ""),
                temperature=ARMS[a.arm].get("temperature"),
                max_tokens=ARMS[a.arm].get("max_tokens"))
    print(f"arm={a.arm} model={conf['model']} -> {'COMPLETE' if complete else 'INCOMPLETE'}")
    print(f"判定行:\n{content}")


import re as _re

def validate(content: str, cells: str = None) -> bool:
    # R2 m3 tightened the row shape (cell ID anchored at line start, non-empty
    # verdict between the bars). The cell COUNT was then hardcoded — ==5 for
    # the five-cell originals, bumped to ==6 for the perturbation batch —
    # and the hardcode false-REDs any reply whose material carries a
    # different cell count (the five-cell first translations failed under
    # the six-cell constant; full-test regression face, 2026-11-13).
    # Structural fix (消除优于处理): the expected cell set is DERIVED from
    # the prompt's grid table — no count lives in this function at all.
    if cells is None:
        cells = "ABCDEF"
    # RV25-01 fix (acceptance 1): empty expected set = mis-pointed input,
    # never COMPLETE ("ok == len('')" was vacuously True). Each cell must
    # appear EXACTLY ONCE with non-empty verdict AND non-empty basis; a
    # duplicate row (contradictory or not) fails.
    if not cells:
        return False
    for cell in cells:
        rows = [l for l in content.splitlines()
                if _re.match(r"\s*" + cell + r"\s*[｜|]\s*([^｜|\n]+?)\s*[｜|]", l)]
        if len(rows) != 1:
            return False  # missing or duplicated verdict
        m = _re.match(r"\s*" + cell + r"\s*[｜|]\s*([^｜|\n]+?)\s*[｜|]\s*(.+)", rows[0])
        if not m or not m.group(1).strip() or not m.group(2).strip():
            return False  # empty verdict or empty basis
    return True


def cells_from_prompt(prompt_text: str) -> str:
    """Derive the expected cell IDs from the prompt's grid table rows."""
    found = []
    for l in prompt_text.splitlines():
        m = _re.match(r"\|\s*([A-Z])\s*\|", l)
        if m and m.group(1) not in found:
            found.append(m.group(1))
    return "".join(found)


def log_attempt(arm, conf, raw, status, t0, content="", reasoning_len=0,
                prompt_digest="", cells="", proto="", temperature=None, max_tokens=None,
                attempt_id=None):
    """RV25-02 fix: unique attempt ID (uuid prefix — same-second/same-arm can
    never overwrite; codex uuid_v7 precedent) + append-only ('x' mode — an
    existing name is a bug, refusing loudly beats replacing a record) + the
    acceptance extras (prompt digest / cell set / sampling params / proto)
    so one row can reconstruct the run conditions."""
    import uuid as _uuid
    OUT.mkdir(exist_ok=True)
    ts = time.strftime("%Y%m%d-%H%M%S")
    aid = attempt_id or _uuid.uuid4().hex[:8]
    rec = {
        "attempt_id": f"{ts}-{aid}", "ts": ts, "arm": arm,
        "model": conf["model"], "version_pin": conf["version_pin"],
        "status": status, "elapsed_s": round(time.time() - t0, 1),
        "reasoning_len": reasoning_len,
        "prompt_digest": prompt_digest, "cells": cells, "proto": proto,
        "temperature": temperature, "max_tokens": max_tokens,
        "usage": (raw or {}).get("usage"),
    }
    with open(OUT / f"attempt-{ts}-{aid}-{arm}.json", "x", encoding="utf-8") as f:
        f.write(json.dumps(rec, ensure_ascii=False, indent=1))
    if raw is not None:
        with open(OUT / f"raw-{ts}-{aid}-{arm}.json", "x", encoding="utf-8") as f:
            f.write(json.dumps(raw, ensure_ascii=False))
    if content:
        with open(OUT / f"reply-{ts}-{aid}-{arm}.md", "x", encoding="utf-8") as f:
            f.write(content)


if __name__ == "__main__":
    main()
