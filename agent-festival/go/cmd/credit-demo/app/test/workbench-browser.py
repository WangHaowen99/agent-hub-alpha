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

DIST = Path(__file__).resolve().parents[2] / "workbench-plugin/frontend/dist"
TRANSPORT = r"""
window.fixture = {statuses: {R1:'waiting', R2:'running'}, sent:[], deferred:[], delay:false, rev:1, nodes:[], layers:[]};
window.WebSocket = class {
  static OPEN = 1;
  readyState = 1;
  constructor() { setTimeout(() => this.onopen?.(), 0); }
  send(raw) {
    const m = JSON.parse(raw), f = window.fixture;
    f.sent.push(m);
    const reply = payload => queueMicrotask(() => this.onmessage?.({data:JSON.stringify({type:'res', id:m.id, ok:true, payload})}));
    if (m.method.endsWith('.plans')) {
      reply({plans:[1,2].map(n => ({plan:'P'+n, version:1, case:'C006', goal:'fixture '+n, ok:true, run:'R'+n, run_status:f.statuses['R'+n]}))});
    } else if (m.method.endsWith('.graph')) {
      const n = m.params.plan === 'P2' || m.params.run === 'R2' ? 2 : 1;
      reply({plan:'P'+n, version:1, case:'C006', goal:'fixture '+n, hash:'fixture', ok:true, rev:++f.rev, nodes:n === 1 ? f.nodes : [], layers:n === 1 ? f.layers : [],
        run:{id:'R'+n, status:f.statuses['R'+n], leases:1, url:''}});
    } else if (m.method.endsWith('.resume')) {
      const finish = () => { f.statuses[m.params.run] = 'running'; reply({run:m.params.run, status:'queued'}); };
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

    def test_receipt_survives_control_unmount_and_is_scoped_to_run(self):
        self.resume()
        expect(self.page.locator(".wb-resume")).to_have_count(0)
        expect(self.page.locator(".wb-notice")).to_contain_text("R1")
        expect(self.page.locator(".wb-notice")).to_contain_text("不表示任务已完成")
        if directory := os.environ.get("EVIDENCE_DIR"):
            Path(directory).mkdir(parents=True, exist_ok=True)
            self.page.screenshot(path=str(Path(directory) / "resume-receipt.png"))
        self.page.locator(".wb-item", has_text="P2 v1").click()
        expect(self.page.locator(".wb-notice")).to_have_count(0)

    def test_late_receipt_does_not_leak_into_another_task(self):
        self.page.evaluate("fixture.delay = true")
        self.resume()
        self.page.locator(".wb-item", has_text="P2 v1").click()
        expect(self.page.locator(".cg-head")).to_contain_text("P2 v1")
        self.page.evaluate("fixture.deferred.forEach(fn => fn())")
        expect(self.page.locator(".wb-notice")).to_have_count(0)

    def test_blocked_is_not_running_or_resumable(self):
        self.page.evaluate("fixture.statuses.R1 = 'blocked'")
        expect(self.page.locator(".cg-head")).to_contain_text("任务受阻")
        expect(self.page.locator(".wb-item.on .chip")).to_contain_text("任务受阻", timeout=10000)
        expect(self.page.locator(".wb-resume")).to_have_count(0)
        expect(self.page.locator(".wb-blocked")).to_contain_text("维护人员")

    def test_running_step_of_a_waiting_task_is_drawn_stopped(self):
        # The local black box is still listed as running after the task stopped.
        self.page.evaluate("""() => {
          fixture.layers = [['intake'], ['related']];
          fixture.nodes = [
            {id:'intake', name:'材料受理', kind:'investigate', task:'t', status:'done', execution:{kind:'agent_bus'},
             remote:{updated_at:'2026-11-18T07:57:23Z', gateway:{last_error:{finished_at:'2026-11-18T07:57:23Z'}}}},
            {id:'related', name:'关联方排查', kind:'blackbox', task:'t', deps:['intake'], status:'running', activity:'读关联方名单 · C006', execution:{kind:'loop_forest'},
             branches:[{goal:'a', status:'started', title:'王氏汽配', activity:'查征信摘要'}]}];
        }""")
        host = self.page.locator(".react-flow__node", has_text="关联方排查")
        expect(host).to_contain_text("已停止 · 最后观察")
        expect(self.page.locator(".react-flow__node", has_text="王氏汽配")).to_contain_text("已停止")
        expect(self.page.locator(".cg .spin")).to_have_count(0)
        expect(self.page.locator(".ce-flow")).to_have_count(0)
        expect(self.page.locator(".lf .sway")).to_have_count(0)
        expect(self.page.locator(".cg-head")).to_contain_text("等待核对远端结果")
        if directory := os.environ.get("EVIDENCE_DIR"):
            Path(directory).mkdir(parents=True, exist_ok=True)
            self.page.screenshot(path=str(Path(directory) / "waiting-stopped-step.png"))
        # The same step is live again once the task runs.
        self.page.evaluate("fixture.statuses.R1 = 'running'")
        expect(host).to_contain_text("读关联方名单", timeout=10000)
        expect(self.page.locator(".cg .spin").first).to_be_visible()


if __name__ == "__main__":
    unittest.main()
