// Section screenshot for a single settings section (deterministic clock).
import { chromium } from 'playwright-core';
const WEB = 'http://localhost:5181';
const API = 'http://127.0.0.1:8080';
const runDir = process.argv[2] ?? 'artifacts/tailwind-shadcn';
const section = process.argv[3] ?? 'sandbox';
const loginRes = await fetch(API + '/api/v1/auth/login', {
  method: 'POST', headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ email: 'uimig@local.dev', password: 'Uimig-2026-Tailwind' }),
});
const body = await loginRes.json();
const state = { cookies: [], origins: [{ origin: WEB, localStorage: [
  { name: 'weknora_token', value: body.token },
  { name: 'weknora_selected_tenant_id', value: String(body.active_tenant?.id ?? body.user?.tenant_id ?? '') },
] }] };
const browser = await chromium.launch();
const ctx = await browser.newContext({ storageState: state, viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
await ctx.clock.install();
const page = await ctx.newPage();
try { await page.goto(WEB + '/platform/settings?section=' + section, { waitUntil: 'networkidle', timeout: 20000 }); } catch {}
await page.waitForTimeout(2000);
await page.screenshot({ path: runDir + '/settings-' + section + '.png', fullPage: true });
console.log('shot settings-' + section);
await browser.close();
