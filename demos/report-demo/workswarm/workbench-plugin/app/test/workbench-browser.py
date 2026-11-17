"""Real built UI, simulated RPC transport; NOT a WorkSwarm/Bus acceptance test.

Run after npm run build: python test/workbench-browser.py
Requires Playwright; set BROWSER_EXECUTABLE to use an installed Chromium browser.
No model, credentials, or external services are used.
"""
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
import os
from pathlib import Path
import threading
import unittest
from playwright.sync_api import sync_playwright, expect

DIST = Path(__file__).resolve().parents[2] / "frontend/dist"
TRANSPORT = r"""
window.fixture = {statuses: {R1:'waiting', R2:'running'}, sent:[], deferred:[], delay:false, rev:1, human:false};
window.WebSocket = class {
  static OPEN = 1;
  readyState = 1;
  constructor() { setTimeout(() => this.onopen?.(), 0); }
  send(raw) {
    const m = JSON.parse(raw), f = window.fixture;
    f.sent.push(m);
    const reply = payload => queueMicrotask(() => this.onmessage?.({data:JSON.stringify({type:'res', id:m.id, ok:true, payload})}));
    if (m.method.endsWith('.plans')) {
      reply({plans:[1,2].map(n => ({plan:'T-'+String(n).repeat(32), version:1, case:'C006', goal:'fixture '+n, ok:true, run:'T-'+String(n).repeat(32), run_status:f.statuses['R'+n]}))});
    } else if (m.method.endsWith('.graph')) {
      const n = (m.params.plan || m.params.run) === 'T-'+ '2'.repeat(32) ? 2 : 1;
      reply({plan:'T-'+String(n).repeat(32), version:1, case:'C006', goal:'fixture '+n, hash:'fixture', ok:true, rev:String(++f.rev), nodes:[], layers:[],
        run:{id:'T-'+String(n).repeat(32), status:f.statuses['R'+n], leases:1, url:''},
        ...(f.human ? {human_review:{url:'http://127.0.0.1:8190/ui/tasks/T-'+String(n).repeat(32),checks:[{node:'check',check_artifact:'fixture',object_digest:'sha256:fixture'}]}} : {})});
    } else if (m.method.endsWith('.resume')) {
      const finish = () => { f.statuses['R'+m.params.run.slice(-1)] = 'running'; reply({run:m.params.run, status:'queued'}); };
      if (f.delay) f.deferred.push(finish); else finish();
    }
  }
};
"""


class QuietHandler(SimpleHTTPRequestHandler):
    def log_message(self, *args):
        pass


class WorkbenchBrowserTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = ThreadingHTTPServer(("127.0.0.1", 0), partial(QuietHandler, directory=str(DIST)))
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.pw = sync_playwright().start()
        cls.browser = cls.pw.chromium.launch(headless=True, executable_path=os.environ.get("BROWSER_EXECUTABLE") or None)

    @classmethod
    def tearDownClass(cls):
        cls.browser.close()
        cls.pw.stop()
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join()

    def setUp(self):
        self.page = self.browser.new_page()
        self.page.add_init_script(TRANSPORT)
        self.page.goto(f"http://127.0.0.1:{self.server.server_port}/index.html")
        expect(self.page.locator(".wb-resume")).to_be_visible()

    def tearDown(self):
        self.page.close()

    def resume(self):
        self.page.locator(".wb-resume input").fill("fixture-only")
        self.page.locator(".wb-resume button").click()

    def test_human_fallback_is_visible_and_disappears_after_resolved_projection(self):
        self.page.evaluate("window.fixture.human=true")
        link=self.page.get_by_role("link",name="前往人工处理")
        expect(link).to_be_visible()
        expect(link).to_have_attribute("href","http://127.0.0.1:8190/ui/tasks/T-"+"1"*32)
        expect(link).to_have_attribute("target","_blank")
        self.page.evaluate("window.fixture.human=false")
        expect(link).to_have_count(0)

    def test_receipt_survives_control_unmount_and_is_scoped_to_run(self):
        self.resume()
        expect(self.page.locator(".wb-resume")).to_have_count(0)
        expect(self.page.locator(".wb-notice").filter(has_text="恢复请求")).to_contain_text("T-11111111111111111111111111111111")
        expect(self.page.locator(".wb-notice").filter(has_text="恢复请求")).to_contain_text("不表示任务已完成")
        if directory := os.environ.get("EVIDENCE_DIR"):
            Path(directory).mkdir(parents=True, exist_ok=True)
            self.page.screenshot(path=str(Path(directory) / "resume-receipt.png"))
        self.page.locator(".wb-item", has_text="T-22222222222222222222222222222222 v1").click()
        expect(self.page.locator(".wb-notice").filter(has_text="恢复请求")).to_have_count(0)

    def test_late_receipt_does_not_leak_into_another_task(self):
        self.page.evaluate("fixture.delay = true")
        self.resume()
        self.page.locator(".wb-item", has_text="T-22222222222222222222222222222222 v1").click()
        expect(self.page.locator(".cg-head")).to_contain_text("T-22222222222222222222222222222222 v1")
        self.page.evaluate("fixture.deferred.forEach(fn => fn())")
        expect(self.page.locator(".wb-notice").filter(has_text="恢复请求")).to_have_count(0)

    def test_blocked_is_not_running_or_resumable(self):
        self.page.evaluate("fixture.statuses.R1 = 'blocked'")
        expect(self.page.locator(".cg-head")).to_contain_text("任务受阻")
        expect(self.page.locator(".wb-item.on .chip")).to_contain_text("任务受阻", timeout=10000)
        expect(self.page.locator(".wb-resume")).to_have_count(0)
        expect(self.page.locator(".wb-blocked")).to_contain_text("维护人员")


if __name__ == "__main__":
    unittest.main()
