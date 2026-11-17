"""Bind execution to source/prompt/model configuration hashes, without persisting model settings."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path


def budgets() -> tuple[int, int, int]:
    result=[]
    for key,default,low,high in [('DEMO_MODEL_MAX_TOKENS',12000,1024,32768),
            ('DEMO_MODEL_NORMAL_ITERATIONS',4,1,32),('DEMO_MODEL_QUALITY_ITERATIONS',8,1,32)]:
        raw=os.environ.get(key,str(default))
        if not raw.isascii() or not raw.isdigit() or not low <= int(raw) <= high:
            raise SystemExit('Invalid model budget: '+key)
        result.append(int(raw))
    return tuple(result)


def model_binding_digest() -> str:
    fields = ['report-model/1', 'OpenAI', os.environ['DEMO_MODEL_BASE_URL'].rstrip('/'),
              os.environ['DEMO_MODEL_NAME'], '0.1', *(str(v) for v in budgets())]
    body = b''.join(str(len(raw)).encode('ascii') + b':' + raw for raw in (f.encode('utf-8') for f in fields))
    return 'sha256:' + hashlib.sha256(body).hexdigest()


def fingerprint(repo: Path) -> str:
    for key in ("DEMO_MODEL_BASE_URL", "DEMO_MODEL_NAME"):
        if not os.environ.get(key):
            raise SystemExit(key + " required")
    files = {}
    schema=repo / "demos/report-demo/scenes/annual-report/report.schema.json"
    files[schema.relative_to(repo).as_posix()]=hashlib.sha256(schema.read_bytes()).hexdigest()
    for module in ("experts-java", "orchestrator-java"):
        base = repo / "demos/report-demo" / module
        for pattern in ("src/main/**/*.java", "src/main/resources/prompts/*.md", "pom.xml"):
            for path in sorted(base.glob(pattern)):
                files[path.relative_to(repo).as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
    tokens,normal,quality=budgets()
    model = {"base_url": os.environ["DEMO_MODEL_BASE_URL"].rstrip("/"), "model": os.environ["DEMO_MODEL_NAME"],
             "temperature": 0.1, "max_tokens": tokens, "normal_max_iterations": normal, "quality_max_iterations": quality}
    body = json.dumps({"files": files, "model": model}, ensure_ascii=True, sort_keys=True, separators=(",", ":")).encode()
    return "sha256:" + hashlib.sha256(body).hexdigest()


def bus_binding(bus: dict) -> dict:
    """Public logical identity only; never hash routes, bearer tokens or secrets."""
    result = {key: bus.get(source, "") for key, source in (
        ("environment_id", "environment_id"), ("tenant", "tenant"),
        ("issuer", "issuer"), ("audience", "audience"), ("caller_subject", "subject"))}
    if any(not isinstance(value, str) or not value.strip() for value in result.values()):
        raise SystemExit("Configure identity and an explicit Bus environment before binding this release")
    if bus.get("token_url") != result["issuer"] + "/protocol/openid-connect/token":
        raise SystemExit("Bus token URL must belong to its explicit issuer")
    return result


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--private-root", type=Path, required=True)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[3]
    digest = fingerprint(repo)
    for path in [args.private_root / "config.json", *(args.private_root / "experts").glob("*.json")]:
        data = json.loads(path.read_text(encoding="utf-8"))
        data["execution_profile"] = digest
        data["model_binding_digest"] = model_binding_digest()
        if path.name == "config.json":
            data["bus_binding"] = bus_binding(data.get("bus", {}))
        path.write_text(json.dumps(data, indent=2), encoding="utf-8")
    print(json.dumps({"execution_profile": digest, "model_settings_persisted": False,
                      "next": "Restart control/compiler/experts before admitting a new graph"}))


if __name__ == "__main__":
    main()
