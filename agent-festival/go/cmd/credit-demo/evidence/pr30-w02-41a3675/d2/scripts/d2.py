"""W02 side of the PR #30 credit-combo D2 matrix, on the W02 environment.

Every resume is done by a person-equivalent click in the real WorkSwarm
plugin page. R-A/R-C reuse W05's scenario definitions (w05_rd_extra.py at
e084318) unchanged, with RESUME_MODE=external so the plugin performs the
resume. O-A/O-B hold a Graph reply inside the plugin page's WebSocket.

usage: d2.py oa|ob|ra-two|ra-report|rb|rc TAG
"""
import os, sys, threading, time, json, subprocess
D = "<repo>/agent-festival/go/cmd/credit-demo/deploy"
os.environ["BUS_TEST_BASE"] = "http://127.0.0.1:5291"; os.environ["BUS_TEST_PROJECT"] = "pr30-w02"
sys.path.insert(0, D); sys.path.insert(0, os.path.expanduser("<workdir>/w05")); sys.path.insert(0, os.path.expanduser("<workdir>"))
import urllib.error, urllib.request
os.chdir(D)
from pw import *
import smoke_bus as sb
import w05_rd_extra as w
assert sb.BASE == "http://127.0.0.1:5291"
w.RESUME_MODE = "external"
w.PASSWORD = PW

INIT = r"""
(() => {
  if (window.__ws) return;
  const desc = Object.getOwnPropertyDescriptor(WebSocket.prototype, 'onmessage');
  const sent = {};
  window.__ws = {log: [], hold: null, sends: []};
  const origSend = WebSocket.prototype.send;
  WebSocket.prototype.send = function (d) {
    try { const m = JSON.parse(d); if (m.type === 'req') { sent[m.id] = {method: m.method, at: performance.now()}; window.__ws.sends.push({id: m.id, method: m.method, at: performance.now()}); } } catch (e) {}
    return origSend.call(this, d);
  };
  window.__domSig = () => {
    const head = (document.querySelector('.cg-head')?.innerText || '').replace(/\s+/g, ' ').slice(0, 160);
    const nodes = Array.from(document.querySelectorAll('.react-flow__node')).map(n => (n.getAttribute('data-id') || '') + ':' + (n.innerText || '').replace(/\s+/g, ' ').slice(0, 40));
    return {head, nodes, err: document.querySelector('.wb-err')?.innerText || null, notice: document.querySelector('.wb-notice')?.innerText || null};
  };
  Object.defineProperty(WebSocket.prototype, 'onmessage', {
    configurable: true,
    get() { return this.__userOnMessage || null; },
    set(fn) {
      this.__userOnMessage = fn;
      desc.set.call(this, (e) => {
        let m; try { m = JSON.parse(String(e.data)); } catch (x) { return fn.call(this, e); }
        const s = m && m.id && sent[m.id];
        const p = m && m.payload;
        const rec = {id: m.id, method: s && s.method, ok: m.ok, code: m.code || null, error: m.error || null,
                     rev: p && p.rev, run: p && p.run && p.run.id, run_status: p && p.run && p.run.status,
                     nodes: p && p.nodes ? p.nodes.map(n => n.id + ':' + n.status) : null,
                     sentAt: s && s.at, recvAt: performance.now()};
        const H = window.__ws.hold;
        if (H && H.armed && s && s.method.endsWith(H.method)) {
          H.armed = false; rec.heldMs = H.ms; window.__ws.log.push(rec);
          if (H.ms < 0) { rec.dropped = true; return; }
          setTimeout(() => {
            rec.domBefore = window.__domSig(); rec.deliveredAt = performance.now();
            fn.call(this, e);
            setTimeout(() => { rec.domAfter = window.__domSig(); }, 400);
          }, H.ms);
          return;
        }
        rec.deliveredAt = rec.recvAt; window.__ws.log.push(rec);
        return fn.call(this, e);
      });
    }
  });
})();
"""
SECOND = """(args) => new Promise((res) => {
  const ws = new WebSocket((location.protocol === "https:" ? "wss" : "ws") + "://" + location.host + "/ws");
  ws.onopen = () => ws.send(JSON.stringify({type: "req", id: "second1", method: "plugin.credit_workbench.resume", params: {run: args[0], password: args[1]}}));
  ws.addEventListener("message", (e) => { const m = JSON.parse(e.data); if (m.type === "res" && m.id === "second1") { ws.close(); res({ok: m.ok, code: m.code, error: m.error}); } });
  setTimeout(() => res({timeout: true}), 25000);
})"""

