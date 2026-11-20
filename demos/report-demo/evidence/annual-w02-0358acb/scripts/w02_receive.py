"""W02 plugin receipt checks on the annual candidate (0358acb), real WorkSwarm plugin page.
usage: w02_receive.py OLD_WAITING_TASK
"""
import hashlib, json, os, sys, time
from pathlib import Path
import requests
from playwright.sync_api import sync_playwright

private = Path(os.environ["private"]); cfg = json.loads((private / "config.json").read_text())
PW = cfg["user_password"]; H = {"Authorization": "Bearer " + cfg["tokens"]["operator"]}
OUT = Path(os.environ.get("W02_EVIDENCE_DIR", ".")); SHOTS = OUT / "shots"; SHOTS.mkdir(exist_ok=True)
OLD = sys.argv[1]
rec = {"source": "yougq/agent-hub-alpha 0358acbad0ce1b0ce73772f31eda845057ba4dc5", "old_task": OLD, "started_at": time.time()}

def get(path):
    r = requests.get(cfg["public_url"] + path, headers=H, timeout=15); r.raise_for_status(); return r.json()

def save():
    (OUT / "evidence" / "w02-receive.json").write_text(json.dumps(rec, ensure_ascii=False, indent=1))

INIT = r"""
(() => {
  if (window.__ws) return;
  const desc = Object.getOwnPropertyDescriptor(WebSocket.prototype, 'onmessage');
  const sent = {};
  window.__ws = {replies: [], sends: []};
  const origSend = WebSocket.prototype.send;
  WebSocket.prototype.send = function (d) {
    try { const m = JSON.parse(d); if (m.type === 'req') { const p = Object.assign({}, m.params || {}); delete p.password;
      sent[m.id] = {method: m.method, at: Date.now(), params: p}; window.__ws.sends.push({id: m.id, method: m.method, at: Date.now(), params: p}); } } catch (e) {}
    return origSend.call(this, d);
  };
  Object.defineProperty(WebSocket.prototype, 'onmessage', {configurable: true,
    get() { return this.__u || null; },
    set(fn) { this.__u = fn; desc.set.call(this, (e) => {
      try { const m = JSON.parse(String(e.data)); const s = m && m.id && sent[m.id];
        if (s) { const p = m.payload || {};
          window.__ws.replies.push({id: m.id, method: s.method, at: Date.now(), ok: m.ok, code: m.code || null, error: m.error || null,
            task: p.plan || null, rev: p.rev || null, run_status: p.run && p.run.status || null, task_status: p.task_status || null,
            receipt: (s.method.endsWith('.graph') || s.method.endsWith('.plans')) ? null : JSON.stringify(p).slice(0, 600)}); } } catch (x) {}
      return fn.call(this, e); }); } });
})();
"""
RAW = """(args) => new Promise((res) => {
  const ws = new WebSocket((location.protocol === 'https:' ? 'wss' : 'ws') + '://' + location.host + '/ws' + (args[2] || ''));
  ws.onopen = () => ws.send(JSON.stringify({type: 'req', id: 'raw' + Date.now(), method: args[0], params: args[1]}));
  ws.addEventListener('message', (e) => { const m = JSON.parse(e.data); if (m.type === 'res' && String(m.id).startsWith('raw')) { ws.close(); res({ok: m.ok, code: m.code || null, error: m.error || null, payload: m.ok ? JSON.stringify(m.payload).slice(0, 800) : null}); } });
  setTimeout(() => res({timeout: true}), 25000);
})"""

def plugin_frame(page):
    for f in page.frames:
        if "/api/application-plugins/" in f.url: return f
    return None

def wsq(f):
    u = f.evaluate("() => { const q = new URLSearchParams(); const g = k => { try { return (localStorage.getItem(k) || '').trim(); } catch (e) { return ''; } };"
                   " if (g('jiuwenswarm:user_id')) q.set('user_id', g('jiuwenswarm:user_id')); if (g('jiuwenswarm.auth.sessionId')) q.set('auth_session', g('jiuwenswarm.auth.sessionId'));"
                   " const s = q.toString(); return s ? '?' + s : ''; }")
    return u

def raw(f, method, params):
    p = dict(params); 
    return f.evaluate(RAW, [method, p, wsq(f)])

def pick(fl, task):
    fl.locator(".wb-item", has_text=task).first.click()

