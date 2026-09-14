// DEV-ONLY screenshot harness for the Tailwind+shadcn migration acceptance.
// Uses the uimig@local.dev account (created via the public register API
// against the local :8080 backend). Captures every top-level route at
// 1440x900 (KB list also at 390x844) for before/after comparison.
import { chromium } from 'playwright-core';

const WEB = 'http://localhost:5181';
const API = 'http://127.0.0.1:8080';
const EMAIL = 'uimig@local.dev';
const PASSWORD = 'Uimig-2026-Tailwind';
const OUT = process.argv[2] ?? 'artifacts/tailwind-shadcn/baseline';

const loginRes = await fetch(API + '/api/v1/auth/login', {
  method: 'POST', headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ email: EMAIL, password: PASSWORD }),
});
const loginBody = await loginRes.json();
if (!loginRes.ok || !loginBody?.token) throw new Error('login failed: ' + JSON.stringify(loginBody).slice(0, 200));
const token = loginBody.token;
const tenantId = String(loginBody.active_tenant?.id ?? loginBody.user?.tenant_id ?? '');

const kbListRes = await fetch(API + '/api/v1/knowledge-bases?page_size=1', {
  headers: { authorization: 'Bearer ' + token, 'x-tenant-id': tenantId },
});
const kbList = await kbListRes.json().catch(() => ({}));
let kbId = kbList?.data?.[0]?.id ?? kbList?.data?.items?.[0]?.id ?? '';
if (kbId === '') {
  const created = await fetch(API + '/api/v1/knowledge-bases', {
    method: 'POST',
    headers: { 'content-type': 'application/json', authorization: 'Bearer ' + token, 'x-tenant-id': tenantId },
    body: JSON.stringify({ name: '迁移视觉验证库', description: 'Tailwind migration visual baseline' }),
  }).then((r) => r.json()).catch(() => ({}));
  kbId = created?.data?.id ?? created?.id ?? '';
}
console.log('kbId =', kbId);

const storageState = {
  cookies: [],
  origins: [{ origin: WEB, localStorage: [
    { name: 'weknora_token', value: token },
    { name: 'weknora_refresh_token', value: loginBody.refresh_token ?? '' },
    { name: 'weknora_selected_tenant_id', value: tenantId },
  ] }],
};

const browser = await chromium.launch();
const ctx = await browser.newContext({ storageState, viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
const page = await ctx.newPage();

async function shot(name, url, waitMs) {
  const wait = waitMs ?? 1800;
  try { await page.goto(WEB + url, { waitUntil: 'networkidle', timeout: 20000 }); } catch {}
  await page.waitForTimeout(wait);
  await page.screenshot({ path: OUT + '/' + name + '.png', fullPage: true });
  console.log('shot', name);
}

await shot('kb-list', '/platform/knowledge-bases');
await shot('agents', '/platform/agents');
await shot('creat-chat', '/platform/creatChat', 2600);
await shot('settings-general', '/platform/settings?section=general');
await shot('settings-models', '/platform/settings?section=models');
await shot('settings-members', '/platform/settings?section=members');
await shot('settings-api', '/platform/settings?section=api');
await shot('tenant', '/platform/tenant');
await shot('organizations', '/platform/organizations');
await shot('configuration', '/platform/configuration');
await shot('administration', '/platform/administration');
await shot('system', '/platform/system');
await shot('dev-markdown', '/platform/dev/markdown');
if (kbId !== '') {
  const enc = encodeURIComponent(kbId);
  await shot('kb-detail', '/platform/knowledge-bases/' + enc);
  await shot('kb-wiki', '/platform/knowledge-bases/' + enc + '?tab=wiki');
  await shot('kb-graph', '/platform/knowledge-bases/' + enc + '?tab=graph');
  await shot('kb-settings', '/knowledgeBase/' + enc + '/settings');
}

const anon = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
const anonPage = await anon.newPage();
const anonRoutes = [['login', '/login'], ['register', '/register'], ['not-found', '/platform/definitely-missing']];
for (const pair of anonRoutes) {
  try { await anonPage.goto(WEB + pair[1], { waitUntil: 'networkidle', timeout: 20000 }); } catch {}
  await anonPage.waitForTimeout(1200);
  await anonPage.screenshot({ path: OUT + '/' + pair[0] + '.png', fullPage: true });
  console.log('shot', pair[0]);
}
const narrow = await browser.newContext({ storageState, viewport: { width: 390, height: 844 }, locale: 'zh-CN' });
const narrowPage = await narrow.newPage();
try { await narrowPage.goto(WEB + '/platform/knowledge-bases', { waitUntil: 'networkidle', timeout: 20000 }); } catch {}
await narrowPage.waitForTimeout(1800);
await narrowPage.screenshot({ path: OUT + '/kb-list-narrow.png', fullPage: true });
console.log('shot kb-list-narrow');
await browser.close();
console.log('done ->', OUT);
