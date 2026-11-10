export default async (page) => {
  const card = page.locator(".mcp-app-view__approval").first();
  if (await card.count()) { await card.getByText("在此应用中始终允许", { exact: true }).click(); await page.waitForTimeout(4500); }
  for (const f of page.frames()) if (await f.locator("#tasks").count().catch(() => 0)) {
    await page.locator(".mcp-app-view__frame").first().scrollIntoViewIfNeeded();
    return (await f.locator("#tasks").innerText()).split("\n").filter((l) => /^T\d|已终结|结果未知|处理中|批准/.test(l)).slice(0, 30).join(" | ");
  }
  return "no console";
};
