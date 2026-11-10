// 用法：node step.mjs <action.mjs> [shot.png]
// 连接常驻 Chrome（CDP 9333），对当前页执行 action 模块的 default(page)，然后截图并打印可见文字。
import { chromium } from "playwright-core";
import { pathToFileURL } from "node:url";
const [action, shot = "shot.png"] = process.argv.slice(2);
const browser = await chromium.connectOverCDP("http://127.0.0.1:9333");
const ctx = browser.contexts()[0];
const page = ctx.pages().find((p) => p.url() !== "about:blank") ?? ctx.pages()[0] ?? (await ctx.newPage());
await page.setViewportSize({ width: 1440, height: 1000 });
if (action) {
  const mod = await import(pathToFileURL(action).href + "?t=" + Date.now());
  const out = await mod.default(page);
  if (out !== undefined) console.log("RESULT:", typeof out === "string" ? out : JSON.stringify(out, null, 1));
}
await page.screenshot({ path: shot });
const text = await page.evaluate(() => document.body.innerText);
console.log("URL:", page.url());
console.log(text.slice(0, 2500));
await browser.close().catch(() => {});
