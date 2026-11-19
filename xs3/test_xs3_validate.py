#!/usr/bin/env python3
"""X-S3 仪器红探针电池（F7 落仓——114号 挂账件，2026-11-15 用户授权执行）。

背景：validate()/attempt 台账的守卫面此前只活在邢 PR 评论的手工复验脚本里，
仓内零调用者（count-drift 族无永久闸——守恒律缺口）。本电池=纯新增文件，
不改 xs3_run.py 一字；每次 xs3_run.py 被触碰（如夏方模板交叉实验启动前）必跑：

  python3 -I xs3/test_xs3_validate.py     # 全 PASS rc=0；任一红 rc=1

族覆盖（每族至少一案，正控在位防「恒 False 假绿」）：
  A validate 守卫族   —— 空派生集/缺格/重格/空判定/空依据（邢三命令原文为锚案）
  B 派生族           —— cells_from_prompt 五格/六格/无格（count 不落硬编码的实证面）
  C attempt 生命周期族 —— 同秒双发不覆盖('x'+uuid)/同名拒绝覆写/一行重建运行条件
  D 解析路径族        —— 强杀窗口 PENDING 在案/畸形体入台账不裸崩/非 str 归一/
                        anthropic blocks 真形分支（R11-F1）/HTTP 失败终态入台账（R11-M1
                        ——四终态路径全覆盖）/choices 分支正控（R11-r3-M1——双解析分支×
                        终态矩阵补格，anthropic×INCOMPLETE 为已裁不报格）/请求体
                        sampling 面（R11-r4-M1——body
                        与台账同源契约的 body 侧）
  E 正控             —— 真形五格/六格 COMPLETE、ASCII 竖杠形、摘要钉

每个红案耦合一个变异：撤销对应守卫 → 本电池必须红（杀灭清单见 134号 执行记录）。
"""
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest.mock as mock

HERE = pathlib.Path(__file__).parent
_spec = importlib.util.spec_from_file_location("xs3_run", HERE / "xs3_run.py")
m = _spec  # module holder; loaded lazily per probe group below
RUN = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(RUN)

FIVE = "ABCDE"
FIVE_GRID_PROMPT = "\n".join(f"| {c} | 判定 | 依据 |" for c in FIVE)
SIX_GRID_PROMPT = "\n".join(f"| {c} | 判定 | 依据 |" for c in "ABCDEF")
# 真形完整回件（全角竖杠——历史 run 的实际形态）
COMPLETE_FIVE = "\n".join(f"{c}｜grant-50000｜依据-{c}" for c in FIVE)
COMPLETE_SIX = "\n".join(f"{c}｜deny｜依据-{c}" for c in "ABCDEF")


def _attempt_rows(out_dir):
    return [json.loads(p.read_text(encoding="utf-8"))
            for p in sorted(out_dir.glob("attempt-*.json"))]


# ── A/B/E. validate + 派生 + 正控（纯函数面，无文件系统） ──────────────

def a_empty_expected_set():
    """邢命令原文案 1：无格 prompt 派生空集 → False（RV25-01——空集恒 True 已结构退役）。"""
    return (RUN.validate("", RUN.cells_from_prompt("no grid here")) is False
            and RUN.validate(COMPLETE_FIVE, cells="") is False)

def a_no_basis():
    """邢命令原文案 2：判定齐但依据空 → False。"""
    return RUN.validate("\n".join(c + "|deny|" for c in FIVE), FIVE) is False

def a_contradiction_duplicate_cell():
    """邢命令原文案 3：同格两行（矛盾或一致）→ False（len(rows)!=1 管重格）。"""
    body = "\n".join(c + "|deny|basis" for c in FIVE) + "\nA|grant-50000|other"
    return RUN.validate(body, FIVE) is False

def a_missing_cell():
    """缺一格（四行回五格 prompt）→ False。"""
    return RUN.validate("\n".join(f"{c}｜deny｜b" for c in "ABCD"), FIVE) is False

def a_empty_verdict():
    """判定空（双杠间无字）→ False。"""
    return RUN.validate("A｜｜依据\n" + "\n".join(f"{c}｜deny｜b" for c in "BCDE"), FIVE) is False

def b_derivation_shapes():
    """派生族：五格 prompt→ABCDE、六格→ABCDEF、无格→空串（count 不硬编码的实证面）。"""
    return (RUN.cells_from_prompt(FIVE_GRID_PROMPT) == FIVE
            and RUN.cells_from_prompt(SIX_GRID_PROMPT) == "ABCDEF"
            and RUN.cells_from_prompt("no table") == "")

