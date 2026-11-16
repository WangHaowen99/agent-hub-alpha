"""Create private configuration for an already provisioned W04 PostgreSQL database.

Does not provision roles, reset passwords, touch RDC's database, or store model settings.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys

import requests

from services import ROLES, REPO, save


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--private-root", type=Path, default=REPO / ".local/report-demo")
    args = parser.parse_args()
    root = args.private_root.resolve()
    root.mkdir(parents=True, exist_ok=True)
    for folder in ("logs", "evidence", "materials", "experts", "artifacts", "bin"):
        (root / folder).mkdir(exist_ok=True)
    path = root / "config.json"
    if path.exists():
        cfg = json.loads(path.read_text(encoding="utf-8"))
    else:
        for key in ("W04_APP_DSN", "W04_MIGRATOR_DSN"):
            if not os.environ.get(key):
                raise SystemExit(key + " required for first preparation")
        cfg = {"listen": "127.0.0.1:8091", "public_url": "http://127.0.0.1:8091", "scope": "report-demo",
               "dsn": os.environ["W04_APP_DSN"], "migrator_dsn": os.environ["W04_MIGRATOR_DSN"],
               "artifact_root": str(root / "artifacts"), "catalog_path": str(REPO / "demos/report-demo/scenes/annual-report/catalog.json"),
               "material_path": str(root / "material.json"), "compiler_url": "http://127.0.0.1:8195", "lease_seconds": 30,
               "tokens": {name: secrets.token_urlsafe(32) for name in ("planner", "worker", "operator", "expert")},
               "user": "demo", "user_password": secrets.token_urlsafe(20), "worker_enabled": False}
    for name, port in ROLES.items():
        role = {"report-extractor": "extract", "business-analyst": "business", "asset-quality-analyst": "quality", "report-writer": "write"}[name]
        token = cfg["tokens"].setdefault("expert:" + name, secrets.token_urlsafe(32))
        expert_path = root / "experts" / (name + ".json")
        if not expert_path.exists():
            save(expert_path, {"agent_id": name, "role": role, "port": port, "control_url": cfg["public_url"], "expert_token": token})
    save(path, cfg)
    scene = REPO / "demos/report-demo/scenes/annual-report/source.json"
    manifest = json.loads(scene.read_text(encoding="utf-8"))
    pdf = root / "materials/cmb-2025.pdf"
    if not pdf.exists():
        response = requests.get(manifest["download_url"], timeout=120)
        response.raise_for_status()
        import hashlib
        if hashlib.sha256(response.content).hexdigest() != manifest["sha256"]:
            raise SystemExit("Published material changed; not saving mismatched source")
        pdf.write_bytes(response.content)
    subprocess.run([sys.executable, str(Path(__file__).with_name("prepare_material.py")), "--pdf", str(pdf), "--output", str(root / "material.json")], check=True)
    print(json.dumps({"private_config": str(path), "model_settings_persisted": False,
                      "next": "Apply private-directory ACL; configure identity and model fingerprint"}))


if __name__ == "__main__":
    main()
