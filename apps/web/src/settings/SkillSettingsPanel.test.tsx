import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

// ---- jsdom 环境（必须在动态 import 面板之前就位：SkillSettingsPanel 现引
// tdesign-react，其 listener/popup 模块在加载期探测 document，之后渲染的
// TTooltip/TPopup 依赖 MutationObserver/requestAnimationFrame/getComputedStyle
// 全局垫片——同 SandboxSettingsPanel.test 的顺序约定）。 ----
const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  ShadowRoot: dom.window.ShadowRoot,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  FocusEvent: dom.window.FocusEvent,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
if (typeof (globalThis as { crypto?: unknown }).crypto === 'undefined') {
  (globalThis as { crypto?: unknown }).crypto = dom.window.crypto;
}
// tdesign popup 定时器/观测器走全局符号（node 无实现，测试垫片）。
(globalThis as unknown as { requestAnimationFrame?: unknown }).requestAnimationFrame
  = (globalThis as unknown as { requestAnimationFrame?: unknown }).requestAnimationFrame
  ?? ((cb: (t: number) => void) => setTimeout(() => cb(Date.now()), 16));
(globalThis as unknown as { cancelAnimationFrame?: unknown }).cancelAnimationFrame
  = (globalThis as unknown as { cancelAnimationFrame?: unknown }).cancelAnimationFrame
  ?? ((id: ReturnType<typeof setTimeout>) => clearTimeout(id));

const { SkillSettingsPanel } = await import('./SkillSettingsPanel.tsx');
const {
  MAX_ENV_VALUE_BYTES, adminSkillEnvClearPayload, buildSkillFileTree, canClearAdminSkillEnv, canDeleteCatalog,
  classifySkillRegisterError, clearSubmittedSkillEnvDrafts, collectSkillDirPaths, editedSkillEnvPayload,
  flattenSkillFileRows, installEntryTone, installErrorLines, installsView, isNamedSandboxBackend, isZipBundle,
  isValidEnvValueLength, maxSkillBundleMB, sandboxPickRows, sandboxTargetLine, splitMarkdownFrontmatter,
} = await import('../configuration/management.ts');
const { formatMessage } = await import('@weknora/i18n');
import type { SandboxConfigRecord, SkillCatalog, SkillCatalogInstallation } from '@weknora/api-client';

const client = {} as never;

