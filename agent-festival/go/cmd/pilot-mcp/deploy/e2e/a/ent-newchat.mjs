// 以 U 身份新开对话 → 选"授信试用"专家 → 发送 MSG → 等待 WAIT 毫秒
const U = process.env.U ?? "alice";
const Q = `?user_id=${U}&group_id=default&bot_id=default&jiuwenclaw_id=debug-gateway&debug_context=1`;
export default async (page) => {
  const net = [];
  if (process.env.NETLOG) page.on("response", async (r) => {
    const u = new URL(r.url());
    const pd = r.request().postData() ?? "";
    if (!/mcp/i.test(u.pathname + pd) && r.status() < 400) return;
    if (/config$/.test(u.pathname)) return;
    let body = ""; try { body = await r.text(); } catch {}
    net.push(`${r.status()} ${r.request().method()} ${u.host}${u.pathname} ${pd.slice(0, 90)} => ${body.slice(0, 200)}`);
  });
  await page.goto(`http://localhost:15184/chat/new${Q}`, { waitUntil: "load" });
  await page.waitForTimeout(5000);
  const skip = page.getByText("跳过", { exact: true });
  if (await skip.count()) await skip.first().click();
  await page.getByText("专家", { exact: true }).first().click();
  await page.waitForTimeout(2000);
  await page.getByText("我的专家", { exact: true }).first().click();
  await page.waitForTimeout(2500);
  await page.getByText("授信试用", { exact: true }).first().hover();
  const use = page.locator('[data-testid="page-card-default-button"][data-variant="credit-pilot"]');
  if (await use.count()) await use.click({ force: true });
  else await page.getByText("使用", { exact: true }).first().click();
  await page.waitForTimeout(2500);
  const box = page.locator('[data-testid="chat-panel-input"]');
  await box.click();
  await page.keyboard.insertText(process.env.MSG ?? "打开授信控制台");
  await page.waitForTimeout(300);
  await page.locator('[data-testid="chat-panel-input-send"]').click();
  await page.waitForTimeout(Number(process.env.WAIT ?? 40000));
  if (process.env.NETLOG) return net.slice(0, 14);
  return (await page.evaluate(() => document.body.innerText)).slice(-Number(process.env.N ?? 900));
};
