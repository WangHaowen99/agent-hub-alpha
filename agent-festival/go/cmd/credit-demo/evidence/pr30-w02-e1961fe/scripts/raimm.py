"""R-A on e1961fe, W02 part: related (Loop Forest) running with a model call in
flight, one remote node becomes UNKNOWN, and the run is resumed from the real
WorkSwarm plugin page as soon as the resume form appears.

usage: raimm.py TAG [attempts]
"""
import os, sys, json, time, threading, subprocess
D = "<repo>/agent-festival/go/cmd/credit-demo/deploy"
os.environ["BUS_TEST_BASE"] = "http://127.0.0.1:5291"; os.environ["BUS_TEST_PROJECT"] = "pr30-w02"
sys.path.insert(0, D); sys.path.insert(0, os.path.expanduser("<workdir>/w05")); sys.path.insert(0, os.path.expanduser("<workdir>"))
os.chdir(D)
from pw import *
import smoke_bus as sb, w05_rd_extra as w

TAG = sys.argv[1]
ATTEMPTS = int(sys.argv[2]) if len(sys.argv) > 2 else 3
ORCH = "pr30-w02-orchestrator-1"
EV = os.path.expanduser("<workdir>/evidence")


def iso(t=None):
    t = time.time() if t is None else t
    return time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(t)) + ".%03dZ" % int((t % 1) * 1000)


def orch_identity():
    out = subprocess.run(["docker", "inspect", ORCH, "--format", "{{.State.StartedAt}}|{{.RestartCount}}|{{.State.Pid}}|{{.Image}}"],
                         capture_output=True, text=True).stdout.strip().split("|")
    tick = subprocess.run(["docker", "exec", ORCH, "sh", "-c", "cut -d' ' -f22 /proc/1/stat"], capture_output=True, text=True).stdout.strip()
    return {"at": iso(), "started_at": out[0], "restart_count": out[1], "host_pid": out[2], "image": out[3][7:19], "pid1_starttime_ticks": tick}


def steps(run):
    return {k: s["status"] for k, s in run["steps"].items()}


def thread_dump(path):
    t0 = time.time()
    r = subprocess.run(["docker", "exec", ORCH, "sh", "-c", "command -v jcmd >/dev/null && jcmd 1 Thread.print || (command -v jstack >/dev/null && jstack 1) || echo NO_JCMD_JSTACK"],
                       capture_output=True, text=True, timeout=60)
    open(path, "w").write(f"# requested {iso(t0)} finished {iso()}\n" + r.stdout + r.stderr)