def e_positive_control_five():
    """正控：真形五格完整回件 → True。"""
    return RUN.validate(COMPLETE_FIVE, RUN.cells_from_prompt(FIVE_GRID_PROMPT)) is True

def e_positive_control_six():
    """正控：六格完整回件 → True——旧硬编码 ==5 在此必红（count-drift 族历史病）。"""
    return RUN.validate(COMPLETE_SIX, RUN.cells_from_prompt(SIX_GRID_PROMPT)) is True

def e_ascii_bars():
    """形容忍正控：ASCII 竖杠同构 → True。"""
    return RUN.validate("\n".join(c + "|deny|basis" for c in FIVE), FIVE) is True

def e_prompt_digest_pinned():
    """摘要钉：sha256 前 12 hex，钉独立字面量（防实现漂移为别的口径）。"""
    return RUN._prompt_digest("x") == "2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881"[:12]


# ── C. attempt 生命周期族（临时目录面） ─────────────────────────────

def c_two_attempts_distinct():
    """邢命令原文案 4：同秒同臂双发 → 两行不覆盖（uuid 前缀唯一）。"""
    with tempfile.TemporaryDirectory() as td:
        RUN.OUT = pathlib.Path(td)
        conf = {"model": "s", "version_pin": "s"}
        with mock.patch.object(RUN.time, "strftime", return_value="20261115-120000"):
            RUN.log_attempt("q", conf, {}, "F", 0.0)
            RUN.log_attempt("q", conf, {}, "S", 0.0)
        rows = _attempt_rows(pathlib.Path(td))
        return len(rows) == 2 and rows[0]["attempt_id"] != rows[1]["attempt_id"]

def c_append_only_refuses_overwrite():
    """同名覆写拒绝：'x' 模式对已存在文件名大声失败（FileExistsError）而非替换记录。"""
    with tempfile.TemporaryDirectory() as td:
        RUN.OUT = pathlib.Path(td)
        with mock.patch.object(RUN.time, "strftime", return_value="20261115-120000"):
            RUN.log_attempt("q", {"model": "s", "version_pin": "s"}, None, "PENDING", 0.0,
                            attempt_id="deadbee1")
            try:
                RUN.log_attempt("q", {"model": "s", "version_pin": "s"}, None, "FAILED", 0.0,
                                attempt_id="deadbee1")
                return False  # 覆写被静默接受 = 闸没了
            except FileExistsError:
                return _attempt_rows(pathlib.Path(td))[0]["status"] == "PENDING"

def c_row_reconstructs_run():
    """一行重建运行条件：attempt_id/ts/status/prompt_digest 12hex/cells/sampling 参数全在。"""
    with tempfile.TemporaryDirectory() as td:
        RUN.OUT = pathlib.Path(td)
        RUN.log_attempt("q", {"model": "m1", "version_pin": "vp"}, None, "PENDING", 0.0,
                        prompt_digest="a" * 12, cells=FIVE, proto="anthropic",
                        temperature=0, max_tokens=32768, attempt_id="aaa11111")
        r = _attempt_rows(pathlib.Path(td))[0]
        return (len(r["attempt_id"].split("-")) == 3
                and r["status"] == "PENDING" and r["prompt_digest"] == "a" * 12
                and r["cells"] == FIVE and r["model"] == "m1"
                and r["temperature"] == 0 and r["max_tokens"] == 32768)


# ── D. 解析路径族（mock 整跑 main——邢 F1 回执验证脚本形态） ──────────

