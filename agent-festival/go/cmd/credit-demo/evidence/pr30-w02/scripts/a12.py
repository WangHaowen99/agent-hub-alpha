"""A12 on the plugin path of the PR #30 stack (WorkSwarm 1142e5fb3 + 编排台 plugin).

Approve a freshly proposed plan in 编排台, then during the run apply disturbances:
  hold@T   hold the next graph reply that carries a run, release it 8 s later (late reply)
  reload@T reload the WorkSwarm page and reopen 编排台
  restart@T docker restart the WorkSwarm container (its Gateway)
  offline:N@T browser offline for N seconds (CDP)
  godown@T / goup@T stop / start the Go orchestration service
Every 500 ms record what the page shows; afterwards check that no node went
backwards, the progress count never fell, and the view came back by itself.
"""
import os
import re
import subprocess
import sys
from pw import *

os.environ["BUS_TEST_BASE"] = "http://127.0.0.1:5291"
os.environ["BUS_TEST_PROJECT"] = "pr30-w02"
sys.path.insert(0, os.environ["CREDIT_DEMO_DEPLOY"])  # agent-festival/go/cmd/credit-demo/deploy
import smoke_bus  # noqa: E402
assert smoke_bus.BASE == "http://127.0.0.1:5291", smoke_bus.BASE

TAG = sys.argv[1]
PLAN_SPEC = sys.argv[2]
RANK = {"plan": 0, "pending": 0, "": 0, "running": 1, "waiting": 1, "unknown": 2, "done": 3, "reused": 3, "failed": 3, "skipped": 3}
SAMPLE = """() => ({
  selected: document.querySelector(".wb-item.on b")?.textContent ?? "",
  chip: document.querySelector(".cg-head .chip:not(.soft)")?.textContent ?? "",
  banner: document.querySelector(".wb-err")?.textContent ?? "",
  graph: !!document.querySelector(".cg"),
  nodes: Object.fromEntries([...document.querySelectorAll(".react-flow__node")].map((el) => {
    const c = el.querySelector(".cn, .cb");
    const s = c ? ([...c.classList].find((x) => x.startsWith("s-")) ?? "").slice(2) : "";
    return [el.getAttribute("data-id"), s];
  }).filter(([, s]) => s)),
})"""

planner = smoke_bus.Planner()
plan = planner.tool("propose_plan", {"case_id": "C006", "goal": f"W02 A12 插件路径 {TAG} {int(time.time())}，仅生成模拟授信报告，不提交银行操作", "agents": smoke_bus.TEAM})
PLAN, V = plan["plan_id"], plan["version"]

hold = {"armed": False, "held": [], "released": [], "delivered_revs": []}


def on_ws(ws):
    server = ws.connect_to_server()

    def from_server(message):
        try:
            m = json.loads(message)
        except Exception:
            ws.send(message)
            return
        payload = m.get("payload") if isinstance(m, dict) else None
        rev = payload.get("rev") if isinstance(payload, dict) else None
        has_run = isinstance(payload, dict) and isinstance(payload.get("run"), dict)
        if rev is not None and has_run:
            if hold["armed"] and not hold["held"]:
                hold["held"].append({"ws": ws, "message": message, "rev": rev, "t": time.time(),
                                     "run_status": payload["run"].get("status")})
                hold["armed"] = False
                return
            hold["delivered_revs"].append(rev)
        ws.send(message)

    server.on_message(from_server)


def sh(cmd):
    return subprocess.run(["sh", "-c", cmd], capture_output=True, text=True).returncode


