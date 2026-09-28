#!/usr/bin/env node
/**
 * Phase I 交互面板盘点（spec: docs/specs/2026-09-24-interaction-panel-parity.md）
 *
 * 对 auto-scan.mjs ALL_PAGES 覆盖的每个页面（fixture 解析同源），headless 双端
 * （Vue :5174 / React :5175，登录态注入照抄 auto-scan.mjs）：
 *   1. 枚举可见可点击元素（aria/CSS 兜底，含纯图标按钮 + hover 门控补漏）；
 *   2. 逐个真实鼠标点击（破坏性/写操作动词类只记录不实点），用"浮层基线 diff"
 *      检测弹出的面板并分类（dialog/drawer/dropdown/popover/select-popup/
 *      date-picker/confirm/message/mention-menu/custom…）——双端弹层 DOM 异构
 *      （Vue .user-menu 内联 vs React .user-dropdown；React .settings-overlay
 *      自定义 modal），故检测不依赖单一组件库类名；
 *   3. 面板只打开不确认：截图留证 → ESC / 安全空白点击 / 兜底整页重载；
 *   4. 产出 inventory.json（页面×端×触发器×面板结果），供 matrix.md 生成。
 *
 * 用法：node scripts/parity/panel-inventory.mjs
 *   PAGES=kb-list,chat 只扫指定页；SHOT_BUDGET=118 截图预算；MAX_CLICKS=40 单页点击上限。
 */
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
import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'node:fs';
import { join } from 'node:path';

const ROOT = '/Users/wuyongjun/trea/WeKnora-fork01';
const EVIDENCE = join(ROOT, 'docs/migrations/react/evidence/vue-react-parity');
const OUT = join(EVIDENCE, 'panel-matrix');
const PNG = join(OUT, 'png');
mkdirSync(PNG, { recursive: true });
const VUE = process.env.PARITY_VUE_URL || 'http://localhost:5174';
const REACT = process.env.PARITY_REACT_URL || 'http://localhost:5175';
const BACKEND = process.env.PARITY_BACKEND || 'http://localhost:8084';
const SHOT_BUDGET = +(process.env.SHOT_BUDGET || 118);
const MAX_CLICKS = +(process.env.MAX_CLICKS || 40);

const PAGE_FILTER = (process.env.PAGES || '').split(',').map(s => s.trim()).filter(Boolean);
const SETTINGS_SECTION_KEYS = [
  'general', 'userprofile', 'mymemory', 'envvars', 'tenant', 'members',
  'models', 'ollama', 'weknoracloud', 'chathistory', 'memory',
  'vectorstore', 'parser', 'storage', 'sandbox', 'skills', 'mcp', 'websearch',
  'system', 'system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log',
];
const INTEGRATION_TAB_KEYS = ['im', 'embed', 'api', 'cli', 'chrome', 'claw'];

const ALL_PAGES = [
  { id: 'kb-list', path: '/platform/knowledge-bases', settle: 3000 },
  { id: 'agents', path: '/platform/agents' },
  { id: 'orgs', path: '/platform/organizations' },
  { id: 'creatchat', path: '/platform/creatChat' },
  { id: 'apps', path: '/platform/apps' },
  { id: 'apps-connections', path: '/platform/apps/connections' },
  { id: 'chat', kind: 'chat', name: '工具调用 Parity Fixture', settle: 3200 },
  { id: 'kb-faq', kind: 'kb', name: 'Parity FAQ Fixture', settle: 3000 },
  { id: 'kb-wiki', kind: 'kb', name: 'Wiki Parity Fixture', settle: 3000 },
  { id: 'kb-demo', kind: 'kb', name: 'Parity KB Demo', settle: 3000 },
  { id: 'kb-wiki-tab-wiki', kind: 'kb', name: 'Wiki Parity Fixture', suffix: '?tab=wiki', settle: 3500 },
  { id: 'kb-wiki-tab-graph', kind: 'kb', name: 'Wiki Parity Fixture', suffix: '?tab=graph', settle: 3500 },
  { id: 'kb-demo-creatchat', kind: 'kb', name: 'Parity KB Demo', suffix: '/creatChat' },
  { id: 'login', path: '/login', auth: false },
  { id: 'register', path: '/register', auth: false },
  { id: 'redirect-system', path: '/platform/system' },
  { id: 'redirect-integrations', path: '/platform/integrations' },
  { id: 'dev-markdown', path: '/platform/dev/markdown', settle: 2600 },
  ...SETTINGS_SECTION_KEYS.map((key) => ({
    id: 'settings-' + key, path: '/platform/settings?section=' + key, settle: 2000,
  })),
  ...INTEGRATION_TAB_KEYS.map((key) => ({
    id: 'settings-integration-' + key, path: '/platform/settings?section=integration-' + key, settle: 2000,
  })),
];
const PAGES = PAGE_FILTER.length ? ALL_PAGES.filter(p => PAGE_FILTER.includes(p.id)) : ALL_PAGES;

