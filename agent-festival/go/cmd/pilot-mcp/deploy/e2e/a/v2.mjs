async function appFrame(page) {
  for (const f of page.frames()) if (await f.locator("#f").count().catch(() => 0)) return f;
  throw new Error("app frame not found");
}
async function approveOnce(page) {
  const card = page.locator(".mcp-app-view__approval").first();
  await card.waitFor({ timeout: 15000 });
  const text = (await card.innerText()).replace(/\s+/g, " ").slice(0, 400);
  await card.getByText("允许", { exact: true }).click();
  await page.waitForTimeout(3500);
  return text;
}
export default async (page) => {
  const log = [];
  let app = await appFrame(page);
  await app.locator("button.primary").click();
  log.push({ step: "submit", approval: await approveOnce(page), receipt: await app.locator("#receipt").innerText(), cmd: await app.locator("#cmdid").innerText() });
  await app.locator("#resend").click();
  log.push({ step: "resend", approval: await approveOnce(page), receipt: await app.locator("#receipt").innerText() });
  await app.locator("#tamper").click();
  log.push({ step: "tamper", approval: await approveOnce(page), receipt: await app.locator("#receipt").innerText() });
  await page.waitForTimeout(4000);
  log.push({ tasks: (await app.locator("#tasks").innerText()).slice(0, 600) });
  await page.locator(".mcp-app-view__frame").first().scrollIntoViewIfNeeded();
  return log;
};
