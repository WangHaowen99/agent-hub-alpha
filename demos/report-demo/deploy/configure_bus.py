"""Register actual Runtime cards and grant the worker through RDC's public admin API."""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import uuid

import requests

from services import ROLES


def register_runtime(session,bus,headers,name,port,bindings):
    expected=bindings[name]
    card_response=session.get(f"http://127.0.0.1:{port}/.well-known/agent-card.json",timeout=10)
    card_response.raise_for_status();card=card_response.json()
    if card.get("name")!=name or card.get("version")!=expected["card_version"]:
        raise RuntimeError("Runtime card identity/version differs from catalog")
    raw_card=json.dumps(card)
    if expected.get("card_digest") and expected["card_digest"] != "sha256:"+hashlib.sha256(raw_card.encode("utf-8")).hexdigest():
        raise RuntimeError("Runtime card digest differs from governed binding")
    response=session.post(bus["rdc_url"]+"/v2/agents/"+name+"/instances",headers=headers,
                          json={"address":"127.0.0.1","port":port,"endpointPath":"/a2a","serviceId":name,
                                "maxConcurrency":1,"frameworkType":"JIUWEN","contractVersion":expected["contract_version"],"capabilityVersion":expected["capability_version"],
                                "agentCardJson":raw_card},timeout=15)
    response.raise_for_status()
    return response.json()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--private-root", type=Path, required=True)
    parser.add_argument("--heartbeat", action="store_true")
    args = parser.parse_args()
    root = args.private_root.resolve()
    cfg = json.loads((root / "config.json").read_text(encoding="utf-8"))
    identity = json.loads((root / "identity.json").read_text(encoding="utf-8"))
    bus = cfg["bus"]
    catalog=json.loads(Path(cfg["catalog_path"]).read_text(encoding="utf-8"))
    bindings={c["agent_id"]:c.get("remote_binding", {"card_version":"1.0.0","contract_version":"1.0.0","capability_version":"1.0.0"}) for c in catalog["capabilities"] if c.get("agent_id")}
    for name in ROLES:
        if name not in bindings or any(not bindings[name].get(k) for k in ("card_version","contract_version","capability_version")):
            raise SystemExit("Explicit remote versions required for "+name)
    session = requests.Session()
    registry_headers = {"Authorization": "Bearer " + bus["registry_token"], "X-Tenant-ID": bus["tenant"]}
    state_path = root / "registrations.json"
    if args.heartbeat:
        import time
        registrations = json.loads(state_path.read_text(encoding="utf-8"))
        while True:
            health={}
            for name, row in registrations.items():
                try:
                    response=session.put(bus["rdc_url"]+"/v2/agents/"+name+"/instances/"+row["instanceId"]+"/heartbeat",headers=registry_headers,timeout=8)
                    if response.status_code==404:
                        registrations[name]=register_runtime(session,bus,registry_headers,name,ROLES[name],bindings)
                    else:response.raise_for_status()
                    health[name]={"healthy":True}
                except requests.RequestException as exc:
                    status=exc.response.status_code if exc.response is not None else None
                    health[name]={"healthy":False,"http_status":status,"error_type":type(exc).__name__}
                    print(json.dumps({"agent":name,"heartbeat_retry":True,**health[name]}),flush=True)
            status={"at":time.time(),"all_healthy":all(item["healthy"] for item in health.values()),"agents":health}
            tmp=root/"heartbeat-status.tmp"
            tmp.write_text(json.dumps(status,indent=2),encoding="utf-8");tmp.replace(root/"heartbeat-status.json")
            time.sleep(8)
    registrations = {}
    for name, port in ROLES.items():
        registrations[name]=register_runtime(session,bus,registry_headers,name,port,bindings)
    state_path.write_text(json.dumps(registrations, indent=2), encoding="utf-8")
    admin = identity["clients"]["report-admin"]
    token = session.post(bus["token_url"], data={"grant_type": "client_credentials", "client_id": admin["client_id"],
                                                "client_secret": admin["client_secret"]}, timeout=10)
    token.raise_for_status()
    admin_headers = {"Authorization": "Bearer " + token.json()["access_token"], "X-Tenant-ID": bus["tenant"]}
    admin_url = bus["rdc_url"] + "/v2/authorization/admin"

    def call(method: str, path: str, body: object = None, allowed: tuple[int, ...] = ()) -> object:
        response = session.request(method, admin_url + path, json=body,
                                   headers={**admin_headers, "Idempotency-Key": str(uuid.uuid4())}, timeout=15)
        if response.status_code in allowed:
            return None
        if not response.ok:
            raise RuntimeError("RDC admin " + path + " HTTP " + str(response.status_code) + " " + response.text[:1000])
        return response.json()

    subject = bus["subject"]
    call("POST", "/users:sync", {"mode": "INCREMENTAL", "iamDomain": identity["issuer"],
                                "users": [{"userId": subject, "displayName": "Annual report local worker", "status": "ACTIVE"}], "removedUserIds": []})
    role_id = "report-demo-executor"
    if call("GET", "/roles/" + role_id, allowed=(404,)) is None:
        call("POST", "/roles", {"roleId": role_id, "name": "Annual report expert access", "enabled": True, "remark": "Local demo only"})
    for path, entries in (("/users/" + subject + "/roles", [{"roleId": role_id}]),
                          ("/roles/" + role_id + "/agents", [{"agentId": name} for name in ROLES])):
        current = call("GET", path)
        call("PUT", path, {"entries": entries, "expectedRevision": current["revision"]})
    preview = call("POST", "/effective-permissions:preview", {"subjectType": "USER", "subjectId": subject})
    (root / "evidence" / "bus-setup.json").write_text(json.dumps({"registered_agents": list(registrations), "permission_preview": preview}, indent=2), encoding="utf-8")
    print(json.dumps({"registered": list(registrations), "real_identity_and_rdc_admin": True}))


if __name__ == "__main__":
    main()
