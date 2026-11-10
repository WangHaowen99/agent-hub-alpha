async function appFrame(page) {
  for (const f of page.frames()) if (await f.locator("#f").count().catch(() => 0)) return f;
  throw new Error("app frame not found");
}
async function approveOnce(page) {
  const card = page.locator(".mcp-app-view__approval").first();
  await card.waitFor({ timeout: 15000 });
  await card.getByText("允许", { exact: true }).click();
}
const firstTask = async (app) => (await app.locator(".task").first().innerText()).replace(/\s+/g, " ");
export default async (page) => {
  const log = [];
  const app = await appFrame(page);
  for (const scenario of ["timeout", "duplicate_receipt"]) {
    await app.locator('select[name="scenario"]').selectOption(scenario);
    await app.locator('input[name="customer_id"]').fill(scenario === "timeout" ? "C-T" : "C-D");
    await app.locator("button.primary").click();
    await approveOnce(page);
    await page.waitForTimeout(3500);
    log.push({ scenario, right_after: await firstTask(app) });
    await page.waitForTimeout(10000);
    log.push({ scenario, after_10s: await firstTask(app) });
  }
  await page.locator(".mcp-app-view__frame").first().scrollIntoViewIfNeeded();
  return log;
};