log, problems = [], []
t0 = time.time()
phase = "approve"
with sync_playwright() as p:
    ctx, page = launch(p)
    ctx.route_web_socket(re.compile(r".*/ws(\?.*)?$"), on_ws)
    cdp = ctx.new_cdp_session(page)
    cdp.send("Network.enable")

    def sample():
        f = frame(page)
        row = {"t": round(time.time() - t0, 1), "phase": phase}
        if not f:
            row["gap"] = "no frame"
        else:
            try:
                row.update(f.evaluate(SAMPLE))
            except Exception as e:
                row["gap"] = "eval: " + str(e)[:60]
        log.append(row)
        return row

    f = open_workbench(page)
    f.locator(".wb-item", has_text="%s v%s" % (PLAN, V)).first.click()
    page.wait_for_timeout(2500)
    f.get_by_role("button", name="批准", exact=True).click()
    page.wait_for_timeout(500)
    f.locator(".cg-gate input[type=password]").fill(PW)
    f.get_by_role("button", name="批准执行").click()
    phase = "run"
    t_approve = time.time() - t0
    steps = []
    for x in PLAN_SPEC.split(","):
        what, at = x.split("@")
        steps.append({"what": what, "at": float(at), "done": False})
    off_until = 0
    for _ in range(600):
        last = sample()
        t = last["t"] - t_approve
        nxt = next((d for d in steps if not d["done"] and t >= d["at"]), None)
        if nxt:
            nxt["done"] = True
            nxt["t"] = round(t, 1)
            what, _, arg = nxt["what"].partition(":")
            phase = what
            shot(page, f"{TAG}-d-{what}-before")
            if what == "hold":
                hold["armed"] = True
                hold["dur"] = float(arg or 8)
            elif what == "reload":
                open_workbench(page, reload=True)
                sample()
            elif what == "restart":
                nxt["rc"] = sh("docker restart pr30-w02-swarm-1")
            elif what == "offline":
                off_until = t + float(arg or 6)
                cdp.send("Network.emulateNetworkConditions", {"offline": True, "latency": 0, "downloadThroughput": -1, "uploadThroughput": -1})
            elif what == "godown":
                nxt["rc"] = sh("docker stop pr30-w02-credit-1")
            elif what == "goup":
                nxt["rc"] = sh("docker start pr30-w02-credit-1")
            page.wait_for_timeout(1500)
            sample()
            shot(page, f"{TAG}-d-{what}-during")
        for h in list(hold["held"]):
            if time.time() - h["t"] >= hold.get("dur", 8):
                newer = [r for r in hold["delivered_revs"] if r > h["rev"]]
                try:
                    h["ws"].send(h["message"])
                    h["delivered"] = True
                except Exception as e:
                    h["delivered"] = "error: " + str(e)[:80]
                hold["released"].append({"rev": h["rev"], "run_status": h["run_status"], "held_s": round(time.time() - h["t"], 1),
                                         "newer_revs_shown_meanwhile": len(newer), "max_rev_before_release": max(hold["delivered_revs"] or [0]),
                                         "delivered": h["delivered"], "release_t": round(time.time() - t0 - t_approve, 1)})
                hold["held"].remove(h)
                phase = "late-delivered"
        if off_until and t >= off_until:
            off_until = 0
            cdp.send("Network.emulateNetworkConditions", {"offline": False, "latency": 0, "downloadThroughput": -1, "uploadThroughput": -1})
        if re.search("验证通过|验证未通过|执行失败", last.get("chip") or "") and all(d["done"] for d in steps) and not hold["held"]:
            page.wait_for_timeout(1500)
            sample()
            break
        page.wait_for_timeout(500)
    shot(page, f"{TAG}-final")
    ctx.close()

best, max_done = {}, -1
for r in log:
    if not r.get("nodes"):
        continue
    for nid, s in r["nodes"].items():
        if nid in best and RANK.get(s, 0) < RANK.get(best[nid], 0):
            problems.append(f"{r['t']}s {r['phase']}: {nid} {best[nid]} -> {s}")
        if nid not in best or RANK.get(s, 0) > RANK.get(best[nid], 0):
            best[nid] = s
    m = re.search(r"执行中 (\d+)/\d+", r.get("chip") or "")
    done = 99 if re.search("已完成|验证", r.get("chip") or "") else (int(m.group(1)) if m else -1)
    if done >= 0:
        if done < max_done:
            problems.append(f"{r['t']}s {r['phase']}: progress {max_done} -> {done}")
        max_done = max(max_done, done)
after_reload = next((r for r in log if r["phase"] == "reload" and r.get("selected")), None)
banners = [f"{r['t']}s {r['phase']}: {r['banner']}" for r in log if r.get("banner")]
run_id = next((k for k, r in state()["runs"].items() if r.get("plan_id") == PLAN and r.get("plan_version", V) == V), None)
summary = {
    "plan": f"{PLAN} v{V}", "run": run_id, "disturbances": steps, "late_reply": hold["released"],
    "samples": len(log), "gaps": len([r for r in log if r.get("gap")]),
    "phases_with_frame_gaps": sorted({r["phase"] for r in log if r.get("gap")}),
    "selected_after_reload": after_reload and after_reload["selected"],
    "banner_samples": len(banners), "first_banner": banners[0] if banners else None, "last_banner": banners[-1] if banners else None,
    "graph_hidden_while_service_down": len([r for r in log if r["phase"] == "godown" and not r.get("gap") and not r.get("graph")]),
    "final_chip": log[-1].get("chip"), "final_nodes": log[-1].get("nodes"),
    "regressions": problems,
}
open(os.path.join(os.environ.get("EVIDENCE_DIR", "evidence"), f"a12-{TAG}-log.json"), "w").write(json.dumps(log, ensure_ascii=False, indent=1))
print(json.dumps(summary, ensure_ascii=False, indent=1))
