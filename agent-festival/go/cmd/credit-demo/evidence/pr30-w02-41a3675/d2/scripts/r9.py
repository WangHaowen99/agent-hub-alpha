"""Second plugin resume of R9 (stuck after report observation was rejected)."""
import os, sys, json, time
sys.path.insert(0, os.path.expanduser("<workdir>"))
from pw import *
RUN = sys.argv[1]
rec = {"run": RUN}
def run(): return state()["runs"][RUN]
r = run()
rec["before"] = {"status": r["status"], "steps": {k: s["status"] for k, s in r["steps"].items()},
                 "report_call": {k: (r["steps"]["report"].get("remote_call") or {}).get(k) for k in ("request_id", "remote_task_id", "status", "reason", "model_events")},
                 "report_gateway_exchanges": (r["steps"]["report"].get("remote_call") or {}).get("gateway_exchanges")}
with sync_playwright() as p:
    ctx, page = launch(p)
    f = open_workbench(page)
    f.locator(".wb-item", has_text="P9 v").first.click(); page.wait_for_timeout(3000)
    rec["control_visible"] = f.locator(".wb-resume").count() > 0
    shot(page, "d2-r9-before-second")
    if rec["control_visible"]:
        f.locator(".wb-resume input[type=password]").fill(PW)
        f.locator(".wb-resume button[type=submit]").click(); page.wait_for_timeout(3000)
        rec["notice"] = f.locator(".wb-notice").first.inner_text() if f.locator(".wb-notice").count() else None
    t0 = time.time()
    while time.time() - t0 < 300 and run()["status"] not in ("done", "failed", "waiting"):
        page.wait_for_timeout(3000)
    page.wait_for_timeout(5000)
    shot(page, "d2-r9-after-second")
    r = run()
    rec["after"] = {"status": r["status"], "steps": {k: s["status"] for k, s in r["steps"].items()},
                    "report_call": {k: (r["steps"]["report"].get("remote_call") or {}).get(k) for k in ("request_id", "remote_task_id", "status", "reason", "model_events")}}
    print(json.dumps(rec, ensure_ascii=False, indent=1))
    ctx.close()
