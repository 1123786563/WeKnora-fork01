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

function renderPanel(role: 'viewer' | 'admin', props: { initialSkills?: never[]; initialCatalog?: SkillCatalog[]; initialSandboxConfigs?: SandboxConfigRecord[] } = {}) {
  return renderToStaticMarkup(React.createElement(SkillSettingsPanel, { client, role, ...props }));
}

// --- Viewer parity -----------------------------------------------------------------

test('skill settings keeps viewer state read-only', () => {
  const html = renderPanel('viewer', { initialSkills: [{ id: 's1', name: 'PDF', description: 'Read PDFs' }] as never });
  assert.match(html, /Installed skills are managed/);
  assert.match(html, /PDF/);
  assert.doesNotMatch(html, /<button/);
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
