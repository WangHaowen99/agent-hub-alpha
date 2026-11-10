export default async (page) => (await page.evaluate(() => document.body.innerText)).slice(-Number(process.env.N ?? 1800));