const GUIDE_KEYS = [
  'weknora:contextual-guide-kb-list:v2', 'weknora:contextual-guide-kb-create:v3',
  'weknora:contextual-guide-tenant-models:v1', 'weknora:contextual-guide-kb-detail:v1',
  'weknora:contextual-guide-chat:v1', 'weknora:contextual-guide-agent-list:v1',
  'weknora:contextual-guide-agent-create:v1', 'weknora:new-user-guide-done:v1',
];

// ---- 已覆盖的 14 个 ix-* 入口签名（矩阵标注"复用"） ----
function matchIx(pageId, c) {
  const aria = c.aria || '', text = c.text || '', guid = c.guid || '', cls = c.cls || '';
  const isKb = pageId.startsWith('kb-');
  if (pageId === 'kb-list' && (guid === 'kb-list-create' || /新建知识库|创建知识库/.test(aria + text))) return 'ix-kb-list-create';
  if (isKb && /kb-settings-button/.test(cls)) return 'ix-kb-settings';
  if (isKb && text === '批量管理') return 'ix-kb-batch';
  if (isKb && (/doc-view-toggle/.test(cls) || aria === '列表视图' || text === '列表视图')) return 'ix-kb-listview';
  if (isKb && (text === '全部标签' || aria === '全部标签')) return 'ix-faq-tagfilter';
  if (isKb && (/检索测试/.test(aria) || /content-bar-icon-btn/.test(cls))) return 'ix-faq-retrieval';
  if (isKb && (aria === '新建' || text === '新建')) return 'ix-faq-import';
  if (pageId === 'chat' && /更多操作|更多/.test(aria)) return 'ix-chat-header-menu';
  if (pageId === 'chat' && guid === 'chat-kb-mention') return 'ix-chat-mention';
  if (pageId === 'agents' && (guid === 'agent-list-create' || /创建智能体/.test(aria + text))) return 'ix-agents-create';
  if (pageId === 'orgs' && text === '我创建的') return 'ix-orgs-created';
  if (pageId === 'orgs' && (/org-create-icon|header-action-btn/.test(cls) || /创建共享空间/.test(aria + text))) return 'ix-orgs-create';
  if (isKb && /mermaid-arch-demo/.test(text)) return 'ix-kb-doc-detail';
  return null;
}

// ---- 只记录不实点（破坏性 / 直接写操作 / 原生文件选择器 / shell 状态切换） ----
const DENY_TEXT = /(删除|清空|卸载|移除|解散|注销|停用|禁用|启用|激活|重置|断开|登出|退出|发送|上传|保存|提交|确定|确认|应用|立即|测试|开始|停止|重启|刷新|同步|复制|下载|导出|登录|注册|搜索|检索|查询|播放|暂停|生成|执行|运行|安装|绑定|解绑|授权|购买|支付|归档|锁定|解锁|恢复|回滚|撤销|重试|新对话|新建对话|新建会话|收藏|置顶|连接|分享到|发布|收起|折叠|侧边栏|展开侧边栏)/;
const DENY_EN = /(delete|remove|save|submit|confirm|cancel|apply|upload|send|sign-?in|sign-?out|sign-?up|logout|run|start|stop|refresh|sync|copy|download|export)/i;
function denied(c) {
  const hay = [c.aria, c.title, c.text, c.guid, c.testid].filter(Boolean).join(' ');
  if (!hay) return null;
  const zh = hay.match(DENY_TEXT); if (zh) return 'deny:' + zh[0];
  const trimmed = hay.trim();
  if (trimmed && !/[\u4e00-\u9fff]/.test(trimmed) && DENY_EN.test(trimmed)) {
    const en = trimmed.match(DENY_EN); if (en) return 'deny:' + en[0];
  }
  return null;
}
// 侧栏导航文本（点击=路由跳转，静态项已覆盖，不必实点）
const NAV_TEXT = /^(知识库|智能体|共享空间|应用|聊天|对话|设置|首页|工作台|应用中心|返回|WeKnora)$/;