SCEN, TAG = sys.argv[1], sys.argv[2]
OUT = os.path.expanduser(f"<workdir>/evidence/d2-{SCEN}.json")
rec = {"scenario": SCEN, "source": "41a36755563ebeb442f2d93050c1d3430d510e80", "operator": "WangHaowen99 (plugin, scripted)", "started_at": time.time()}


def plan_of(run_id, plan_id=None):
    for p in go("/api/plans").get("plans", []):
        if p.get("run") == run_id or (plan_id and p.get("plan") == plan_id): return p
    return {"plan": plan_id, "version": 1}


def pick(f, plan, v):
    f.locator(".wb-item", has_text=f"{plan} v").first.click(); f.wait_for_timeout(2500)


def plugin_resume(f, run_id, tag):
    f.locator(".wb-resume input[type=password]").fill(PW)
    f.evaluate("() => { window.__resumeT0 = performance.now(); }")
    f.locator(".wb-resume button[type=submit]").click()
    samples = []
    for _ in range(8):
        samples.append({"run_status": sb.read_run(run_id)["status"], "dom": f.evaluate("window.__domSig()")})
        f.wait_for_timeout(1000)
    return samples


def wait_status(run_id, statuses, seconds=600):
    t0 = time.time()
    while time.time() - t0 < seconds:
        s = sb.read_run(run_id)["status"]
        if s in statuses: return s
        time.sleep(1)
    return sb.read_run(run_id)["status"]


def events_for(run_id):
    run = sb.read_run(run_id)
    snap = sb.call_snapshot(run)
    sends, events = w.transport_sends({c["request_id"] for c in snap.values()}, {c["remote_task_id"] for c in snap.values() if c["remote_task_id"]})
    return sends, events


def run_w05(fn, ev):
    try: fn(ev)
    except Exception as exc: ev["error"] = repr(exc)


