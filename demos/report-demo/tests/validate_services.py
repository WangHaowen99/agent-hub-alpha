"""Read-only native service/identity readiness check. Not an end-to-end business acceptance."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys

import requests

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "deploy"))
from services import ROLES


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--private-root", type=Path, required=True)
    args = parser.parse_args()
    root = args.private_root.resolve()
    cfg = json.loads((root / "config.json").read_text(encoding="utf-8"))
    identity = json.loads((root / "identity.json").read_text(encoding="utf-8"))
    checks = []

    def probe(name: str, url: str, headers: dict | None = None, expected: tuple = (200,), verify=None) -> None:
        try:
            response = requests.get(url, headers=headers, timeout=8)
            passed = response.status_code in expected and (verify is None or verify(response))
            checks.append({"check": name, "passed": passed, "http_status": response.status_code})
        except requests.RequestException as exc:
            checks.append({"check": name, "passed": False, "error_type": type(exc).__name__})

    probe("control_and_pg", cfg["public_url"] + "/healthz")
    probe("java_compiler", cfg["compiler_url"] + "/healthz")
    probe("identity_discovery", identity["issuer"] + "/.well-known/openid-configuration")
    registry_headers = {"Authorization": "Bearer " + cfg["bus"]["registry_token"], "X-Tenant-ID": "demo"}
    for name, port in ROLES.items():
        probe("runtime_card:" + name, f"http://127.0.0.1:{port}/.well-known/agent-card.json", verify=lambda r,n=name: r.json().get("name") == n)
        probe("runtime_rejects_no_identity:" + name, f"http://127.0.0.1:{port}/v1/query", expected=(401, 403))
        probe("rdc_candidates:" + name, cfg["bus"]["rdc_url"] + "/v2/agents/" + name + "/instances", registry_headers,
              verify=lambda r: bool(r.json().get("candidates", r.json().get("instances", []))))
    probe("workswarm_web", "http://127.0.0.1:5173/")
    result = {"all_passed": all(row["passed"] for row in checks), "checks": checks,
              "scope": "read-only readiness; no model planning/expert execution/business acceptance asserted"}
    (root / "evidence" / "service-readiness.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    print(json.dumps(result))
    if not result["all_passed"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
