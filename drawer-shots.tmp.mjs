const { chromium } = await import('playwright-core');
import { readFileSync } from 'node:fs';

const credsEnv = readFileSync(new URL('file:///Users/wuyongjun/.weknora-parity-creds.env'), 'utf8');
const creds = Object.fromEntries(credsEnv.split('\n').filter((l) => l.includes('=')).map((l) => {
  const i = l.indexOf('=');
  return [l.slice(0, i).trim(), l.slice(i + 1).trim()];
}));
const res = await fetch('http://localhost:8084/api/v1/auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: creds.PARITY_TEST_EMAIL, password: creds.PARITY_TEST_PASSWORD }),
});
const j = await res.json();
const auth = { token: j.token, refreshToken: j.refresh_token || '', user: j.user, tenantId: String(j.active_tenant?.id ?? j.user?.tenant_id ?? '') };
const GUIDE_KEYS = ['weknora:contextual-guide-kb-list:v2','weknora:contextual-guide-kb-create:v3','weknora:contextual-guide-tenant-models:v1','weknora:contextual-guide-kb-detail:v1','weknora:contextual-guide-chat:v1','weknora:contextual-guide-agent-list:v1','weknora:contextual-guide-agent-create:v1','weknora:new-user-guide-done:v1'];

const ctx = await chromium.launch().then((b) => b.newContext({ viewport: { width: 1280, height: 720 } }));
const page = await ctx.newPage();
await page.goto('http://localhost:5175/login', { waitUntil: 'domcontentloaded' });
await page.evaluate(([a, guideKeys]) => {
  localStorage.setItem('weknora_token', a.token);
  if (a.refreshToken) localStorage.setItem('weknora_refresh_token', a.refreshToken);
  localStorage.setItem('weknora_user', JSON.stringify(a.user));
  localStorage.setItem('weknora_selected_tenant_id', a.tenantId);
  localStorage.setItem('weknora_react_session_v1', JSON.stringify({
    credential: { kind: 'bearer', accessToken: a.token, ...(a.refreshToken ? { refreshToken: a.refreshToken } : {}) },
    tenantId: a.tenantId, preferences: {},
  }));
  localStorage.setItem('weknora_react_legacy_import_v1', 'complete');
  for (const k of guideKeys) localStorage.setItem(k, '1');
}, [auth, GUIDE_KEYS]);

const OUT = '/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/tdm-int/docs/migrations/react/evidence/vue-react-parity/drawer-proofs';

async function shot(name, url, open, settle = 1600) {
  await page.goto(url, { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(settle);
  if (open) {
    const ok = await open();
    if (!ok) { console.error(`[${name}] open-fallback-used`); }
    await page.waitForTimeout(settle);
  }
  await page.screenshot({ path: `${OUT}/${name}.png` });
  console.log(`[${name}] saved`);
}

// 1) models 编辑器抽屉：settings models 区，点击第一个模型行/编辑入口
await shot('settings-models-editor', 'http://localhost:5175/platform/settings?section=models', async () => {
  const candidates = [
    '[data-guide="settings-model-edit"]',
    '[aria-label*="编辑"]',
    '.model-card .model-actions button',
  ];
  for (const sel of candidates) {
    const els = await page.$$(sel);
    for (const el of els) {
      if (await el.isVisible()) { await el.click(); return true; }
    }
  }
  const btns = await page.$$('button');
  for (const b of btns) {
    const t = (await b.textContent() || '').trim();
    if ((t === '编辑' || t === '配置') && await b.isVisible()) { await b.click(); return true; }
  }
  return false;
}, 2200);

// 2) sandbox 配置抽屉
await shot('settings-sandbox-config', 'http://localhost:5175/platform/settings?section=sandbox', async () => {
  const btns = await page.$$('button');
  for (const b of btns) {
    const t = (await b.textContent() || '').trim();
    if ((t.includes('配置') || t.includes('编辑')) && await b.isVisible()) { await b.click(); return true; }
  }
  return false;
}, 2200);

// 3) Engine（解析器引擎）抽屉
await shot('settings-parser-engine', 'http://localhost:5175/platform/settings?section=parser', async () => {
  const btns = await page.$$('button');
  for (const b of btns) {
    const t = (await b.textContent() || '').trim();
    if ((t.includes('配置') || t.includes('编辑') || t.includes('引擎')) && await b.isVisible()) { await b.click(); return true; }
  }
  return false;
}, 2200);

await ctx.close();
