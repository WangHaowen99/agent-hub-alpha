"""Recreate the orchestrator with the transport-test overlay (compose.bus-test.yaml),
so its Gateway traffic goes through fault-proxy, exactly as in the earlier R-A runs."""
import os, sys
D = "<repo>/agent-festival/go/cmd/credit-demo/deploy"
sys.path.insert(0, D); os.chdir(D)
import demo
demo.BUS = True
orig = demo.run
def run(cmd, **kw):
    cmd = list(cmd)
    if cmd[:2] == ["docker", "compose"]:
        j = max(k for k, v in enumerate(cmd) if str(v).endswith("compose.bus.yaml"))
        cmd[j + 1:j + 1] = ["-f", demo.HERE / "compose.bus-test.yaml"]
    return orig(cmd, **kw)
demo.run = run
demo.compose(*sys.argv[1:])