def _mocked_run(td, response_body=None, urlopen_side_effect=None, prompt_text=FIVE_GRID_PROMPT + "\n"):
    """在临时目录内整跑 main()：钉死时钟、注入钥匙/argv、拦截 urlopen。"""
    RUN.OUT = pathlib.Path(td)
    prompt_file = pathlib.Path(td) / "prompt.md"
    prompt_file.write_text(prompt_text, encoding="utf-8")
    with tempfile.TemporaryDirectory() as fake_results:
        RUN.OUT = pathlib.Path(fake_results)  # 台账另放，prompt 同目录不冲突
        with mock.patch.dict(RUN.os.environ, {"XS3_KEY": "x"}), \
             mock.patch.object(sys, "argv", ["r", "--arm", "qwen", "--prompt", str(prompt_file)]), \
             mock.patch.object(RUN.time, "strftime", return_value="20261115-130000"), \
             mock.patch("urllib.request.urlopen",
                        side_effect=urlopen_side_effect,
                        **({} if urlopen_side_effect else {})) as _u:
            if response_body is not None:
                resp = mock.MagicMock()
                resp.read.return_value = json.dumps(response_body).encode()
                resp.__enter__.return_value = resp
                _u.return_value = resp
                _u.side_effect = None
            try:
                RUN.main()
                exit_seen = None
            except SystemExit as e:
                exit_seen = e
            except KeyboardInterrupt:
                exit_seen = "KILL"
        rows = _attempt_rows(pathlib.Path(fake_results))
    return exit_seen, rows

def d_kill_window_pending():
    """强杀窗口：请求发出即 SIGKILL（urlopen 抛 KeyboardInterrupt）→ PENDING 行在案
    （RV25-02 验收第二分句——请求前铸造+先落行）。"""
    with tempfile.TemporaryDirectory() as td:
        def killed(*a, **k):
            raise KeyboardInterrupt
        exit_seen, rows = _mocked_run(td, urlopen_side_effect=killed)
        return (exit_seen == "KILL" and len(rows) == 1
                and rows[0]["status"] == "PENDING")

def d_parse_fail_lands_ledger():
    """畸形体（非 dict JSON）→ SystemExit 且 FAILED:parse 行入台账（RV25-03——
    解析失败零裸崩、零漏记；PENDING+FAILED 两行=一个生命周期两条记录）。
    断言顺序无关：两行文件名只差 uuid hex，sorted 序随机（首版探针的掷硬币病）。"""
    with tempfile.TemporaryDirectory() as td:
        exit_seen, rows = _mocked_run(td, response_body=[])
        statuses = {r["status"] for r in rows}
        return (isinstance(exit_seen, SystemExit) and len(rows) == 2
                and "PENDING" in statuses
                and any("parse" in s for s in statuses))

def d_nonstr_content_normalised():
    """非 str content（int）→ 归一为空 → INCOMPLETE 落行不裸崩（R8 M2——
    「EVERY parse path」声明当年漏掉的面，修复后不可再漏）。顺序无关断言同 D2。"""
    with tempfile.TemporaryDirectory() as td:
        exit_seen, rows = _mocked_run(
            td, response_body={"choices": [{"message": {"content": 12345}}]})
        statuses = [r["status"] for r in rows]
        return (exit_seen is None and len(rows) == 2
                and statuses.count("PENDING") == 1
                and statuses.count("INCOMPLETE") == 1)

def d_anthropic_blocks_branch():
    """R11-F1(M) 修复：anthropic content-blocks 分支（volcano 兼容层真形——两臂
    ARMS proto 即此形，未来实验实际走的路径）首覆盖：text 块抽出 → COMPLETE。
    断肢变异（type=="text" 漂移/抽空）下静默全 INCOMPLETE 不崩——本案红。"""
    with tempfile.TemporaryDirectory() as td:
        exit_seen, rows = _mocked_run(
            td, response_body={"content": [{"type": "text", "text": COMPLETE_FIVE}]})
        statuses = [r["status"] for r in rows]
        return (exit_seen is None and len(rows) == 2
                and statuses.count("PENDING") == 1
                and statuses.count("COMPLETE") == 1)

def d_choices_branch_positive():
    """R11-r3-M1 修复：choices 分支正控（OpenAI 形合法回件→COMPLETE）——双解析
    分支×终态矩阵补最后一格。键漂移变异（msg.get("content")→"cntent"）下
    D3（空 content→INCOMPLETE）症状同形零分辨力，本案是其唯一杀器。"""
    with tempfile.TemporaryDirectory() as td:
        exit_seen, rows = _mocked_run(
            td, response_body={"choices": [{"message": {"content": COMPLETE_FIVE}}]})
        statuses = [r["status"] for r in rows]
        return (exit_seen is None and len(rows) == 2
                and statuses.count("PENDING") == 1
                and statuses.count("COMPLETE") == 1)

