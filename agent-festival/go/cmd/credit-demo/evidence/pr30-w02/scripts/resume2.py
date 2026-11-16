"""Resume an existing waiting run from the 编排台 plugin page and record what happened."""
import subprocess
import sys
from pw import *

RUN, PLAN, V, TAG = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]


def dx(*a):
    return subprocess.run(["docker", "exec", "pr30-w02-fault-proxy-1", *a], check=True, capture_output=True, text=True).stdout


def events():
    raw = dx("sh", "-c", "cat /tmp/bus-fault-events.jsonl 2>/dev/null || true")
    return [json.loads(l) for l in raw.splitlines() if l.strip()]


def calls(run_id):
    run = state()["runs"].get(run_id, {})
    out = {}
    for k, s in run.get("steps", {}).items():
        c = s.get("remote_call")
        if c:
            out[k] = {f: c.get(f) for f in ("request_id", "remote_task_id", "status", "artifact_id", "model_events")}
            out[k]["step_status"] = s.get("status")
    return out


PATCH = """() => {
  window.__resumeSends = 0;
  const o = WebSocket.prototype.send;
  WebSocket.prototype.send = function (d) {
    if (String(d).indexOf("plugin.credit_workbench.resume") >= 0) window.__resumeSends++;
    return o.call(this, d);
  };
}"""
DOUBLE = """() => { const fm = document.querySelector(".wb-resume"); fm.requestSubmit(); fm.requestSubmit(); }"""
SECOND = """(args) => new Promise((res) => {
  const ws = new WebSocket((location.protocol === "https:" ? "wss" : "ws") + "://" + location.host + "/ws");
  ws.onopen = () => ws.send(JSON.stringify({type: "req", id: "second1", method: "plugin.credit_workbench.resume", params: {run: args[0], password: args[1]}}));
  ws.onmessage = (e) => { const m = JSON.parse(e.data); if (m.type === "res" && m.id === "second1") { ws.close(); res({ok: m.ok, code: m.code, error: m.error}); } };
  setTimeout(() => res({timeout: true}), 20000);
})"""

rec = {"run": RUN, "plan": PLAN, "version": V}
ev_start = len(events())
with sync_playwright() as p:
    ctx, page = launch(p)
    f = open_workbench(page)
    f.locator(".wb-item", has_text="%s v%s" % (PLAN, V)).first.click()
    page.wait_for_timeout(3000)
    run = state()["runs"][RUN]
    rec["status_at_open"] = run["status"]
    rec["resume_control_visible"] = f.locator(".wb-resume").count() > 0
    rec["resume_control_text"] = f.locator(".wb-resume").first.inner_text() if rec["resume_control_visible"] else None
    rec["list_chip"] = f.locator(".wb-item.on .chip").first.inner_text()
    rec["head_chip"] = f.locator(".cg-head .chip:not(.soft)").first.inner_text()
    shot(page, f"{TAG}-50-waiting")
    before = calls(RUN)
    rec["calls_before_resume"] = before
    f.evaluate(PATCH)

    # 1) wrong password
    s1, e1 = state(), len(events())
    f.locator(".wb-resume input[type=password]").fill("wrong-" + PW[-3:])
    f.locator(".wb-resume button[type=submit]").click()
    page.wait_for_timeout(3000)
    status_el = f.locator(".wb-resume [role=status]")
    rec["wrong_msg"] = status_el.inner_text() if status_el.count() else None
    s2, e2 = state(), len(events())
    rec["wrong"] = {"rev_before": s1["rev"], "rev_after": s2["rev"], "status_after": s2["runs"][RUN]["status"],
                    "transport_events_delta": e2 - e1, "rpc_sends": f.evaluate("window.__resumeSends"),
                    "password_cleared": f.locator(".wb-resume input[type=password]").input_value() == ""}
    shot(page, f"{TAG}-51-resume-wrong")

    # 2) correct password, submitted twice in the same tick (double click)
    f.locator(".wb-resume input[type=password]").fill(PW)
    s3 = state()
    f.evaluate("() => { window.__resumeSends = 0; }")
    f.evaluate(DOUBLE)
    page.wait_for_timeout(2500)
    rec["ok_msg"] = status_el.inner_text() if status_el.count() else None
    rec["rpc_sends_after_double_submit"] = f.evaluate("window.__resumeSends")
    s4 = state()
    rec["ok"] = {"rev_before": s3["rev"], "rev_after": s4["rev"], "status_after": s4["runs"][RUN]["status"]}
    shot(page, f"{TAG}-52-resume-ok")

    # 3) a later second resume of the same run through the plugin RPC
    rec["second_resume"] = f.evaluate(SECOND, [RUN, PW])
    s5 = state()
    rec["after_second"] = {"rev": s5["rev"], "status": s5["runs"][RUN]["status"]}

    t0 = time.time()
    while time.time() - t0 < 480:
        page.wait_for_timeout(4000)
        if state()["runs"][RUN]["status"] in ("done", "failed", "waiting"):
            break
    page.wait_for_timeout(10000)
    shot(page, f"{TAG}-53-final")
    run = state()["runs"][RUN]
    after = calls(RUN)
    rec["calls_after"] = after
    rec["final_status"] = run["status"]
    rec["final_chip"] = f.locator(".cg-head .chip:not(.soft)").first.inner_text()
    rec["resume_control_after"] = f.locator(".wb-resume").count() > 0
    ev = events()
    ids = {c["request_id"] for c in after.values()}
    tasks = {c["remote_task_id"] for c in after.values() if c["remote_task_id"]}
    rec["transport_events_all"] = [e for e in ev if e.get("id") in ids or e.get("task_id") in tasks]
    rec["sendmessage_per_request"] = {i: sum(1 for e in ev if e.get("method") == "SendMessage" and e.get("id") == i) for i in ids}
    rec["remote_nodes"] = len(after)
    rec["same_request_ids"] = all(after[k]["request_id"] == v["request_id"] for k, v in before.items())
    rec["same_task_ids"] = all((not v["remote_task_id"]) or after[k]["remote_task_id"] == v["remote_task_id"] for k, v in before.items())
    unknown = [k for k, v in before.items() if v["step_status"] == "unknown"]
    rec["unknown_nodes_before"] = unknown
    rec["unknown_node_gettask_after_resume"] = [e for e in ev[ev_start:] if e.get("method") == "GetTask" and e.get("task_id") in {before[k]["remote_task_id"] for k in unknown}]
    print(json.dumps(rec, ensure_ascii=False, indent=1))
    ctx.close()
