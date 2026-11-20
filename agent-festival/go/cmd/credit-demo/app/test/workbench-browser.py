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
      reply({plan:'P'+n, version:1, case:'C006', goal:'fixture '+n, hash:'fixture', ok:true, rev:++f.rev, nodes:n === 1 ? f.nodes : [], layers:n === 1 ? f.layers : [], bus:n === 1 ? f.bus : undefined,
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

    def test_sent_remote_call_of_a_waiting_task_is_in_flight_not_live(self):
        # A sent Agent Bus call is still out when the task stops for a check.
        self.page.evaluate("""() => {
          fixture.layers = [['intake'], ['industry']];
          fixture.nodes = [
            {id:'intake', name:'材料受理', kind:'investigate', task:'t', status:'done', execution:{kind:'agent_bus'},
             remote:{updated_at:'2026-11-18T07:57:18Z'}},
            {id:'industry', name:'行业研究员', kind:'investigate', task:'t', deps:['intake'], status:'waiting', activity:'专业 Agent 执行中，查询原任务',
             execution:{kind:'agent_bus'}, remote:{request_id:'q2', task_id:'t2', updated_at:'2026-11-18T07:57:20Z'}}];
        }""")
        card = self.page.locator(".react-flow__node", has_text="行业研究员")
        expect(card).to_contain_text("远端在途 · 最后观察")
        expect(self.page.locator(".cn-row.act")).to_have_count(0)
        expect(self.page.locator(".ce-flow")).to_have_count(0)
        expect(self.page.locator(".cg-head")).to_contain_text("等待核对远端结果")
        if directory := os.environ.get("EVIDENCE_DIR"):
            Path(directory).mkdir(parents=True, exist_ok=True)
            self.page.wait_for_timeout(1500)  # let the canvas fit both cards
            self.page.screenshot(path=str(Path(directory) / "waiting-inflight-call.png"))
        # Once the task runs again the call is drawn as live waiting.
        self.page.evaluate("fixture.statuses.R1 = 'running'")
        expect(card).to_contain_text("专业 Agent 执行中，查询原任务", timeout=10000)
        expect(self.page.locator(".cn-row.act").first).to_be_visible()


    THREE_TIERS = """() => {
      const t0 = '2026-11-20T03:10:00Z', t1 = '2026-11-20T03:10:02Z', t2 = '2026-11-20T03:10:05Z';
      const sendErr = {rpc_id:'q3', method:'SendMessage', started_at:t0, finished_at:t1, outcome:'transport_error', sequence:4};
      const receipt = {rpc_id:'q2', method:'SendMessage', started_at:t0, finished_at:t1, http_status:200, outcome:'task_received', sequence:3};
      const call = (id, extra) => ({request_id:id, task_id:'', agent_started_at:'', intent_saved_at:t0, updated_at:t0, ...extra});
      fixture.layers = [['intake'], ['financial', 'industry', 'legal']];
      fixture.bus = {planned_nodes:4, services:4, linked_tasks:2, completed:1, unresolved:1, failed:0, reused:0};
      fixture.nodes = [
        {id:'intake', name:'材料受理', kind:'investigate', task:'t', status:'done', execution:{kind:'agent_bus'}, remote:call('q0', {task_id:'t0'})},
        // Owner counterexample: a running step whose call has only its intent record.
        {id:'financial', name:'财务分析师', kind:'investigate', task:'t', deps:['intake'], status:'running', activity:'派发远端调用',
         execution:{kind:'agent_bus'}, remote:call('q1', {})},
        {id:'industry', name:'行业研究员', kind:'investigate', task:'t', deps:['intake'], status:'waiting', activity:'专业 Agent 执行中，查询原任务',
         execution:{kind:'agent_bus'}, remote:call('q2', {task_id:'t2', updated_at:t2, gateway:{send_count:1, query_count:0, first_send:receipt, first_receipt:receipt, last_exchange:receipt}})},
        {id:'legal', name:'合规审查', kind:'investigate', task:'t', deps:['intake'], status:'unknown', activity:'Gateway 未回执，查询原任务',
         execution:{kind:'agent_bus'}, remote:call('q3', {status:'unknown', updated_at:t2, gateway:{send_count:1, query_count:0, first_send:sendErr, last_exchange:sendErr, last_error:sendErr}})}];
    }"""

    def tier_lines(self):
        cards = {name: self.page.locator(".react-flow__node", has_text=name) for name in ("财务分析师", "行业研究员", "合规审查")}
        expect(cards["财务分析师"]).to_contain_text("已登记，发送待确认")
        expect(cards["行业研究员"]).to_contain_text("远端在途 · 最后观察")
        expect(cards["合规审查"]).to_contain_text("结果未知 · 最后观察")
        rows = {name: card.locator(".cn-row") for name, card in cards.items()}
        return {name: (row.inner_text(), row.get_attribute("title")) for name, row in rows.items()}

    def test_three_call_tiers_are_stable_and_send_nothing(self):
        self.page.evaluate(self.THREE_TIERS)
        first = self.tier_lines()
        self.assertNotIn("远端在途", first["财务分析师"][0])
        self.assertIn("没有 Gateway 发送记录", first["财务分析师"][1])
        self.assertIn("保留原请求（q3）", first["合规审查"][1])
        expect(self.page.locator(".cg .spin")).to_have_count(0)
        expect(self.page.locator(".ce-flow")).to_have_count(0)
        if directory := os.environ.get("EVIDENCE_DIR"):
            Path(directory).mkdir(parents=True, exist_ok=True)
            self.page.wait_for_timeout(1500)  # let the canvas fit the cards
            self.page.screenshot(path=str(Path(directory) / "three-tiers.png"))
            for name, file in (("财务分析师", "tier1-unconfirmed.png"), ("行业研究员", "tier2-inflight.png"), ("合规审查", "tier3-unknown.png")):
                self.page.locator(".react-flow__node", has_text=name).screenshot(path=str(Path(directory) / file))
        # Polls keep refreshing the same snapshot: the lines do not change.
        polls = self.page.evaluate("fixture.rev")
        self.page.wait_for_function(f"fixture.rev >= {polls + 2}", timeout=10000)
        self.assertEqual(self.tier_lines(), first)
        # Display-only interactions: the call list and a step's detail.
        self.page.locator(".cg-bus-list summary").click()
        expect(self.page.locator(".cg-call-list")).to_contain_text("已登记，发送待确认")
        self.page.locator(".react-flow__node", has_text="财务分析师").click()
        expect(self.page.locator(".cg-detail")).to_contain_text("已登记，发送待确认")
        methods = self.page.evaluate("fixture.sent.map(m => m.method)")
        self.assertTrue(methods)
        self.assertEqual(sorted({m.rsplit(".", 1)[-1] for m in methods}), ["graph", "plans"], methods)
        # A reload fetches the same snapshot again and draws it the same.
        self.page.reload()
        expect(self.page.locator(".wb-resume")).to_be_visible()
        self.page.evaluate(self.THREE_TIERS)
        self.assertEqual(self.tier_lines(), first)
        methods = self.page.evaluate("fixture.sent.map(m => m.method)")
        self.assertEqual(sorted({m.rsplit(".", 1)[-1] for m in methods}), ["graph", "plans"], methods)

if __name__ == "__main__":
    unittest.main()
