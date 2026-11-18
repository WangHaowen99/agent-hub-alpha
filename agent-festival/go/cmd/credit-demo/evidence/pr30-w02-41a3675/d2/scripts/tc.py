"""R-D (plugin part): two plugin clients (two tabs) resume the same waiting run at the same instant."""
import os, sys, json, time
D = "<repo>/agent-festival/go/cmd/credit-demo/deploy"
os.environ["BUS_TEST_BASE"] = "http://127.0.0.1:5291"; os.environ["BUS_TEST_PROJECT"] = "pr30-w02"
sys.path.insert(0, D); sys.path.insert(0, os.path.expanduser("<workdir>/w05")); sys.path.insert(0, os.path.expanduser("<workdir>"))
os.chdir(D)
from pw import *
import smoke_bus as sb, w05_rd_extra as w
rec = {"scenario": "two-plugin-clients", "source": "41a36755563ebeb442f2d93050c1d3430d510e80"}
w.set_fault("drop-query-once")
run_id, plan_id = sb.new_run("two-plugin-clients"); rec["run"] = run_id
t0 = time.time()
while time.time() - t0 < 600 and sb.read_run(run_id)["status"] not in ("waiting", "done", "failed"): time.sleep(1)
r = sb.read_run(run_id); rec["status_before"] = r["status"]; rec["leases_before"] = len(r.get("leases", []))
before = sb.call_snapshot(r)
with sync_playwright() as p:
    ctx, page1 = launch(p)
    page2 = ctx.new_page()
    frames = []
    for pg in (page1, page2):
        f = open_workbench(pg); f.wait_for_timeout(5000)
        f.locator(".wb-item", has_text=f"{plan_id} v").first.click(); f.wait_for_timeout(2500)
        f.locator(".wb-resume input[type=password]").fill(PW)
        frames.append(f)
    # fire both submits back to back without waiting for either reply
    for f in frames:
        f.evaluate("() => document.querySelector('.wb-resume').requestSubmit()")
    page1.wait_for_timeout(4000)
    rec["notices"] = [f.locator(".wb-notice").first.inner_text() if f.locator(".wb-notice").count() else None for f in frames]
    page1.screenshot(path=os.path.expanduser("<workdir>/shots/d2-tc-client1.png"))
    page2.screenshot(path=os.path.expanduser("<workdir>/shots/d2-tc-client2.png"))
    t0 = time.time()
    while time.time() - t0 < 600 and sb.read_run(run_id)["status"] not in ("done", "failed", "waiting"): time.sleep(2)
    r = sb.read_run(run_id); after = sb.call_snapshot(r)
    rec["final_status"] = r["status"]; rec["leases_after"] = len(r.get("leases", []))
    rec["kept_request_and_task"] = w.kept_identity(before, after, list(before))
    rec["send_counts"], rec["transport_events"] = w.transport_sends({c["request_id"] for c in after.values()}, {c["remote_task_id"] for c in after.values() if c["remote_task_id"]})
    open(os.path.expanduser("<workdir>/evidence/d2-tc.json"), "w").write(json.dumps(sb.redact(rec), ensure_ascii=False, indent=1))
    print(json.dumps({k: rec[k] for k in ("run", "status_before", "notices", "final_status", "leases_before", "leases_after", "kept_request_and_task", "send_counts")}, ensure_ascii=False))
    ctx.close()
