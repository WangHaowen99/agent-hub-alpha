"""Persistent-profile Playwright helpers for the pr30-w02 WorkSwarm (127.0.0.1:5292)."""
import json, os, sys, time, urllib.request
from playwright.sync_api import sync_playwright
BASE = "http://127.0.0.1:5292/"
GO = "http://127.0.0.1:5291"
SHOTS = os.path.expanduser("<workdir>/shots")
PROFILE = os.path.expanduser("<workdir>/profile")
PW = open(os.environ["DEMO_PASSWORD_FILE"]).read().strip()
_opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
def go(path):
    with _opener.open(GO + path, timeout=20) as r: return json.load(r)
def state(): return go("/debug/state")
def launch(p, headless=True):
    ctx = p.chromium.launch_persistent_context(PROFILE, headless=headless, viewport={"width": 1440, "height": 1000}, args=["--no-proxy-server"])
    page = ctx.pages[0] if ctx.pages else ctx.new_page()
    return ctx, page
def shot(page, name): page.screenshot(path=f"{SHOTS}/{name}.png"); return f"{SHOTS}/{name}.png"
def dismiss(page):
    for t in ["跳过", "以后再说", "稍后", "我知道了", "关闭"]:
        b = page.get_by_text(t, exact=True)
        try:
            if b.count(): b.first.click(timeout=1500); page.wait_for_timeout(500)
        except Exception: pass
def frame(page):
    for f in page.frames:
        if "/api/application-plugins/credit-workbench/" in f.url: return f
    return None
def open_workbench(page, reload=False):
    if reload: page.reload(wait_until="domcontentloaded")
    else: page.goto(BASE, wait_until="domcontentloaded")
    page.wait_for_timeout(6000); dismiss(page)
    page.get_by_text("编排台", exact=True).first.click()
    for _ in range(60):
        if frame(page): break
        page.wait_for_timeout(250)
    page.wait_for_timeout(2500)
    return frame(page)