// ---- 页内注入代码 ----
const CORE = `
const ISEL = 'button, a, [role="menuitem"], [role="button"], [role="tab"], [role="switch"], [role="combobox"], [data-guide], [data-testid], [class*="btn"], [class*="button"], [class*="icon-btn"], [class*="trigger"], .user-button, .t-select, .t-date-picker, .t-cascader, .t-tree-select, .t-color-picker, summary';
// 注意：.settings-overlay/.settings-modal 不在枚举排除之列——settings 页本体就渲染在
// 常驻 settings-overlay 容器里（双端同构），排除会吞掉全部设置页触发器；弹层检测层
// （overlaysNow）仍跟踪该类，点击新增实例照样进 diff。
const OVERLAY_AT = '.t-popup, .t-dialog, .t-drawer, .t-popconfirm, .t-select__dropdown, .t-tooltip, [class*="menu-panel"], [class*="artifact-drawer"], [class*="sandbox-drawer"], [class*="craft-panel"]';
const sigOf = (el) => {
  const cls = (el.className && el.className.toString) ? el.className.toString().trim().split(/\\s+/).slice(0, 5).join('.') : '';
  return [el.tagName.toLowerCase(), cls, el.getAttribute('aria-label') || '', el.getAttribute('title') || '',
    el.getAttribute('data-guide') || '', el.getAttribute('data-testid') || '',
    (el.innerText || el.textContent || '').trim().replace(/\\s+/g, ' ').slice(0, 40)].join('|');
};
const descOf = (el) => {
  const r = el.getBoundingClientRect();
  const cls = (el.className && el.className.toString) ? el.className.toString().trim().split(/\\s+/).slice(0, 5).join('.') : '';
  return {
    tag: el.tagName.toLowerCase(), id: el.id || '', cls, aria: el.getAttribute('aria-label') || '',
    title: el.getAttribute('title') || '', guid: el.getAttribute('data-guide') || '',
    testid: el.getAttribute('data-testid') || '', role: el.getAttribute('role') || '',
    text: (el.innerText || el.textContent || '').trim().replace(/\\s+/g, ' ').slice(0, 40),
    x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height),
    href: el.tagName === 'A' ? (el.getAttribute('href') || '') : '',
    sig: sigOf(el),
  };
};
function enumCore() {
  const out = []; const rects = [];
  for (const el of document.querySelectorAll(ISEL)) {
    if (el.closest(OVERLAY_AT)) continue;
    const tagOk = /^(button|a|summary)$/i.test(el.tagName) || el.getAttribute('role') || el.getAttribute('data-guide') || el.getAttribute('data-testid') || /icon-btn|trigger|user-button/.test(el.className || '');
    if (!tagOk && el.querySelector('button, a, [role="menuitem"], [role="button"], [class*="trigger"]')) continue;
    const r = el.getBoundingClientRect();
    if (r.width < 3 || r.height < 3) continue;
    if (r.right <= 0 || r.bottom <= 0 || r.left >= innerWidth || r.top >= innerHeight) continue;
    const s = getComputedStyle(el);
    if (s.visibility === 'hidden' || s.display === 'none' || +s.opacity === 0) continue;
    if (r.width > 520 || r.height > 160) continue;
    const cx = r.x + r.width / 2, cy = r.y + r.height / 2;
    let dup = false;
    for (const ar of rects) {
      if (cx >= ar.x && cx <= ar.x + ar.width && cy >= ar.y && cy <= ar.y + ar.height) {
        // 中心命中已收录矩形：仅在面积相近（icon-in-button 紧包裹）时视为重复；
        // 大容器（列表行等）内的子按钮仍要单独枚举。
        if (ar.width * ar.height <= r.width * r.height * 3.2) { dup = true; break; }
      }
    }
    if (dup) continue;
    rects.push(r);
    out.push(descOf(el));
  }
  return out;
}`;

