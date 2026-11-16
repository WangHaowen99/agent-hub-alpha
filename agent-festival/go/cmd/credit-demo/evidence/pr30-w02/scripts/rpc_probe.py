import asyncio, json, sys, websockets
async def main():
    out = []
    async with websockets.connect("ws://127.0.0.1:5292/ws", max_size=2**24) as ws:
        cases = [("plugin.credit_workbench.plans", {}), ("plugin.credit_workbench.graph", {"plan": "NOPE", "v": "1"}),
                 ("plugin.credit_workbench.approve", {}), ("plugin.credit_workbench.resume", {}),
                 ("plugin.credit_workbench.no_such_method", {})]
        for i, (m, p) in enumerate(cases):
            await ws.send(json.dumps({"type": "req", "id": f"probe{i}", "method": m, "params": p}))
            while True:
                msg = json.loads(await asyncio.wait_for(ws.recv(), 20))
                if msg.get("type") == "res" and msg.get("id") == f"probe{i}":
                    out.append({"method": m, "params": p, "ok": msg.get("ok"), "code": msg.get("code"), "error": msg.get("error"), "payload": msg.get("payload")})
                    break
    print(json.dumps(out, ensure_ascii=False, indent=1))
asyncio.run(main())
