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
// playwright 解析：优先 apps/web 的 @playwright/test（pnpm 安装），
// 失败则回退到本目录隔离安装的 playwright-core + ms-playwright 缓存的
// headless shell（apps/web 的 node_modules 曾被并行会话清空过）。
let chromium;
try {
  // CJS require 最确定：ESM import 该包的命名导出探测在重装后间歇失败
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
// Settings section 键表：frontend/src/views/settings/Settings.vue navItems +
// currentSection 分支 + frontend/src/config/integrations.ts INTEGRATION_TABS。
const SETTINGS_SECTION_KEYS = [
  'general', 'userprofile', 'mymemory', 'envvars', 'tenant', 'members',
  'models', 'ollama', 'weknoracloud', 'chathistory', 'memory',
  'vectorstore', 'parser', 'storage', 'sandbox', 'skills', 'mcp', 'websearch',
  'system', 'system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log',
];
const INTEGRATION_TAB_KEYS = ['im', 'embed', 'api', 'cli', 'chrome', 'claw'];

const ALL_PAGES = [
  // —— 核心平台路由 ——
  { id: 'kb-list', path: '/platform/knowledge-bases' },
  { id: 'agents', path: '/platform/agents' },
  { id: 'orgs', path: '/platform/organizations' },
  { id: 'creatchat', path: '/platform/creatChat' },
  { id: 'apps', path: '/platform/apps' },
  { id: 'apps-connections', path: '/platform/apps/connections' },
  { id: 'chat', kind: 'chat', name: '工具调用 Parity Fixture' },
  // —— KB fixture 页 + 子视图（tab / KB 内新建对话） ——
  { id: 'kb-faq', kind: 'kb', name: 'Parity FAQ Fixture' },
  { id: 'kb-wiki', kind: 'kb', name: 'Wiki Parity Fixture' },
  { id: 'kb-demo', kind: 'kb', name: 'Parity KB Demo' },
  { id: 'kb-wiki-tab-wiki', kind: 'kb', name: 'Wiki Parity Fixture', suffix: '?tab=wiki', settle: 3500 },
  { id: 'kb-wiki-tab-graph', kind: 'kb', name: 'Wiki Parity Fixture', suffix: '?tab=graph', settle: 3500 },
  { id: 'kb-demo-creatchat', kind: 'kb', name: 'Parity KB Demo', suffix: '/creatChat' },
  // —— 免登录页 ——
  // login/register 的展示轮播（SLIDES 自动切换）相位在两端独立，截图会落在
  // 不同 slide 上产生假差异。截图前冻结动画（pause/play 接口或 animation-play-state），
  // 并等待首帧 slide 稳定，使两端定格在同一张。
  // syncAnimPhase：.animated-bg 装饰动画（nodePulse/lineFlow 纯 CSS 循环动画）在
  // settle 后 pause 的暂停相位是任意的，双端冻结相位不同 → run 间 275↔1387px 波动。
  // 确定性处理：把每条无限循环 CSS 动画 seek 到 keyframe 0% 相位（currentTime=
  // effect.delay，见 waitForSteady 注释——React 端 delay 类未生成规则故全部 0，
  // Vue 端 0~3s 错峰，seek 到各自 delay 才能让双端统一落在同一 keyframe 相位）。
  { id: 'login', path: '/login', auth: false, settle: 3200, freezeCarousel: true, syncAnimPhase: true },
  { id: 'register', path: '/register', auth: false, settle: 3200, freezeCarousel: true, syncAnimPhase: true },
  // —— 重定向行为（两端应落到同一目标页） ——
  { id: 'redirect-system', path: '/platform/system' },
  { id: 'redirect-integrations', path: '/platform/integrations' },
  // —— dev-only 页（两端 dev server 均启用） ——
  { id: 'dev-markdown', path: '/platform/dev/markdown', settle: 2200 },
  // —— 设置：全部 section（Vue Settings.vue navItems 权威键表） ——
  ...SETTINGS_SECTION_KEYS.map((key) => ({
    id: 'settings-' + key, path: '/platform/settings?section=' + key, settle: 2000,
  })),
  ...INTEGRATION_TAB_KEYS.map((key) => ({
    id: 'settings-integration-' + key, path: '/platform/settings?section=integration-' + key, settle: 2000,
  })),
  // —— 交互态：点击后截图（两端各自解析候选目标，找不到则截当前态并记 warning） ——
  // 注意：本条目必须扫知识库"列表"页——不能带 kind:'kb'（会被 fixture 解析改写到
  // KB 详情路径）。Vue 端新建按钮是 t-tooltip + 纯图标 t-button（无 aria/title/text，
  // KnowledgeBaseList.vue:11-17）；React 端 aria/title=新建知识库、data-guide 同名
  // （App.tsx:965）；clickText 保留兜底。
  { id: 'ix-kb-list-create', path: '/platform/knowledge-bases',
    actions: [{ clickAria: ['新建知识库'], clickCss: ['[data-guide="kb-list-create"]'], clickText: ['新建知识库', '创建知识库', '新建'] }] },
  // Vue 端设置按钮是纯图标 button.kb-settings-button（KnowledgeBase.vue:2450-2453），
  // React 端 aria/title=设置（KnowledgeDocumentsPage / FAQPage faq-kb-settings-button 同名）。
  { id: 'ix-kb-settings', kind: 'kb', name: 'Parity KB Demo',
    actions: [{ clickCss: ['.kb-settings-button'], clickAria: ['知识库设置', '设置'] }] },
  { id: 'ix-kb-batch', kind: 'kb', name: 'Parity KB Demo',
    actions: [{ clickText: ['批量管理'] }] },
  // Vue 端视图切换两个纯图标按钮无 aria/title（KnowledgeBase.vue:2665-2677），
  // nth=1 是"列表视图"（第 2 个）；React 端 aria=列表视图 命中。
  { id: 'ix-kb-listview', kind: 'kb', name: 'Parity KB Demo',
    actions: [{ clickCss: ['.doc-view-toggle button >> nth=1'], clickAria: ['列表视图', '列表'] }] },
  { id: 'ix-faq-tagfilter', kind: 'kb', name: 'Parity FAQ Fixture',
    actions: [{ clickText: ['全部标签'] }] },
  { id: 'ix-faq-retrieval', kind: 'kb', name: 'Parity FAQ Fixture',
    // Vue 端检索测试入口是 t-tooltip + 纯图标按钮（无 aria/title，FAQEntryManager.vue:225-229），
    // clickAria 只能命中 React；补 CSS 兜底命中 Vue 图标（svg.t-icon-search 唯一）。
    actions: [{ clickAria: ['检索测试'], clickCss: ['.content-bar-icon-btn:has(svg.t-icon-search)'] }] },
  // 导入入口是两步：先点"新建"下拉（React aria=新建 命中；Vue 端是 t-dropdown +
  // 纯图标按钮，trailing 区第 1 个 content-bar-icon-btn，FAQEntryManager.vue:203-214），
  // 再点菜单项"导入 FAQ"（两端菜单项可见文本均为 i18n faqImport.importButton='导入 FAQ'）。
  { id: 'ix-faq-import', kind: 'kb', name: 'Parity FAQ Fixture',
    actions: [
      { clickAria: ['新建'], clickCss: ['.content-bar-icon-btn >> nth=0'] },
      { clickText: ['导入 FAQ', '导入'] },
    ] },
  { id: 'ix-chat-header-menu', kind: 'chat', name: '工具调用 Parity Fixture',
    actions: [{ clickAria: ['更多操作', '更多'] }] },
  { id: 'ix-chat-mention', kind: 'chat', name: '工具调用 Parity Fixture',
    // composer 的 @ 按钮：双端同名 data-guide="chat-kb-mention"（Vue
    // Input-field.vue:2769 / React packages/views composer.tsx）。clickAria 全
    // miss——Vue 是纯图标 div 无 aria/title，React aria-label='知识库' 又与
    // 侧栏「知识库」导航同名会误伤；clickCss 精确命中后双端各自打开 @ 弹层
    // （Vue .mention-menu / React #wk-chat-mention-listbox）。clickText '@' 兜底
    // 有坑：DOM 序更早的侧栏邮箱 parity-test@local.dev 含 '@'，仅在前两级
    // 同时失效时才会误中（点击冒泡到 user-button 偶然打开菜单）。
    actions: [{ clickAria: ['@提及知识库', '提及知识库', 'mention'], clickCss: ['[data-guide="chat-kb-mention"]'], clickText: ['@'] }] },
  // 两端新建按钮均为纯图标（Vue AgentList.vue:11-15 t-tooltip 无 aria；React
  // AgentsPage.tsx:582 aria/title=创建智能体）；data-guide 两端同名可命中 Vue。
  { id: 'ix-agents-create', path: '/platform/agents',
    actions: [{ clickCss: ['[data-guide="agent-list-create"]'], clickAria: ['创建智能体'], clickText: ['新建智能体', '创建智能体', '新建'] }] },
  { id: 'ix-orgs-created', path: '/platform/organizations',
    actions: [{ clickText: ['我创建的'] }] },
  // Vue 端创建按钮是 header 第 2 个纯图标 t-button（OrganizationList.vue:16-21，
  // 图标 img.org-create-icon 唯一）；React 端 aria/title=创建共享空间。
  { id: 'ix-orgs-create', path: '/platform/organizations',
    actions: [{ clickCss: ['.header-action-btn:has(.org-create-icon)'], clickAria: ['创建共享空间'], clickText: ['创建共享空间', '新建共享空间', '创建空间'] }] },
  { id: 'ix-kb-doc-detail', kind: 'kb', name: 'Parity KB Demo',
    actions: [{ clickText: ['mermaid-arch-demo'] }] },
  // —— 面板扫描项（panel-matrix Phase I 全量入表，55 项；id 规范 px-<页面>-<面板>）——
  // 来源：docs/migrations/react/evidence/vue-react-parity/panel-matrix/matrix.md
  // 「建议新增扫描项 55 项 = 表内 53 行（parser 10 卡归并代表入口）+ 全局壳 2」；
  // 选择器兜底链照 matrix（含 C 类命名异构归并：.kb-info-button↔aria 查看知识库信息、
  // .t-select-input↔.t-select、composer 触发器异构等）。破坏性面板只打开不确认；
  // hover 门控触发器用真实 hover 序列（hoverCss，矩阵已标注）。
  //
  // —— 批 1：全局壳 + 对话域 ——
  // 用户菜单 dropdown（42 个登录页通用壳触发器，代表页 kb-list；双端同名 .user-button）
  { id: 'px-shell-user-menu', path: '/platform/knowledge-bases',
    actions: [{ clickCss: ['.user-button'] }] },
  // 会话行"更多" popover：hover 门控（matrix：hover .submenu_item → click .menu-more-wrap，
  // 双端同名；React 端菜单面板常驻 DOM 打开只切状态——aria-expanded 口径，matrix 口径 4）
  { id: 'px-shell-session-more', kind: 'chat', name: '工具调用 Parity Fixture',
    actions: [{ hoverCss: ['.submenu_item'], clickCss: ['.menu-more-wrap'] }] },
  // chat 沙箱终端：Vue drawer(chat-sandbox-panel) vs React 首检出 tooltip（异构 A3，待核）
  { id: 'px-chat-sandbox', kind: 'chat', name: '工具调用 Parity Fixture',
    actions: [{ clickAria: ['沙箱终端'], clickCss: ['.sandbox-header-toggle__btn'] }] },
  // chat 添加到知识库：Vue drawer(t-drawer--right) vs React dialog(wk-bookmark-dialog)（异构 A1）。
  // 双端均为 answer-toolbar 纯图标 t-button（title=添加到知识库，无可见文本），
  // clickText 无命中——走 clickAria(title) + 图标 CSS 兜底；pickLast 取最新一条
  // 回答的工具栏（React 历史消息也渲染 toolbar，首个可见匹配会错位取景）。
  { id: 'px-chat-addtokb', kind: 'chat', name: '工具调用 Parity Fixture',
    actions: [{ clickAria: ['添加到知识库'], clickCss: ['.answer-toolbar button:has(.t-icon-bookmark-add)', '.wk-chat-bookmark'], pickLast: true }] },
  // chat 请求信息：Vue popover(chat-request-info-popup) vs React dialog(chat-request-card)（异构 A2）。
  // 触发器同为纯图标按钮 title=请求信息（Vue ChatRequestInfoButton.vue / React message-face.tsx），
  // pickLast 同上取最新一条回答。
  { id: 'px-chat-reqinfo', kind: 'chat', name: '工具调用 Parity Fixture',
    actions: [{ clickAria: ['请求信息'], pickLast: true }] },
  // composer 智能体选择（B1 组）：Vue .control-btn.agent-mode-btn（文案"快速问答"）与
  // React .wk-chat-agent-chip（aria=选择智能体）打开同一 agent-selector overlay——
  // 双端触发器异构、面板同功能，一条兜底链覆盖两行矩阵记录
  { id: 'px-chat-agent-selector', kind: 'chat', name: '工具调用 Parity Fixture',
    actions: [{ clickAria: ['选择智能体'], clickCss: ['.agent-mode-btn', '.wk-chat-agent-chip'], clickText: ['快速问答'] }] },
  // composer 模型选择：Vue .model-selector-trigger → model-selector-overlay；React 为
  // native select 的 .wk-chat-model-chip（B1 待核实对端）
  { id: 'px-chat-model-selector', kind: 'chat', name: '工具调用 Parity Fixture',
    actions: [{ clickCss: ['.model-selector-trigger', '.wk-chat-model-chip'] }] },
  // chat 附件按钮 tooltip（B8）：hover 门控 t-tooltip（Vue .attachment-upload-btn /
  // React aria=上传附件 的 .wk-chat-control-icon），纯 hover 序列不点击（避免文件选择器）
  { id: 'px-chat-attach-tooltip', kind: 'chat', name: '工具调用 Parity Fixture',
    actions: [{ hoverCss: ['.attachment-upload-btn', '[aria-label="上传附件"]'] }] },
  // —— 批 2：知识库域 ——
  // KB 面包屑下拉（A4：Vue t-popup vs React 自定义 dropdown，5 页同款逐页入表）
  { id: 'px-kb-faq-breadcrumb', kind: 'kb', name: 'Parity FAQ Fixture',
    actions: [{ clickCss: ['.breadcrumb-link.dropdown'] }] },
  { id: 'px-kb-demo-breadcrumb', kind: 'kb', name: 'Parity KB Demo',
    actions: [{ clickCss: ['.breadcrumb-link.dropdown'] }] },
  { id: 'px-kb-wiki-breadcrumb', kind: 'kb', name: 'Wiki Parity Fixture',
    actions: [{ clickCss: ['.breadcrumb-link.dropdown'] }] },
  { id: 'px-kb-wiki-tab-graph-breadcrumb', kind: 'kb', name: 'Wiki Parity Fixture', suffix: '?tab=graph', settle: 3500,
    actions: [{ clickCss: ['.breadcrumb-link.dropdown'] }] },
  { id: 'px-kb-wiki-tab-wiki-breadcrumb', kind: 'kb', name: 'Wiki Parity Fixture', suffix: '?tab=wiki', settle: 3500,
    actions: [{ clickCss: ['.breadcrumb-link.dropdown'] }] },
  // FAQ 卡片更多菜单（.card-more-btn 双端一致）
  { id: 'px-kb-faq-card-more', kind: 'kb', name: 'Parity FAQ Fixture',
    actions: [{ clickCss: ['.card-more-btn'] }] },
  // 查看知识库信息（.kb-info-button(Vue) ↔ aria 查看知识库信息(React)，popover+drawer；代表页 kb-faq）
  { id: 'px-kb-faq-kb-info', kind: 'kb', name: 'Parity FAQ Fixture',
    actions: [{ clickCss: ['.kb-info-button'], clickAria: ['查看知识库信息'] }] },
  // 按标签筛选 select 两半区（ix-faq-tagfilter 已扫主区；prefix/suffix 半区 3 页逐项）
  { id: 'px-kb-faq-tagfilter-prefix', kind: 'kb', name: 'Parity FAQ Fixture',
    actions: [{ clickCss: ['.doc-tag-filter-trigger__prefix'] }] },
  { id: 'px-kb-faq-tagfilter-suffix', kind: 'kb', name: 'Parity FAQ Fixture',
    actions: [{ clickCss: ['.doc-tag-filter-trigger__suffix'] }] },
  { id: 'px-kb-demo-tagfilter-prefix', kind: 'kb', name: 'Parity KB Demo',
    actions: [{ clickCss: ['.doc-tag-filter-trigger__prefix'] }] },
  { id: 'px-kb-demo-tagfilter-suffix', kind: 'kb', name: 'Parity KB Demo',
    actions: [{ clickCss: ['.doc-tag-filter-trigger__suffix'] }] },
  { id: 'px-kb-wiki-tagfilter-prefix', kind: 'kb', name: 'Wiki Parity Fixture',
    actions: [{ clickCss: ['.doc-tag-filter-trigger__prefix'] }] },
  { id: 'px-kb-wiki-tagfilter-suffix', kind: 'kb', name: 'Wiki Parity Fixture',
    actions: [{ clickCss: ['.doc-tag-filter-trigger__suffix'] }] },
  // KB 文档工具栏筛选 select（.t-select-input(Vue) ↔ .t-select(React) 命名异构；
  // 双端同名 .doc-type-select，取第 1 个 = 文件类型筛选，代表整排 select 弹层）。
  // B2 批 2 修正：FAQ KB 的 FAQ 视图工具栏没有该 select（matrix kb-faq 行无此
  // 触发器，基线假零）——代表页改文档库 kb-demo；kb-wiki 页同款由下项覆盖。
  { id: 'px-kb-faq-doctype-select', kind: 'kb', name: 'Parity KB Demo',
    actions: [{ clickCss: ['.doc-type-select >> nth=0'] }] },
  { id: 'px-kb-wiki-doctype-select', kind: 'kb', name: 'Wiki Parity Fixture',
    actions: [{ clickCss: ['.doc-type-select >> nth=0'] }] },
  // wiki 工具栏（B2：树形视图/新建目录/全库概览 React 未检出，新建页面双端 dialog）
  { id: 'px-kb-wiki-tab-wiki-newpage', kind: 'kb', name: 'Wiki Parity Fixture', suffix: '?tab=wiki', settle: 3500,
    actions: [{ clickAria: ['新建页面'], clickCss: ['.wiki-tab-bar-action'] }] },
  { id: 'px-kb-wiki-tab-wiki-newdir', kind: 'kb', name: 'Wiki Parity Fixture', suffix: '?tab=wiki', settle: 3500,
    actions: [{ clickAria: ['新建目录'] }] },
  { id: 'px-kb-wiki-tab-wiki-treeview', kind: 'kb', name: 'Wiki Parity Fixture', suffix: '?tab=wiki', settle: 3500,
    actions: [{ clickAria: ['树形视图'], clickCss: ['.wiki-view-toggle-btn.active'] }] },
  { id: 'px-kb-wiki-tab-wiki-overview', kind: 'kb', name: 'Wiki Parity Fixture', suffix: '?tab=wiki', settle: 3500,
    actions: [{ clickText: ['全库概览'] }] },
  // —— 批 3：设置——账户/模型/引擎卡片 ——
  // userprofile 修改密码（popover+confirm；只开第一层 popover，绝不点确认）
  { id: 'px-userprofile-change-password', path: '/platform/settings?section=userprofile', settle: 2000,
    actions: [{ clickAria: ['修改密码'] }] },
  { id: 'px-mymemory-usage-hint', path: '/platform/settings?section=mymemory', settle: 2000,
    actions: [{ clickAria: ['查看哪些记忆会在对话里被使用'] }] },
  // models 模型卡片 drawer（代表 mock-embedding-model；React 为 wk-model-editor 异构 DOM）
  { id: 'px-models-model-card', path: '/platform/settings?section=models', settle: 2000,
    actions: [{ clickCss: ['.model-card--embedding.model-card--clickable'], clickText: ['mock-embedding-model'] }] },
  // models 卡片更多菜单（A9：.model-card__more(Vue .t-button__text span) / React lost 待补测）
  { id: 'px-models-card-more', path: '/platform/settings?section=models', settle: 2000,
    actions: [{ clickCss: ['.model-card__more', '.model-card__action-btn'] }] },
  // parser 引擎卡片 drawer（代表：内置 DocReader；其余 9 张同款归并——matrix 批 3.5）
  { id: 'px-parser-engine-builtin', path: '/platform/settings?section=parser', settle: 2000,
    actions: [{ clickCss: ['.engine-card--builtin'] }] },
  // storage backend 卡片 drawer（代表 Parity COS）+ 卡片更多菜单（A9 同族）
  { id: 'px-storage-backend-card', path: '/platform/settings?section=storage', settle: 2000,
    actions: [{ clickCss: ['.backend-card--cos.backend-card--clickable'], clickText: ['Parity COS'] }] },
  { id: 'px-storage-card-more', path: '/platform/settings?section=storage', settle: 2000,
    actions: [{ clickCss: ['.backend-card__action-btn'] }] },
  // vectorstore 添加数据库（B4：Vue drawer / React 未检出）+ PostgreSQL 卡片（B5：React drawer）
  { id: 'px-vectorstore-add-db', path: '/platform/settings?section=vectorstore', settle: 2000,
    actions: [{ clickCss: ['.store-card--add'], clickText: ['添加数据库'] }] },
  { id: 'px-vectorstore-pg-card', path: '/platform/settings?section=vectorstore', settle: 2000,
    actions: [{ clickCss: ['.backend-card.is-env'], clickText: ['PostgreSQL'] }] },
  // hint 类 popover（hint-trigger 同款两处）
  { id: 'px-sandbox-what-is-hint', path: '/platform/settings?section=sandbox', settle: 2000,
    actions: [{ clickAria: ['什么是沙箱？'], clickCss: ['.hint-trigger'] }] },
  { id: 'px-envvars-sandbox-key-hint', path: '/platform/settings?section=envvars', settle: 2000,
    actions: [{ clickAria: ['沙箱密钥说明'], clickCss: ['.hint-trigger'] }] },
  // skills 添加技能（A8：Vue drawer vs React dialog+drawer）
  { id: 'px-skills-add', path: '/platform/settings?section=skills', settle: 2000,
    actions: [{ clickText: ['添加技能'] }] },
  // mcp 添加服务 drawer（双端一致）
  { id: 'px-mcp-add-service', path: '/platform/settings?section=mcp', settle: 2000,
    actions: [{ clickCss: ['.service-card--add'], clickText: ['添加服务'] }] },
  // websearch provider 卡片 drawer（代表 Tavily）+ 卡片更多菜单（A9 同族）
  { id: 'px-websearch-provider-card', path: '/platform/settings?section=websearch', settle: 2000,
    actions: [{ clickCss: ['.provider-card--tavily.provider-card--clickable'], clickText: ['Tavily'] }] },
  { id: 'px-websearch-card-more', path: '/platform/settings?section=websearch', settle: 2000,
    actions: [{ clickCss: ['.provider-card__more'] }] },
  // platform-api-keys 创建（drawer 双端一致；只开不创建）
  { id: 'px-platform-api-keys-create', path: '/platform/settings?section=platform-api-keys', settle: 2000,
    actions: [{ clickText: ['创建平台 API Key'] }] },
  // members 角色权限说明（A7：popover vs popover+dialog）
  { id: 'px-members-rbac-hint', path: '/platform/settings?section=members', settle: 2000,
    actions: [{ clickAria: ['角色权限说明'] }] },
  // ollama 重新检测（A11 低置信：React 检出疑似 backdrop，Vue 无）
  { id: 'px-ollama-redetect', path: '/platform/settings?section=ollama', settle: 2000,
    actions: [{ clickText: ['重新检测'] }] },
  // —— 批 4：系统/集成/免登录 ——
  // system-global（/platform/system 落点）：配置来源与优先级 hint + 创建用户（A5：popover vs dialog）
  { id: 'px-system-auth-priority', path: '/platform/settings?section=system-global', settle: 2000,
    actions: [{ clickAria: ['配置来源与优先级'], clickCss: ['.hint-trigger'] }] },
  { id: 'px-system-create-user', path: '/platform/settings?section=system-global', settle: 2000,
    actions: [{ clickCss: ['.create-user-trigger'], clickAria: ['创建用户'] }] },
  // integration（/platform/integrations 落点 im）：按智能体筛选（A6）+ 添加渠道（B3）
  { id: 'px-integrations-agent-filter', path: '/platform/integrations', settle: 2000,
    actions: [{ clickAria: ['按智能体筛选'], clickCss: ['.integrations-agent-filter'] }] },
  { id: 'px-integrations-add-channel', path: '/platform/integrations', settle: 2000,
    actions: [{ clickCss: ['.channel-card--add'], clickText: ['添加渠道'] }] },
  { id: 'px-integration-embed-agent-filter', path: '/platform/settings?section=integration-embed', settle: 2000,
    actions: [{ clickAria: ['按智能体筛选'] }] },
  // integration-api 创建 API Key（B6：Vue drawer / React 未检出）
  { id: 'px-integration-api-create-key', path: '/platform/settings?section=integration-api', settle: 2000,
    actions: [{ clickText: ['创建 API Key'] }] },
  // 免登录：语言切换 dropdown + 创建账户（mode 切换到注册表单；postSettle 覆盖切换过渡）
  { id: 'px-login-lang', path: '/login', auth: false, settle: 3200, freezeCarousel: true, syncAnimPhase: true,
    actions: [{ clickText: ['简体中文'], clickCss: ['.header-link'] }] },
  { id: 'px-register-lang', path: '/register', auth: false, settle: 3200, freezeCarousel: true, syncAnimPhase: true,
    actions: [{ clickText: ['简体中文'], clickCss: ['.header-link'] }] },
  { id: 'px-login-register-confirm', path: '/login', auth: false, settle: 3200, freezeCarousel: true, syncAnimPhase: true, postSettle: 3200,
    actions: [{ clickText: ['创建账户'], clickCss: ['.register-cta__button'] }] },
  { id: 'px-register-register-confirm', path: '/register', auth: false, settle: 3200, freezeCarousel: true, syncAnimPhase: true, postSettle: 3200,
    actions: [{ clickText: ['创建账户'], clickCss: ['.register-cta__button'] }] },
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

// PATH 上的 python3 可能缺 numpy/PIL（homebrew 升级会换环境）；探测一次，
// 选第一个能跑 pixdiff 依赖的解释器。
let _pythonBin;
function pythonBin() {
  if (_pythonBin) return _pythonBin;
  const candidates = ['python3', '/usr/bin/python3', '/opt/homebrew/Caskroom/miniconda/base/bin/python3'];
  const probe = join(ROOT, 'scripts/parity/pixdiff.py');
  for (const bin of candidates) {
    try {
      execFileSync(bin, ['-c', 'import numpy, PIL'], { stdio: 'ignore' });
      _pythonBin = bin;
      return bin;
    } catch { /* try next */ }
  }
  _pythonBin = 'python3';
  return _pythonBin;
}

// 在单端页面按候选解析可点击目标：优先 aria-label/title 精确包含，
// 再 CSS 选择器（供两端纯图标按钮等无障碍名缺失的入口使用），再可见文本精确、
// 再子串。每个候选枚举全部匹配（而非仅第一个）：React 页面常有多达十几个
// 同名隐藏文本节点（tooltip/弹层模板），首个往往不可见。命中即点击；
// 全部未命中返回 false（记 warning，截图当前态——差异本身会体现在像素 diff 里）。
async function clickFirst(page, action) {
  const candidates = [];
  if (action.clickAria) candidates.push(...action.clickAria.map((t) => ({ by: 'aria', t })));
  if (action.clickCss) candidates.push(...action.clickCss.map((s) => ({ by: 'css', s })));
  if (action.clickText) candidates.push(...action.clickText.map((t) => ({ by: 'text', t })));
  for (const c of candidates) {
    try {
      let locs;
      if (c.by === 'aria') {
        locs = await page.locator(`[aria-label*="${c.t}"], [title*="${c.t}"]`).all();
      } else if (c.by === 'css') {
        locs = await page.locator(c.s).all();
      } else {
        locs = [
          ...await page.getByText(c.t, { exact: true }).all(),
          ...await page.getByText(c.t, { exact: false }).all(),
        ];
      }
      // pickLast（panel-matrix px 项）：消息工具栏类触发器双端 DOM 数量不同（Vue
      // 只给最后一条回答渲染 toolbar，React 每条 assistant 消息都有）——首个可见
      // 匹配在 React 端可能是滚出视口上缘的历史消息按钮，click 自动滚动会错位
      // 双端取景。取最后一个可见匹配＝最新消息的工具栏，双端同一逻辑按钮。
      if (action.pickLast) {
        for (let i = locs.length - 1; i >= 0; i--) {
          if (await locs[i].isVisible().catch(() => false)) { await locs[i].click(); return true; }
        }
      } else {
        for (const loc of locs) {
          if (await loc.isVisible().catch(() => false)) { await loc.click(); return true; }
        }
      }
    } catch { /* next candidate */ }
  }
  return false;
}

function pixdiff(vuePng, reactPng, diffPng) {
  const out = execFileSync(pythonBin(), [
    join(ROOT, 'scripts/parity/pixdiff.py'), vuePng, reactPng, diffPng,
  ], { encoding: 'utf8' });
  return JSON.parse(out);
}

// 稳态门：双端各自截图前统一执行——网络空闲 → 字体就绪 → 差异化 settle → 冻结动画定格。
// 目的：消除 Vue 端路由切换瞬态白屏 / 字体闪烁 / 过渡动画中间态被截入的伪差
// （历史坑：瞬态白屏伪造恶化）。与 login/register 的 freezeCarousel 叠加生效
// （后者冻结轮播相位，本门只冻结过渡动画，不改变已渲染稳态）。
// 顺序注意：freeze 必须在 settle 之后、紧贴截图前——若放在 settle 前，点击/路由
// 触发的入场过渡会被冻在开头（实测：新建知识库对话框半透明幽灵态 46% 伪差）。
// noFreeze：getAnimations().pause() 会把 spinner/进度条等循环动画冻在中间态，
// 若某页确证因此抬差，在 ALL_PAGES 该项加 noFreeze:true 跳过动画冻结。
// syncPhase（页标志 syncAnimPhase:true）：pause 只保证动画停在"某个"相位——
// 暂停时刻取决于双端各自的导航时序，是任意的。装饰循环动画（login/register
// .animated-bg 的 nodePulse/lineFlow）因此 run 间波动（275↔1387px）。确定性
// 定格：把每条无限循环 CSS 动画 seek 到 currentTime = effect.delay（= 该动画
// keyframe 0% 相位）。注意不能统一 currentTime=0——delay>0 的动画在 0 时刻
// 处于 fill:none 延迟期渲染基线态，而 delay=0 的落在 keyframe 0%，两端 delay
// 不一致时会系统性分叉（实证：React auth 页 [animation-delay:${...}] 模板
// 插值类 Tailwind 未生成规则，全部 delay=0；Vue 端 0~3s 错峰 → currentTime=0
// 双端 44k px 分叉）。seek 到各自 delay 后双端统一落在 keyframe 0%（nodePulse
// opacity .65/scale 1、lineFlow dashoffset 0），与端侧 delay 是否生效无关。
//   - 仅 iterations===Infinity 的 CSSAnimation 参与 seek：一次性入场/切换动画
//     必须保持 pause 末态（fill:forwards 已稳定），seek 回 0 会回退 UI 状态。
//   - CSSTransition 一律不 seek（归零会把已完成过渡回退到过渡前状态，破坏
//     freezeCarousel 已定格的轮播态）。
//   - SVG SMIL：根 svg.setCurrentTime(0)（无 SMIL 元素时为无害 no-op）。
//   - rAF/canvas：本仓库无此类装饰动画（headless 探针实证 login 双端 24 个
//     全为 CSSAnimation），如未来出现需页面侧提供确定性时间源，扫描器无法注入。
async function waitForSteady(page, settleMs, noFreeze, syncPhase) {
  await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {});
  await page.evaluate(() => document.fonts.ready).catch(() => {});
  await page.waitForTimeout(settleMs);
  if (!noFreeze) {
    await page.evaluate((syncPhase) => {
      document.getAnimations().forEach(a => {
        a.pause?.();
        if (!syncPhase) return;
        if (typeof CSSAnimation !== 'undefined' && a instanceof CSSAnimation) {
          const t = a.effect?.getTiming?.();
          // 无限循环装饰动画：seek 到 keyframe 0% 相位（delay 起点，负 delay 钳 0）
          if (t && t.iterations === Infinity) {
            try { a.currentTime = Math.max(0, t.delay || 0); } catch { /* 不可 seek 则保持 pause 态 */ }
          }
        }
      });
      if (syncPhase) {
        for (const svg of document.querySelectorAll('svg')) {
          if (typeof svg.setCurrentTime === 'function') { try { svg.setCurrentTime(0); } catch { /* next */ } }
        }
      }
    }, !!syncPhase).catch(() => {});
  }
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
    // T12c：settings-system 的「服务运行时长」行 = live(Date.now()-started_at)，
    // 两端顺序截图（相隔数秒）必差秒数文本。login/register freezeCarousel 同款
    // 确定性处理：全 run 冻结一次时钟字面量，经 addInitScript 注入（URL guard
    // 只在 section=system 生效，其余页面不受影响）；两端同一冻结值 → 运行时长
    // 文本完全一致。
    const scanClock = Date.now();
    for (const authedPage of [vuePage, reactPage]) {
      // 精确匹配 section 参数（'system' 而非 'system-global' 等前缀子串，
      // 避免误冻结其他系统管理分区的轮询/计时逻辑）。
      await authedPage.addInitScript(`if (new URLSearchParams(location.search).get('section') === 'system') { const frozen = ${scanClock}; Date.now = () => frozen; }`);
      // T12c：settings-runtime-queues 的「更新于 HH:mm:ss」由 5s 轮询响应的
      // server timestamp 驱动，双端截图相位不同必差秒数文本。冻结
      // toLocaleTimeString 为同一字面量（渲染值确定性；系统时钟本身不受影响）。
      await authedPage.addInitScript(`if (new URLSearchParams(location.search).get('section') === 'runtime-queues') { const frozen = new Date(${scanClock}).toLocaleTimeString('zh-CN', { hour12: false }); Date.prototype.toLocaleTimeString = function () { return frozen; }; }`);
    }
    // 免登录页（login/register 等）：全新 context，不注入任何会话
    const anonVue = await (await browser.newContext()).newPage();
    const anonReact = await (await browser.newContext()).newPage();
    const anonPages = { vue: anonVue, react: anonReact };

    for (const p of PAGES) {
      const fixturePath = fixtures[p.id];
      const path = p.kind ? (fixturePath ? fixturePath + (p.suffix || '') : undefined) : p.path;
      const entry = { id: p.id, path: path || p.path, status: 'ok', diff_pct: null };
      if (!path) { entry.status = 'skipped-no-fixture'; results.push(entry); console.log(`[skip] ${p.id}（fixture 未找到）`); continue; }
      try {
        const shots = [];
        const warnings = [];
        if (p.freezeCarousel) {
          // login/register 展示轮播由 JS 定时器切换，两端相位独立会落入不同
          // slide 造成整块假差异。轮询双端左半区特征文本，直到一致（≤15s）。
          const readSlides = async (pg) => pg.evaluate(() => {
            const out = [];
            for (const e of document.querySelectorAll('body *')) {
              if (e.children.length > 0) continue;
              const r = e.getBoundingClientRect();
              if (r.x > 700 || r.x < 300 || r.y < 150 || r.y > 650 || r.width === 0) continue;
              const t = (e.textContent || '').trim().replace(/\s+/g, ' ');
              if (t) out.push(t);
            }
            return out.sort().join('|');
          }).catch(() => '');
          const deadline = Date.now() + 15000;
          let a = '', b2 = '';
          while (Date.now() < deadline) {
            a = await readSlides(vuePage);
            b2 = await readSlides(reactPage);
            if (a && a === b2) break;
            await new Promise((r) => setTimeout(r, 700));
          }
        }
        for (const [tag, page, base] of [['vue', vuePage, VUE], ['react', reactPage, REACT]]) {
          const active = p.auth === false ? anonPages[tag] : page;
          await active.goto(base + path, { waitUntil: 'domcontentloaded', timeout: 20000 });
          // dev server 首次编译/HMR full-reload 会打断首帧；稳态门（网络空闲 +
          // 字体就绪 + 动画冻结）后走每页差异化 settle，消除瞬态白屏/字体/过渡
          // 动画伪差（R5xx chat 页间歇 93% 假阳性的根因是网络瞬态）。
          await waitForSteady(active, p.settle ?? 2400, p.noFreeze, p.syncAnimPhase);
          // T12c：集成页展示的 API base URL 取 window.location.origin（Vue
          // :5174 / React :5175 各自渲染），双端文本必差。截图前把双端
          // localhost:端口 统一替换为同一字面量（chrome connect 输入框值、
          // claw env 示例、cli/api tab 的 base 展示）。输入框走原型级 value
          // setter 拦截——React remount/受控回写会重置直接赋值。确定性归一
          // 同款于 system 时钟冻结。仅 integration-* 分区执行。
          if (p.id.startsWith('settings-integration-')) {
            await active.evaluate(() => {
              const fix = (str) => str.replace(/localhost:\d{2,5}/g, 'localhost:port');
              const walk = (node) => {
                for (const child of node.childNodes) {
                  if (child.nodeType === 3 && child.textContent && child.textContent.includes('localhost:')) {
                    child.textContent = fix(child.textContent);
                  } else if (child.nodeType === 1) {
                    walk(child);
                  }
                }
              };
              walk(document.body);
              for (const proto of [HTMLInputElement.prototype, HTMLTextAreaElement.prototype]) {
                const desc = Object.getOwnPropertyDescriptor(proto, 'value');
                if (!desc || !desc.set) continue;
                Object.defineProperty(proto, 'value', {
                  ...desc,
                  set(v) { desc.set.call(this, typeof v === 'string' && v.includes('localhost:') ? fix(v) : v); },
                });
              }
              for (const input of document.querySelectorAll('input, textarea')) {
                if (typeof input.value === 'string' && input.value.includes('localhost:')) input.value = fix(input.value);
              }
            }).catch(() => {});
          }
          // T12c：settings-system 的「UI 版本」行 commit 后缀 = 各自 dev server
          // 启动时 vite define 烘进的 git HEAD 短哈希（frontend/ 树与 tdm-int
          // worktree 树不同 HEAD，dev server 长驻进程、无法按轮注入），双端渲染
          // `0.8.0 (176704368)` / `0.8.0 (754bfb5d0)` 必差。确定性归一（同
          // system 时钟冻结 / integration localhost 归一先例）：截图前把匹配
          // `v\d+\.\d+\.\d+(-\w+)?\s*\(?[0-9a-f]{7,9}\)?` 的文本统一替换为固定
          // 字面量 v0.0.0 (parity)。作用域仅 section=system（URLSearchParams
          // 精确匹配，不误伤 system-global / runtime-queues 等分区）。注意两端
          // 版本与 commit 均为独立文本节点（"0.8.0 " 文本节点 + span.commit-info
          // "(hash)"），合并正则在单节点内匹配不到，需按节点分别归一：
          // commit-info span 整体替换为 "(parity)"（顺带抹平 React JSX 前导
          // 空格差），其前邻版本文本节点归一为 v0.0.0。MutationObserver 兜底
          // 轮询重渲染回写原始哈希（归一幂等，重入无害）。
          if (p.id === 'settings-system') {
            await active.evaluate(() => {
              if (new URLSearchParams(location.search).get('section') !== 'system') return;
              // test 与 replace 分用字面量：/g 正则的 test 有 lastIndex 状态，
              // 循环里会跨节点泄漏命中位置（replace 则始终从头扫描）。
              const COMBINED_T = /v?\d+\.\d+\.\d+(-\w+)?\s*\(?[0-9a-f]{7,9}\)?/;
              const COMBINED = /v?\d+\.\d+\.\d+(-\w+)?\s*\(?[0-9a-f]{7,9}\)?/g;
              const VERSION_T = /v?\d+\.\d+\.\d+(-\w+)?/;
              const VERSION = /v?\d+\.\d+\.\d+(-\w+)?/g;
              // 括号锚定：避免误伤纯数字行（如数据库迁移版本号）；"(unknown)" 不匹配。
              const COMMIT = /\([0-9a-f]{7,9}\)/;
              const normalize = () => {
                // 1) 合并形态：单节点内完整的 "v1.2.3 (a1b2c3d)"。
                const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
                for (let n = walker.nextNode(); n; n = walker.nextNode()) {
                  if (COMBINED_T.test(n.textContent)) {
                    n.textContent = n.textContent.replace(COMBINED, 'v0.0.0 (parity)');
                  }
                }
                // 2) 分裂节点形态（当前双端实况）：span.commit-info + 前邻版本文本节点。
                for (const ci of document.querySelectorAll('.commit-info')) {
                  if (COMMIT.test(ci.textContent || '')) ci.textContent = '(parity)';
                  let prev = ci.previousSibling;
                  while (prev && !(prev.nodeType === 3 && prev.textContent.trim())) prev = prev.previousSibling;
                  if (prev && VERSION_T.test(prev.textContent)) {
                    prev.textContent = prev.textContent.replace(VERSION, 'v0.0.0');
                  }
                }
              };
              normalize();
              new MutationObserver(normalize).observe(document.body, { childList: true, subtree: true, characterData: true });
            }).catch(() => {});
          }
          if (p.actions) {
            for (const action of p.actions) {
              let ok = false;
              // hover 门控触发器（panel-matrix 标注）：先真实 hover 揭示/触发（CSS
              // :hover 门控的会话行"更多"、t-tooltip 类附件提示），再走点击兜底链。
              // hover 与 click 一样按候选链取首个可见命中；命中后停留 hoverWait 让
              // 揭示过渡/tooltip 展示走完——此间不冻结动画（freeze 会把揭示过渡冻
              // 在开头），定格交给点击后的稳态门。
              if (action.hoverCss) {
                for (const s of action.hoverCss) {
                  try {
                    for (const loc of await active.locator(s).all()) {
                      if (await loc.isVisible().catch(() => false)) { await loc.hover(); ok = true; break; }
                    }
                  } catch { /* next candidate */ }
                  if (ok) break;
                }
                await active.waitForTimeout(action.hoverWait ?? 700);
              }
              if (action.clickAria || action.clickCss || action.clickText) {
                ok = await clickFirst(active, action);
              }
              if (!ok) warnings.push(tag + ' 未命中 ' + JSON.stringify(action));
              // 点击可能触发新的过渡动画/数据请求，截图前再过一遍稳态门定格。
              // postSettle：面板项可覆盖（如 login 创建账户点击后同路由切注册表单，
              // 需要更长过渡窗口）。
              await waitForSteady(active, p.postSettle ?? 1100, p.noFreeze, p.syncAnimPhase);
            }
          }
          const file = join(outDir, `${p.id}-${tag}.png`);
          await active.screenshot({ path: file });
          shots.push(file);
        }
        const d = pixdiff(shots[0], shots[1], join(outDir, `${p.id}-diff.png`));
        entry.diff_pct = d.diff_pct;
        entry.hot_cells = d.hot_cells?.slice(0, 6) || [];
        entry.urls = { vue: await vuePage.url(), react: await reactPage.url() };
        if (warnings.length) entry.warnings = warnings;
        console.log(`[done] ${p.id}: diff ${d.diff_pct}%${warnings.length ? ' (warn:' + warnings.length + ')' : ''}`);
      } catch (e) {
        entry.status = 'error'; entry.error = e.message.split(String.fromCharCode(10))[0];
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