def attempt(n, f, page):
    rec = {"attempt": n, "source": "e1961fe8dcfc1c859b515854f793f24948d844fe", "operator": "WangHaowen99 (plugin page, scripted)"}
    rec["identity_before"] = orch_identity()
    t_start = time.time()
    rec["log_since"] = iso(t_start - 1)
    run_id, plan_id = sb.new_run(f"ra-e196-{TAG}-{n}")
    rec.update(run=run_id, plan=plan_id, created_at=iso(t_start))
    # select the plan in the plugin as soon as it is listed
    for _ in range(80):
        item = f.locator(".wb-item", has_text=f"{plan_id} v")
        if item.count():
            item.first.click(); break
        f.wait_for_timeout(250)
    rec["selected_at"] = iso()
    armed = None
    while time.time() - t_start < 300:
        run = state()["runs"][run_id]
        st = steps(run)
        remote = {k: (s.get("remote_call") or {}).get("status") for k, s in run["steps"].items() if s.get("remote_call")}
        done_remote = [k for k, v in remote.items() if v == "completed"]
        inflight_remote = [k for k, v in remote.items() if v not in ("completed",) and st.get(k) in ("running", "waiting")]
        if st.get("related") == "running" and st.get("intake") == "done" and inflight_remote and len(done_remote) >= 1:
            w.set_fault("drop-query-once")
            armed = {"at": iso(), "steps": st, "remote_call_status": remote, "related_activity": run["steps"]["related"].get("activity")}
            break
        if run["status"] in ("done", "failed", "waiting") or st.get("related") == "done":
            break
        time.sleep(0.1)
    rec["armed"] = armed
    if not armed:
        rec["skipped"] = "related was not running while a remote call was in flight"
        rec["final_status"] = sb.read_run(run_id)["status"]
        return rec, False
    # wait for waiting; resume from the plugin as soon as the form shows
    t_wait = None
    while time.time() - t_start < 600:
        run = state()["runs"][run_id]
        if run["status"] == "waiting" and t_wait is None:
            t_wait = time.time(); rec["waiting_detected_at"] = iso(t_wait)
            rec["steps_at_waiting"] = steps(run)
            rec["before_resume"] = sb.call_snapshot(run)
            dump = threading.Thread(target=thread_dump, args=(f"{EV}/{TAG}-dump-{n}.txt",), daemon=True); dump.start()
        if t_wait is not None and f.locator(".wb-resume").count():
            f.locator(".wb-resume input[type=password]").fill(PW)
            f.locator(".wb-resume button[type=submit]").click()
            t_submit = time.time(); rec["resume_submitted_at"] = iso(t_submit)
            rec["waiting_to_submit_ms"] = round((t_submit - t_wait) * 1000)
            break
        if run["status"] in ("done", "failed") and t_wait is None:
            break
        f.wait_for_timeout(100)
    if t_wait is None:
        rec["no_waiting"] = sb.read_run(run_id)["status"]
        return rec, False
    page.screenshot(path=f"{SHOTS}/{TAG}-{n}-1-waiting-submitted.png")
    rec["identity_at_waiting"] = orch_identity()
    f.wait_for_timeout(1500)
    rec["notice"] = f.locator(".wb-notice").first.inner_text() if f.locator(".wb-notice").count() else None
    page.screenshot(path=f"{SHOTS}/{TAG}-{n}-2-after-resume.png")
    resumes = 1
    while time.time() - t_start < 900:
        run = state()["runs"][run_id]
        if run["status"] in ("done", "failed"):
            break
        if run["status"] == "waiting" and f.locator(".wb-resume").count() and resumes < 3:
            rec.setdefault("extra_resumes", []).append({"at": iso(), "steps": steps(run)})
            f.locator(".wb-resume input[type=password]").fill(PW)
            f.locator(".wb-resume button[type=submit]").click(); resumes += 1
            f.wait_for_timeout(2000)
        f.wait_for_timeout(1000)
    f.wait_for_timeout(4000)
    page.screenshot(path=f"{SHOTS}/{TAG}-{n}-3-final.png")
    run = sb.read_run(run_id)
    after = sb.call_snapshot(run)
    rec["identity_after"] = orch_identity()
    rec["final_status"] = run["status"]; rec["final_reason"] = run.get("final")
    rec["resumes_from_plugin"] = resumes
    rec["steps_final"] = steps(run)
    rec["after"] = after
    before = rec["before_resume"]
    completed = [k for k, v in before.items() if v.get("status") == "completed" or v.get("artifact_id")]
    rec["completed_before"] = completed
    rec["completed_unchanged"] = all(after[k][f2] == before[k][f2] for k in completed for f2 in ("request_id", "artifact_id", "artifact_digest", "model_events"))
    rec["kept_request_and_task"] = w.kept_identity(before, after, list(before))
    sends, events = w.transport_sends({c["request_id"] for c in after.values()}, {c["remote_task_id"] for c in after.values() if c["remote_task_id"]})
    rec["send_counts"] = sends; rec["transport_events"] = events
    logs = subprocess.run(["docker", "logs", "--timestamps", "--since", rec["log_since"], ORCH], capture_output=True, text=True)
    open(f"{EV}/{TAG}-orchestrator-{n}.log", "w").write(logs.stdout + logs.stderr)
    return rec, True


with sync_playwright() as p:
    ctx, page = launch(p)
    f = open_workbench(page)
    results = []
    for n in range(1, ATTEMPTS + 1):
        rec, ok = attempt(n, f, page)
        open(f"{EV}/{TAG}-attempt-{n}.json", "w").write(json.dumps(sb.redact(rec), ensure_ascii=False, indent=1))
        results.append({k: rec.get(k) for k in ("attempt", "run", "armed", "skipped", "no_waiting", "waiting_to_submit_ms", "final_status", "resumes_from_plugin", "completed_before", "completed_unchanged", "kept_request_and_task", "send_counts")})
        print(json.dumps(results[-1], ensure_ascii=False)[:1500], flush=True)
        if ok:
            break
        # let the skipped run finish before the next attempt
        t0 = time.time()
        while time.time() - t0 < 300 and sb.read_run(rec["run"])["status"] not in ("done", "failed", "waiting"):
            time.sleep(2)
    ctx.close()
