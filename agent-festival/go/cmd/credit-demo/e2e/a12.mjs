// A12 end-to-end check for 编排台. Usage (needs playwright-core and Chrome):
//   BASE=http://<workswarm>/ PICK="P20 v1" PLAN="reload@8,restart@18" \
//   RESTART='<command that restarts the WorkSwarm gateway>' node a12.mjs
//   BASE=... PICK="P21 v1" PLAN="offline:6@5,godown@15,goup@25" \
//   GODOWN='<stop the orchestration service>' GOUP='<start it>' node a12.mjs
// The plan must be awaiting approval; the demo password is used to approve it.
// A12 end to end on the remote WorkSwarm: approve PICK in 编排台, then during
// the run reload WorkSwarm, restart its gateway, and cut the browser's network.
// Every 500 ms record what the page shows; afterwards check that no step went
// backwards, the progress count never fell, and the view came back by itself.
import { chromium } from "playwright-core";
import { execFileSync } from "node:child_process";
import { writeFileSync } from "node:fs";

const BASE = process.env.BASE;                // the WorkSwarm address, e.g. http://host:port/
if (!BASE || !process.env.PICK) throw new Error("set BASE and PICK");
const PICK = process.env.PICK;              // e.g. "P20 v1"
const RESTART = process.env.RESTART ?? "";  // shell command that restarts the WorkSwarm gateway
const OUT = process.env.OUT ?? "a12";
const rank = { plan: 0, pending: 0, running: 1, waiting: 1, unknown: 2, done: 3, reused: 3, failed: 3, skipped: 3 };

const browser = await chromium.launch({ executablePath: process.env.CHROME || undefined, args: ["--no-proxy-server"] });
const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
const page = await ctx.newPage();
const cdp = await ctx.newCDPSession(page);
await cdp.send("Network.enable");
await cdp.send("Network.setCacheDisabled", { cacheDisabled: true });

