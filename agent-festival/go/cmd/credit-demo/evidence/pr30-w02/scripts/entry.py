import sys
from pw import *
PLAN, V, TAG = sys.argv[1], sys.argv[2], sys.argv[3]
rec = {"plan": PLAN, "version": V}
def calls(run_id):
    run = state()["runs"].get(run_id, {})
    return {k: {f: (s.get("remote_call") or {}).get(f) for f in ("request_id", "remote_task_id", "status", "model_events")} for k, s in run.get("steps", {}).items() if s.get("remote_call")}
with sync_playwright() as p:
    ctx, page = launch(p)
    f = open_workbench(page)
    f.locator(".wb-item", has_text=f"{PLAN} v{V}").first.click(); page.wait_for_timeout(3000)
    rec["selected"] = f.locator(".wb-item.on b").inner_text()
    rec["chip_before"] = f.locator(".cg-head .chip:not(.soft)").first.inner_text()
    rec["nodes"] = f.evaluate("[...document.querySelectorAll(\".react-flow__node\")].map(e=>e.getAttribute(\"data-id\"))")
    shot(page, f"{TAG}-10-workbench-graph")
    summ = f.locator(".cg-bus-list summary")
    rec["bus_overview_summary"] = summ.first.inner_text() if summ.count() else None
    if summ.count(): summ.first.click(); page.wait_for_timeout(1200); shot(page, f"{TAG}-11-bus-list")
    rec["bus_overview_text"] = f.locator(".cg-bus-overview").first.inner_text()[:1500] if f.locator(".cg-bus-overview").count() else None
    node = f.locator(".react-flow__node[data-id=\"financial\"]")
    if node.count():
        node.first.click(); page.wait_for_timeout(1500); shot(page, f"{TAG}-12-node-detail")
        d = f.locator(".cg-detail")
        rec["node_detail_text"] = d.first.inner_text()[:1500] if d.count() else None
        x = f.locator(".cg-detail .x")
        if x.count(): x.first.click(); page.wait_for_timeout(500)
    # approve: wrong password first
    st0 = state(); rec["rev_before_wrong"] = st0["rev"]; rec["runs_before"] = len(st0.get("runs", {}))
    f.get_by_role("button", name="批准", exact=True).click(); page.wait_for_timeout(600)
    f.locator(".cg-gate input[type=password]").fill("wrong-" + PW[-3:])
    f.get_by_role("button", name="批准执行").click(); page.wait_for_timeout(2500)
    rec["wrong_msg"] = f.locator(".cg-gate .msg").inner_text() if f.locator(".cg-gate .msg").count() else None
    st1 = state(); rec["rev_after_wrong"] = st1["rev"]; rec["runs_after_wrong"] = len(st1.get("runs", {}))
    shot(page, f"{TAG}-13-approve-wrong")
    f.locator(".cg-gate input[type=password]").fill(PW)
    f.get_by_role("button", name="批准执行").click(); page.wait_for_timeout(2500)
    shot(page, f"{TAG}-14-approve-ok")
    st2 = state(); rec["rev_after_ok"] = st2["rev"]
    newruns = [r for r in st2["runs"] if r not in st0.get("runs", {})]
    rec["run"] = newruns[0] if newruns else None
    rec["approved_via"] = st2["runs"][rec["run"]].get("approved_via") if rec["run"] else None
    t0 = time.time(); i = 0; samples = []
    while time.time() - t0 < 600:
        page.wait_for_timeout(5000); i += 1
        chip = f.locator(".cg-head .chip:not(.soft)").first.inner_text()
        st = f.evaluate("Object.fromEntries([...document.querySelectorAll(\".react-flow__node\")].map(e=>{const c=e.querySelector(\".cn, .cb\");return [e.getAttribute(\"data-id\"), c?([...c.classList].find(x=>x.startsWith(\"s-\"))||\"\").slice(2):\"\"]}))")
        samples.append({"t": round(time.time() - t0), "chip": chip, "nodes": st})
        if i % 6 == 0: shot(page, f"{TAG}-2{i//6:02d}-run")
        run = state()["runs"][rec["run"]]
        if run["status"] in ("done", "failed", "waiting") and ("验证" in chip or run["status"] != "done"): break
    page.wait_for_timeout(3000); shot(page, f"{TAG}-30-final")
    run = state()["runs"][rec["run"]]
    rec.update(final_status=run["status"], final_chip=f.locator(".cg-head .chip:not(.soft)").first.inner_text(), final_text=(run.get("final") or "")[:300], rev_final=state()["rev"], calls=calls(rec["run"]), samples=samples)
    print(json.dumps(rec, ensure_ascii=False, indent=1))
    ctx.close()