function sandboxConfig(id: string, name: string, sandboxType = 'docker'): SandboxConfigRecord {
  return { id, name, sandbox_type: sandboxType, config: {}, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' };
}

function installation(sandboxConfigId: string, status: SkillCatalogInstallation['status'], enabled = true, extra: Partial<SkillCatalogInstallation> = {}): SkillCatalogInstallation {
  return { skillId: `skill-${sandboxConfigId}`, sandboxConfigId, status, enabled, ...extra };
}

function catalogItem(id: string, name: string, installations: SkillCatalogInstallation[], extra: Partial<SkillCatalog> = {}): SkillCatalog {
  return { id, name, installations, ...extra };
}

function renderPanel(role: 'viewer' | 'admin' | 'owner' | 'system-admin', props: { initialSkills?: never[]; initialCatalog?: SkillCatalog[]; initialSandboxConfigs?: SandboxConfigRecord[] } = {}) {
  return renderToStaticMarkup(React.createElement(SkillSettingsPanel, { client, role, ...props }));
}

// --- Viewer parity -----------------------------------------------------------------

test('skill settings keeps viewer state read-only', () => {
  const html = renderPanel('viewer', { initialSkills: [{ id: 's1', name: 'PDF', description: 'Read PDFs' }] as never });
  assert.match(html, new RegExp(formatMessage('zh-CN', 'settings.skills.description')));
  assert.match(html, /PDF/);
  assert.doesNotMatch(html, /<button/);
});

test('system admins receive the Vue admin skill catalog controls', () => {
  const html = renderPanel('system-admin', { initialCatalog: [], initialSandboxConfigs: [] });
  assert.match(html, /添加技能/);
  assert.doesNotMatch(html, /<h3/);
});

// --- Loading / empty states (SkillSettings.vue:14-33) --------------------------------

test('admin sees the shared loading copy until the catalog resolves', () => {
  const html = renderPanel('admin');
  assert.match(html, /加载中/);
  assert.doesNotMatch(html, /添加技能/);
});

test('empty catalog renders the Vue empty state with the sandbox hint and shortcut', () => {
  const html = renderPanel('admin', { initialCatalog: [], initialSandboxConfigs: [] });
  assert.match(html, /还没有技能。添加后可以装到一份或多份沙箱。/);
  assert.match(html, /当前没有沙箱。带脚本的技能登记后无法运行，可先去配置沙箱。/);
  assert.match(html, /添加技能/);
  assert.match(html, /去配置沙箱/);
});

test('empty catalog renders the TDesign empty anatomy: illustration plus the 暂无数据 title line', () => {
  const html = renderPanel('admin', { initialCatalog: [], initialSandboxConfigs: [] });
  // t-empty default (type="empty"): 48px EmptySvg illustration above the
  // zh-CN locale title "暂无数据" (SkillSettings.vue:20 + TDesign locale).
  assert.match(html, /svg[^>]*viewBox="0 0 48 48"/, 'renders the TDesign "no result" illustration');
  assert.match(html, /暂无数据/, 'renders the t-empty default title line');
});

test('catalog section header renders the Vue title with the help-circle tooltip trigger', () => {
  const html = renderPanel('admin', { initialCatalog: [catalogItem('cat-1', 'PDF', [])], initialSandboxConfigs: [sandboxConfig('cfg-1', 'Docker dev')] });
  assert.match(html, /技能管理/);
  const helpLabel = formatMessage('zh-CN', 'settings.skills.helpTooltip');
  assert.match(html, new RegExp(`aria-label="${helpLabel}"`), 'help icon carries the tooltip copy as its accessible name');
  // Vue .section-header__help 承载 cursor:help（SkillSettings.vue:1250-1253，
  // 平移 CSS 在 settings.td.css skills §）；静态标记断言落在类名上。
  assert.match(html, /section-header__help/, 'help icon uses the Vue .section-header__help affordance class');
  assert.match(html, /t-icon-help-circle/, 'trigger is the TDesign help-circle glyph');
  assert.doesNotMatch(html, new RegExp(`>${helpLabel}</`), 'tooltip content stays closed (portal) in static markup');
});

test('empty catalog with sandbox configs drops the sandbox shortcut (SkillSettings.vue:21-31)', () => {
  const html = renderPanel('admin', { initialCatalog: [], initialSandboxConfigs: [sandboxConfig('cfg-1', 'Docker dev')] });
  assert.match(html, /还没有技能/);
  assert.doesNotMatch(html, /去配置沙箱/);
});

// --- Catalog cards (SkillSettings.vue:35-129) -----------------------------------------

test('catalog cards render name, version, compacted description, files and delete actions', () => {
  const html = renderPanel('admin', {
    initialCatalog: [catalogItem('cat-1', 'PDF', [], { version: '1.2.0', description: 'Read   and\n  write\u00a0PDFs' })],
    initialSandboxConfigs: [sandboxConfig('cfg-1', 'Docker dev')],
  });
  assert.match(html, /PDF/);
  assert.match(html, /1.2.0/);
  assert.match(html, /Read and write PDFs/);
  assert.match(html, /查看文件/);
  assert.match(html, /从目录删除/);
  assert.match(html, /skill-card--add/);
});

test('a live install hides the delete action and summarizes the single sandbox (SkillSettings.vue:56-60, 710-716)', () => {
  const html = renderPanel('admin', {
    initialCatalog: [catalogItem('cat-1', 'PDF', [installation('cfg-1', 'ready', true, { sandboxConfigName: 'Docker dev' })])],
    initialSandboxConfigs: [sandboxConfig('cfg-1', 'Docker dev'), sandboxConfig('cfg-2', 'Remote')],
  });
  assert.doesNotMatch(html, /从目录删除/);
  assert.match(html, /已安装到 Docker dev/);
  assert.match(html, /skill-card--installed/);
});

test('multiple installs collapse into the count summary (SkillSettings.vue:715)', () => {
  const html = renderPanel('admin', {
    initialCatalog: [catalogItem('cat-1', 'PDF', [
      installation('cfg-1', 'ready', true, { sandboxConfigName: 'Docker dev' }),
      installation('cfg-2', 'ready', false, { sandboxConfigName: 'Remote' }),
    ])],
    initialSandboxConfigs: [sandboxConfig('cfg-1', 'Docker dev'), sandboxConfig('cfg-2', 'Remote')],
  });
  assert.match(html, /已安装到 2 个沙箱/);
});

test('an idle card without sandboxes shows the no-installs label (SkillSettings.vue:67-69)', () => {
  const html = renderPanel('admin', { initialCatalog: [catalogItem('cat-1', 'PDF', [])], initialSandboxConfigs: [] });
  assert.match(html, /尚未装到任何沙箱/);
});

test('an idle card with a single remaining sandbox offers install (SkillSettings.vue:70-78)', () => {
  const html = renderPanel('admin', {
    initialCatalog: [catalogItem('cat-1', 'PDF', [])],
    initialSandboxConfigs: [sandboxConfig('cfg-1', 'Docker dev')],
  });
  assert.match(html, /安装到沙箱/);
  assert.match(html, /skill-card--idle/);
});

test('busy installs render the live dot and keep the installed card tone (SkillSettings.vue:75)', () => {
  const html = renderPanel('admin', {
    initialCatalog: [catalogItem('cat-1', 'PDF', [installation('cfg-1', 'installing', false, { sandboxConfigName: 'Docker dev' })])],
    initialSandboxConfigs: [sandboxConfig('cfg-1', 'Docker dev')],
  });
  assert.match(html, /skill-card__entry-dot/);
  assert.match(html, /skill-card__entry--busy/);
});

test('a ready install with a stale bundle hash is flagged outdated (SkillSettings.vue:644-650, 669-674)', () => {
  const html = renderPanel('admin', {
    initialCatalog: [catalogItem('cat-1', 'PDF', [installation('cfg-1', 'ready', true, { sandboxConfigName: 'Docker dev', bundleSha256: 'old' })], { bundleSha256: 'new' })],
    initialSandboxConfigs: [sandboxConfig('cfg-1', 'Docker dev')],
  });
  assert.match(html, /skill-card__entry--stale/);
});

// --- Shared localization across the migrated keys --------------------------------------

test('skill settings consumes shared five-locale settings keys (settingsT pattern)', () => {
  assert.equal(formatMessage('zh-CN', 'settings.skills.installedOnName', { name: 'Docker' }), '已安装到 Docker');
  assert.equal(formatMessage('en-US', 'settings.skills.installedOnName', { name: 'Docker' }), 'Installed on Docker');
  assert.equal(formatMessage('ja-JP', 'settings.skills.addSkill'), 'スキルを追加');
  assert.equal(formatMessage('ko-KR', 'settings.skills.addSkill'), '스킬 추가');
  assert.equal(formatMessage('ru-RU', 'settings.skills.addSkill'), 'Добавить навык');
});

// --- Helper parity: installs view (SkillSettings.vue:488-708) ---------------------------

test('live installs exclude removed rows and gate catalog deletion (SkillSettings.vue:488-493)', () => {
  const removedOnly = catalogItem('cat-1', 'PDF', [installation('cfg-1', 'removed', false)]);
  const live = catalogItem('cat-2', 'PDF', [installation('cfg-1', 'ready', true)]);
  assert.equal(canDeleteCatalog(removedOnly), true);
  assert.equal(canDeleteCatalog(live), false);
});

test('installs view sorts by failure, busy, stale, off, then name (SkillSettings.vue:680-708)', () => {
  const item = catalogItem('cat-1', 'PDF', [
    installation('cfg-ready', 'ready', true, { sandboxConfigName: 'Zeta', bundleSha256: 'same' }),
    installation('cfg-off', 'ready', false, { sandboxConfigName: 'Alpha' }),
    installation('cfg-failed', 'failed', false, { sandboxConfigName: 'Mid' }),
    installation('cfg-busy', 'installing', false, { sandboxConfigName: 'Beta' }),
    installation('cfg-stale', 'ready', true, { sandboxConfigName: 'Gamma', bundleSha256: 'old' }),
  ], { bundleSha256: 'same' });
  const view = installsView(item, []);
  assert.deepEqual(view.installs.map((row) => row.sandboxConfigId), ['cfg-failed', 'cfg-busy', 'cfg-stale', 'cfg-off', 'cfg-ready']);
  assert.equal(view.needsPanel, true);
  assert.equal(installEntryTone(item, view.installs[2]!), 'stale');
  assert.equal(installEntryTone(item, view.installs[3]!), 'off');
});

test('sandbox pick rows keep busy and ready rows selectable-free (SkillSettings.vue:547-577)', () => {
  const item = catalogItem('cat-1', 'PDF', [
    installation('cfg-ready', 'ready', true),
    installation('cfg-busy', 'installing', false),
    installation('cfg-failed', 'failed', false),
  ]);
  const configs = [sandboxConfig('cfg-ready', 'Ready'), sandboxConfig('cfg-busy', 'Busy'), sandboxConfig('cfg-failed', 'Failed'), sandboxConfig('cfg-free', 'Free')];
  const remaining = sandboxPickRows(item, configs, 'remaining', []);
  assert.deepEqual(remaining.map((row) => row.config.id), ['cfg-busy', 'cfg-failed', 'cfg-free']);
  assert.deepEqual(remaining.map((row) => row.selectable), [false, true, true]);
  const session = sandboxPickRows(item, configs, 'remaining', ['cfg-ready']);
  assert.deepEqual(session.map((row) => row.config.id).includes('cfg-ready'), true);
  const all = sandboxPickRows(item, configs, 'all', []);
  assert.equal(all.length, 4);
});

// --- Helper parity: register validation (SkillSettings.vue:941-979, utils/index.ts:23-42) --

test('register failures map onto the Vue copy buckets with their limits', () => {
  assert.deepEqual(classifySkillRegisterError('bundle cannot exceed 256 MB'), { kind: 'bundleTooLarge' });
  assert.deepEqual(classifySkillRegisterError('skill directory holds more than 2000 files'), { kind: 'bundleTooManyFiles', count: '2000' });
  assert.deepEqual(classifySkillRegisterError('archive has more than 4096 zip entries'), { kind: 'bundleTooManyZipEntries', count: '4096' });
  assert.deepEqual(classifySkillRegisterError('archive holds more than 500 files'), { kind: 'bundleTooManyFiles', count: '500' });
  assert.equal(classifySkillRegisterError('network hiccup'), null);
});

test('bundle size ceiling mirrors the Vue runtime clamp', () => {
  assert.equal(maxSkillBundleMB(undefined), 256);
  assert.equal(maxSkillBundleMB({}), 256);
  assert.equal(maxSkillBundleMB({ MAX_SKILL_BUNDLE_SIZE_MB: 6000 }), 512);
  assert.equal(maxSkillBundleMB({ MAX_SKILL_BUNDLE_SIZE_MB: 10 }), 50);
  assert.equal(maxSkillBundleMB({ MAX_SKILL_BUNDLE_SIZE_MB: 10, MAX_FILE_SIZE_MB: 80 }), 80);
});

test('zip bundles are recognized by extension or mime type (SkillSettings.vue:941)', () => {
  assert.equal(isZipBundle({ name: 'skill.ZIP' }), true);
  assert.equal(isZipBundle({ name: 'skill', type: 'application/zip' }), true);
  assert.equal(isZipBundle({ name: 'skill.tar.gz' }), false);
});

// --- Helper parity: sandbox meta (SkillSettings.vue:509-531, api/system/index.ts:919) ------

test('sandbox target lines mirror the Vue backend metadata', () => {
  assert.equal(sandboxTargetLine({ ...sandboxConfig('c1', 'Local', 'docker'), config: { docker: { image: 'weknora/skills:2' } } }), 'weknora/skills:2');
  assert.equal(sandboxTargetLine({ ...sandboxConfig('c2', 'Remote', 'e2b'), config: { e2b: { api_url: 'https://api.e2b.dev/v1' } } }), 'api.e2b.dev');
  assert.equal(sandboxTargetLine({ ...sandboxConfig('c3', 'Bad', 'cube'), config: { cube: { api_url: 'not a url' } } }), 'not a url');
  assert.equal(sandboxTargetLine(sandboxConfig('c4', 'Empty', 'e2b')), '');
  assert.equal(isNamedSandboxBackend('local'), false);
  assert.equal(isNamedSandboxBackend('docker'), true);
});

// --- Helper parity: install transcripts and env vars (SandboxSkillsPanel.vue:1071, envVarState.ts) ---

test('multi-line verification failures stay one finding per line', () => {
  assert.deepEqual(installErrorLines('line one\n  line two  \n\n'), ['line one', 'line two']);
  assert.deepEqual(installErrorLines(undefined), []);
});

test('env drafts round-trip through declaration filtering and in-flight cleanup', () => {
  assert.deepEqual(editedSkillEnvPayload(['A', 'B'], { A: '1', C: '2' }), { A: '1' });
  assert.deepEqual(editedSkillEnvPayload(['A'], undefined), {});
  assert.deepEqual(adminSkillEnvClearPayload('A'), { A: '' });
  assert.equal(canClearAdminSkillEnv({ isSet: true }), true);
  assert.equal(canClearAdminSkillEnv({ isSet: false }), false);
  assert.deepEqual(clearSubmittedSkillEnvDrafts({ A: '1', B: 'new' }, { A: '1', B: 'old' }), { B: 'new' });
  assert.equal(isValidEnvValueLength('a'.repeat(MAX_ENV_VALUE_BYTES)), true);
  assert.equal(isValidEnvValueLength('a'.repeat(MAX_ENV_VALUE_BYTES + 1)), false);
  assert.equal(MAX_ENV_VALUE_BYTES, 8192);
});

// --- Helper parity: file browser tree (SkillFilesPanel.vue:332-398) ----------------------

test('the file tree pins SKILL.md first and directories before files', () => {
  const tree = buildSkillFileTree([
    { path: 'docs/setup.md', size: 12 },
    { path: 'SKILL.md', size: 4 },
    { path: 'assets/logo.png', size: 51 },
  ]);
  assert.deepEqual(tree.map((node) => node.path), ['SKILL.md', 'assets', 'docs']);
  assert.deepEqual(collectSkillDirPaths(tree), ['assets', 'docs']);
  const collapsed = flattenSkillFileRows(tree, new Set());
  assert.deepEqual(collapsed.map((row) => row.path), ['SKILL.md', 'assets', 'docs']);
  const expanded = flattenSkillFileRows(tree, new Set(['docs', 'assets']));
  assert.deepEqual(expanded.map((row) => [row.path, row.depth]), [['SKILL.md', 0], ['assets', 0], ['assets/logo.png', 1], ['docs', 0], ['docs/setup.md', 1]]);
});

// --- Helper parity: SKILL.md frontmatter (SkillFilesPanel.vue:427-471) -------------------

test('frontmatter parsing unquotes values and pretty-prints JSON', () => {
  const parsed = splitMarkdownFrontmatter('---\nname: "pdf skill"\nallowed-tools: [\"Read\", \"Write\"]\n# comment\nnot a pair\ncount: 3\n---\n\nbody text');
  assert.deepEqual(parsed.fields, [
    { key: 'name', value: 'pdf skill', code: false },
    { key: 'allowed-tools', value: JSON.stringify(['Read', 'Write'], null, 2), code: true },
    { key: 'count', value: '3', code: false },
  ]);
  assert.equal(parsed.body.trim(), 'body text');
  assert.deepEqual(splitMarkdownFrontmatter('# just markdown').fields, []);
});

// ---- jsdom interaction coverage: Vue SkillInstallTimeline + progress SSE parity
//（jsdom 全局与垫片已在文件头就位，先于面板动态 import。） ----
const { createRoot } = await import('react-dom/client');
const { act } = await import('react');

function settle(ms = 60) {
  return act(async () => { await new Promise((resolve) => setTimeout(resolve, ms)); });
}

function findButton(label: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find((button) => (button.textContent ?? '').includes(label)) as HTMLButtonElement | undefined;
}

type SkillStubOverrides = {
  installed?: Record<string, unknown>;
  progressFrames?: Array<{ percent: number; stage: string; done: boolean; status?: string; log?: string }>;
  transcriptFrames?: unknown[];
  messages?: unknown[];
  guidance?: { accepting: boolean; messages: Array<{ id: string; content: string; status: string }> };
  steerFail?: boolean;
};

function skillStubClient(overrides: SkillStubOverrides = {}) {
  const calls = { steer: [] as unknown[], reinstall: [] as unknown[], progressRuns: 0, transcriptRuns: 0 };
  const installed = overrides.installed ?? { id: 'sk-1', name: 'PDF', enabled: true, status: 'installing', installSessionId: 'sess-1', installMessageId: 'msg-1' };
  const client = {
    sandboxConfigurations: { list: async () => ({ items: [sandboxConfig('cfg-1', 'Docker dev')] }) },
    configuration: {
      agents: { get: async () => ({ id: 'builtin-skill-installer', name: 'Installer', description: '', avatar: '', config: {} }) },
      models: { list: async () => [] },
      skills: {
        catalog: {
          list: async () => [catalogItem('cat-1', 'PDF', [installation('cfg-1', 'installing', true, { skillId: 'sk-1', sandboxConfigName: 'Docker dev' })])],
          files: async () => [{ path: 'SKILL.md', size: 24 }],
          file: async () => ({ path: 'SKILL.md', size: 24, encoding: 'utf-8', content: '---\nname: pdf\n---\n\n# Hello\n\n**bold** body' }),
        },
        installed: {
          get: async () => installed,
          reinstall: async (...args: unknown[]) => { calls.reinstall.push(args); return { skillId: 'sk-1' }; },
        },
      },
    },
    sessions: { messages: async () => overrides.messages ?? [] },
    sandbox: {
      skills: {
        followInstallEvents: async (_configId: string, _skillId: string, onEvent: (frame: { event: unknown; terminal: boolean }) => void) => {
          calls.progressRuns += 1;
          for (const frame of overrides.progressFrames ?? []) {
            onEvent({ event: frame, terminal: frame.done === true });
          }
        },
        followTranscript: async (_configId: string, _skillId: string, onFrame: (frame: unknown) => void) => {
          calls.transcriptRuns += 1;
          for (const frame of overrides.transcriptFrames ?? [
            { response_type: 'install_prompt', content: 'Install pdf-skill' },
            { response_type: 'thinking', content: 'planning' },
            { response_type: 'answer', content: 'All set' },
            { response_type: 'complete' },
          ]) onFrame(frame);
          return true;
        },
        guidance: async () => overrides.guidance ?? { accepting: true, messages: [{ id: 'g1', content: 'use apt', status: 'pending' }] },
        steer: async (...args: unknown[]) => {
          calls.steer.push(args);
          if (overrides.steerFail) throw new Error('steer rejected');
          return { success: true };
        },
      },
    },
  } as never;
  return { client, calls };
}

async function openManageDrawer(client: unknown) {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(React.createElement(SkillSettingsPanel, {
      client: client as never,
      role: 'admin',
      initialCatalog: [catalogItem('cat-1', 'PDF', [installation('cfg-1', 'installing', true, { skillId: 'sk-1', sandboxConfigName: 'Docker dev' })])],
      initialSandboxConfigs: [sandboxConfig('cfg-1', 'Docker dev')],
    }));
  });
  return { container, root };
}