const frameOf = () => page.frames().find((f) => f.url().includes("/api/application-plugins/credit-workbench/"));
async function openWorkbench(reload) {
  if (reload) await page.reload({ waitUntil: "domcontentloaded" });
  else await page.goto(BASE, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(9000);
  for (const t of ["跳过", "以后再说", "稍后"]) {
    const b = page.getByText(t, { exact: true });
    if (await b.count()) await b.first().click().catch(() => {});
  }
  await page.getByText("编排台", { exact: true }).first().click();
  for (let i = 0; i < 40 && !frameOf(); i++) await page.waitForTimeout(250);
  await page.waitForTimeout(3000);
}

const log = [];
const t0 = Date.now();
let phase = "approve";
async function sample() {
  const f = frameOf();
  const row = { t: (Date.now() - t0) / 1000, phase };
  if (!f) { row.gap = "no frame"; log.push(row); return; }
  try {
    Object.assign(row, await f.evaluate(() => ({
      selected: document.querySelector(".wb-item.on b")?.textContent ?? "",
      chip: document.querySelector(".cg-head .chip:not(.soft)")?.textContent ?? "",
      banner: document.querySelector(".wb-err")?.textContent ?? "",
      graph: !!document.querySelector(".cg"),
      nodes: Object.fromEntries([...document.querySelectorAll(".react-flow__node")].map((el) => {
        const c = el.querySelector(".cn, .cb");
        const s = c ? ([...c.classList].find((x) => x.startsWith("s-")) ?? "").slice(2) : "";
        return [el.getAttribute("data-id"), s];
      }).filter(([, s]) => s)),
    })));
  } catch (e) { row.gap = "eval: " + e.message.slice(0, 60); }
  log.push(row);
}
const shot = (name) => page.screenshot({ path: `${OUT}-${name}.png` }).catch(() => {});

await openWorkbench(false);
let f = frameOf();
await f.locator(".wb-item", { hasText: PICK }).first().click();
await page.waitForTimeout(2500);
await f.getByRole("button", { name: "批准", exact: true }).click();
await f.locator("input[type=password]").fill("demo");
await f.getByRole("button", { name: "批准执行" }).click();
phase = "run";

// Disturbances, each at a time after approval: reload | restart (WorkSwarm
// gateway) | offline:<seconds> | godown (stop the orchestration service) | goup.
const plan = (process.env.PLAN ?? "reload@8,restart@18,offline:6@45").split(",").map((x) => {
  const [what, at] = x.split("@"); return { what, at: Number(at), done: false };
});
const sh = (cmd) => { try { execFileSync("sh", ["-c", cmd], { stdio: "ignore" }); } catch { /* reported by the log */ } };
let tApprove = (Date.now() - t0) / 1000, offUntil = 0;
for (let i = 0; i < 360; i++) {
  await sample();
  const last = log[log.length - 1], t = last.t - tApprove;
  const next = plan.find((d) => !d.done && t >= d.at);
  if (next) {
    next.done = true;
    const [what, arg] = next.what.split(":");
    await shot(`d-${what}-before`);
    phase = what;
    if (what === "reload") { await openWorkbench(true); await sample(); }
    if (what === "restart") sh(RESTART);
    if (what === "offline") { offUntil = t + Number(arg || 6); await cdp.send("Network.emulateNetworkConditions", { offline: true, latency: 0, downloadThroughput: -1, uploadThroughput: -1 }); }
    if (what === "godown") sh(process.env.GODOWN);
    if (what === "goup") sh(process.env.GOUP);
    await page.waitForTimeout(1500); await sample(); await shot(`d-${what}-during`);
  }
  if (offUntil && t >= offUntil) {
    offUntil = 0;
    await cdp.send("Network.emulateNetworkConditions", { offline: false, latency: 0, downloadThroughput: -1, uploadThroughput: -1 });
  }
  if (/验证通过|验证未通过/.test(last.chip) && plan.every((d) => d.done)) { await page.waitForTimeout(1500); await sample(); break; }
  await page.waitForTimeout(500);
}
await shot("6-final");
writeFileSync(`${OUT}-log.json`, JSON.stringify(log, null, 1));

// ---- checks ----
const problems = [];
const best = {};
let maxDone = -1;
for (const r of log) {
  if (!r.nodes) continue;
  for (const [id, s] of Object.entries(r.nodes)) {
    if (best[id] !== undefined && rank[s] < rank[best[id]]) problems.push(`${r.t.toFixed(1)}s ${r.phase}: ${id} ${best[id]} -> ${s}`);
    if (best[id] === undefined || rank[s] > rank[best[id]]) best[id] = s;
  }
  const m = /执行中 (\d+)\/\d+/.exec(r.chip ?? "");
  const done = /已完成|验证/.test(r.chip ?? "") ? 99 : m ? Number(m[1]) : -1;
  if (done >= 0) { if (done < maxDone) problems.push(`${r.t.toFixed(1)}s ${r.phase}: progress ${maxDone} -> ${done}`); maxDone = Math.max(maxDone, done); }
}
const afterReload = log.find((r) => r.phase === "reload" && r.selected);
const banners = log.filter((r) => r.banner).map((r) => `${r.t.toFixed(1)}s ${r.phase}: ${r.banner}`);
const graphLost = log.filter((r) => r.phase === "godown" && !r.gap && !r.graph);
console.log(JSON.stringify({
  samples: log.length, gaps: log.filter((r) => r.gap).length,
  selected_after_reload: afterReload?.selected, banner_samples: banners.length, first_banner: banners[0], last_banner: banners[banners.length - 1],
  graph_hidden_while_service_down: graphLost.length, phases_with_frame_gaps: [...new Set(log.filter((r) => r.gap).map((r) => r.phase))], final_chip: log[log.length - 1].chip, final_nodes: log[log.length - 1].nodes,
  regressions: problems,
}, null, 1));
await browser.close();
