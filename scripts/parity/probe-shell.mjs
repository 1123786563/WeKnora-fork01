#!/usr/bin/env node
// Probe: dump the shell sidebar DOM structure (nav items, session list, user menu)
// from both Vue and React for the given path. Usage: node probe-shell.mjs /platform/agents
let chromium;
try {
  const { createRequire } = await import('node:module');
  const requireFromWeb = createRequire('/Users/wuyongjun/trea/WeKnora-fork01/apps/web/package.json');
  chromium = requireFromWeb('@playwright/test').chromium;
} catch {
  const { chromium: core } = await import('playwright-core');
  chromium = core;
}
import { readFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';

const ROOT = '/Users/wuyongjun/trea/WeKnora-fork01';
const VUE = process.env.PARITY_VUE_URL || 'http://localhost:5174';
const REACT = process.env.PARITY_REACT_URL || 'http://localhost:5175';
const BACKEND = process.env.PARITY_BACKEND || 'http://localhost:8084';
const path = process.argv[2] || '/platform/agents';

const env = {};
for (const line of readFileSync(join(process.env.HOME, '.weknora-parity-creds.env'), 'utf8').split('\n')) {
  const m = line.match(/^([A-Z_]+)=(.*)$/);
  if (m) env[m[1]] = m[2];
}
const res = await fetch(BACKEND + '/api/v1/auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: env.PARITY_TEST_EMAIL, password: env.PARITY_TEST_PASSWORD }),
});
const j = await res.json();
const auth = { token: j.token, refreshToken: j.refresh_token || '', user: j.user, tenantId: String(j.active_tenant?.id ?? '') };

const GUIDE_KEYS = [
  'weknora:contextual-guide-kb-list:v2', 'weknora:contextual-guide-kb-create:v3',
  'weknora:contextual-guide-tenant-models:v1', 'weknora:contextual-guide-kb-detail:v1',
  'weknora:contextual-guide-chat:v1', 'weknora:contextual-guide-agent-list:v1',
  'weknora:contextual-guide-agent-create:v1', 'weknora:new-user-guide-done:v1',
];