// Radix popper positioning observes element resize; jsdom has no implementation.
class ResizeObserverStub { observe() {} unobserve() {} disconnect() {} }
(globalThis as unknown as { ResizeObserver?: unknown }).ResizeObserver = (globalThis as unknown as { ResizeObserver?: unknown }).ResizeObserver ?? ResizeObserverStub;

test('the header help icon opens and closes the Vue tooltip on hover', async () => {
  const { client } = skillStubClient();
  const { container, root } = await openManageDrawer(client);
  try {
    const helpLabel = formatMessage('zh-CN', 'settings.skills.helpTooltip');
    const trigger = container.querySelector('.section-header__help') as SVGElement | null;
    assert.ok(trigger, 'help-circle icon sits next to the section title');
    assert.ok(trigger!.getAttribute('aria-label') === helpLabel, 'trigger carries the tooltip copy as its accessible name');
    assert.equal(document.body.textContent?.includes(helpLabel), false, 'tooltip stays closed until interaction (portal unmounted)');
    // t-tooltip trigger=hover（两端默认一致）：mouseenter 打开、mouseleave 关闭，
    // 弹层 portal 到 body（.skill-settings__help-tooltip）。
    await act(async () => trigger!.dispatchEvent(new dom.window.MouseEvent('mouseenter', { bubbles: false })));
    await settle(320);
    assert.equal(document.body.textContent?.includes(helpLabel), true, 'hovering the help icon opens the tooltip');
    assert.equal(Boolean(document.querySelector('.skill-settings__help-tooltip')), true, 'popup renders with the Vue overlay class');
    await act(async () => trigger!.dispatchEvent(new dom.window.MouseEvent('mouseleave', { bubbles: false })));
    await settle(520);
    assert.equal(document.body.textContent?.includes(helpLabel), false, 'tooltip closes again on mouseleave');
  } finally {
    await act(async () => root.unmount());
    document.body.replaceChildren();
  }
});