with sync_playwright() as p:
    ctx, page = launch(p)
    ctx.add_init_script(INIT)
    f = open_workbench(page)
    rec["init_ok"] = f.evaluate("!!window.__ws")
    if SCEN in ("oa", "ob"):
        run_id, plan_id = sb.new_run(SCEN)
        rec["run"] = run_id
        pl = plan_of(run_id, plan_id); f.wait_for_timeout(5500)
        pick(f, pl["plan"], pl["version"])
        # wait until the run is actually progressing on screen
        for _ in range(60):
            if sb.read_run(run_id)["status"] == "running": break
            f.wait_for_timeout(500)
        f.wait_for_timeout(1500)
        ms = 6000 if SCEN == "oa" else 25000
        f.evaluate(f"() => {{ window.__ws.hold = {{method: '.graph', ms: {ms}, armed: true}}; window.__armT = performance.now(); }}")
        for _ in range(40):
            if f.evaluate("window.__ws.log.some(r => r.heldMs)"): break
            f.wait_for_timeout(100)
        held = f.evaluate("window.__ws.log.find(r => r.heldMs)")
        rec["held"] = {k: held.get(k) for k in ("id", "rev", "run_status", "nodes", "sentAt", "recvAt")}
        if SCEN == "oa":
            # start a new poll loop for the same task (switch away and back) while the old reply is held
            others = f.locator(".wb-item").filter(has_not_text=f"{pl['plan']} v")
            others.first.click(); f.wait_for_timeout(1200)
            pick(f, pl["plan"], pl["version"])
            f.wait_for_timeout(ms + 2500)
        else:
            f.wait_for_timeout(ms + 3000)
        shot(page, f"{TAG}-{SCEN}")
        log = f.evaluate("window.__ws.log")
        sends = f.evaluate("window.__ws.sends")
        held = next(r for r in log if r.get("heldMs"))
        rec["held"].update({k: held.get(k) for k in ("deliveredAt", "domBefore", "domAfter")})
        rec["held_age_at_delivery_ms"] = round(held["deliveredAt"] - held["sentAt"]) if held.get("deliveredAt") else None
        rec["graph_replies"] = [{k: r.get(k) for k in ("id", "rev", "run_status", "sentAt", "recvAt", "deliveredAt", "heldMs", "ok", "code")} for r in log if (r.get("method") or "").endswith(".graph")]
        rec["graph_requests"] = [s for s in sends if s["method"].endswith(".graph")]
        newer_first = [r for r in log if (r.get("method") or "").endswith(".graph") and not r.get("heldMs") and r.get("rev") is not None
                       and held.get("deliveredAt") and r["deliveredAt"] < held["deliveredAt"] and r["rev"] > held["rev"]]
        rec["newer_delivered_before_held"] = [{"id": r["id"], "rev": r["rev"]} for r in newer_first]
        rec["view_unchanged_by_held"] = held.get("domBefore") == held.get("domAfter") if held.get("domAfter") else None
        if SCEN == "ob":
            rec["requests_while_held"] = [s for s in sends if s["method"].endswith(".graph") and held["sentAt"] < s["at"] < (held.get("deliveredAt") or 1e18)]
        rec["final_status"] = wait_status(run_id, ("done", "failed", "waiting"))
        rec["send_counts"], rec["transport_events"] = events_for(run_id)
    elif SCEN in ("ra-two", "ra-report"):
        ev = {"scenario": SCEN, "snapshots": [], "environment": sb.environment_record()}
        fn = (lambda e: w.ra_completed_then_unknown(e, at_report=True)) if SCEN == "ra-report" else w.ra_completed_then_unknown
        th = threading.Thread(target=run_w05, args=(fn, ev), daemon=True); th.start()
        for _ in range(240):
            if ev.get("run_id"): break
            time.sleep(0.5)
        run_id = ev["run_id"]; rec["run"] = run_id
        st = wait_status(run_id, ("waiting", "done", "failed"))
        rec["status_before_resume"] = st
        pl = plan_of(run_id, ev.get("plan_id")); f.wait_for_timeout(5500)
        pick(f, pl["plan"], pl["version"]); f.wait_for_timeout(2500)
        rec["dom_at_waiting"] = f.evaluate("window.__domSig()")
        rec["steps_at_waiting"] = {k: s["status"] for k, s in sb.read_run(run_id)["steps"].items()}
        shot(page, f"{TAG}-{SCEN}-waiting")
        if st == "waiting":
            rec["after_resume_samples"] = plugin_resume(f, run_id, TAG)
        th.join(timeout=700)
        f.wait_for_timeout(3000)
        rec["dom_final"] = f.evaluate("window.__domSig()")
        shot(page, f"{TAG}-{SCEN}-final")
        rec["w05_evidence"] = {k: ev.get(k) for k in ("variant", "armed", "status_before_resume", "completed_before", "unknown_before", "resumes", "checks", "transport_events", "error")}
        rec["final_run"] = {"status": sb.read_run(run_id)["status"], "final": sb.read_run(run_id).get("final"),
                            "steps": {k: s["status"] for k, s in sb.read_run(run_id)["steps"].items()}}
    elif SCEN == "rb":
        w.set_fault("drop-query-once")
        run_id, plan_id = sb.new_run("rb-plugin"); rec["run"] = run_id
        rec["status_before_resume"] = wait_status(run_id, ("waiting", "done", "failed"))
        before = sb.call_snapshot(sb.read_run(run_id)); rec["before"] = before
        rec["leases_before"] = len(sb.read_run(run_id).get("leases", []))
        pl = plan_of(run_id, plan_id); f.wait_for_timeout(5500)
        pick(f, pl["plan"], pl["version"])
        f.evaluate("() => { window.__ws.hold = {method: '.resume', ms: -1, armed: true}; }")
        f.locator(".wb-resume input[type=password]").fill(PW)
        f.locator(".wb-resume button[type=submit]").click()
        f.wait_for_timeout(2000)
        rec["status_2s_after_submit"] = sb.read_run(run_id)["status"]
        rec["dom_2s"] = f.evaluate("window.__domSig()")
        f.wait_for_timeout(21000)
        rec["dom_after_client_timeout"] = f.evaluate("window.__domSig()")
        shot(page, f"{TAG}-rb-after-timeout")
        rec["resume_rpc_sends"] = f.evaluate("window.__ws.sends.filter(s => s.method.endsWith('.resume')).length")
        rec["resume_reply_dropped"] = f.evaluate("window.__ws.log.filter(r => r.dropped).map(r => ({id: r.id, ok: r.ok, code: r.code}))")
        f = open_workbench(page, reload=True)
        pick(f, pl["plan"], pl["version"])
        rec["dom_after_reopen"] = f.evaluate("window.__domSig()")
        rec["second_resume"] = f.evaluate(SECOND, [run_id, PW])
        rec["final_status"] = wait_status(run_id, ("done", "failed", "waiting"))
        run = sb.read_run(run_id); after = sb.call_snapshot(run)
        rec["leases_after"] = len(run.get("leases", []))
        rec["kept_request_and_task"] = w.kept_identity(before, after, list(before))
        rec["send_counts"], rec["transport_events"] = events_for(run_id)
        shot(page, f"{TAG}-rb-final")
    elif SCEN == "rc":
        w.set_fault("drop-query-once")
        run_id, plan_id = sb.new_run("rc-plugin"); rec["run"] = run_id
        rec["status_before_fault"] = wait_status(run_id, ("waiting", "done", "failed"))
        before = sb.call_snapshot(sb.read_run(run_id)); rec["before"] = before
        pl = plan_of(run_id, plan_id); f.wait_for_timeout(5500)
        pick(f, pl["plan"], pl["version"])
        injected = False
        try:
            rec["injected_at"] = time.time()
            sb.docker("credit", "exec", "python", "-c", "from pathlib import Path; p=Path('/data/state.json'); backup=p.with_suffix('.bus-test-backup'); assert not backup.exists(); p.rename(backup); p.mkdir()")
            injected = True
            rec["during_fault_samples"] = plugin_resume(f, run_id, TAG)
            shot(page, f"{TAG}-rc-during-fault")
            try:
                with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(sb.BASE + "/healthz", timeout=5) as r:
                    rec["healthz_during_fault"] = {"at": time.time(), "http": r.status}
            except urllib.error.HTTPError as exc:
                rec["healthz_during_fault"] = {"at": time.time(), "http": exc.code, "body": exc.read().decode()[:120]}
        finally:
            if injected:
                rec["restore_at"] = time.time()
                sb.docker("credit", "exec", "python", "-c", "from pathlib import Path; p=Path('/data/state.json'); p.rmdir(); p.with_suffix('.bus-test-backup').rename(p)")
                sb.docker("credit", "restart")
                rec["credit_healthy_again"] = w.wait_healthy("credit")
        rec["status_after_restore"] = sb.read_run(run_id)["status"]
        page.wait_for_timeout(4000)
        f = open_workbench(page, reload=True)
        pick(f, pl["plan"], pl["version"])
        if rec["status_after_restore"] == "waiting":
            rec["after_restore_samples"] = plugin_resume(f, run_id, TAG)
        rec["final_status"] = wait_status(run_id, ("done", "failed", "waiting"))
        after = sb.call_snapshot(sb.read_run(run_id))
        rec["kept_request_and_task"] = w.kept_identity(before, after, list(before))
        rec["send_counts"], rec["transport_events"] = events_for(run_id)
        shot(page, f"{TAG}-rc-final")
    rec["finished_at"] = time.time()
    open(OUT, "w").write(json.dumps(sb.redact(rec), ensure_ascii=False, indent=1))
    print(json.dumps({k: rec.get(k) for k in ("run", "final_status", "held_age_at_delivery_ms", "newer_delivered_before_held", "view_unchanged_by_held", "status_before_resume", "final_run", "send_counts")}, ensure_ascii=False)[:1500])
    ctx.close()