const browser = await chromium.launch();
const out = {};
for (const [tag, base] of [['vue', VUE], ['react', REACT]]) {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto(base + '/login', { waitUntil: 'domcontentloaded', timeout: 15000 });
  await page.evaluate(([a, guideKeys, isReact]) => {
    localStorage.setItem('weknora_token', a.token);
    if (a.refreshToken) localStorage.setItem('weknora_refresh_token', a.refreshToken);
    localStorage.setItem('weknora_user', JSON.stringify(a.user));
    localStorage.setItem('weknora_selected_tenant_id', a.tenantId);
    if (isReact) {
      localStorage.setItem('weknora_react_session_v1', JSON.stringify({
        credential: { kind: 'bearer', accessToken: a.token, ...(a.refreshToken ? { refreshToken: a.refreshToken } : {}) },
        tenantId: a.tenantId, preferences: {},
      }));
      localStorage.setItem('weknora_react_legacy_import_v1', 'complete');
    }
    for (const k of guideKeys) localStorage.setItem(k, '1');
    localStorage.setItem('sidebar_collapsed', 'true');
  }, [auth, GUIDE_KEYS, tag === 'react']);
  await page.goto(base + path, { waitUntil: 'domcontentloaded', timeout: 20000 });
  await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {});
  await page.evaluate(() => document.fonts.ready).catch(() => {});
  await page.waitForTimeout(2400);
  out[tag] = await page.evaluate(() => {
    const rect = (el) => { const r = el.getBoundingClientRect(); return [Math.round(r.x * 10) / 10, Math.round(r.y * 10) / 10, Math.round(r.width * 10) / 10, Math.round(r.height * 10) / 10]; };
    const lines = [];
    // logo row icons
    for (const sel of ['.logo_row', '.header-icon-btn', '.header-icon-img', '.sidebar-toggle', 'aside .logo', '.logo_box']) {
      const el = document.querySelector(sel);
      if (el) lines.push(`EL ${sel} ${rect(el)} html=${el.outerHTML.slice(0, 160).replace(/\s+/g, ' ')}`);
    }
    // nav items
    document.querySelectorAll('.menu_item, aside nav a, .plat-shell__item').forEach((el) => {
      lines.push(`NAV ${rect(el)} text=${(el.textContent || '').trim().slice(0, 20)} cls=${el.className.toString().slice(0, 80)}`);
    });
    // icons inside nav
    document.querySelectorAll('.menu_icon, .menu_icon img, aside nav a svg, aside nav a img').forEach((el) => {
      lines.push(`ICON ${rect(el)} tag=${el.tagName} cls=${el.className.toString().slice(0, 60)} src=${el.getAttribute('src') || ''}`);
    });
    // session list
    document.querySelectorAll('.timeline_header, .submenu_item_p, .session-list-row, aside nav[aria-label] h3, aside nav[aria-label] li').forEach((el) => {
      lines.push(`SESS ${rect(el)} text=${(el.textContent || '').trim().slice(0, 24)} cls=${el.className.toString().slice(0, 90)}`);
    });
    // user menu
    for (const sel of ['.user-menu', '.user-button', '.user-avatar', '.user-info', '.user-name', '.user-email', '.user-tenant-name', '.user-tenant-meta', '.dropdown-icon', '.menu_bottom']) {
      const el = document.querySelector(sel);
      if (el) lines.push(`UM ${sel} ${rect(el)} text=${(el.textContent || '').trim().slice(0, 30)}`);
    }
    // session row internals
    document.querySelectorAll('.session-chat-row, aside nav[aria-label] li').forEach((el) => {
      const title = el.querySelector('.submenu_title-text, .submenu_title, button span:nth-last-of-type(1), li > button > span:last-of-type');
      const pin = el.querySelector('.submenu_pin_icon');
      const more = el.querySelector('.menu-more-wrap, summary');
      const r = el.getBoundingClientRect();
      const tr = title ? title.getBoundingClientRect() : null;
      const btn = el.querySelector('.submenu_item, li > button') || el;
      lines.push(`FONT row=${getComputedStyle(btn).fontFamily}`);
      lines.push(`FONTH html=${getComputedStyle(document.documentElement).fontFamily} inline=${document.documentElement.getAttribute('style') || ''}`);
      lines.push(`FONTB body=${getComputedStyle(document.body).fontFamily} inline=${document.body.getAttribute('style') || ''}`);
      lines.push(`ROW y=${Math.round(r.y)} title=${title ? JSON.stringify(title.textContent.slice(0, 18)) + ' @' + Math.round(tr.x) + ',w' + Math.round(tr.width) : 'none'} pin=${pin ? Math.round(pin.getBoundingClientRect().x) : '-'} more=${more ? Math.round(more.getBoundingClientRect().width) : '-'} cs=${getComputedStyle(el.querySelector('.submenu_item, li > button') || el).color}`);
    });
    // guide probe: what's at the residual band, plus any guide elements
    for (const [px, py] of [[625, 290], [625, 300], [620, 285], [300, 300]]) {
      const el = document.elementFromPoint(px, py);
      if (el) lines.push(`PT (${px},${py}) ${el.tagName}.${(el.className.toString ? el.className.toString() : '').slice(0, 50)} text=${(el.textContent || '').trim().slice(0, 20)} rect=${rect(el)}`);
    }
    document.querySelectorAll('[class*="guide"]').forEach((el) => {
      const cs = getComputedStyle(el);
      if (cs.display !== 'none' && cs.visibility !== 'hidden') lines.push(`GUIDE ${el.tagName}.${el.className.toString().slice(0, 60)} ${rect(el)} text=${(el.textContent || '').trim().slice(0, 16)}`);
    });
    // session source filter
    const sf = document.querySelector('.session-list-scope-header, .session-source-filter');
    if (sf) lines.push(`FILTER ${rect(sf)} ${sf.outerHTML.slice(0, 120)}`);
    return lines;
  });
  await ctx.close();
}
await browser.close();
for (const tag of ['vue', 'react']) {
  console.log(`\n========== ${tag} ==========`);
  for (const l of out[tag]) console.log(l);
}