test('the manage drawer mounts the install timeline and live progress from the SSE sources', async () => {
  const { client, calls } = skillStubClient({
    progressFrames: [{ percent: 42, stage: 'building', done: false }],
  });
  const { root } = await openManageDrawer(client);
  try {
    await act(async () => { findButton('已安装到 Docker dev')?.click(); });
    await settle();
    const section = document.querySelector('.skill-manage__section--transcript');
    assert.ok(section, 'transcript section renders');
    assert.match(section?.textContent ?? '', /安装过程/);
    const progress = document.querySelector('.skill-manage__progress');
    assert.ok(progress, 'progress ring renders while installing');
    assert.match(progress?.textContent ?? '', /42%/);
    assert.equal(calls.progressRuns, 1, 'one install-events follow');
    const timeline = document.querySelector('.skill-timeline');
    assert.ok(timeline, 'timeline renders');
    assert.match(timeline?.querySelector('.skill-timeline__prompt')?.textContent ?? '', /Install pdf-skill/);
    assert.match(timeline?.textContent ?? '', /All set/);
    assert.match(timeline?.textContent ?? '', /use apt/);
    assert.match(timeline?.textContent ?? '', /待处理/);
    const composer = timeline?.querySelector('textarea');
    assert.ok(composer, 'guidance composer renders while live');
    // S6：disabled 的 tdesign Button 渲染 div.t-button（台账 #7），经类查询。
    const send = Array.from(document.querySelectorAll('.t-button')).find((button) => (button.textContent ?? '').includes('发送说明'));
    assert.ok(send, 'send guidance button renders');
    assert.equal(send.classList.contains('t-is-disabled'), true, 'send disabled until text is entered');
  } finally {
    await act(async () => root.unmount());
    document.body.replaceChildren();
  }
});

