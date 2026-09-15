// Chat-route screenshot harness v2 — retry session view until painted.
import { chromium } from 'playwright-core';
const WEB = 'http://localhost:5181';
const API = 'http://127.0.0.1:8080';
const OUT = process.argv[2];
const loginRes = await fetch(API + '/api/v1/auth/login', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email: 'uimig@local.dev', password: 'Uimig-2026-Tailwind' }) });
const loginBody = await loginRes.json();
if (!loginRes.ok || !loginBody?.token) throw new Error('login failed');
const token = loginBody.token;
const tenantId = String(loginBody.active_tenant?.id ?? loginBody.user?.tenant_id ?? '');
const storageState = { cookies: [], origins: [{ origin: WEB, localStorage: [ { name: 'weknora_token', value: token }, { name: 'weknora_refresh_token', value: loginBody.refresh_token ?? '' }, { name: 'weknora_selected_tenant_id', value: tenantId } ] }] };
const browser = await chromium.launch();
const ctx = await browser.newContext({ storageState, viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
const page = await ctx.newPage();
async function dismissTour() {
  for (let i = 0; i < 8; i++) {
    const skipBtn = page.locator('button', { hasText: /跳过/ }).first();
    if (await skipBtn.count() && await skipBtn.isVisible().catch(() => false)) {
      await skipBtn.click().catch(() => {});
      await page.waitForTimeout(600);
    } else break;
  }
}
async function sessionPainted() {
  return (await page.getByText('视觉验证消息：请给出一个表格、一段代码与一个引用。').count()) > 0;
}
await page.goto(WEB + '/platform/creatChat', { waitUntil: 'networkidle', timeout: 20000 }).catch(() => {});
await page.waitForTimeout(2500);
await dismissTour();
console.log('tour dismissed (empty)');
await page.screenshot({ path: OUT + '/chat-empty.png', fullPage: true });
console.log('shot chat-empty');
const row = page.getByText('视觉验证会话').first();
await row.waitFor({ state: 'visible', timeout: 20000 });
let ok = false;
for (let attempt = 1; attempt <= 4 && !ok; attempt++) {
  await row.click({ timeout: 8000 }).catch(() => {});
  await page.waitForTimeout(3500);
  await dismissTour();
  if (await sessionPainted()) { ok = true; break; }
  console.log('attempt', attempt, 'not painted; reloading creatChat');
  await page.goto(WEB + '/platform/creatChat', { waitUntil: 'networkidle', timeout: 20000 }).catch(() => {});
  await page.waitForTimeout(2200);
  await dismissTour();
  await row.waitFor({ state: 'visible', timeout: 20000 });
}
console.log('session painted:', ok, 'url:', page.url());
await page.waitForTimeout(700);
await page.screenshot({ path: OUT + '/chat-session.png', fullPage: true });
console.log('shot chat-session');
await browser.close();
