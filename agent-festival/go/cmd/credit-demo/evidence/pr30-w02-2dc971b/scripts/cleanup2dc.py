import os, sys, time, json
D = "<repo>/agent-festival/go/cmd/credit-demo/deploy"
os.environ["BUS_TEST_BASE"] = "http://127.0.0.1:5291"; os.environ["BUS_TEST_PROJECT"] = "pr30-w02"
sys.path.insert(0, D); sys.path.insert(0, os.path.expanduser("<workdir>/w05")); os.chdir(D)
import smoke_bus as sb, w05_rd_extra as w
w.set_fault("pass")
out = {}
for r in sys.argv[1:]:
    before = sb.call_snapshot(sb.read_run(r))
    sb.resume(r)
    t0 = time.time()
    while time.time() - t0 < 300 and sb.read_run(r)["status"] not in ("done", "failed", "blocked"):
        time.sleep(2)
    run = sb.read_run(r); after = sb.call_snapshot(run)
    sends, _ = w.transport_sends({c["request_id"] for c in after.values()}, {c["remote_task_id"] for c in after.values() if c["remote_task_id"]})
    out[r] = {"final": run["status"], "kept_identity": w.kept_identity(before, after, list(before)), "send_counts": sends}
print(json.dumps(out, ensure_ascii=False))
open(os.path.expanduser("<workdir>/evidence/e2dc/cleanup-resume.json"), "w").write(json.dumps(out, ensure_ascii=False, indent=1))
