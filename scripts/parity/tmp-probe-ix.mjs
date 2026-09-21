#!/usr/bin/env node
// 临时探针：抓取三态弹窗/抽屉 DOM 结构 + 关键计算样式，输出 vue/react JSON 供对比
let chromium;
try {
  const { createRequire } = await import('node:module');
  const requireFromWeb = createRequire('/Users/wuyongjun/trea/WeKnora-fork01/apps/web/package.json');
  chromium = requireFromWeb('@playwright/test').chromium;
  if (!chromium) throw new Error('no chromium export');
} catch {
  const { chromium: coreChromium } = await import('playwright-core');
  const headlessShell = process.env.HOME +
    '/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell';
  chromium = {
    launch(opts = {}) {
      return coreChromium.launch({ ...opts, executablePath: opts.executablePath || headlessShell });
    },
  };
}
import { readFileSync, writeFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';

const ROOT = '/Users/wuyongjun/trea/WeKnora-fork01';
const VUE = 'http://localhost:5174';
const REACT = 'http://localhost:5175';
const BACKEND = 'http://localhost:8084';
const OUT = process.env.OUT_DIR || join(ROOT, 'scripts/parity/tmp-probe-out');

function loadCreds() {
  const p = join(process.env.HOME, '.weknora-parity-creds.env');
  const env = {};
  for (const line of readFileSync(p, 'utf8').split('\n')) {
    const m = line.match(/^([A-Z_]+)=(.*)$/);
    if (m) env[m[1]] = m[2];
  }
  return env;
}
async function login() {
  const creds = loadCreds();
  const res = await fetch(BACKEND + '/api/v1/auth/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: creds.PARITY_TEST_EMAIL, password: creds.PARITY_TEST_PASSWORD }),
  });
  const j = await res.json();
  return { token: j.token, refreshToken: j.refresh_token || '', user: j.user, tenantId: String(j.active_tenant?.id ?? j.user?.tenant_id ?? '') };
}
const GUIDE_KEYS = [
  'weknora:contextual-guide-kb-list:v2', 'weknora:contextual-guide-kb-create:v3',
  'weknora:contextual-guide-tenant-models:v1', 'weknora:contextual-guide-kb-detail:v1',
  'weknora:contextual-guide-chat:v1', 'weknora:contextual-guide-agent-list:v1',
  'weknora:contextual-guide-agent-create:v1', 'weknora:new-user-guide-done:v1',
];

const STATES = {
  'ix-kb-list-create': { path: '/platform/knowledge-bases', actions: [{ clickAria: ['新建知识库'], clickCss: ['[data-guide="kb-list-create"]'] }] },
  'ix-faq-retrieval': { path: '/platform/knowledge-bases/0e4de98e-1400-413e-b627-eb3c65181f2f', actions: [{ clickAria: ['检索测试'], clickCss: ['.content-bar-icon-btn:has(svg.t-icon-search)'] }] },
  'ix-kb-doc-detail': { path: '/platform/knowledge-bases/dca0db93-2aba-4cf2-b386-d75d9e069b1c', actions: [{ clickText: ['mermaid-arch-demo'] }] },
};