with sync_playwright() as pw:
    browser = pw.chromium.launch(headless=True, executable_path=str(Path.home() / ".cache/ms-playwright/chromium-1243/chrome-linux64/chrome"))
    ctx = browser.new_context(viewport={"width": 1500, "height": 1000}, accept_downloads=True)
    ctx.add_init_script(INIT)
    page = ctx.new_page()
    page.goto("http://127.0.0.1:5173/chat/new"); page.wait_for_timeout(2500)
    page.get_by_role("button", name="编排台", exact=True).click(); page.wait_for_timeout(3000)
    fl = page.frame_locator("iframe"); f = plugin_frame(page)
    rec["plugin_frame_url"] = f.url if f else None

    # ---- (2) BLOCKED reason / resume entry on the old task bound to the previous model binding
    old_t = get("/v1/tasks/" + OLD)
    rec["old_before"] = {"status": old_t["status"], "binding": old_t["model_binding_digest"], "current_binding": cfg["model_binding_digest"],
                         "generation": old_t.get("generation"), "nodes": {k: [v.get("status"), v.get("reason")] for k, v in (old_t.get("nodes") or {}).items()}}
    pick(fl, OLD); page.wait_for_timeout(2500)
    rec["old_ui_before"] = {"resume_control": fl.locator(".wb-resume").count(), "blocked_banner": fl.locator(".wb-blocked").count(),
                            "head": fl.locator(".cg-head").first.inner_text()[:200]}
    page.screenshot(path=str(SHOTS / "w02-02-old-before.png"))
    if rec["old_ui_before"]["resume_control"]:
        fl.locator(".wb-resume input[type=password]").fill(PW)
        fl.locator(".wb-resume").evaluate("fm => fm.requestSubmit()")
        second = raw(f, "plugin.credit_workbench.resume", {"run": OLD, "password": PW})
        rec["old_second_client_resume"] = second
        page.wait_for_timeout(1500)
        rec["old_notice"] = fl.locator(".wb-notice").first.inner_text() if fl.locator(".wb-notice").count() else None
    t0 = time.time(); seen = []
    while time.time() - t0 < 90:
        st = get("/v1/tasks/" + OLD)["status"]
        if not seen or seen[-1] != st: seen.append(st)
        if st in ("BLOCKED", "FAILED", "DELIVERED") or (len(seen) > 1 and st in ("WAITING_INPUT",)): break
        time.sleep(2)
    page.wait_for_timeout(3000)
    old_t = get("/v1/tasks/" + OLD)
    rec["old_status_sequence"] = seen
    rec["old_after"] = {"status": old_t["status"], "generation": old_t.get("generation"),
                        "nodes": {k: [v.get("status"), v.get("reason")] for k, v in (old_t.get("nodes") or {}).items()}}
    pick(fl, OLD); page.wait_for_timeout(2500)
    rec["old_ui_after"] = {"resume_control": fl.locator(".wb-resume").count(), "blocked_banner": fl.locator(".wb-blocked").first.inner_text() if fl.locator(".wb-blocked").count() else None,
                           "list_chip": fl.locator(".wb-item", has_text=OLD).first.inner_text()[:120], "head": fl.locator(".cg-head").first.inner_text()[:200]}
    page.screenshot(path=str(SHOTS / "w02-02-old-after.png"))
    save()

    # ---- new candidate through a real dialogue
    prior = {r["plan"] for r in get("/v1/workbench/plans")["plans"]}
    chat = ctx.new_page(); chat.goto("http://127.0.0.1:5173/chat/new"); chat.wait_for_timeout(1500)
    for label in ("专家", "我的专家", "年报协作规划助手", "使用"):
        chat.get_by_text(label, exact=True).first.click(); chat.wait_for_timeout(500)
    ed = chat.get_by_test_id("chat-panel-input")
    ed.fill("请使用年报协作技能，基于固定招商银行2025年报快照，生成本集团2025对比2024的经营与资产质量简报计划，覆盖八项指标及来源。独立资料提取、经营、资产质量专家工作后交写作专家，包含程序检查和最多一次定向修订。先真实读取目录和调用能力选择工具，再生成带选择记录的新候选图，交我确认。本次为新任务，请不要使用历史任务图。")
    ed.press("Enter"); rec["dialogue_submitted_at"] = time.time()
    task = None; dl = time.time() + 360
    while time.time() < dl and not task:
        for r in get("/v1/workbench/plans")["plans"]:
            if r["plan"] in prior: continue
            c = get("/v1/tasks/" + r["plan"])
            if c.get("model_binding_digest") == cfg["model_binding_digest"] and c["graph"].get("selection"): task = c; break
        time.sleep(1.5)
    if not task:
        rec["error"] = "no fresh candidate"; save(); sys.exit(2)
    T = task["task_id"]; rec["task"] = T; rec["candidate_nodes"] = len(task["graph"]["nodes"])
    rec["zero_calls_before_confirmation"] = task["status"] == "AWAITING_APPROVAL" and get(f"/v1/tasks/{T}/calls") == []
    page.bring_to_front(); page.wait_for_timeout(5500)
    pick(fl, T); page.wait_for_timeout(2500)
    wb = get("/v1/workbench/tasks/" + T)
    # ---- (3) admission: UI submit, then a duplicate admission through a second plugin client
    fl.locator(".cg-head button").click()
    fl.locator(".cg-gate input[type=password]").fill(PW)
    fl.locator(".cg-gate button[type=submit]").click()
    page.wait_for_timeout(1500)
    rec["admit_dup_same_hash"] = raw(f, "plugin.credit_workbench.approve", {"plan": T, "version": 1, "hash": wb["hash"], "password": PW})
    rec["admit_after"] = {k: get("/v1/tasks/" + T).get(k) for k in ("status", "approved_by", "generation")}
    page.screenshot(path=str(SHOTS / "w02-03-admitted.png"))
    # ---- run; (1) generation/rev observed by the plugin
    dl = time.time() + 900; seq = []
    while time.time() < dl:
        st = get("/v1/tasks/" + T)["status"]
        if not seq or seq[-1] != st: seq.append(st)
        if st in ("AWAITING_ACCEPTANCE", "COMPUTE_COMPLETED", "FAILED", "NEEDS_REVIEW", "BLOCKED", "WAITING_INPUT"): break
        time.sleep(1)
    rec["run_status_sequence"] = seq
    page.wait_for_timeout(3000)
    rec["calls_after_run"] = [{k: c.get(k) for k in ("agent_id", "state", "remote_task_id", "conflicted")} for c in get(f"/v1/tasks/{T}/calls")]
    wb = get("/v1/workbench/tasks/" + T); rec["human_review"] = wb.get("human_review")
    page.screenshot(path=str(SHOTS / "w02-01-run-settled.png"))
    if seq[-1] in ("AWAITING_ACCEPTANCE", "COMPUTE_COMPLETED"):
        qualified = [r for r in wb["reports"] if r["can_approve"]]
        art = qualified[-1]["artifact_id"]; rec["artifact"] = art; rec["projection_report_digest"] = qualified[-1]["digest"]
        pick(fl, T); page.wait_for_timeout(1500)
        det = fl.locator(f'details[data-artifact="{art}"]'); det.locator("summary").click()
        fl.locator("input[type=password]").last.fill(PW); det.get_by_role("button", name="批准此报告", exact=True).click(); page.wait_for_timeout(1500)
        rec["approve_report_dup"] = raw(f, "plugin.credit_workbench.command", {"task": T, "action": "approve_report", "artifact": art, "password": PW})
        fl.locator("input[type=password]").last.fill(PW); det.get_by_role("button", name="交付此报告", exact=True).click()
        dl = time.time() + 30
        while time.time() < dl and get("/v1/tasks/" + T)["status"] != "DELIVERED": time.sleep(0.5)
        sends = f.evaluate("window.__ws.sends")
        deliver = [s for s in sends if s["method"].endswith(".command") and s["params"].get("action") == "deliver"]
        cmd_id = deliver[-1]["params"].get("command_id") if deliver else None
        rec["deliver_ui_command_id_present"] = bool(cmd_id)
        rec["deliver_dup_same_command_id"] = raw(f, "plugin.credit_workbench.command", {"task": T, "action": "deliver", "artifact": art, "command_id": cmd_id, "password": PW}) if cmd_id else None
        rec["deliver_other_command_id"] = raw(f, "plugin.credit_workbench.command", {"task": T, "action": "deliver", "artifact": art, "command_id": "w02-dup-" + str(int(time.time())), "password": PW})
        t = get("/v1/tasks/" + T)
        rec["delivered"] = {"status": t["status"], "delivery_receipts": t.get("delivery_receipts"), "report_approvals": t.get("report_approvals")}
        # ---- (4) report digest: plugin download vs projection vs formal
        fl.locator("input[type=password]").last.fill(PW)
        with page.expect_download() as pend:
            det.get_by_role("button", name="下载正式 JSON", exact=True).click()
        raw_bytes = Path(pend.value.path()).read_bytes()
        rec["download_digest"] = "sha256:" + hashlib.sha256(raw_bytes).hexdigest()
        rec["download_equals_projection"] = rec["download_digest"] == rec["projection_report_digest"]
        rec["verify"] = {k: v for k, v in get(f"/v1/tasks/{T}/verify").items() if not isinstance(v, (list, dict))}
        page.screenshot(path=str(SHOTS / "w02-04-delivered.png"))
    replies = f.evaluate("window.__ws.replies")
    g = [r for r in replies if (r["method"] or "").endswith(".graph") and r["task"] == T and r["rev"] is not None]
    revs = [int(r["rev"]) for r in g]
    rec["graph_replies_for_task"] = len(g)
    rec["rev_first_last"] = [revs[0], revs[-1]] if revs else None
    rec["rev_non_decreasing_in_arrival_order"] = all(b >= a for a, b in zip(revs, revs[1:]))
    rec["final_generation"] = get("/v1/tasks/" + T).get("generation")
    rec["all_plugin_rpc_replies"] = [{k: r[k] for k in ("method", "ok", "code", "error", "receipt")} for r in replies if not (r["method"] or "").endswith((".graph", ".plans"))]
    rec["finished_at"] = time.time()
    save()
    print(json.dumps({k: rec.get(k) for k in ("task", "run_status_sequence", "old_status_sequence", "rev_first_last", "rev_non_decreasing_in_arrival_order", "download_equals_projection")}, ensure_ascii=False))
    browser.close()
