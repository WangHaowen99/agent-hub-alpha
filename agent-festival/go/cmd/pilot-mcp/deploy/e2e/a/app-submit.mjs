// 在控制台里以用户点击提交一笔（CUST），批准一次宿主确认，等待后返回首条任务
async function appFrame(page) {
  for (const f of page.frames()) if (await f.locator("#f").count().catch(() => 0)) return f;
  throw new Error("app frame not found");
}
export default async (page) => {
  const app = await appFrame(page);
  await app.locator('input[name="customer_id"]').fill(process.env.CUST ?? "C-K3");
  await app.locator("button.primary").click();
  const card = page.locator(".mcp-app-view__approval").first();
  await card.waitFor({ timeout: 15000 });
  await card.getByText("允许", { exact: true }).click();
  await page.waitForTimeout(5000);
  await page.locator(".mcp-app-view__frame").first().scrollIntoViewIfNeeded();
  return (await app.locator("body").innerText()).split("\n").slice(0, 8).join(" | ");
};