def d_request_body_matches_arms():
    """R11-r4-M1 修复：请求体 sampling 面首钉——body 键集穷尽（恰四键）+各键==ARMS 单源值+prompt
    原文（v1.1「body 与台账同源」契约的 body 侧；台账侧=C3）。temperature=0 是
    差分对照控制变量：body 漂移则 attempt 行为错误 body 作证且无产物可对勘
    （raw 只存响应不存请求）。闭包捕获真 Request 对象，请求前捕获即杀。"""
    with tempfile.TemporaryDirectory() as td:
        captured = {}
        def capture_then_kill(req, timeout=None):
            captured["body"] = json.loads(req.data)
            raise KeyboardInterrupt
        exit_seen, rows = _mocked_run(td, urlopen_side_effect=capture_then_kill)
        body = captured.get("body", {})
        return (exit_seen == "KILL" and len(rows) == 1
                and rows[0]["status"] == "PENDING"
                and set(body) == {"model", "temperature", "max_tokens", "messages"}
                and body.get("model") == "qwen3.8-flash"
                and body.get("temperature") == 0
                and body.get("max_tokens") == 32768
                and body.get("messages") == [{"role": "user", "content": FIVE_GRID_PROMPT + "\n"}])

def d_http_failure_lands_ledger():
    """R11-M1(M) 修复：HTTP 失败终态路径（except Exception→FAILED 落行+exit）首
    覆盖——SUT 契约自列四终态（parse-fail/HTTP-fail/COMPLETE/INCOMPLETE）至此全
    覆。网关抖动=档案在案高频事件族；该路径断掉则 HTTP 失败只留悬挂 PENDING，
    与强杀窗口不可区分（RV25-02 立法要消除的审计链歧义）。
    并钉 main→log_attempt 接线缝（m1）：FAILED 行须带真 prompt_digest 与派生
    cells——期望摘要为独立钉字面量（判别钉：参考不由 SUT 自产）。"""
    with tempfile.TemporaryDirectory() as td:
        import urllib.error
        def http_err(*a, **k):
            raise urllib.error.URLError("gateway 502")
        exit_seen, rows = _mocked_run(td, urlopen_side_effect=http_err)
        statuses = [r["status"] for r in rows]
        failed = [r for r in rows if r["status"].startswith("FAILED:")]
        return (isinstance(exit_seen, SystemExit) and len(rows) == 2
                and statuses.count("PENDING") == 1 and len(failed) == 1
                and failed[0]["prompt_digest"] == "7c89b1a1548b"
                and failed[0]["cells"] == FIVE)


CASES = [
    ("A1 empty_expected_set(邢案1)", a_empty_expected_set),
    ("A2 no_basis(邢案2)", a_no_basis),
    ("A3 contradiction_dup(邢案3)", a_contradiction_duplicate_cell),
    ("A4 missing_cell", a_missing_cell),
    ("A5 empty_verdict", a_empty_verdict),
    ("B1 derivation 5/6/none", b_derivation_shapes),
    ("E1 positive five(正控)", e_positive_control_five),
    ("E2 positive six(正控·count-drift)", e_positive_control_six),
    ("E3 ascii_bars(正控)", e_ascii_bars),
    ("E4 prompt_digest 钉", e_prompt_digest_pinned),
    ("C1 two_attempts_distinct(邢案4)", c_two_attempts_distinct),
    ("C2 append_only 拒覆写", c_append_only_refuses_overwrite),
    ("C3 row_reconstructs", c_row_reconstructs_run),
    ("D1 kill_window PENDING", d_kill_window_pending),
    ("D2 parse_fail 入台账", d_parse_fail_lands_ledger),
    ("D3 nonstr_content 归一", d_nonstr_content_normalised),
    ("D4 anthropic_blocks 真形", d_anthropic_blocks_branch),
    ("D5 http_failure 入台账", d_http_failure_lands_ledger),
    ("D6 choices 分支正控", d_choices_branch_positive),
    ("D7 请求体 sampling 钉", d_request_body_matches_arms),
]

if __name__ == "__main__":
    fails = 0
    for name, fn in CASES:
        try:
            ok = bool(fn())
        except Exception as e:  # 探针自身崩溃=红（fail-loud，不吞）
            print(f"FAIL {name}  [probe crashed: {type(e).__name__}: {e}]")
            fails += 1
            continue
        print(("PASS " if ok else "FAIL ") + name)
        fails += 0 if ok else 1
    print(f"{'='*40}\n{len(CASES)-fails}/{len(CASES)} PASS")
    sys.exit(0 if fails == 0 else 1)
