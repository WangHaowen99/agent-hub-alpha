"""#59 three-tier labels in the real pr30-w02 (2dc971b + plugin 2c022ea):
two Agent Bus calls in flight, one GetTask dropped -> the task waits with one
call unknown and another still in flight. Screenshots before/after a reload,
and checks that viewing sends nothing and writes nothing."""
import os, sys, json, time, collections
D = "<repo>/agent-festival/go/cmd/credit-demo/deploy"
os.environ["BUS_TEST_BASE"] = "http://127.0.0.1:5291"; os.environ["BUS_TEST_PROJECT"] = "pr30-w02"
sys.path.insert(0, D); sys.path.insert(0, os.path.expanduser("<workdir>/w05")); sys.path.insert(0, os.path.expanduser("<workdir>"))
os.chdir(D)
from pw import *
import smoke_bus as sb, w05_rd_extra as w
EV = os.path.expanduser("<workdir>/evidence/e2dc")
OLD_UNKNOWN = sys.argv[1] if len(sys.argv) > 1 else None   # e.g. R18

def iso(t=None):
    t = time.time() if t is None else t
    return time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(t)) + ".%03dZ" % int((t % 1) * 1000)

def calls(run):
    out = {}
    for k, s in run["steps"].items():
        c = s.get("remote_call")
        if not c: continue
        ex = c.get("gateway_exchanges") or {}
        out[k] = {"step": s["status"], "call": c["status"], "request_id": c["request_id"], "task_id": c.get("remote_task_id"),
                  "agent_started_at": c.get("agent_started_at"), "intent_saved_at": c.get("created_at"),
                  "gateway": dict(collections.Counter(f'{e["method"]}/{e["outcome"]}' for e in ex.values()))}
    return out

def proxy_events():
    return [json.loads(l) for l in sb.docker("fault-proxy", "exec", "cat", "/tmp/bus-fault-events.jsonl").splitlines() if l.strip()]

def view(f, page, plan_id, tag):
    for _ in range(80):
        item = f.locator(".wb-item", has_text=f"{plan_id} v")
        if item.count(): item.first.click(); break
        f.wait_for_timeout(250)
    f.wait_for_timeout(3000)
    shot(page, tag)
    nodes = {}
    for i in range(f.locator(".react-flow__node").count()):
        n = f.locator(".react-flow__node").nth(i)
        txt = n.inner_text().strip()
        if not txt: continue
        notes = [n.locator(".cn-row.note").nth(j).get_attribute("title") for j in range(n.locator(".cn-row.note").count())]
        nodes[txt.splitlines()[0]] = {"text": txt.replace("\n", " | "), "note_titles": notes,
                                      "spinning": n.locator(".spin").count()}
    return nodes

rec = {"source": "2dc971b1fa9d592bb6d61b7f61c398856cae056f", "plugin": "2c022ea26a97630e827b05e68777e50a668e88ee"}
w.set_fault("pass")
t0 = time.time()
run_id, plan_id = sb.new_run("w02-59-inflight")
rec.update(run=run_id, plan=plan_id, created_at=iso(t0))
armed = None
while time.time() - t0 < 300:
    run = state()["runs"][run_id]
    c = calls(run)
    out = [k for k, v in c.items() if v["call"] == "waiting" and v["task_id"]]
    if len(out) >= 2:
        w.set_fault("drop-query-once"); armed = {"at": iso(), "calls": c}; break
    if run["status"] in ("done", "failed", "waiting"): break
    time.sleep(0.1)
rec["armed"] = armed
if not armed:
    rec["skipped"] = "never had two Agent Bus calls in flight"; print(json.dumps(rec, ensure_ascii=False)); sys.exit(1)
while time.time() - t0 < 600:
    run = state()["runs"][run_id]
    if run["status"] in ("waiting", "failed", "done"): break
    time.sleep(0.2)
rec["status_at_halt"] = run["status"]; rec["halted_at"] = iso()
rec["steps_at_halt"] = {k: s["status"] for k, s in run["steps"].items()}
rec["calls_at_halt"] = calls(run)
before_run = state()["runs"][run_id]
before_old = state()["runs"][OLD_UNKNOWN] if OLD_UNKNOWN else None
ev_before = proxy_events()
with sync_playwright() as p:
    ctx, page = launch(p)
    f = open_workbench(page)
    rec["view_1"] = view(f, page, plan_id, "e2dc-59-1-inflight")
    if OLD_UNKNOWN:
        rec["view_old_unknown"] = view(f, page, "P" + OLD_UNKNOWN[1:], "e2dc-59-3-unknown")
    f = open_workbench(page, reload=True)
    rec["view_2_after_reload"] = view(f, page, plan_id, "e2dc-59-2-inflight-reloaded")
    ctx.close()
ev_after = proxy_events()
after_run = state()["runs"][run_id]
rec["display_same_after_reload"] = {k: v["text"] for k, v in rec["view_1"].items()} == {k: v["text"] for k, v in rec["view_2_after_reload"].items()}
rec["proxy_events_during_display"] = len(ev_after) - len(ev_before)
rec["run_unchanged_by_display"] = before_run == after_run
if OLD_UNKNOWN:
    rec["old_unknown_unchanged_by_display"] = before_old == state()["runs"][OLD_UNKNOWN]
open(f"{EV}/w02-59-real.json", "w").write(json.dumps(sb.redact(rec), ensure_ascii=False, indent=1))
print(json.dumps({k: rec[k] for k in ("run", "plan", "status_at_halt", "steps_at_halt", "display_same_after_reload", "proxy_events_during_display", "run_unchanged_by_display")}, ensure_ascii=False))
for k, v in rec["view_1"].items(): print("  ", v["text"][:160], "| spin", v["spinning"])
if OLD_UNKNOWN:
    print("old unknown unchanged:", rec["old_unknown_unchanged_by_display"])
    for k, v in rec["view_old_unknown"].items(): print("  old:", v["text"][:160])
