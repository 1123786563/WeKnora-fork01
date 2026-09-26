// R457 A1 round localized the editor-section headings; R484 (R482 B1 差异4)
// re-pointed them at the exact Vue per-tab section-header keys
// (KnowledgeBaseEditorModal.vue + each settings component's own
// h2/h3 + section-description) and dropped the React eyebrow line, so the
// rendered copy is byte-identical to the Vue drawer instead of the fresh
// kbSettings.sections.* paraphrase. This file pins that contract.
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
  // 分块段换 tdesign 控件（Select/Slider/Switch/InputNumber，弹层 Popup 系
  // 需要 Element/Node/SVGElement/rAF —— SandboxSettingsPanel.test 同款先例）。
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
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

// ---- unit: every section resolves the Vue section-header keys -------

test('every editor section carries the Vue per-tab heading keys', () => {
  const documentSections = getKnowledgeSettingsSections(baseKnowledgeBase);
  const faqSections = getKnowledgeSettingsSections({ id: 'kb-457-faq', name: 'FAQ', type: 'faq' });
  const expectedKeys: Record<string, [string, string]> = {
    basic: ['knowledgeEditor.basic.title', 'knowledgeEditor.basic.description'],
    models: ['knowledgeEditor.models.title', 'knowledgeEditor.models.description'],
    vectorStore: ['kbSettings.vectorStore.title', 'kbSettings.vectorStore.description'],
    faq: ['knowledgeEditor.faq.title', 'knowledgeEditor.faq.description'],
    parser: ['kbSettings.parser.title', 'kbSettings.parser.description'],
    multimodal: ['knowledgeEditor.multimodal.title', 'knowledgeEditor.multimodal.description'],
    asr: ['knowledgeEditor.asr.title', 'knowledgeEditor.asr.description'],
    storage: ['kbSettings.storage.title', 'kbSettings.storage.selectDescription'],
    chunking: ['knowledgeEditor.chunking.title', 'knowledgeEditor.chunking.description'],
    graph: ['graphSettings.title', 'graphSettings.description'],
    advanced: ['knowledgeEditor.advanced.title', 'knowledgeEditor.advanced.description'],
    datasource: ['dataSource.title', 'dataSource.description'],
    share: ['organization.share.title', 'knowledgeEditor.share.description'],
    activity: ['knowledgeEditor.activity.title', 'knowledgeEditor.activity.description'],
  };
  for (const section of [...documentSections, ...faqSections]) {
    const pair = expectedKeys[section.key]!;
    assert.equal(section.labelKey, pair[0], `labelKey for ${section.key}`);
    assert.equal(section.descriptionKey, pair[1], `descriptionKey for ${section.key}`);
    // en-US fallback strings stay in place for raw-contract consumers.
    assert.ok(section.label.length > 0, `fallback label kept for ${section.key}`);
    assert.ok(section.description.length > 0, `fallback description kept for ${section.key}`);
  }
  assert.equal(documentSections.length + faqSections.length, 20, '13 document sections + 7 FAQ-visible sections');
});

// ---- render: zh-CN headings byte-identical to the Vue copy -----------

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

function heading(): { title: string; description: string } {
  /* chunking 分区例外（px2-kb-settings-nav 同构）：Vue KBChunkingSettings
     自带 sticky .section-header（h2 + .section-description），页级 h3 不渲染。 */
  const chunkingH2 = document.body.querySelector('.kb-chunking-settings .section-header h2');
  const title = chunkingH2 ?? document.body.querySelector('#knowledge-settings-section-title');
  const description = chunkingH2
    ? document.body.querySelector('.kb-chunking-settings .section-header .section-description')
    : [...document.body.querySelectorAll('.wk-muted')][0];
  assert.ok(title && description, 'expected the section title and description nodes');
  return { title: title.textContent ?? '', description: description.textContent ?? '' };
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

test('zh-CN renders the Vue section headings and descriptions without the React eyebrow', async () => {
  const expectations: Array<[string, string, string]> = [
    ['basic', '基本信息', '设置知识库的名称和描述信息'],
    ['models', '模型配置', '为知识库选择合适的 AI 模型'],
    ['vectorStore', '向量存储', '选择此知识库要写入的向量存储。绑定不可更改 — 如需将现有 KB 迁移到其他存储，请创建新 KB 并重新索引。'],
    ['parser', '解析引擎', '为不同文件类型选择文档解析引擎。未配置的文件类型将使用内置解析引擎。'],
    ['chunking', '分块设置', '控制上传文档在嵌入前的切分方式。默认值适用于大多数场景，仅在检索质量异常时调整。'],
    ['multimodal', '图像处理配置', '配置图像内容理解能力，启用后支持图片等非文本内容的解析和检索'],
    ['asr', '音频语音识别', '配置语音识别（ASR），启用后可上传音频文件并整段转写为文本（常见格式：mp3、wav、m4a、flac、ogg 等）。暂不支持视频上传。'],
    ['storage', '存储引擎', '选择此知识库绑定的具体存储实例。'],
    ['datasource', '数据源管理', '配置外部数据源，自动同步内容到知识库'],
    ['share', '共享到共享空间', '将知识库共享给空间，让空间成员可以访问和使用'],
    ['activity', '活动记录', '查看这个知识库的重要变更与后台任务入口，记录默认遵循审计日志保留策略。'],
    ['graph', '知识图谱配置', '配置实体-关系提取功能，自动从文本中抽取实体和关系构建知识图谱（注意：这与 Wiki 知识库中的「页面链接图谱」是两回事——前者是基于 LLM 的实体-关系图，后者是 Wiki 页面之间的引用关系图）'],
    ['advanced', '高级设置', '配置问题生成等高级功能'],
  ];
  for (const [section, title, description] of expectations) {
    await renderSection(section);
    const text = heading();
    assert.equal(text.title, title, `zh-CN ${section} title`);
    assert.equal(text.description, description, `zh-CN ${section} description`);
    assert.equal(document.body.querySelector('.wkbs-content .wk-eyebrow'), null, `${section} renders no React eyebrow line`);
  }
});

test('zh-CN FAQ base localizes the faq section heading', async () => {
  await renderSection('faq', { id: 'kb-457-faq', name: 'FAQ', type: 'faq' });
  const text = heading();
  assert.equal(text.title, '问答');
  assert.equal(text.description, '设置 FAQ 知识库的索引策略和问答组织方式');
});

test('en-US renders the Vue section heading copy', async () => {
  await renderSection('basic', baseKnowledgeBase, 'en-US');
  const text = heading();
  assert.equal(text.title, 'Basic Information');
  assert.equal(text.description, 'Configure the knowledge base name and description');

  await renderSection('parser', baseKnowledgeBase, 'en-US');
  const parser = heading();
  assert.equal(parser.title, 'Parser Engine');
  assert.equal(parser.description, 'Select document parser engines for different file types. Unconfigured file types will use the built-in parser.');

  await renderSection('activity', baseKnowledgeBase, 'en-US');
  const activity = heading();
  assert.equal(activity.title, 'Activity');
  assert.equal(activity.description, 'Important changes and background-task entry points for this knowledge base. Retention follows the audit-log policy.');
});