test('guidance text is steered into the live run with the Vue payload and appended as pending', async () => {
  const { client, calls } = skillStubClient({ progressFrames: [] });
  const { root } = await openManageDrawer(client);
  try {
    await act(async () => { findButton('已安装到 Docker dev')?.click(); });
    await settle();
    const textarea = document.querySelector('textarea') as HTMLTextAreaElement;
    assert.ok(textarea, 'composer textarea renders');
    const setter = Object.getOwnPropertyDescriptor(dom.window.HTMLTextAreaElement.prototype, 'value')?.set;
    await act(async () => {
      setter?.call(textarea, 'pin apt version');
      textarea.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    });
    const send = Array.from(document.querySelectorAll('.t-button')).find((button) => (button.textContent ?? '').includes('发送说明')) as HTMLButtonElement;
    await act(async () => { send.click(); });
    await settle();
    assert.equal(calls.steer.length, 1);
    const [configId, skillId, payload] = calls.steer[0] as [string, string, Record<string, string>];
    assert.equal(configId, 'cfg-1');
    assert.equal(skillId, 'sk-1');
    // The stub replaces the api-client method, so the camelCase input is what
    // the panel hands over; wire-level snake_case is asserted in skill-install.test.ts.
    assert.equal(payload.expectedMessageId, 'msg-1');
    assert.match(payload.steerId, /^[0-9a-f-]{36}$/);
    assert.equal(payload.content, 'pin apt version');
    const timeline = document.querySelector('.skill-timeline');
    assert.match(timeline?.textContent ?? '', /pin apt version/, 'appended pending guidance is rendered');
    assert.equal(textarea.value, '', 'composer clears after send');
  } finally {
    await act(async () => root.unmount());
    document.body.replaceChildren();
  }
});

