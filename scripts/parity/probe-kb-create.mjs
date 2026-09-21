#!/usr/bin/env node
// 探针：两端打开新建知识库弹窗，对比关键元素几何/计算样式。
let chromium;
try {
  const { createRequire } = await import('node:module');
  const requireFromWeb = createRequire('/Users/wuyongjun/trea/WeKnora-fork01/apps/web/package.json');
  chromium = requireFromWeb('@playwright/test').chromium;
} catch {
  const { chromium: coreChromium } = await import('playwright-core');
  const shell = process.env.HOME + '/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell';
  chromium = { launch: (o = {}) => coreChromium.launch({ ...o, executablePath: o.executablePath || shell }) };
}
import { readFileSync, existsSync } from 'node:fs';

const VUE = 'http://localhost:5174', REACT = 'http://localhost:5175', BACKEND = 'http://localhost:8084';
function loadCreds() {
  const env = {};
  for (const line of readFileSync(process.env.HOME + '/.weknora-parity-creds.env', 'utf8').split('\n')) {
    const m = line.match(/^([A-Z_]+)=(.*)$/); if (m) env[m[1]] = m[2];
  }
  return env;
}
const creds = loadCreds();
const res = await fetch(BACKEND + '/api/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: creds.PARITY_TEST_EMAIL, password: creds.PARITY_TEST_PASSWORD }) });
const j = await res.json();
const auth = { token: j.token, refreshToken: j.refresh_token || '', user: j.user, tenantId: String(j.active_tenant?.id ?? j.user?.tenant_id ?? '') };
const GUIDE_KEYS = ['weknora:contextual-guide-kb-list:v2', 'weknora:contextual-guide-kb-create:v3', 'weknora:new-user-guide-done:v1'];

const browser = await chromium.launch();
async function probe(base, isReact) {
  const page = await (await browser.newContext({ viewport: { width: 1280, height: 720 } })).newPage();
  await page.goto(base + '/login', { waitUntil: 'domcontentloaded' });
  await page.evaluate(([a, keys, isR]) => {
    localStorage.setItem('weknora_token', a.token);
    if (a.refreshToken) localStorage.setItem('weknora_refresh_token', a.refreshToken);
    localStorage.setItem('weknora_user', JSON.stringify(a.user));
    localStorage.setItem('weknora_selected_tenant_id', a.tenantId);
    if (isR) {
      localStorage.setItem('weknora_react_session_v1', JSON.stringify({ credential: { kind: 'bearer', accessToken: a.token }, tenantId: a.tenantId, preferences: {} }));
      localStorage.setItem('weknora_react_legacy_import_v1', 'complete');
    }
    for (const k of keys) localStorage.setItem(k, '1');
  }, [auth, GUIDE_KEYS, isReact]);
  await page.goto(base + '/platform/knowledge-bases', { waitUntil: 'domcontentloaded' });
  await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {});
  await page.waitForTimeout(2400);
  // 点新建
  const cands = ['[data-guide="kb-list-create"]', '[aria-label*="新建知识库"]'];
  let clicked = false;
  for (const sel of cands) {
    for (const loc of await page.locator(sel).all()) {
      if (await loc.isVisible().catch(() => false)) { await loc.click(); clicked = true; break; }
    }
    if (clicked) break;
  }
  await page.waitForTimeout(1100);
  return page.evaluate(() => {
    const r = (sel) => {
      const el = document.querySelector(sel);
      if (!el) return null;
      const b = el.getBoundingClientRect();
      const cs = getComputedStyle(el);
      return { rect: [Math.round(b.x), Math.round(b.y), Math.round(b.width), Math.round(b.height)], bg: cs.backgroundColor, radius: cs.borderRadius, border: cs.border, font: cs.fontSize + '/' + cs.lineHeight, padding: cs.padding, margin: cs.margin, color: cs.color, display: cs.display, gap: cs.gap, resize: cs.resize };
    };
    const backdrop = document.querySelector('.wk-dialog-backdrop, .t-dialog__wrap, [class*="dialog"]');
    let backdropInfo = null;
    if (backdrop) {
      const cs = getComputedStyle(backdrop);
      backdropInfo = { cls: backdrop.className, bg: cs.backgroundColor, backdropFilter: cs.backdropFilter, position: cs.position };
    }
    const dlg = document.querySelector('.wk-dialog, .t-dialog__card, [role="dialog"]');
    return {
      backdropInfo,
      dialog: dlg ? (() => { const b = dlg.getBoundingClientRect(); const cs = getComputedStyle(dlg); return { rect: [Math.round(b.x), Math.round(b.y), Math.round(b.width), Math.round(b.height)], cls: dlg.className, radius: cs.borderRadius, boxShadow: cs.boxShadow.slice(0, 60) }; })() : null,
      els: {
        sidebar: r('.kb-editor-sidebar, .settings-sidebar'),
        sidebarTitle: r('.kb-editor-sidebar-title'),
        navItem: r('.kb-editor-nav-item, .settings-nav-item'),
        navItemActive: r('.kb-editor-nav-item.is-active, .t-menu__item.t-is-active, .settings-nav-item.is-active'),
        navIcon: r('.kb-editor-nav-icon svg, .settings-nav-item svg'),
        content: r('.kb-editor-content, .settings-content'),
        typeFrame: r('.kb-create-type-frame, .t-radio-group'),
        nameInput: r('[data-guide="kb-create-name"], .t-input__inner'),
        ragCard: r('fieldset[data-guide="kb-create-indexing"] label, .t-form__item'),
        descTextarea: r('.kb-editor-content textarea, .t-textarea__inner'),
        footer: r('.kb-editor-footer, .settings-footer, .t-dialog__footer'),
        cancelBtn: r('.kb-editor-btn-cancel'),
        saveBtn: r('.kb-editor-btn-save'),
        descCount: r('.kb-editor-desc-count'),
      },
    };
  });
}
const vue = await probe(VUE, false);
const react = await probe(REACT, true);
console.log('VUE ', JSON.stringify(vue, null, 1));
console.log('REACT', JSON.stringify(react, null, 1));
await browser.close();
