#!/bin/bash
# 打印 Go 桩服务最近 N 次调用看到的身份
curl -s --noproxy '*' http://127.0.0.1:15183/debug/state | python3 -I -c '
import json,sys; d=json.load(sys.stdin); n=int(sys.argv[1])
for c in d["pilot"]["calls"][-n:]:
    i=c["identity"]; print(c["at"][11:19], c["tool"].replace("pilot_",""), i["issuer"], "verified="+str(i.get("user_verified",False)), "actor="+str(i.get("actor")), "aud="+str(i.get("user_audience")), i.get("problems") or "")' "${1:-6}"
