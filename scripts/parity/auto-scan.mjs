#!/usr/bin/env node
/**
 * Vue/React parity 自动截图对比扫描（R492 起，供定时任务调用）
 *
 * 流程：
 *  1. 从环境变量读 fixture 凭据（~/.weknora-parity-creds.env）
 *  2. POST /api/v1/auth/login 拿 token（后端默认 http://localhost:8084，可用 PARITY_BACKEND 覆盖）
 *  3. headless chromium 两个 context 分别注入 localStorage 登录态 + 屏蔽新手引导
 *  4. 按 PAGES 清单逐页双端截图（1280x720，与 R492 手动轮同规格）
 *  5. 调 python3 pixdiff.py 做像素对比（容差 8）
 *  6. 输出 report.json / report.md 到 auto-scan/<timestamp>/
 *
 * 用法：node scripts/parity/auto-scan.mjs
 * 退出码：0=扫描完成（无论差异多少）；1=环境故障（服务不在线/登录失败）。
 */
import pw from '/Users/wuyongjun/trea/WeKnora-fork01/apps/web/node_modules/@playwright/test/index.js';
const { chromium } = pw;
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = '/Users/wuyongjun/trea/WeKnora-fork01';
const EVIDENCE = join(ROOT, 'docs/migrations/react/evidence/vue-react-parity');
const VUE = process.env.PARITY_VUE_URL || 'http://localhost:5174';
const REACT = process.env.PARITY_REACT_URL || 'http://localhost:5175';
const BACKEND = process.env.PARITY_BACKEND || 'http://localhost:8084';

// ---- 凭据：只从 env 文件读，源码不写字面量 ----
function loadCreds() {
  const p = process.env.PARITY_CREDS_FILE || join(process.env.HOME, '.weknora-parity-creds.env');
  if (!existsSync(p)) throw new Error(`凭据文件不存在：${p}`);
  const env = {};
  for (const line of readFileSync(p, 'utf8').split('\n')) {
    const m = line.match(/^([A-Z_]+)=(.*)$/);
    if (m) env[m[1]] = m[2];
  }
  if (!env.PARITY_TEST_EMAIL || !env.PARITY_TEST_PASSWORD) throw new Error('凭据文件缺 PARITY_TEST_EMAIL/PASSWORD');
  return env;
}

// ---- 页面清单（Vue 为基准；id 与 R492 台账一致）----
// 可用环境变量 PAGES=chat,kb-list 过滤（逗号分隔的 id），便于单页快验。
const PAGE_FILTER = (process.env.PAGES || '').split(',').map(s => s.trim()).filter(Boolean);
const ALL_PAGES = [
  { id: 'kb-list', path: '/platform/knowledge-bases' },
  { id: 'agents', path: '/platform/agents' },
  { id: 'orgs', path: '/platform/organizations' },
  { id: 'settings-general', path: '/platform/settings?section=general' },
  { id: 'creatchat', path: '/platform/creatChat' },
  { id: 'apps', path: '/platform/apps' },
  { id: 'apps-connections', path: '/platform/apps/connections' },
  // fixture 数据页：运行时按名字解析 ID，找不到则跳过并标注
  { id: 'kb-faq', kind: 'kb', name: 'Parity FAQ Fixture' },
  { id: 'kb-wiki', kind: 'kb', name: 'Wiki Parity Fixture' },
  { id: 'kb-demo', kind: 'kb', name: 'Parity KB Demo' },
  { id: 'chat', kind: 'chat', name: '工具调用 Parity Fixture' },
];
const PAGES = PAGE_FILTER.length ? ALL_PAGES.filter(p => PAGE_FILTER.includes(p.id)) : ALL_PAGES;

// 屏蔽新手引导（两端同名键；来源 frontend/src/config/contextualGuides.ts）
const GUIDE_KEYS = [
  'weknora:contextual-guide-kb-list:v2', 'weknora:contextual-guide-kb-create:v3',
  'weknora:contextual-guide-tenant-models:v1', 'weknora:contextual-guide-kb-detail:v1',
  'weknora:contextual-guide-chat:v1', 'weknora:contextual-guide-agent-list:v1',
  'weknora:contextual-guide-agent-create:v1', 'weknora:new-user-guide-done:v1',
];