const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
const enumAt = (scrollRatio) => new AsyncFunction(`${CORE}
window.scrollTo(0, Math.max(0, (document.documentElement.scrollHeight - innerHeight) * ${scrollRatio}));
await new Promise(r => setTimeout(r, 350));
return enumCore();`);

// hover 门控补漏（真实鼠标 hover 首个会话行——CSS :hover 揭示 menu-more 等，
// 合成 mouseenter 无法触发 CSS 伪类）
const ROW_SEL = '.submenu_item, [class*="session-item"], [class*="session_row"]';
async function hoverProbe(page) {
  const n = await page.locator(ROW_SEL).first().count().catch(() => 0);
  if (!n) return [];
  await page.locator(ROW_SEL).first().hover({ timeout: 4000 }).catch(() => {});
  await page.waitForTimeout(500);
  const list = await page.evaluate(new Function(`${CORE}; return enumCore();`)).catch(() => []);
  return list.map(c => ({ ...c, hover: true, rowSel: ROW_SEL }));
}

// 触发器 aria-expanded 兜底：React 会话行菜单等"面板常驻 DOM、打开只切状态"的
// 场景 diff 探测不到新元素，用触发器自身 aria-expanded 判定。
const ariaStateOf = new Function('sig', `${CORE}
for (const el of document.querySelectorAll(ISEL)) {
  if (sigOf(el) !== sig) continue;
  return { expanded: el.getAttribute('aria-expanded'), hasPopup: el.getAttribute('aria-haspopup') };
}
return null;`);

// 真实点击前的标记：全 DOM（不限视口）按 sig 找可见元素 → scrollIntoView → data-px-target
const markSig = new Function('sig', `${CORE}
for (const el of document.querySelectorAll(ISEL)) {
  if (el.closest(OVERLAY_AT)) continue;
  if (sigOf(el) !== sig) continue;
  const r = el.getBoundingClientRect();
  if (r.width < 3 || r.height < 3) continue;
  const s = getComputedStyle(el);
  if (s.visibility === 'hidden' || s.display === 'none' || +s.opacity === 0) continue;
  el.scrollIntoView({ block: 'nearest' });
  el.setAttribute('data-px-target', '1');
  return true;
}
return false;`);
const clearMark = new Function(`document.querySelectorAll('[data-px-target]').forEach(e => e.removeAttribute('data-px-target'));`);

// 浮层探测器：白名单面板类 OR fixed/absolute/sticky 的可见元素（基线 diff 用；
// message/toast/spinner/loading/guide 类瞬态或常驻噪音直接排除）
const overlaysNow = new Function(`${CORE}
const WL = '.t-dialog, .t-drawer, .t-popup, .t-dropdown__menu, .t-select__dropdown, .t-popconfirm, .t-date-picker__panel, .t-color-picker__panel, .t-image-viewer, .t-cascader__panel, [role="dialog"], [role="menu"], [role="listbox"], [role="tooltip"], .mention-menu, .user-menu, .settings-overlay, .settings-modal, [class*="drawer"], [class*="modal"], [class*="popover"], [class*="dropdown"], [class*="overlay"], [class*="popup"], [class*="confirm"], [class*="craft-panel"], [class*="menu-panel"], [class*="session-action-menu"], [class*="session-group"]';
const NOISE = /spinner|loading|skeleton|t-message|t-toast|guide|carousel|swiper|animated|node-|line-/;
const out = [];
for (const el of document.body.querySelectorAll('*')) {
  const t = el.tagName;
  if (t === 'SCRIPT' || t === 'STYLE' || t === 'LINK' || t === 'META' || t === 'SVG' || t === 'PATH' || t === 'HEAD') continue;
  const r = el.getBoundingClientRect();
  if (r.width < 40 || r.height < 24) continue;
  if (r.right <= 0 || r.bottom <= 0 || r.left >= innerWidth || r.top >= innerHeight) continue;
  const s = getComputedStyle(el);
  if (s.visibility === 'hidden' || s.display === 'none' || +s.opacity === 0) continue;
  const cls = (el.className && el.className.toString) ? el.className.toString() : '';
  if (NOISE.test(cls)) continue;
  const pos = s.position;
  const wl = el.matches(WL);
  if (!wl && pos !== 'fixed' && pos !== 'absolute' && pos !== 'sticky') continue;
  out.push({
    key: [t.toLowerCase(), cls.slice(0, 40), el.id || ''].join('|'),
    cls: cls.slice(0, 80), id: el.id, role: el.getAttribute('role') || '', tag: t.toLowerCase(),
    text: (el.innerText || '').trim().replace(/\\s+/g, ' ').slice(0, 60),
    x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height),
  });
}
return out;`);

