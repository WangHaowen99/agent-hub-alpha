export default async (page) => {
  const panelToggle = page.locator('button[aria-label*="进度"], button[title*="进度"]');
  const card = page.locator(".mcp-app-view__approval").first();
  const btns = await card.locator("button").allInnerTexts().catch(() => []);
  await card.scrollIntoViewIfNeeded().catch(() => {});
  return { approvalButtons: btns, html: (await card.innerText().catch(() => "")).slice(0, 300) };
};