async function api(path, token, tenantId, init = {}) {
  const res = await fetch(BACKEND + path, {
    ...init,
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}`, 'X-Tenant-ID': String(tenantId), ...(init.headers || {}) },
  });
  return { status: res.status, body: await res.json().catch(() => null) };
}

async function login() {
  const creds = loadCreds();
  const res = await fetch(BACKEND + '/api/v1/auth/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: creds.PARITY_TEST_EMAIL, password: creds.PARITY_TEST_PASSWORD }),
  });
  const j = await res.json().catch(() => null);
  if (!j || j.success !== true || !j.token) throw new Error(`登录失败 HTTP ${res.status}`);
  return { token: j.token, refreshToken: j.refresh_token || '', user: j.user, tenantId: String(j.active_tenant?.id ?? j.user?.tenant_id ?? '') };
}

async function resolveFixtures(token, tenantId) {
  const out = {};
  const kbs = await api('/api/v1/knowledge-bases?page_size=50', token, tenantId);
  const kbItems = kbs.body?.data?.items || kbs.body?.data || [];
  for (const p of PAGES.filter(p => p.kind === 'kb')) {
    const hit = (Array.isArray(kbItems) ? kbItems : []).find(k => k.name === p.name);
    if (hit) out[p.id] = `/platform/knowledge-bases/${hit.id}`;
  }
  const sess = await api('/api/v1/sessions?page=1&page_size=30', token, tenantId);
  const sItems = sess.body?.data?.sessions || sess.body?.data?.items || sess.body?.data || [];
  for (const p of PAGES.filter(p => p.kind === 'chat')) {
    const hit = (Array.isArray(sItems) ? sItems : []).find(s => (s.title || s.name) === p.name);
    if (hit) out[p.id] = `/platform/chat/${hit.id || hit.session_id}`;
  }
  return out;
}

async function newAuthedPage(ctx, base, auth) {
  const page = await ctx.newPage();
  await page.setViewportSize({ width: 1280, height: 720 });
  // 先访问 origin 一次以获得 localStorage 写入权限
  await page.goto(base + '/login', { waitUntil: 'domcontentloaded', timeout: 15000 });
  await page.evaluate(([a, guideKeys, isReact]) => {
    localStorage.setItem('weknora_token', a.token);
    if (a.refreshToken) localStorage.setItem('weknora_refresh_token', a.refreshToken);
    localStorage.setItem('weknora_user', JSON.stringify(a.user));
    localStorage.setItem('weknora_selected_tenant_id', a.tenantId);
    // React 的 canonical 会话键（legacy-session.ts readReactPlatformState 优先读它）
    if (isReact) {
      localStorage.setItem('weknora_react_session_v1', JSON.stringify({
        credential: { kind: 'bearer', accessToken: a.token, ...(a.refreshToken ? { refreshToken: a.refreshToken } : {}) },
        tenantId: a.tenantId,
        preferences: {},
      }));
      localStorage.setItem('weknora_react_legacy_import_v1', 'complete');
    }
    for (const k of guideKeys) localStorage.setItem(k, '1');
  }, [auth, GUIDE_KEYS, base === REACT]);
  return page;
}

function pixdiff(vuePng, reactPng, diffPng) {
  const out = execFileSync('python3', [
    join(EVIDENCE, 'screenshots/r492-20260920/pixdiff.py'), vuePng, reactPng, diffPng,
  ], { encoding: 'utf8' });
  return JSON.parse(out);
}

async function main() {
  // 0. 服务在线检查
  for (const [name, base] of [['vue', VUE], ['react', REACT], ['backend', BACKEND]]) {
    try {
      const r = await fetch(base + '/', { signal: AbortSignal.timeout(4000) });
      if (r.status >= 500) throw new Error(String(r.status));
    } catch (e) {
      console.error(`[env] ${name} (${base}) 不在线：${e.message}`);
      process.exit(1);
    }
  }

  const auth = await login();
  const fixtures = await resolveFixtures(auth.token, auth.tenantId);
  console.log(`[auth] 登录成功 tenant=${auth.tenantId}；fixture 解析 ${Object.keys(fixtures).length}/${PAGES.filter(p => p.kind).length}`);

  const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  const outDir = join(EVIDENCE, 'auto-scan', stamp);
  mkdirSync(outDir, { recursive: true });

  const browser = await chromium.launch();
  const results = [];
  try {
    const vuePage = await newAuthedPage(await browser.newContext(), VUE, auth);
    const reactPage = await newAuthedPage(await browser.newContext(), REACT, auth);

    for (const p of PAGES) {
      const path = p.kind ? fixtures[p.id] : p.path;
      const entry = { id: p.id, path: path || p.path, status: 'ok', diff_pct: null };
      if (!path) { entry.status = 'skipped-no-fixture'; results.push(entry); console.log(`[skip] ${p.id}（fixture 未找到）`); continue; }
      try {
        const shots = [];
        for (const [tag, page, base] of [['vue', vuePage, VUE], ['react', reactPage, REACT]]) {
          await page.goto(base + path, { waitUntil: 'domcontentloaded', timeout: 20000 });
          await page.waitForTimeout(3000);
          const file = join(outDir, `${p.id}-${tag}.png`);
          await page.screenshot({ path: file });
          shots.push(file);
        }
        const d = pixdiff(shots[0], shots[1], join(outDir, `${p.id}-diff.png`));
        entry.diff_pct = d.diff_pct;
        entry.hot_cells = d.hot_cells?.slice(0, 6) || [];
        entry.urls = { vue: await vuePage.url(), react: await reactPage.url() };
        console.log(`[done] ${p.id}: diff ${d.diff_pct}%`);
      } catch (e) {
        entry.status = 'error'; entry.error = e.message.split('\n')[0];
        console.log(`[err ] ${p.id}: ${entry.error}`);
      }
      results.push(entry);
    }
  } finally {
    await browser.close();
  }

  // 报告
  const scanned = results.filter(r => r.status === 'ok');
  const report = {
    stamp, vue: VUE, react: REACT, backend: BACKEND,
    summary: {
      pages: results.length, ok: scanned.length,
      over_1pct: scanned.filter(r => r.diff_pct > 1).length,
      avg_diff_pct: scanned.length ? +(scanned.reduce((s, r) => s + r.diff_pct, 0) / scanned.length).toFixed(2) : null,
    },
    results,
  };
  writeFileSync(join(outDir, 'report.json'), JSON.stringify(report, null, 2));

  const lines = [
    `# Parity 自动扫描 ${stamp}`,
    '',
    `- 端点：Vue ${VUE} / React ${REACT} / 后端 ${BACKEND}`,
    `- 页面：${report.summary.ok}/${report.summary.pages} 成功，>${1}% 差异 ${report.summary.over_1pct} 页，平均差异 ${report.summary.avg_diff_pct}%`,
    '',
    '| 页面 | diff% | 状态 |', '|---|---|---|',
    ...results.map(r => `| ${r.id} | ${r.diff_pct ?? '-'} | ${r.status} |`),
  ];
  writeFileSync(join(outDir, 'report.md'), lines.join('\n') + '\n');
  console.log(`[report] ${join(outDir, 'report.md')}`);
}

main().catch(e => { console.error('[fatal]', e.message); process.exit(1); });