test('a failed run replays the durable history and reinstalls with guidance on retry', async () => {
  const { client, calls } = skillStubClient({
    installed: { id: 'sk-1', name: 'PDF', enabled: true, status: 'failed', installSessionId: 'sess-1', installMessageId: 'msg-1' },
    messages: [
      { response_type: 'install_prompt', content: 'Install pdf-skill' },
      { response_type: 'answer', content: 'earlier run output' },
    ],
  });
  const { root } = await openManageDrawer(client);
  try {
    await act(async () => { findButton('已安装到 Docker dev')?.click(); });
    await settle();
    assert.equal(calls.transcriptRuns, 0, 'a finished run reads durable history first');
    const timeline = document.querySelector('.skill-timeline');
    assert.ok(timeline, 'timeline renders');
    assert.match(timeline?.querySelector('.skill-timeline__prompt')?.textContent ?? '', /Install pdf-skill/);
    assert.match(timeline?.textContent ?? '', /earlier run output/);
    const retryOf = () => Array.from(document.querySelectorAll('.t-button')).find((button) => (button.textContent ?? '').includes('携带说明重新安装'));
    assert.ok(retryOf(), 'retry composer button renders when the run can be retried');
    const textarea = document.querySelector('textarea') as HTMLTextAreaElement;
    const setter = Object.getOwnPropertyDescriptor(dom.window.HTMLTextAreaElement.prototype, 'value')?.set;
    await act(async () => {
      setter?.call(textarea, 'install libxml first');
      textarea.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    });
    // S6：disabled→enabled 会把 tdesign Button 的根标签从 div 换回 button
    // （台账 #7），输入后重新查询再点击。
    await act(async () => { retryOf()?.click(); });
    await settle();
    assert.deepEqual(calls.reinstall, [['cfg-1', 'sk-1', 'install libxml first']]);
  } finally {
    await act(async () => root.unmount());
    document.body.replaceChildren();
  }
});

