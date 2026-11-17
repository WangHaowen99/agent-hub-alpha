"""Native tool and source paths shared by Windows and Linux launch/verification."""
from __future__ import annotations

import os
from pathlib import Path
import shutil

REPO = Path(__file__).resolve().parents[3]


def executable(name: str) -> str:
    return name + (".exe" if os.name == "nt" else "")


def java() -> Path:
    home = os.environ.get("JAVA_HOME")
    if home:
        path = Path(home) / "bin" / executable("java")
    else:
        found = shutil.which(executable("java"))
        if not found:
            raise SystemExit("Set JAVA_HOME to a native JDK 21 or put java on PATH")
        path = Path(found)
    if not path.is_file():
        raise SystemExit("Native Java executable missing: " + str(path))
    return path.resolve()


def go() -> Path:
    isolated = REPO / ".local/tools/go/bin" / executable("go")
    found = os.environ.get("GO_BINARY") or (str(isolated) if isolated.is_file() else shutil.which(executable("go")))
    if not found or not Path(found).is_file():
        raise SystemExit("Set GO_BINARY to a native Go executable or put go on PATH")
    return Path(found).resolve()


def maven() -> Path:
    found=shutil.which("mvn.cmd" if os.name == "nt" else "mvn")
    if not found:
        raise SystemExit("Put native Maven on PATH")
    return Path(found).resolve()


def workswarm() -> Path:
    supplied = os.environ.get("WORKSWARM_SOURCE")
    path = Path(supplied) if supplied else REPO / ".local/sources/jiuwenswarm"
    if not path.is_dir():
        raise SystemExit("Set WORKSWARM_SOURCE to the pinned WorkSwarm source directory")
    return path.resolve()


def keycloak() -> Path:
    return Path(os.environ.get("KEYCLOAK_HOME", str(REPO / ".local/tools/keycloak-26.8.2"))).resolve()
