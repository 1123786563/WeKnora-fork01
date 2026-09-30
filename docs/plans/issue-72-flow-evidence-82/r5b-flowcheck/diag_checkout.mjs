// r5b-flowcheck diagnosis: dump what the checkout face actually shows.
import { login, requirePlaywright } from '../_browser_lib.mjs';
const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
const { chromium } = requirePlaywright();
const browser = await chromium.launch();
const page = await (await browser.newContext()).newPage();
page.on('console', (m) => { if (m.type() === 'error') console.log('[console.error]', m.text()); });
page.on('response', (r) => {
  if (r.url().includes('/api/') && r.status() >= 400) console.log('[http]', r.status(), r.url().replace(WEB, ''));
});
await login(page, WEB, process.env.FLOW82_EMAIL_A, process.env.FLOW82_R5_PW);
await page.goto(`${WEB}/platform/billing/checkout`);
await page.waitForTimeout(8000);
console.log('---- main text ----');
console.log((await page.locator('main').innerText().catch(() => '(no main)')).slice(0, 2000));
await page.screenshot({ path: new URL('.', import.meta.url).pathname + 'diag-checkout.png', fullPage: true });
await browser.close();