// 提取弹窗容器（含 overlay）内的结构化树
async function extract(page) {
  return page.evaluate(() => {
    const pick = (el) => {
      const cs = getComputedStyle(el);
      const r = el.getBoundingClientRect();
      return {
        tag: el.tagName, cls: String(el.className && el.className.baseVal !== undefined ? el.className.baseVal : el.className).slice(0, 80),
        box: [Math.round(r.x), Math.round(r.y), Math.round(r.width), Math.round(r.height)],
        style: {
          bg: cs.backgroundColor, fg: cs.color, fs: cs.fontSize, fw: cs.fontWeight, ff: cs.fontFamily.slice(0, 40),
          lh: cs.lineHeight, pad: cs.padding, mar: cs.margin, br: cs.borderRadius, bd: cs.borderWidth + ' ' + cs.borderStyle + ' ' + cs.borderColor,
          disp: cs.display, gap: cs.gap, h: cs.height, w: cs.width, pos: cs.position, bs: cs.boxShadow.slice(0, 60), bfr: cs.backdropFilter,
          ai: cs.alignItems, jc: cs.justifyContent, ta: cs.textAlign, ls: cs.letterSpacing, ws: cs.whiteSpace, ovf: cs.overflow,
        },
      };
    };
    // 找弹窗根：优先 element-ui/tdesign 弹层类，否则取最上层 fixed 元素
    const candidates = [...document.querySelectorAll('.el-overlay, .t-dialog, .el-dialog, .t-drawer, .el-drawer, [role="dialog"], .modal, .overlay')];
    const visible = candidates.filter(e => { const r = e.getBoundingClientRect(); return r.width > 100 && r.height > 100; });
    const roots = visible.length ? visible : [...document.querySelectorAll('body > *')].filter(e => { const r = e.getBoundingClientRect(); return r.width > 300; });
    const seen = new Set();
    const out = [];
    const walk = (el, depth, path) => {
      if (depth > 14 || out.length > 900) return;
      const r = el.getBoundingClientRect();
      if (r.width === 0 && r.height === 0 && el.children.length === 0) return;
      const node = { ...pick(el), path };
      const txt = [...el.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent.trim()).join(' ').trim();
      if (txt) node.text = txt.slice(0, 60);
      out.push(node);
      for (const c of el.children) {
        if (seen.has(c)) continue;
        seen.add(c);
        walk(c, depth + 1, path + '>' + c.tagName.toLowerCase() + (c.className && typeof c.className === 'string' ? '.' + c.className.split(' ').slice(0, 2).join('.') : ''));
      }
    };
    for (const rootEl of roots.slice(0, 4)) {
      out.push({ marker: 'ROOT', ...pick(rootEl), rootCls: String(rootEl.className).slice(0, 100) });
      walk(rootEl, 0, rootEl.tagName.toLowerCase());
    }
    return out;
  });
}

async function clickFirst(page, action) {
  const candidates = [];
  if (action.clickAria) candidates.push(...action.clickAria.map(t => ({ by: 'aria', t })));
  if (action.clickCss) candidates.push(...action.clickCss.map(s => ({ by: 'css', s })));
  if (action.clickText) candidates.push(...action.clickText.map(t => ({ by: 'text', t })));
  for (const c of candidates) {
    try {
      let locs;
      if (c.by === 'aria') locs = await page.locator(`[aria-label*="${c.t}"], [title*="${c.t}"]`).all();
      else if (c.by === 'css') locs = await page.locator(c.s).all();
      else locs = [...await page.getByText(c.t, { exact: true }).all(), ...await page.getByText(c.t, { exact: false }).all()];
      for (const loc of locs) {
        if (await loc.isVisible().catch(() => false)) { await loc.click(); return true; }
      }
    } catch {}
  }
  return false;
}

const auth = await login();
import { mkdirSync } from 'node:fs';
mkdirSync(OUT, { recursive: true });
const browser = await chromium.launch();
const which = process.env.WHICH || 'both';
for (const [stateId, cfg] of Object.entries(STATES)) {
  for (const [tag, base] of [['vue', VUE], ['react', REACT]]) {
    if (which !== 'both' && which !== tag) continue;
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto(base + '/login', { waitUntil: 'domcontentloaded' });
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
    await page.goto(base + cfg.path, { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {});
    await page.waitForTimeout(2400);
    for (const action of cfg.actions) { await clickFirst(page, action); await page.waitForTimeout(1100); }
    const data = await extract(page);
    writeFileSync(join(OUT, `${stateId}-${tag}.json`), JSON.stringify(data, null, 1));
    console.log(`[probe] ${stateId}-${tag}: ${data.length} nodes`);
    await ctx.close();
  }
}
await browser.close();