function classify(p) {
  const c = ((p.cls || '') + ' ' + (p.id || '') + ' ' + (p.role || '')).toLowerCase();
  if (c.includes('mention')) return 'mention-menu';
  if (c.includes('date-picker')) return 'date-picker';
  if (c.includes('color-picker')) return 'color-picker';
  if (c.includes('cascader')) return 'cascader';
  if (c.includes('tree-select')) return 'tree-select';
  if (c.includes('popconfirm') || c.includes('confirm')) return 'confirm';
  if (c.includes('select__dropdown') || c.includes('select-dropdown') || p.role === 'listbox' || (c.includes('t-select') && c.includes('popup'))) return 'select-popup';
  if (c.includes('selector-overlay')) return 'select-popup';
  if (c.includes('session-group') || c.includes('group-card') || c.includes('source-filter')) return 'dropdown';
  if (c.includes('dropdown') || p.role === 'menu' || c.includes('user-menu')) return 'dropdown';
  if (c.includes('artifact-drawer') || c.includes('sandbox') || c.includes('craft-panel')) return 'drawer';
  if (c.includes('drawer')) return 'drawer';
  if (c.includes('settings-overlay') || c.includes('settings-modal') || c.includes('dialog') || c.includes('backdrop') || p.role === 'dialog' || c.includes('modal')) return 'dialog';
  if (c.includes('image-viewer') || c.includes('image-view')) return 'image-viewer';
  if (c.includes('popup') || c.includes('popover') || p.role === 'tooltip') return 'popover';
  return 'popup-other';
}

