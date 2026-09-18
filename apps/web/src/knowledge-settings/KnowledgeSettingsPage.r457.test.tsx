// R457 A1 round: the editor-section headings/descriptions (the eyebrow, the h3
// title and the muted sentence above every section) were the last hardcoded
// English on the knowledge-settings surface ("Basics", "Parser", "Storage",
// "Name, description and knowledge-base type", …). Vue's navGroups/sidebar
// copy is already i18n through knowledgeEditor.navGroups.* /
// knowledgeEditor.sidebar.* (verified against KnowledgeBaseEditorModal.vue
// navGroups L637-669), so this round adds fresh kbSettings.sections.* keys for
// all five locales, keeps the en-US strings byte-identical as fallbacks and
// resolves the rendered headings through the locale translator.
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
dom.window.localStorage.setItem('locale', 'zh-CN');
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLButtonElement: dom.window.HTMLButtonElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  PointerEvent: dom.window.PointerEvent,
  NodeFilter: dom.window.NodeFilter,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { KnowledgeSettingsPage, getKnowledgeSettingsSections } = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;

const baseKnowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-457',
  name: 'Product docs',
  type: 'document',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
};

// ---- unit: every section carries structured kbSettings.sections.* keys -------

test('every editor section carries kbSettings.sections label/description keys', () => {
  const documentSections = getKnowledgeSettingsSections(baseKnowledgeBase);
  const faqSections = getKnowledgeSettingsSections({ id: 'kb-457-faq', name: 'FAQ', type: 'faq' });
  for (const section of [...documentSections, ...faqSections]) {
    assert.match(section.labelKey, new RegExp(`^kbSettings\\.sections\\.${section.key}\\.label$`), `labelKey for ${section.key}`);
    assert.match(section.descriptionKey, new RegExp(`^kbSettings\\.sections\\.${section.key}\\.description$`), `descriptionKey for ${section.key}`);
    // en-US fallback strings stay in place (byte-identical pre-R457 copy).
    assert.ok(section.label.length > 0, `fallback label kept for ${section.key}`);
    assert.ok(section.description.length > 0, `fallback description kept for ${section.key}`);
  }
  assert.equal(documentSections.length + faqSections.length, 20, '13 document sections + 7 FAQ-visible sections');
});

// ---- render: zh-CN headings, no hardcoded English; en-US unchanged -----------

function clientFor(): WeKnoraClient {
  const request = async () => ({ success: true });
  return {
    request,
    configuration: { models: { list: async () => [] } },
    dataSources: {
      list: async () => [],
      types: async () => [],
    },
    knowledgeBases: {
      documents: { list: async () => ({ data: [], total: 0 }) },
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
        activity: async () => ({ success: true, data: [] }),
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderSection(section: string, kb: KnowledgeSettingsInput = baseKnowledgeBase, locale = 'zh-CN'): Promise<void> {
  dom.window.localStorage.setItem('locale', locale);
  if (mountedRoot) {
    const previous = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { previous.unmount(); });
    document.body.innerHTML = '';
  }
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client: clientFor(), knowledgeBase: kb, role: 'admin', initialSection: section as never }));
  });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
}

function heading(): { eyebrow: string; title: string; description: string } {
  const eyebrow = document.body.querySelector('.wk-eyebrow');
  const title = document.body.querySelector('#knowledge-settings-section-title');
  const description = [...document.body.querySelectorAll('.wk-muted')][0];
  assert.ok(eyebrow && title && description, 'expected the section eyebrow, title and description nodes');
  return { eyebrow: eyebrow.textContent ?? '', title: title.textContent ?? '', description: description.textContent ?? '' };
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
  dom.window.localStorage.setItem('locale', 'zh-CN');
});

test('zh-CN renders localized section headings and descriptions without hardcoded English', async () => {
  const expectations: Array<[string, string, string]> = [
    ['basic', '基本信息', '知识库名称、描述与类型'],
    ['models', '模型配置', '语言模型与嵌入模型'],
    ['parser', '解析引擎', '按文件类型的解析规则'],
    ['chunking', '分块设置', '分块大小与切分行为'],
    ['storage', '存储引擎', '文件与文档实例'],
    ['datasource', '数据源', '外部连接器与同步状态'],
    ['share', '共享管理', '可访问此知识库的空间'],
    ['activity', '活动记录', '近期配置变更'],
    ['graph', '知识图谱', '实体与关系抽取'],
    ['advanced', '高级设置', '问题生成与其他选项'],
  ];
  for (const [section, title, description] of expectations) {
    await renderSection(section);
    const text = heading();
    assert.equal(text.eyebrow, title, `zh-CN ${section} eyebrow`);
    assert.equal(text.title, title, `zh-CN ${section} title`);
    assert.equal(text.description, description, `zh-CN ${section} description`);
    assert.doesNotMatch(text.title, /[a-z]/, `zh-CN ${section} title has no English residue`);
  }
});

test('zh-CN FAQ base localizes the faq section heading', async () => {
  await renderSection('faq', { id: 'kb-457-faq', name: 'FAQ', type: 'faq' });
  const text = heading();
  assert.equal(text.title, 'FAQ 设置');
  assert.equal(text.description, 'FAQ 索引模式');
});

test('en-US keeps the pre-R457 section heading copy byte-identical', async () => {
  await renderSection('basic', baseKnowledgeBase, 'en-US');
  const text = heading();
  assert.equal(text.eyebrow, 'Basics');
  assert.equal(text.title, 'Basics');
  assert.equal(text.description, 'Name, description and knowledge-base type');

  await renderSection('parser', baseKnowledgeBase, 'en-US');
  const parser = heading();
  assert.equal(parser.title, 'Parser');
  assert.equal(parser.description, 'File-type parser rules');

  await renderSection('activity', baseKnowledgeBase, 'en-US');
  const activity = heading();
  assert.equal(activity.title, 'Activity');
  assert.equal(activity.description, 'Recent configuration changes');
});
