// 模拟 Manager 登录：以 U 身份打开企业版前端，写入联合登录记录（用户令牌），刷新后截图
import { readFileSync } from "node:fs";
const U = process.env.U ?? "alice";
const BASE = process.env.BASE ?? "http://localhost:15184";
const Q = (u) => `?user_id=${u}&group_id=default&bot_id=default&jiuwenclaw_id=debug-gateway&debug_context=1`;
export default async (page) => {
  const token = process.env.NO_TOKEN ? "" : readFileSync(process.env.TOKEN_FILE ?? `/tmp/aha-tokens/${U}`, "utf8").trim();
  await page.goto(`${BASE}/chat/new${Q(U)}`, { waitUntil: "domcontentloaded" });
  await page.evaluate(([u, t]) => {
    if (t) localStorage.setItem("openjiuwen_federation", JSON.stringify({ user_id: u, token_type: "Bearer", access_token: t }));
    else localStorage.removeItem("openjiuwen_federation");
  }, [U, token]);
  await page.goto(`${BASE}/chat/new${Q(U)}`, { waitUntil: "load" });
  await page.waitForTimeout(6000);
  const skip = page.getByText("跳过", { exact: true });
  if (await skip.count()) await skip.first().click();
  return { transport: await page.evaluate(() => [window.__JIUWENSWARM_EDITION__, window.__JIUWEN_WEB_TRANSPORT__, window.__JIUWEN_LOGIN_AUTH_SIMULATE__]) };
};
