"""U-A on 41a3675 through the real WorkSwarm plugin: resume receipt survives the
waiting->queued/running unmount, is labelled with its run, and does not follow
the user to another task."""
import sys, subprocess
sys.path.insert(0, "<repo>/agent-festival/go/cmd/credit-demo/deploy")
from pw import *
os.environ["BUS_TEST_BASE"] = "http://127.0.0.1:5291"; os.environ["BUS_TEST_PROJECT"] = "pr30-w02"
import smoke_bus
assert smoke_bus.BASE == "http://127.0.0.1:5291", smoke_bus.BASE
TAG = sys.argv[1]
def dx(*a): return subprocess.run(["docker", "exec", "pr30-w02-fault-proxy-1", *a], check=True, capture_output=True, text=True).stdout
def events():
    raw = dx("sh", "-c", "cat /tmp/bus-fault-events.jsonl 2>/dev/null || true")
    return [json.loads(l) for l in raw.splitlines() if l.strip()]
def calls(run_id):
    run = state()["runs"].get(run_id, {})
    return {k: {f: (s.get("remote_call") or {}).get(f) for f in ("request_id", "remote_task_id", "status", "artifact_id", "model_events")} | {"step_status": s.get("status")} for k, s in run.get("steps", {}).items() if s.get("remote_call")}
def notice(f):
    n = f.locator(".wb-notice")
    return n.first.inner_text() if n.count() else None
rec = {"source": "41a36755563ebeb442f2d93050c1d3430d510e80"}
planner = smoke_bus.Planner()
plan = planner.tool("propose_plan", {"case_id": "C006", "goal": f"W02 U-A 恢复回执验证 {int(time.time())}，仅生成模拟授信报告，不提交银行操作", "agents": smoke_bus.TEAM})
rec["plan"], rec["version"] = plan["plan_id"], plan["version"]
ev0 = len(events())
dx("python", "-c", "from pathlib import Path; Path('/tmp/bus-fault-mode').write_text('drop-query-once')")
rec["fault_mode"] = "drop-query-once"
with sync_playwright() as p:
    ctx, page = launch(p)
    f = open_workbench(page)
    rec["plugin_has_notice_code"] = f.evaluate("() => [...document.scripts].map(s => s.src).join(' ')")
    f.locator(".wb-item", has_text="%s v%s" % (rec["plan"], rec["version"])).first.click(); page.wait_for_timeout(2500)
    st0 = state()
    f.get_by_role("button", name="批准", exact=True).click(); page.wait_for_timeout(600)
    f.locator(".cg-gate input[type=password]").fill(PW)
    f.get_by_role("button", name="批准执行").click(); page.wait_for_timeout(2500)
    run_id = [r for r in state()["runs"] if r not in st0["runs"]][0]; rec["run"] = run_id
    t0 = time.time()
    while time.time() - t0 < 420:
        page.wait_for_timeout(3000)
        if state()["runs"][run_id]["status"] in ("waiting", "done", "failed"): break
    page.wait_for_timeout(4000)
    rec["status_after_fault"] = state()["runs"][run_id]["status"]
    shot(page, f"{TAG}-60-waiting")
    rec["resume_control_visible"] = f.locator(".wb-resume").count() > 0
    before = calls(run_id); rec["calls_before_resume"] = before
    rec["completed_before"] = [k for k, v in before.items() if v["artifact_id"]]
    # wrong password: message now lives in the page-level notice
    s1, e1 = state(), len(events())
    f.locator(".wb-resume input[type=password]").fill("wrong-" + PW[-3:])
    f.locator(".wb-resume button[type=submit]").click(); page.wait_for_timeout(2500)
    s2, e2 = state(), len(events())
    rec["wrong"] = {"notice": notice(f), "rev_before": s1["rev"], "rev_after": s2["rev"], "status_after": s2["runs"][run_id]["status"], "transport_events_delta": e2 - e1}
    shot(page, f"{TAG}-61-resume-wrong")
    # correct password
    f.locator(".wb-resume input[type=password]").fill(PW)
    f.locator(".wb-resume button[type=submit]").click()
    samples = []
    t1 = time.time()
    while time.time() - t1 < 12:
        samples.append({"t": round(time.time() - t1, 1), "run_status": state()["runs"][run_id]["status"],
                        "control": f.locator(".wb-resume").count(), "notice": notice(f),
                        "head_chip": f.locator(".cg-head .chip:not(.soft)").first.inner_text()})
        page.wait_for_timeout(1000)
    rec["after_accept_samples"] = samples
    shot(page, f"{TAG}-62-receipt-kept")
    # switch to another task: receipt must not follow
    other = f.locator(".wb-item").filter(has_not_text="%s v%s" % (rec["plan"], rec["version"])).first
    rec["other_item"] = other.inner_text().split("\n")[0]
    other.click(); page.wait_for_timeout(3000)
    rec["after_switch"] = {"notice": notice(f), "head": f.locator(".cg-head").first.inner_text().split("\n")[0]}
    shot(page, f"{TAG}-63-switched")
    f.locator(".wb-item", has_text="%s v%s" % (rec["plan"], rec["version"])).first.click(); page.wait_for_timeout(3000)
    rec["after_switch_back"] = {"notice": notice(f)}
    t0 = time.time()
    while time.time() - t0 < 420:
        page.wait_for_timeout(4000)
        if state()["runs"][run_id]["status"] in ("done", "failed", "waiting"): break
    page.wait_for_timeout(6000); shot(page, f"{TAG}-64-final")
    after = calls(run_id); rec["calls_after"] = after
    rec["final_status"] = state()["runs"][run_id]["status"]
    rec["final_chip"] = f.locator(".cg-head .chip:not(.soft)").first.inner_text()
    ev = events()[ev0:]
    ids = {c["request_id"] for c in after.values()}
    rec["sendmessage_per_request"] = {i: sum(1 for e in ev if e.get("method") == "SendMessage" and e.get("id") == i) for i in ids}
    rec["same_request_ids"] = all(after[k]["request_id"] == v["request_id"] for k, v in before.items())
    rec["same_task_ids"] = all((not v["remote_task_id"]) or after[k]["remote_task_id"] == v["remote_task_id"] for k, v in before.items())
    print(json.dumps(rec, ensure_ascii=False, indent=1))
    ctx.close()
