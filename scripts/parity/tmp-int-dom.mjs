#!/usr/bin/env node
// 临时 DOM 对比：集成面板 Vue vs React，输出结构/样式差异摘要
let chromium;
try {
  const { createRequire } = await import('node:module');
  const requireFromWeb = createRequire('/Users/wuyongjun/trea/WeKnora-fork01/apps/web/package.json');
  chromium = requireFromWeb('@playwright/test').chromium;
} catch {
  const { chromium: coreChromium } = await import('playwright-core');
  const headlessShell = process.env.HOME + '/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell';
  chromium = { launch(opts = {}) { return coreChromium.launch({ ...opts, executablePath: opts.executablePath || headlessShell }); } };
}
import { readFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';

const VUE = 'http://localhost:5174';
const REACT = 'http://localhost:5175';
const BACKEND = 'http://localhost:8084';
const GUIDE_KEYS = ['weknora:contextual-guide-kb-list:v2','weknora:contextual-guide-kb-create:v3','weknora:contextual-guide-tenant-models:v1','weknora:contextual-guide-kb-detail:v1','weknora:contextual-guide-chat:v1','weknora:contextual-guide-agent-list:v1','weknora:contextual-guide-agent-create:v1','weknora:new-user-guide-done:v1'];

function loadCreds() {
  const env = {};
  for (const line of readFileSync(join(process.env.HOME, '.weknora-parity-creds.env'), 'utf8').split('\n')) {
    const m = line.match(/^([A-Z_]+)=(.*)$/);
    if (m) env[m[1]] = m[2];
  }
  return env;
}
async function login() {
  const creds = loadCreds();
  const res = await fetch(BACKEND + '/api/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: creds.PARITY_TEST_EMAIL, password: creds.PARITY_TEST_PASSWORD }) });
  const j = await res.json();
  if (!j || j.success !== true || !j.token) throw new Error('login failed');
  return { token: j.token, refreshToken: j.refresh_token || '', user: j.user, tenantId: String(j.active_tenant?.id ?? j.user?.tenant_id ?? '') };
}
async function newAuthedPage(ctx, base, auth) {
  const page = await ctx.newPage();
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto(base + '/login', { waitUntil: 'domcontentloaded', timeout: 15000 });
  await page.evaluate(([a, guideKeys, isReact]) => {
    localStorage.setItem('weknora_token', a.token);
    if (a.refreshToken) localStorage.setItem('weknora_refresh_token', a.refreshToken);
    localStorage.setItem('weknora_user', JSON.stringify(a.user));
    localStorage.setItem('weknora_selected_tenant_id', a.tenantId);
    if (isReact) {
      localStorage.setItem('weknora_react_session_v1', JSON.stringify({ credential: { kind: 'bearer', accessToken: a.token, ...(a.refreshToken ? { refreshToken: a.refreshToken } : {}) }, tenantId: a.tenantId, preferences: {} }));
      localStorage.setItem('weknora_react_legacy_import_v1', 'complete');
    }
    for (const k of guideKeys) localStorage.setItem(k, '1');
  }, [auth, GUIDE_KEYS, base === REACT]);
  return page;
}

const key = process.argv[2] || 'api';
const auth = await login();
const browser = await chromium.launch();
try {
  const out = {};
  for (const [tag, base] of [['vue', VUE], ['react', REACT]]) {
    const page = await newAuthedPage(await browser.newContext(), base, auth);
    await page.goto(base + '/platform/settings?section=integration-' + key, { waitUntil: 'domcontentloaded', timeout: 20000 });
    await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {});
    await page.waitForTimeout(2400);
    out[tag] = await page.evaluate(() => {
      // 找设置抽屉 body：Vue .integrations-settings__body，React 尽量同容器
      const pick = document.querySelector('.integrations-settings__body, .integrations-settings__body--landing, [class*="integrations-settings"]');
      const root = pick || document.body;
      const outline = (el, depth) => {
        if (!el || depth > 16) return null;
        const cs = getComputedStyle(el);
        const r = el.getBoundingClientRect();
        const kids = [...el.children].map((c) => outline(c, depth + 1)).filter(Boolean);
        const info = {
          tag: el.tagName.toLowerCase(),
          cls: typeof el.className === 'string' ? el.className.slice(0, 120) : '',
          text: kids.length === 0 ? (el.textContent || '').trim().slice(0, 80) : '',
          rect: [Math.round(r.x), Math.round(r.y), Math.round(r.width), Math.round(r.height)],
          style: [cs.fontSize, cs.fontWeight, cs.color, cs.backgroundColor, cs.borderRadius, cs.border, cs.padding, cs.margin, cs.display].join(' | '),
        };
        if (kids.length) info.kids = kids;
        return info;
      };
      return outline(root, 0);
    });
    await page.close();
  }
  console.log(JSON.stringify(out, null, 1));
} finally { await browser.close(); }
