// 新开对话 → 选"授信试用"专家 → 发送 MSG → 等待 WAIT 毫秒
export default async (page) => {
  await page.goto("http://localhost:15182/chat/new", { waitUntil: "networkidle" });
  await page.waitForTimeout(1500);
  const skip = page.getByText("跳过", { exact: true });
  if (await skip.count()) await skip.first().click();
  await page.getByText("专家", { exact: true }).first().click();
  await page.waitForTimeout(1500);
  await page.getByText("我的专家", { exact: true }).first().click();
  await page.waitForTimeout(2000);
  await page.getByText("授信试用", { exact: true }).first().hover();
  await page.locator('[data-testid="page-card-default-button"][data-variant="credit-pilot"]').click({ force: true });
  await page.waitForTimeout(2000);
  const box = page.locator('[data-testid="chat-panel-input"]');
  await box.click();
  await page.keyboard.insertText(process.env.MSG ?? "打开授信控制台");
  await page.waitForTimeout(300);
  await page.locator('[data-testid="chat-panel-input-send"]').click();
  await page.waitForTimeout(Number(process.env.WAIT ?? 30000));
};
