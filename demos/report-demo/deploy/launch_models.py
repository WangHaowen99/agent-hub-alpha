"""Launch model services without putting the API key in command arguments/files.

Interactive terminals use a hidden prompt; automation can supply one line over
standard input. Child services receive the key only in their environment.
"""
from __future__ import annotations

import argparse
import getpass
import os
from pathlib import Path
import subprocess
import sys


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--model", required=True)
    parser.add_argument("--private-root", type=Path, required=True)
    parser.add_argument("--restart", action="store_true")
    parser.add_argument("services", nargs="+", choices=["report-extractor", "business-analyst", "asset-quality-analyst", "report-writer", "workswarm-agentserver"])
    args = parser.parse_args()
    key = getpass.getpass("Model API key (hidden): ") if sys.stdin.isatty() else sys.stdin.readline().rstrip("\r\n")
    if not key.strip():
        raise SystemExit("Empty model credential")
    env = os.environ.copy()
    env.update(DEMO_MODEL_API_KEY=key, DEMO_MODEL_BASE_URL=args.base_url, DEMO_MODEL_NAME=args.model)
    directory = Path(__file__).resolve().parent
    subprocess.run([sys.executable, str(directory / "bind_profile.py"), "--private-root", str(args.private_root)], env=env, check=True)
    if args.restart:
        subprocess.run([sys.executable, str(directory / "services.py"), "stop", *args.services, "--private-root", str(args.private_root)], env=env, check=True)
    subprocess.run([sys.executable, str(directory / "services.py"), "start", *args.services, "--private-root", str(args.private_root)], env=env, check=True)


if __name__ == "__main__":
    main()
