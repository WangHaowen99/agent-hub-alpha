"""Local native process lifecycle. No model credentials are written to disk.

Run with the isolated WorkSwarm Python. Configuration and PID records live in
the ignored private directory; stop only processes whose creation time matches.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import time
from urllib.parse import urlparse

import psutil
import requests
from filelock import FileLock
from platform_tools import REPO, executable, java, keycloak, workswarm

SOLUTION = Path(os.environ.get("SOLUTION_SOURCE", str(REPO / ".local/sources/agent-solution"))) / "common/agent-bus"
ROLES = {"report-extractor": 8093, "business-analyst": 8094,
         "asset-quality-analyst": 8096, "report-writer": 8097}


def save(path: Path, value: object) -> None:
    path.write_text(json.dumps(value, indent=2), encoding="utf-8")


def configure(root: Path) -> None:
    identity = json.loads((root / "identity.json").read_text(encoding="utf-8"))
    password = os.environ.get("RDC_DB_PASSWORD")
    if not password:
        raise SystemExit("Set RDC_DB_PASSWORD for the existing agent_rdc database")
    jdbc=os.environ.get("RDC_DB_URL", "jdbc:postgresql://127.0.0.1:5432/agent_rdc")
    jdbc+=("&" if "?" in jdbc else "?")+"ApplicationName=agent-hub-report-demo-rdc&tcpKeepAlive=true"
    properties = {
        "server.address": "127.0.0.1", "server.port": 8092,
        "spring.datasource.url":jdbc,
        "spring.datasource.username": os.environ.get("RDC_DB_USER", "agent_rdc"),
        "spring.datasource.password": password, "spring.datasource.hikari.maximum-pool-size": 8,
        "rdc.deployment-discovery.enabled": "false",
        "openjiuwen.registry.route-handle-secret": identity["route_handle_secret"],
        "openjiuwen.registry.auth.token": identity["registry_token"],
        "openjiuwen.authorization.enabled": "true", "openjiuwen.authorization.tenant-id": "demo",
        "openjiuwen.authorization.identity.keycloak.issuer": identity["issuer"],
        "openjiuwen.authorization.identity.keycloak.audience": identity["audience"],
        "openjiuwen.authorization.identity.keycloak.client-id": "report-runtime-introspect",
        "openjiuwen.authorization.identity.keycloak.client-secret": identity["clients"]["report-runtime-introspect"]["client_secret"],
        "logging.level.root": "WARN",
    }
    # Java Properties escaping, including non-ASCII, is unambiguous.
    def escaped(value: object) -> str:
        return str(value).replace("\\", "\\\\").replace("\n", "\\n").replace("\r", "\\r")
    (root / "rdc.properties").write_text("\n".join(k + "=" + escaped(v) for k, v in properties.items()) + "\n", encoding="utf-8")
    gateway = {"server.address": "127.0.0.1", "server.port": 8080,
               "gateway.path-mode": "direct", "gateway.rdc.v2-base-url": "http://127.0.0.1:8092",
               "gateway.rdc.token": identity["registry_token"], "gateway.rdc.tenant-id": "demo",
               "gateway.authorization.enabled": "true", "logging.level.root": "WARN"}
    (root / "gateway.properties").write_text("\n".join(k + "=" + escaped(v) for k, v in gateway.items()) + "\n", encoding="utf-8")
    print("Native RDC/Gateway private configuration prepared; authorization enabled")


def command(name: str, root: Path, env: dict[str, str]) -> tuple[list[str], str]:
    env["REPORT_DEMO_CONFIG"] = str(root / "config.json")
    if name == "keycloak":
        admin_path = root / "identity-admin.json"
        if not admin_path.exists():
            import secrets
            save(admin_path, {"user": "demo-admin", "password": secrets.token_urlsafe(32)})
        admin = json.loads(admin_path.read_text(encoding="utf-8"))
        env.update(KC_BOOTSTRAP_ADMIN_USERNAME=admin["user"], KC_BOOTSTRAP_ADMIN_PASSWORD=admin["password"],
                   JAVA_HOME=str(java().parent.parent), JAVA_OPTS_KC_HEAP="-Xms64m -Xmx256m")
        kc = keycloak() / "bin" / ("kc.bat" if os.name == "nt" else "kc.sh")
        launcher = ["cmd.exe", "/d", "/c", str(kc)] if os.name == "nt" else [str(kc)]
        return launcher + ["start-dev", "--http-host=127.0.0.1", "--http-port=8180"], "http://127.0.0.1:8180/realms/master/.well-known/openid-configuration"
    if name.startswith("workswarm-"):
        source = workswarm()
        env.update(PYTHONPATH=str(source), JIUWENSWARM_DATA_DIR=str(root / "workswarm-home"),
                   AGENT_SERVER_HOST="127.0.0.1", AGENT_SERVER_PORT="18092", WEB_HOST="127.0.0.1", WEB_PORT="19000")
        model_keys=("DEMO_MODEL_API_KEY", "DEMO_MODEL_BASE_URL", "DEMO_MODEL_NAME")
        if all(env.get(key) for key in model_keys):
            # Web/Gateway also resolve the public model catalog from these
            # placeholders. AgentServer-only aliases leave a fresh UI unconfigured.
            env.update(API_KEY=env["DEMO_MODEL_API_KEY"], API_BASE=env["DEMO_MODEL_BASE_URL"], MODEL_NAME=env["DEMO_MODEL_NAME"],
                       MODEL_PROVIDER="OpenAI", ENDPOINT_PROFILE=env.get("DEMO_MODEL_ENDPOINT_PROFILE","deepseek"))
        if name == "workswarm-agentserver":
            if os.name=="nt":
                bootstrap=REPO/'demos/report-demo/workswarm/bootstrap'
                env.update(PYTHONPATH=str(bootstrap)+os.pathsep+str(source),REPORT_DEMO_WINDOWS_HISTORY_COMPAT="1")
            for key in ("DEMO_MODEL_API_KEY", "DEMO_MODEL_BASE_URL", "DEMO_MODEL_NAME"):
                if not env.get(key):
                    raise SystemExit(key + " must be set in the launching process")
            return [sys.executable, "-m", "jiuwenswarm.server.app_agentserver", "--port", "18092"], "http://127.0.0.1:18092/"
        if name == "workswarm-gateway":
            return [sys.executable, "-m", "agent_gateway.app_gateway", "--host", "127.0.0.1", "--port", "19000"], "http://127.0.0.1:19000/"
        if name == "workswarm-web":
            return [sys.executable, "-m", "jiuwenswarm.channels.web.app_web", "--host", "127.0.0.1", "--port", "5173", "--dist", str(source / "jiuwenswarm/channels/web/frontend/dist")], "http://127.0.0.1:5173/"
    if name == "control":
        return [str(root / "bin" / executable("report-control"))], "http://127.0.0.1:8091/healthz"
    if name == "compiler":
        return [str(java()), "-Xms64m", "-Xmx384m", "-jar", str(REPO / "demos/report-demo/orchestrator-java/target/report-orchestrator.jar")], "http://127.0.0.1:8195/healthz"
    if name in ("rdc", "gateway"):
        jar = SOLUTION / ("registry-discovery-center/target/agent-rdc-0.1.3.jar" if name == "rdc" else "agent-gateway/target/agent-gateway-0.1.3.jar")
        return [str(java()), "-Xms64m", "-Xmx256m", "-jar", str(jar), "--spring.config.additional-location=" + (root / (name + ".properties")).as_uri()], ("http://127.0.0.1:8092/v2/agents" if name == "rdc" else "http://127.0.0.1:8080/.well-known/agent-card.json")
    if name in ROLES:
        for key in ("DEMO_MODEL_API_KEY", "DEMO_MODEL_BASE_URL", "DEMO_MODEL_NAME"):
            if not env.get(key):
                raise SystemExit(key + " must be set in the launching process")
        from bind_profile import fingerprint
        cfg = json.loads((root / "config.json").read_text(encoding="utf-8"))
        if cfg.get("execution_profile") and fingerprint(REPO) != cfg["execution_profile"]:
            raise SystemExit("Source/model profile changed; run bind_profile.py and restart control/compiler/experts")
        port = ROLES[name]
        env.update(REPORT_EXPERT_CONFIG=str(root / "experts" / (name + ".json")),
                   REPORT_AGENT_ID=name, REPORT_EXPERT_PORT=str(port), REPORT_AGENT_PUBLIC_URL=f"http://127.0.0.1:{port}/a2a")
        return [str(java()), "-Xms64m", "-Xmx384m", "-jar", str(REPO / "demos/report-demo/experts-java/target/report-experts.jar")], f"http://127.0.0.1:{port}/.well-known/agent-card.json"
    if name == "registry-heartbeat":
        return [sys.executable, str(Path(__file__).with_name("configure_bus.py")), "--private-root", str(root), "--heartbeat"], "http://127.0.0.1:8092/v2/agents"
    raise SystemExit("Unknown service: " + name)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["configure", "start", "stop", "status", "worker-enable", "worker-disable", "adopt-keycloak"])
    parser.add_argument("names", nargs="*")
    parser.add_argument("--private-root", type=Path, default=REPO / ".local/report-demo")
    parser.add_argument("--pid",type=int)
    args = parser.parse_args()
    root = args.private_root.resolve()
    root.mkdir(parents=True,exist_ok=True)
    # Multiple launchers must not overwrite each other's PID records with
    # stale snapshots while starting different services.
    with FileLock(str(root / "services.lock"),timeout=180):
        manage(args,root)


def manage(args,root: Path) -> None:
    if args.action=="adopt-keycloak":
        if not args.pid:raise SystemExit("Explicit --pid required")
        process=psutil.Process(args.pid)
        marker=str(keycloak()).replace("\\","/").lower()
        cmd=" ".join(process.cmdline()).replace("\\","/").lower()
        listening=any(c.status==psutil.CONN_LISTEN and c.laddr.port==8180 for c in process.net_connections(kind="tcp"))
        if marker not in cmd or not listening:raise SystemExit("PID is not this workspace's Keycloak listener")
        records=root / "services.json";state=json.loads(records.read_text(encoding="utf-8")) if records.exists() else {}
        state["keycloak"]={"pid":process.pid,"created":process.create_time(),"health":"http://127.0.0.1:8180/realms/master/.well-known/openid-configuration"};save(records,state)
        print("Verified and adopted existing workspace Keycloak listener")
        return
    if args.action in ("worker-enable", "worker-disable"):
        path=root / "config.json"
        cfg=json.loads(path.read_text(encoding="utf-8"));cfg["worker_enabled"]=args.action=="worker-enable";save(path,cfg)
        print("worker_enabled="+str(cfg["worker_enabled"])+"; restart compiler to apply")
        return
    if args.action == "configure":
        configure(root)
        return
    records = root / "services.json"
    state = json.loads(records.read_text(encoding="utf-8")) if records.exists() else {}
    names = args.names or list(state)
    for name in names:
        record = state.get(name, {})
        process = None
        try:
            candidate = psutil.Process(record.get("pid", -1))
            if abs(candidate.create_time() - record.get("created", 0)) < .01:
                process = candidate
        except (psutil.Error, ValueError):
            pass
        if args.action == "status":
            print(json.dumps({"service": name, "pid": process.pid if process else None, "running": bool(process)}))
        elif args.action == "stop":
            if record and not process and psutil.pid_exists(record.get("pid", -1)):
                raise SystemExit(name+": recorded PID exists but creation time differs; inspect ownership before stopping")
            if process:
                descendants = process.children(recursive=True)
                for child in reversed(descendants):
                    child.terminate()
                process.terminate()
                _, alive = psutil.wait_procs([process] + descendants, timeout=8)
                for child in alive:
                    child.kill()
            state.pop(name, None)
            save(records, state)
            print(name + " stopped")
        elif not process:
            env = os.environ.copy()
            cmd, health = command(name, root, env)
            (root / "logs").mkdir(exist_ok=True)
            with (root / "logs" / (name + ".out.log")).open("ab") as out, (root / "logs" / (name + ".err.log")).open("ab") as err:
                child = subprocess.Popen(cmd, cwd=REPO, env=env, stdout=out, stderr=err,
                                         start_new_session=os.name != "nt",
                                         creationflags=subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0)
            state[name] = {"pid": child.pid, "created": psutil.Process(child.pid).create_time(), "health": health}
            save(records, state)
            deadline = time.monotonic() + 55
            ready = False
            while time.monotonic() < deadline:
                if child.poll() is not None:
                    break
                if name=="registry-heartbeat":
                    try:
                        status=json.loads((root/"heartbeat-status.json").read_text(encoding="utf-8"))
                        ready=bool(status["all_healthy"] and status["at"]>=state[name]["created"])
                    except (OSError,ValueError,KeyError):ready=False
                    if ready:break
                    time.sleep(.5)
                    continue
                try:
                    # Protected discovery may return 401; that proves listener readiness only.
                    headers = {}
                    if name in ("rdc", "registry-heartbeat"):
                        cfg = json.loads((root / "config.json").read_text(encoding="utf-8"))
                        headers = {"Authorization": "Bearer " + cfg["bus"]["registry_token"], "X-Tenant-ID": cfg["bus"]["tenant"]}
                    response = requests.get(health, headers=headers, timeout=2)
                    if response.status_code in (200, 401, 403, 404) or (name == "workswarm-agentserver" and response.status_code in (400,426)):
                        owned={child.pid}
                        try:
                            owned.update(p.pid for p in psutil.Process(child.pid).children(recursive=True))
                            port=urlparse(health).port
                            ready=any(c.status==psutil.CONN_LISTEN and c.laddr.port==port and c.pid in owned for c in psutil.net_connections(kind="tcp"))
                        except psutil.Error:
                            ready=False
                        if ready:break
                except requests.RequestException:
                    pass
                time.sleep(.5)
            print(json.dumps({"service": name, "pid": child.pid, "listener_ready": ready, "exit": child.poll()}))
            if not ready:
                raise SystemExit("Inspect private " + name + " logs")
        else:
            print(name + " already running")


if __name__ == "__main__":
    main()
