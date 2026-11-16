import sys
from pw import *
MSG = sys.argv[1]; TAG = sys.argv[2]
before = {p["plan"] for p in go("/api/plans").get("plans", [])}
with sync_playwright() as p:
    ctx, page = launch(p)
    page.goto(BASE, wait_until="domcontentloaded"); page.wait_for_timeout(6000); dismiss(page)
    page.get_by_text("专家", exact=True).first.click(); page.wait_for_timeout(2500)
    page.get_by_text("我的专家", exact=True).first.click(); page.wait_for_timeout(2500)
    shot(page, f"{TAG}-01-experts")
    page.get_by_text("授信编排助手").first.click(); page.wait_for_timeout(2500)
    use = page.get_by_role("button", name="使用")
    if use.count(): use.first.click(); page.wait_for_timeout(4000)
    shot(page, f"{TAG}-02-expert-chat")
    box = page.locator("[data-testid=\"chat-panel-input\"]").first
    box.click(); page.keyboard.insert_text(MSG); page.wait_for_timeout(500); page.keyboard.press("Enter")
    t0 = time.time(); new = None
    while time.time() - t0 < 300:
        page.wait_for_timeout(5000)
        plans = go("/api/plans").get("plans", [])
        new = [x for x in plans if x["plan"] not in before]
        if new and time.time() - t0 > 20: page.wait_for_timeout(15000); break
    shot(page, f"{TAG}-03-chat-after")
    print(json.dumps({"elapsed": round(time.time() - t0), "new_plans": new}, ensure_ascii=False))
    print(page.evaluate("document.body.innerText")[-1500:])
    ctx.close()