test('the files browser renders SKILL.md as markdown with the frontmatter table', async () => {
  const { client } = skillStubClient({ progressFrames: [] });
  const { root } = await openManageDrawer(client);
  try {
    await act(async () => {
      const files = document.querySelector('button[aria-label="查看文件"]') as HTMLButtonElement | null;
      files?.click();
    });
    await settle();
    const markdown = document.querySelector('.skill-files-panel__markdown');
    assert.ok(markdown, 'markdown preview renders');
    assert.match(markdown?.innerHTML ?? '', /<h1[^>]*>Hello<\/h1>/);
    assert.match(markdown?.innerHTML ?? '', /<strong>bold<\/strong>/);
    const meta = document.querySelector('.skill-files-panel__meta');
    assert.ok(meta, 'frontmatter table renders');
    assert.match(meta?.textContent ?? '', /name/);
    assert.match(meta?.textContent ?? '', /pdf/);
  } finally {
    await act(async () => root.unmount());
    document.body.replaceChildren();
  }
});

// --- Zip upload progress (SkillSettings.vue:208 + SandboxSkillsPanel.vue:64-72) ------

const { formatMessage: fmt } = await import('@weknora/i18n');
const {
  SkillUploadProgress,
  registerSkillCatalogWithProgress,
  uploadPercentFromProgress,
} = await import('./SkillSettingsPanel.tsx');
const { uploadProgressListener } = await import('../platform/http.ts');

