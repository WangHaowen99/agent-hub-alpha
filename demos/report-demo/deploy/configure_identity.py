"""Provision the isolated local demo realm using actual Keycloak admin APIs.

Only identity client secrets are kept in private runtime configuration. Model
credentials are never read or written by this script.
"""
from __future__ import annotations

import argparse
import json
import secrets
from pathlib import Path

import requests


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--private-root", type=Path, required=True)
    parser.add_argument("--base", default="http://127.0.0.1:8180")
    args = parser.parse_args()
    root = args.private_root.resolve()
    admin = json.loads((root / "identity-admin.json").read_text(encoding="utf-8"))
    session = requests.Session()
    result = session.post(args.base + "/realms/master/protocol/openid-connect/token", timeout=15,
                          data={"client_id": "admin-cli", "grant_type": "password", "username": admin["user"], "password": admin["password"]})
    result.raise_for_status()
    session.headers["Authorization"] = "Bearer " + result.json()["access_token"]
    admin_base = args.base + "/admin/realms"

    def call(method: str, path: str, body: object | None = None, allowed: tuple = (200, 201, 204)):
        response = session.request(method, admin_base + path, json=body, timeout=15)
        if response.status_code not in allowed:
            raise RuntimeError(f"Identity administration failed: {method} {path} HTTP {response.status_code}")
        return response.json() if response.content else None

    realm = "/report-demo"
    if session.get(admin_base + realm, timeout=10).status_code == 404:
        call("POST", "", {"realm": "report-demo", "enabled": True, "accessTokenLifespan": 3600, "sslRequired": "none"})
    for role in ["authz-admin", "authz-checker"]:
        if session.get(admin_base + realm + "/roles/" + role, timeout=10).status_code == 404:
            call("POST", realm + "/roles", {"name": role})
    clients = {}
    for client_id, role in [("report-worker", "authz-checker"), ("report-admin", "authz-admin"), ("report-runtime-introspect", None)]:
        existing = call("GET", realm + "/clients?clientId=" + client_id)
        if not existing:
            call("POST", realm + "/clients", {"clientId": client_id, "enabled": True, "publicClient": False,
                 "serviceAccountsEnabled": True, "standardFlowEnabled": False, "fullScopeAllowed": False, "secret": secrets.token_urlsafe(32),
                 "protocol": "openid-connect", "protocolMappers": [{"name": "report-audience", "protocol": "openid-connect",
                 "protocolMapper": "oidc-audience-mapper", "config": {"included.custom.audience": "report-demo", "included.client.audience": "report-runtime-introspect", "access.token.claim": "true", "id.token.claim": "false"}}]})
            existing = call("GET", realm + "/clients?clientId=" + client_id)
        uid = existing[0]["id"]
        representation = call("GET", realm + "/clients/" + uid)
        representation["fullScopeAllowed"] = False
        call("PUT", realm + "/clients/" + uid, representation)
        for mapper in call("GET", realm + "/clients/" + uid + "/protocol-mappers/models"):
            if mapper["name"] == "report-audience":
                mapper["config"]["included.client.audience"] = "report-runtime-introspect"
                call("PUT", realm + "/clients/" + uid + "/protocol-mappers/models/" + mapper["id"], mapper)
        mappers = call("GET", realm + "/clients/" + uid + "/protocol-mappers/models")
        # Current Keycloak chooses the client audience when both options are
        # configured in one mapper. Use separate mappers for both recipients.
        if not any(m["name"] == "report-application-audience" for m in mappers):
            call("POST", realm + "/clients/" + uid + "/protocol-mappers/models",
                 {"name": "report-application-audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
                  "config": {"included.custom.audience": "report-demo", "access.token.claim": "true", "id.token.claim": "false", "introspection.token.claim": "true"}})
        secret = call("GET", realm + "/clients/" + uid + "/client-secret")["value"]
        account = call("GET", realm + "/clients/" + uid + "/service-account-user")
        if role:
            definition = call("GET", realm + "/roles/" + role)
            call("POST", realm + "/users/" + account["id"] + "/role-mappings/realm", [definition])
            call("POST", realm + "/clients/" + uid + "/scope-mappings/realm", [definition])
        clients[client_id] = {"client_id": client_id, "client_secret": secret, "subject": account["id"]}

    cfg_path = root / "config.json"
    cfg = json.loads(cfg_path.read_text(encoding="utf-8"))
    registry_token = cfg.get("bus", {}).get("registry_token", secrets.token_urlsafe(32))
    environment_id = cfg.get("bus", {}).get("environment_id") or "report-demo-" + secrets.token_hex(16)
    cfg["bus"] = {"gateway_url": "http://127.0.0.1:8080", "rdc_url": "http://127.0.0.1:8092", "tenant": "demo",
                  "environment_id": environment_id, "issuer": args.base + "/realms/report-demo", "audience": "report-demo",
                  "registry_token": registry_token, "token_url": args.base + "/realms/report-demo/protocol/openid-connect/token",
                  **clients["report-worker"]}
    from bind_profile import bus_binding
    cfg["bus_binding"] = bus_binding(cfg["bus"])
    cfg_path.write_text(json.dumps(cfg, indent=2), encoding="utf-8")
    identity = {"issuer": args.base + "/realms/report-demo", "audience": "report-demo", "clients": clients,
                "registry_token": registry_token, "route_handle_secret": secrets.token_urlsafe(40)}
    state_path = root / "identity.json"
    if state_path.exists():
        old = json.loads(state_path.read_text(encoding="utf-8"))
        identity["route_handle_secret"] = old["route_handle_secret"]
    state_path.write_text(json.dumps(identity, indent=2), encoding="utf-8")
    for expert in (root / "experts").glob("*.json"):
        data = json.loads(expert.read_text(encoding="utf-8"))
        data["identity"] = {"introspection_url": identity["issuer"] + "/protocol/openid-connect/token/introspect",
                            "client_id": "report-runtime-introspect", "client_secret": clients["report-runtime-introspect"]["client_secret"],
                            "issuer":identity["issuer"],"audience":identity["audience"],
                            "allowed_client_id": "report-worker"}
        expert.write_text(json.dumps(data, indent=2), encoding="utf-8")
    token = requests.post(cfg["bus"]["token_url"], data={"grant_type": "client_credentials", "client_id": "report-worker",
                          "client_secret": clients["report-worker"]["client_secret"]}, timeout=15)
    token.raise_for_status()
    verification = requests.post(identity["issuer"] + "/protocol/openid-connect/token/introspect",
                                 data={"client_id": "report-runtime-introspect", "client_secret": clients["report-runtime-introspect"]["client_secret"],
                                       "token": token.json()["access_token"]}, timeout=15)
    verification.raise_for_status()
    audience = verification.json().get("aud", [])
    audience = [audience] if isinstance(audience, str) else audience
    assert verification.json()["active"] and verification.json()["client_id"] == "report-worker" and "report-demo" in audience
    print(json.dumps({"realm": "report-demo", "worker_subject": clients["report-worker"]["subject"],
                      "real_token_introspection_passed": True, "model_credentials_persisted": False}))


if __name__ == "__main__":
    main()