// ---- 凭据/登录/fixture：与 auto-scan.mjs 一致 ----
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
async function api(path, token, tenantId, init = {}) {
  const res = await fetch(BACKEND + path, {
    ...init,
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}`, 'X-Tenant-ID': String(tenantId), ...(init.headers || {}) },
  });
  return { status: res.status, body: await res.json().catch(() => null) };
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
  page.setDefaultTimeout(20000);
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
  }, [auth, GUIDE_KEYS, base === REACT]);
  page.on('filechooser', () => {});
  return page;
}
async function steady(page, settleMs) {
  await page.waitForLoadState('networkidle', { timeout: 9000 }).catch(() => {});
  await page.evaluate(() => document.fonts.ready).catch(() => {});
  await page.waitForTimeout(settleMs ?? 2400);
}

// 基线 map：key -> count（含 hover 枚举后常驻的浮层）
async function snapshot(page) {
  const list = await page.evaluate(overlaysNow).catch(() => []);
  const m = new Map();
  for (const x of list) m.set(x.key, (m.get(x.key) || 0) + 1);
  return m;
}
// diff：新增 key 或同 key 计数增加（同类的第二个实例也算新面板）
function diffOverlays(after, base) {
  const news = [];
  for (const x of after) {
    const before = base.get(x.key) || 0;
    const seenOfKey = news.filter(n => n.key === x.key).length;
    if (seenOfKey < before) continue; // 该 key 已有 before 个旧实例
    news.push(x);
  }
  return news;
}
async function panelsOpenCount(page, base) {
  const after = await page.evaluate(overlaysNow).catch(() => []);
  return diffOverlays(after, base).length;
}

async function closePanels(page, base, path, base0) {
  if (!(await panelsOpenCount(page, base0))) return { how: 'clean' };
  await page.keyboard.press('Escape').catch(() => {});
  await page.waitForTimeout(400);
  if (!(await panelsOpenCount(page, base0))) return { how: 'esc' };
  for (const [x, y] of [[640, 15], [15, 700], [1265, 700], [640, 712], [15, 15]]) {
    const safe = await page.evaluate(([x, y]) => {
      const el = document.elementFromPoint(x, y);
      if (!el) return false;
      return !el.closest('button, a, input, select, textarea, [role="menuitem"], [role="button"], [role="tab"], [role="switch"], [class*="select"], [class*="btn"], [class*="button"], [class*="trigger"]');
    }, [x, y]).catch(() => false);
    if (safe) {
      await page.mouse.click(x, y).catch(() => {});
      await page.waitForTimeout(420);
      if (!(await panelsOpenCount(page, base0))) return { how: 'outside' };
    }
  }
  await page.keyboard.press('Escape').catch(() => {});
  await page.waitForTimeout(280);
  if (!(await panelsOpenCount(page, base0))) return { how: 'esc2' };
  await page.goto(base + path, { waitUntil: 'domcontentloaded', timeout: 20000 });
  await steady(page, 2300);
  return { how: 'reload' };
}

function slug(s) {
  return (s || '').replace(/[^a-zA-Z0-9\u4e00-\u9fff]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 34) || 'trig';
}

async function clickReal(page, sig) {
  let marked = await page.evaluate(markSig, sig).catch(() => false);
  if (!marked) {
    // 重载/切换后异步渲染的元素可能迟到：延迟重试一次
    await page.waitForTimeout(1300);
    marked = await page.evaluate(markSig, sig).catch(() => false);
    if (!marked) return 'lost';
  }
  try {
    await page.click('[data-px-target]', { timeout: 6000 });
    await page.evaluate(clearMark).catch(() => {});
    return true;
  } catch {
    await page.evaluate(clearMark).catch(() => {});
    const marked2 = await page.evaluate(markSig, sig).catch(() => false);
    if (!marked2) return 'lost';
    try { await page.click('[data-px-target]', { timeout: 3500, force: true }); return true; } catch { return 'blocked'; }
  }
}

async function main() {
  for (const [name, base] of [['vue', VUE], ['react', REACT], ['backend', BACKEND]]) {
    try {
      const r = await fetch(base + '/', { signal: AbortSignal.timeout(4000) });
      if (r.status >= 500) throw new Error(String(r.status));
    } catch (e) { console.error(`[env] ${name} (${base}) 不在线：${e.message}`); process.exit(1); }
  }
  const auth = await login();
  const fixtures = await resolveFixtures(auth.token, auth.tenantId);
  console.log(`[auth] tenant=${auth.tenantId} fixtures=${Object.keys(fixtures).length}/${PAGES.filter(p => p.kind).length}`);

  const results = [];
  let shotCount = 0;
  const seenFinal = {};
  const browser = await chromium.launch();
  try {
    const vuePage = await newAuthedPage(await browser.newContext(), VUE, auth);
    const reactPage = await newAuthedPage(await browser.newContext(), REACT, auth);
    const anonVue = await (await browser.newContext()).newPage();
    const anonReact = await (await browser.newContext()).newPage();
    await anonVue.setViewportSize({ width: 1280, height: 720 });
    await anonReact.setViewportSize({ width: 1280, height: 720 });

    for (const p of PAGES) {
      const fixturePath = fixtures[p.id];
      const path = p.kind ? (fixturePath ? fixturePath + (p.suffix || '') : undefined) : p.path;
      if (!path) { console.log(`[skip] ${p.id}（fixture 未找到）`); continue; }
      const entry = { id: p.id, path, ends: {} };
      for (const [tag, base] of [['vue', VUE], ['react', REACT]]) {
        const page = p.auth === false ? (tag === 'vue' ? anonVue : anonReact) : (tag === 'vue' ? vuePage : reactPage);
        try {
          await page.goto(base + path, { waitUntil: 'domcontentloaded', timeout: 22000 });
          await steady(page, p.settle);
          const finalPath = page.url().replace(base, '').split('#')[0];
          if (seenFinal[finalPath] && seenFinal[finalPath] !== p.id && results.find(r => r.id === seenFinal[finalPath])) {
            const src = results.find(r => r.id === seenFinal[finalPath]);
            entry.ends[tag] = { finalPath, clonedFrom: src.id, candidates: null };
            console.log(`[dup ] ${p.id}/${tag} → ${finalPath}（同 ${src.id}）`);
            continue;
          }
          if (!seenFinal[finalPath]) seenFinal[finalPath] = p.id;
          entry.ends[tag] = { finalPath, candidates: [] };

          // 枚举：3 段滚动 + 真实 hover 探针（hover 在基线快照前，其常驻浮层进基线）
          const candMap = new Map();
          for (const ratio of [0, 0.6, 1]) {
            const list = await page.evaluate(enumAt(ratio)).catch(() => []);
            for (const c of list) if (!candMap.has(c.sig)) candMap.set(c.sig, c);
          }
          await page.evaluate(() => window.scrollTo(0, 0)).catch(() => {});
          const hovered = await hoverProbe(page);
          for (const c of hovered) if (!candMap.has(c.sig)) candMap.set(c.sig, c);
          await page.waitForTimeout(300);
          // hover 揭示的候选先处理（离开 hover 区即失效）
          const cands = [...candMap.values()].sort((a, b) => (b.hover ? 1 : 0) - (a.hover ? 1 : 0));
          let base0 = await snapshot(page);

          const seenKey = new Set();
          let clicks = 0, reloads = 0, panelsFound = 0, shots = 0;
          const shotTypes = new Set();
          for (const c of cands) {
            const ix = matchIx(p.id, c);
            const key = (c.guid || c.testid || c.aria || c.title || c.text || c.cls || c.tag) + '|' + (c.href || '');
            if (seenKey.has(key)) continue;
            seenKey.add(key);
            const rec = { ...c, ix: ix || undefined, category: 'click', panel: null, close: null };

            if (!ix && c.href && !['#', ''].includes(c.href) && !/^javascript:/i.test(c.href)) { rec.category = 'nav-link'; entry.ends[tag].candidates.push(rec); continue; }
            if (!ix && (NAV_TEXT.test((c.text || '').trim()) || NAV_TEXT.test((c.aria || '').trim()))) { rec.category = 'nav-link'; rec.reason = '侧栏/导航文本'; entry.ends[tag].candidates.push(rec); continue; }
            if (c.tag === 'div' && /submenu_item|menu_item|session-item|session-row/.test(c.cls)) { rec.category = 'nav-link'; rec.reason = '会话/侧栏行（点击即导航）'; entry.ends[tag].candidates.push(rec); continue; }
            if (c.role === 'tab') { rec.category = 'inline-tab'; rec.reason = 'tab 切换内联视图，非面板'; entry.ends[tag].candidates.push(rec); continue; }
            if (c.role === 'switch') { rec.category = 'toggle'; rec.reason = '开关，内联状态变更'; entry.ends[tag].candidates.push(rec); continue; }
            if (/pagination|pager/i.test(c.cls + ' ' + c.aria + ' ' + c.testid)) { rec.category = 'pagination'; entry.ends[tag].candidates.push(rec); continue; }
            const deny = denied(c);
            if (deny && !(ix && ['ix-kb-list-create', 'ix-agents-create', 'ix-orgs-create', 'ix-faq-import'].includes(ix))) {
              rec.category = 'deny'; rec.reason = deny;
              entry.ends[tag].candidates.push(rec); continue;
            }
            if (clicks >= MAX_CLICKS) { rec.category = 'cap'; rec.reason = '单页点击上限'; entry.ends[tag].candidates.push(rec); continue; }

            const preUrl = page.url();
            if (c.hover && c.rowSel) await page.locator(c.rowSel).first().hover({ timeout: 3000 }).catch(() => {});
            const clickRes = await clickReal(page, c.sig);
            if (clickRes !== true) { rec.category = clickRes; entry.ends[tag].candidates.push(rec); continue; }
            clicks++;
            await page.waitForTimeout(950);
            if (page.url() !== preUrl) {
              rec.category = 'nav-jump'; rec.reason = '点击后路由变化';
              await page.goto(base + path, { waitUntil: 'domcontentloaded', timeout: 20000 });
              await steady(page, 1600); reloads++;
              entry.ends[tag].candidates.push(rec); continue;
            }
            const after = await page.evaluate(overlaysNow).catch(() => []);
            const news = diffOverlays(after, base0);
            // aria-expanded 兜底：面板常驻 DOM、打开只切状态（React 会话行菜单）
            const aState = news.length ? null : await page.evaluate(ariaStateOf, c.sig).catch(() => null);
            const openedViaAria = !news.length && aState && aState.expanded === 'true';
            if (news.length || openedViaAria) {
              const typed = news.map(x => ({ ...x, type: classify(x) }));
              if (openedViaAria && !typed.length) {
                typed.push({ type: aState.hasPopup === 'menu' ? 'dropdown' : 'popover', cls: '(aria-expanded)', text: '', x: c.x, y: c.y, w: 0, h: 0 });
              }
              rec.panel = { types: [...new Set(typed.map(x => x.type))], items: typed.slice(0, 4).map(({ type, cls, text, x, y, w, h }) => ({ type, cls: (cls || '').slice(0, 50), text: (text || '').slice(0, 30), at: `${x},${y} ${w}x${h}` })) };
              const real = typed.filter(x => x.type !== 'message');
              if (real.length) {
                panelsFound++;
                const mainType = real[0].type;
                if (!shotTypes.has(mainType) && shotCount < SHOT_BUDGET) {
                  shotTypes.add(mainType); shotCount++; shots++;
                  rec.shot = `${p.id}-${slug(c.guid || c.aria || c.title || c.text || c.cls)}-${tag}.png`;
                  await page.screenshot({ path: join(PNG, rec.shot) }).catch(() => {});
                }
                const closeRes = await closePanels(page, base, path, base0);
                rec.close = closeRes.how;
                // aria 路径的关闭校验：触发器 expanded 应回 false
                if (openedViaAria) {
                  const st = await page.evaluate(ariaStateOf, c.sig).catch(() => null);
                  if (st && st.expanded === 'true') {
                    await page.keyboard.press('Escape').catch(() => {});
                    await page.waitForTimeout(350);
                    await page.mouse.click(640, 15).catch(() => {});
                    await page.waitForTimeout(350);
                  }
                }
                if (closeRes.how === 'reload') {
                  reloads++;
                  base0 = await snapshot(page); // 重载后重建基线
                } else {
                  // 吸收关闭后仍常驻的新浮层（防级联误报）
                  const cur = await snapshot(page);
                  for (const [k, v] of cur) if (v > (base0.get(k) || 0)) base0.set(k, v);
                }
              } else {
                rec.category = 'toast-only';
              }
            }
            entry.ends[tag].candidates.push(rec);
          }
          entry.ends[tag].summary = { candidates: cands.length, clicked: clicks, panels: panelsFound, shots, reloads };
          console.log(`[done] ${p.id}/${tag}: cands=${cands.length} clicked=${clicks} panels=${panelsFound} shots=${shots} reloads=${reloads}`);
        } catch (e) {
          entry.ends[tag] = { error: String(e.message).split('\n')[0] };
          console.log(`[err ] ${p.id}/${tag}: ${entry.ends[tag].error}`);
        }
      }
      results.push(entry);
      writeFileSync(join(OUT, 'inventory.json'), JSON.stringify({ stamp: new Date().toISOString(), vue: VUE, react: REACT, shotBudget: SHOT_BUDGET, results }, null, 1));
    }
  } finally {
    await browser.close();
  }
  const tot = { cands: 0, clicked: 0, panels: 0 };
  for (const r of results) for (const t of ['vue', 'react']) {
    const s = r.ends[t]?.summary; if (!s) continue;
    tot.cands += s.candidates; tot.clicked += s.clicked; tot.panels += s.panels;
  }
  console.log(`[report] ${join(OUT, 'inventory.json')} | total cands=${tot.cands} clicked=${tot.clicked} panels=${tot.panels} shots=${shotCount}`);
}

main().catch(e => { console.error('[fatal]', e); process.exit(1); });