test('upload percent math mirrors the Vue XHR byte calculation', () => {
  // Vue api layer: Math.round((loaded * 100) / total) (frontend/src/api/system/index.ts:1117),
  // clamped like KnowledgeBaseList.vue clampProgress (1601).
  assert.equal(uploadPercentFromProgress({ loaded: 0, total: 1000 }), 0);
  assert.equal(uploadPercentFromProgress({ loaded: 475, total: 1000 }), 48);
  assert.equal(uploadPercentFromProgress({ loaded: 1500, total: 1000 }), 100);
  assert.equal(uploadPercentFromProgress({ loaded: 12, total: 0 }), 0);
});

test('the shared skillUploading key formats the percent text', () => {
  assert.equal(fmt('en-US', 'settings.sandbox.skillUploading', { percent: 42 }), 'Uploading 42%');
  assert.equal(fmt('zh-CN', 'settings.sandbox.skillUploading', { percent: 7 }).includes('7'), true);
});

test('SkillUploadProgress renders the percent text plus a progressbar', () => {
  const t = (key: string, values?: Record<string, string | number>) =>
    values ? 'Uploading ' + String(values.percent) + '%' : key;
  const html = renderToStaticMarkup(React.createElement(SkillUploadProgress, { percent: 42, t }));
  assert.match(html, /Uploading 42%/);
  assert.match(html, /role="status"/);
  assert.match(html, /role="progressbar"/);
  assert.match(html, /aria-valuenow="42"/);
  assert.match(html, /width:42%/);
});

test('skill catalog register streams upload percent through the transport channel', async () => {
  const realCreate = URL.createObjectURL;
  const realRevoke = URL.revokeObjectURL;
  const revoked: string[] = [];
  let counter = 0;
  (URL as unknown as { createObjectURL: (value: Blob) => string }).createObjectURL = () => 'blob:skill-' + (counter += 1);
  (URL as unknown as { revokeObjectURL: (uri: string) => void }).revokeObjectURL = (uri: string) => { revoked.push(uri); };
  try {
    const captured: Array<Record<string, unknown>> = [];
    const client = {
      configuration: {
        skills: {
          catalog: {
            register: async (input: { file: Record<string, unknown> }) => {
              captured.push(input.file);
              return { id: 'cat-9', name: 'PDF' };
            },
          },
        },
      },
    };
    const percents: number[] = [];
    const file = new File(['zip-bytes'], 'skill.zip', { type: 'application/zip' });
    const pending = registerSkillCatalogWithProgress(client as never, file, (percent) => percents.push(percent));
    // The helper registers the observer synchronously before its first await,
    // so the uri and the listener are observable without yielding.
    const uri = String(captured[0]!.uri);
    const observer = uploadProgressListener(uri);
    assert.ok(observer, 'the register call registers its progress observer');
    observer({ loaded: 250, total: 1000 });
    observer({ loaded: 1000, total: 1000 });
    const result = (await pending) as { id: string };
    assert.deepEqual(percents, [25, 100]);
    assert.deepEqual(captured[0], { uri, name: 'skill.zip', type: 'application/zip', size: 9 });
    assert.equal(result.id, 'cat-9');
    assert.equal(uploadProgressListener(uri), undefined, 'the observer is released after completion');
  } finally {
    URL.createObjectURL = realCreate;
    URL.revokeObjectURL = realRevoke;
  }
});
