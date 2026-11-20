import os, sys, json
sys.path.insert(0, os.path.expanduser("<workdir>"))
from pw import *
st = state()
reasons = {}
for k, r in st["runs"].items():
    reasons.setdefault(r.get("final_output") or "", []).append(k)
rec = {"blocked_reasons": {k: sorted(v) for k, v in reasons.items()}, "statuses": {k: r["status"] for k, r in st["runs"].items()}}
with sync_playwright() as p:
    ctx, page = launch(p)
    f = open_workbench(page)
    f.locator(".wb-item", has_text="P9 v").first.click(); page.wait_for_timeout(3500)
    rec["P9_list_chip"] = f.locator(".wb-item.on .chip").first.inner_text()
    rec["P9_head_chip"] = f.locator(".cg-head .chip:not(.soft)").first.inner_text()
    rec["P9_resume_form"] = f.locator(".wb-resume").count()
    rec["P9_blocked_banner"] = f.locator(".wb-blocked").first.inner_text() if f.locator(".wb-blocked").count() else None
    page.screenshot(path=f"{SHOTS}/e196-old-task-blocked.png")
    ctx.close()
json.dump(rec, open(os.path.expanduser("<workdir>/evidence/e196/old-tasks-blocked.json"), "w"), ensure_ascii=False, indent=1)
print(json.dumps(rec, ensure_ascii=False, indent=1))
