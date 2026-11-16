"""Bind execution to source/prompt/model configuration hashes, without persisting model settings."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path


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
    model = {"base_url": os.environ["DEMO_MODEL_BASE_URL"].rstrip("/"), "model": os.environ["DEMO_MODEL_NAME"],
             "temperature": 0.1, "max_tokens": 6000, "normal_max_iterations": 4, "quality_max_iterations": 8}
    body = json.dumps({"files": files, "model": model}, ensure_ascii=True, sort_keys=True, separators=(",", ":")).encode()
    return "sha256:" + hashlib.sha256(body).hexdigest()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--private-root", type=Path, required=True)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[3]
    digest = fingerprint(repo)
    for path in [args.private_root / "config.json", *(args.private_root / "experts").glob("*.json")]:
        data = json.loads(path.read_text(encoding="utf-8"))
        data["execution_profile"] = digest
        path.write_text(json.dumps(data, indent=2), encoding="utf-8")
    print(json.dumps({"execution_profile": digest, "model_settings_persisted": False,
                      "next": "Restart control/compiler/experts before admitting a new graph"}))


if __name__ == "__main__":
    main()
