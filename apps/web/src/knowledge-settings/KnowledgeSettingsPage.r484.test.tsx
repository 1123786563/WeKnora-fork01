// R484 G1 (R482 B1 差异4 — KB 设置抽屉 13-tab 系统性差异族): the React
// knowledge-settings surface must render the Vue KnowledgeBaseEditorModal
// contract per tab — the Vue per-tab section title/description copy
// (knowledgeEditor.* / kbSettings.* / graphSettings.* / dataSource.* /
// organization.share.title), the Vue footer (取消 + 保存并关闭 with cancel
// discarding drafts), the Vue KBParserSettings per-file-type engine list,
// the Vue DataSourceSettings form inside the drawer (no duplicated standalone
// page header), and the Vue activity end hint (没有更早的记录了) with the
// icon-only refresh trigger.
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
dom.window.localStorage.setItem('locale', 'en-US');
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLButtonElement: dom.window.HTMLButtonElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  // tdesign-react Select/Popup 运行时（parserSettings 平移为 tdesign Select）。
  Element: dom.window.Element,
  SVGElement: dom.window.SVGElement,
  DocumentFragment: dom.window.DocumentFragment,
  Node: dom.window.Node,
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
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
const {
  KnowledgeSettingsPage,
  getKnowledgeSettingsSections,
} = await import('./KnowledgeSettingsPage.tsx');
type KnowledgeSettingsInput = import('./KnowledgeSettingsPage.tsx').KnowledgeSettingsInput;

// Mirrors the knowledge-base row the Vue editor loads for an existing KB.
const knowledgeBase: KnowledgeSettingsInput = {
  id: 'kb-1',
  name: 'Product docs',
  type: 'document',
  summary_model_id: 'llm-1',
  embedding_model_id: 'embed-1',
  chunking_config: {
    chunk_size: 700,
    chunk_overlap: 90,
    separators: ['\n\n', '\n'],
    enable_parent_child: true,
    parent_chunk_size: 4096,
    child_chunk_size: 384,
    strategy: 'auto',
    token_limit: 0,
    languages: ['zh'],
    table_metadata_instructions: 'keep headers',
  },
  question_generation_config: { enabled: true, question_count: 5, custom_instructions: 'gen rules' },
  vlm_config: { enabled: false, model_id: '', description_language: '', custom_instructions: '' },
  asr_config: { enabled: false, model_id: '', language: '' },
};

const liveModels = [
  { id: 'llm-1', name: 'gpt-x', display_name: 'GPT X', type: 'KnowledgeQA', source: 'remote' },
  { id: 'embed-1', name: 'bge-m3', display_name: 'BGE M3', type: 'Embedding', source: 'local' },
];

// Vue editorResources parser catalogue (KBParserSettings loadEngines): the
// builtin engine covers the complex formats, simple covers text/image/audio
// families, anydoc the office family, mineru pdf — enough to exercise the
// known file-type families plus per-group default resolution.
const parserCatalogue = [
  { Name: 'builtin', Description: 'Built-in', Available: true, FileTypes: ['pdf', 'docx', 'doc', 'pptx', 'ppt', 'xlsx', 'xls', 'epub', 'mhtml', 'csv', 'md', 'markdown', 'txt', 'json'] },
  { Name: 'mineru', Description: 'MinerU', Available: true, FileTypes: ['pdf'] },
  { Name: 'anydoc', Description: 'AnyDoc', Available: true, FileTypes: ['docx', 'doc', 'pptx', 'ppt', 'xlsx', 'xls'] },
  { Name: 'simple', Description: 'Simple', Available: true, FileTypes: ['md', 'markdown', 'txt', 'csv', 'json', 'jpg', 'jpeg', 'png', 'gif', 'bmp', 'tiff', 'webp', 'mp3', 'wav', 'm4a', 'flac', 'ogg'] },
];

interface UiCalls { requests: Array<{ method: string; path: string; body: Record<string, unknown> }> }

interface ClientOverrides {
  parserEngines?: Array<{ Name: string; Description: string; Available?: boolean; FileTypes?: string[] }>;
  activity?: () => Promise<{ data?: Array<Record<string, unknown>>; next_cursor?: number }> | { data?: Array<Record<string, unknown>>; next_cursor?: number };
  dataSources?: Array<Record<string, unknown>>;
}

function clientFor(calls: UiCalls, overrides: ClientOverrides = {}): WeKnoraClient {
  const request = async (input: { method: string; path: string; body: Record<string, unknown> }) => {
    calls.requests.push({ method: input.method, path: input.path, body: input.body });
    return { success: true };
  };
  return {
    request,
    configuration: {
      models: { list: async () => liveModels },
    },
    identity: {
      organizations: {
        list: async () => [],
        knowledgeBaseShares: {
          list: async () => [],
          create: async () => ({ success: true }),
          remove: async () => ({ success: true }),
          update: async () => ({ success: true }),
        },
      },
    },
    dataSources: {
      list: async () => overrides.dataSources ?? [],
      types: async () => [],
    },
    knowledgeBases: {
      documents: {
        list: async (kbId: string, params: Record<string, number> = {}) => {
          const query = new URLSearchParams(Object.entries(params).map(([key, value]) => [key, String(value)]));
          const suffix = query.toString();
          await request({ method: 'GET', path: `/api/v1/knowledge-bases/${encodeURIComponent(kbId)}/knowledge${suffix ? `?${suffix}` : ''}`, body: {} });
          return { data: [], total: 0 };
        },
      },
      settings: {
        parserEngines: async () => ({ data: overrides.parserEngines ?? parserCatalogue }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
        activity: overrides.activity ?? (async () => ({ data: [], next_cursor: undefined })),
      },
    },
  } as unknown as WeKnoraClient;
}

let mountedRoot: Root | undefined;

async function renderPage(client: WeKnoraClient, props: Record<string, unknown> = {}): Promise<void> {
  const container = document.createElement('div');
  document.body.appendChild(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot!.render(React.createElement(KnowledgeSettingsPage, { client, knowledgeBase, role: 'admin', ...props }));
  });
  await act(async () => { await Promise.resolve(); });
}

async function openSection(section: string): Promise<void> {
  const button = document.body.querySelector(`button[data-section="${section}"]`);
  assert.ok(button, `expected a ${section} section button; got: ${JSON.stringify([...document.body.querySelectorAll('button[data-section]')].map((candidate) => candidate.textContent))}`);
  await act(async () => { button!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
}

function labeledControl(selector: string, label: string): HTMLElement {
  const control = [...document.body.querySelectorAll<HTMLElement>(selector)].find((candidate) => candidate.getAttribute('aria-label') === label);
  assert.ok(control, `expected a ${selector} with aria-label "${label}"; got: ${JSON.stringify([...document.body.querySelectorAll(selector)].map((candidate) => candidate.getAttribute('aria-label')))}`);
  return control;
}

function setNativeValue(element: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  const prototype = element instanceof dom.window.HTMLTextAreaElement ? dom.window.HTMLTextAreaElement.prototype : dom.window.HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(prototype, 'value')!.set!;
  setter.call(element, value);
  element.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  element.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
}

function setSelectValue(select: HTMLSelectElement, value: string): void {
  [...select.options].forEach((option) => { option.selected = option.value === value; });
  select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
}

// ---- tdesign Select（Vue t-select 同构）驱动 helper ------------------------
function parserTrigger(group: string): HTMLElement {
  const row = document.body.querySelector(`[data-parser-group="${group}"]`);
  const trigger = row?.querySelector('.t-select__wrap') as HTMLElement | null;
  assert.ok(trigger, `expected the ${group} parser engine select`);
  return trigger;
}
function parserTriggerValue(group: string): string {
  return (parserTrigger(group).querySelector('input.t-input__inner') as HTMLInputElement | null)?.value ?? '';
}
async function pickParserEngine(group: string, optionText: string): Promise<void> {
  const inner = parserTrigger(group).querySelector('.t-input') as HTMLElement;
  await act(async () => { inner.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  let options = Array.from(document.body.querySelectorAll<HTMLElement>('.t-select-option'));
  if (options.length === 0) {
    await act(async () => { inner.dispatchEvent(new dom.window.MouseEvent('mousedown', { bubbles: true, cancelable: true })); });
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    options = Array.from(document.body.querySelectorAll<HTMLElement>('.t-select-option'));
  }
  const target = options.find((el) => (el.textContent ?? '').trim() === optionText);
  assert.ok(target, `option missing: ${optionText} (have ${options.map((el) => el.textContent).join(',')})`);
  await act(async () => { target.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
}

function buttonByText(text: string): HTMLButtonElement {
  const button = [...document.body.querySelectorAll('button')].find((candidate) => (candidate.textContent ?? '').trim() === text);
  assert.ok(button, `expected a button labelled "${text}"`);
  return button as HTMLButtonElement;
}

afterEach(async () => {
  if (mountedRoot) {
    const root = mountedRoot;
    mountedRoot = undefined;
    await act(async () => { root.unmount(); });
  }
  document.body.innerHTML = '';
});

// Vue per-tab heading keys: the modal itself renders the basic header
// (KnowledgeBaseEditorModal.vue section-title/desc) and each settings
// component renders its own (KBModelConfig/KBVectorStoreSettings/
// KBParserSettings/KBChunkingSettings/KBAdvancedSettings/KBStorageSettings/
// GraphSettings/DataSourceSettings/KBShareSettings/KnowledgeBaseActivitySettings
// h2/h3 + section-description). The React sections list must point at those
// exact keys instead of the React-invented kbSettings.sections.* copy.
test('section headings resolve through the Vue per-tab copy keys', () => {
  const expected: Record<string, [string, string]> = {
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
  const sections = getKnowledgeSettingsSections(knowledgeBase);
  assert.equal(sections.length, 13, 'document KB carries the full 13-tab set (faq only on FAQ bases)');
  for (const section of sections) {
    const pair = expected[section.key];
    assert.ok(pair, `unexpected section ${section.key}`);
    assert.equal(section.labelKey, pair[0], `${section.key} title key must match the Vue section header`);
    assert.equal(section.descriptionKey, pair[1], `${section.key} description key must match the Vue section header`);
  }
  const faqSections = getKnowledgeSettingsSections({ ...knowledgeBase, type: 'faq' });
  const faqSection = faqSections.find((section) => section.key === 'faq');
  assert.ok(faqSection, 'FAQ bases expose the faq tab');
  assert.equal(faqSection!.labelKey, expected.faq![0]);
  assert.equal(faqSection!.descriptionKey, expected.faq![1]);
});

// R482 B1 #9-21: the React surface rendered its own eyebrow line plus the
// React-invented kbSettings.sections.* descriptions and R455 overview tiles;
// Vue renders exactly one section title + description per tab.
test('each tab renders the Vue section title and description without the React eyebrow or overview tiles', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));

  const cases: Array<[string, string, RegExp]> = [
    ['basic', 'Basic Information', /Configure the knowledge base name and description/],
    ['models', 'Model Configuration', /Select appropriate AI models/],
    ['vectorStore', 'Vector Store', /binding is permanent/],
    ['parser', 'Parser Engine', /Unconfigured file types will use the built-in parser/],
    ['multimodal', 'Image Processing Configuration', /Configure image content understanding/],
    ['asr', 'Audio Speech Recognition', /Configure ASR/],
    ['storage', 'Storage Engine', /Select the specific storage instance/],
    ['chunking', 'Chunking Settings', /split before embedding/],
    ['graph', 'Knowledge Graph Configuration', /entity-relationship extraction/],
    ['advanced', 'Advanced Settings', /question generation and other advanced features/],
    ['datasource', 'Data Sources', /sync content into this knowledge base/],
    ['share', 'Share to Shared Space', /Share the knowledge base with spaces/],
    ['activity', 'Activity', /Important changes and background-task entry points/],
  ];
  for (const [key, title, description] of cases) {
    await openSection(key);
    const heading = document.body.querySelector('#knowledge-settings-section-title');
    assert.ok(heading, `${key} heading`);
    assert.equal((heading.textContent ?? '').trim(), title, `${key} renders the Vue section title`);
    assert.match(document.body.textContent ?? '', description, `${key} renders the Vue section description`);
    assert.equal(document.body.querySelector('.wkbs-content .wk-eyebrow'), null, `${key} must not render the React eyebrow line`);
  }

  // Overview tiles (R455/R456 additions) carry no Vue counterpart inside the
  // drawer — the visible copy must be gone.
  await openSection('datasource');
  const datasourceBody = document.body.textContent ?? '';
  assert.equal(datasourceBody.includes('Add an external data source'), false, 'datasource overview-tile copy must be gone');
  assert.equal(datasourceBody.includes('No data sources'), false, 'datasource overview-tile empty copy must be gone');
  await openSection('share');
  // The share dialog's own empty state (organization.share.noShares, part of
  // the deferred share-form alignment) stays; the overview-tile copy
  // (kbSettings.summary.share.emptyDetail) must be gone.
  assert.equal((document.body.textContent ?? '').includes('No spaces have access'), false, 'share overview-tile copy must be gone');
  await openSection('activity');
  assert.equal((document.body.textContent ?? '').includes('recent'), false, 'activity overview-tile copy must be gone');
});

// Vue settings-footer (KnowledgeBaseEditorModal.vue settings-footer-actions):
// 「取消」(common.cancel) discards the drafts and closes; the primary button
// is knowledgeEditor.buttons.saveAndClose (保存并关闭) in edit mode.
test('footer renders Cancel plus Save and Close, and cancel discards draft edits', async () => {
  const calls: UiCalls = { requests: [] };
  let closeCalls = 0;
  await renderPage(clientFor(calls), { onClose: () => { closeCalls += 1; } });

  assert.ok(buttonByText('Cancel'), 'the Vue footer cancel button renders');
  const save = buttonByText('Save and Close');
  assert.ok(save, 'the Vue footer primary button is Save and Close in edit mode');
  assert.equal([...document.body.querySelectorAll('button')].some((candidate) => (candidate.textContent ?? '').trim() === 'Save Configuration'), false, 'the React-invented single Save Configuration button is gone');

  // Edit the name, then cancel: the draft must revert and the host is closed.
  const name = labeledControl('input', 'Knowledge Base Name') as HTMLInputElement;
  await act(async () => { setNativeValue(name, 'Discarded draft'); });
  assert.equal(name.value, 'Discarded draft');
  await act(async () => { buttonByText('Cancel').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  const nameAfter = labeledControl('input', 'Knowledge Base Name') as HTMLInputElement;
  assert.equal(nameAfter.value, 'Product docs', 'cancel discards the unsaved edit like the Vue handleClose');
  assert.equal(closeCalls, 1, 'cancel closes the host surface when onClose is provided');
});

// Vue KBParserSettings: one row per file-type family (label + extension
// tags), a per-row engine select whose default engine is suffixed
// 「(默认)」, the xlsx first-row checkbox while the Excel family is on
// builtin, and buildCompleteRules() persisting one rule per group.
test('parser section renders the Vue per-file-type engine list and saves complete rules', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls));
  await openSection('parser');

  // Known families from the catalogue, Vue labels.
  const rowLabels = ['PDF Documents', 'Word Documents', 'Presentations', 'Excel Spreadsheets', 'E-books', 'Web Archives', 'CSV Files', 'Markdown', 'Plain Text', 'JSON Files', 'Images', 'Audio'];
  const bodyText = document.body.textContent ?? '';
  for (const label of rowLabels) assert.ok(bodyText.includes(label), `expected a ${label} file-type row`);
  assert.ok(bodyText.includes('.pdf'), 'extension tags render with the leading dot');
  assert.ok(bodyText.includes('.docx'));
  assert.ok(bodyText.includes('.xlsx'));

  // Per-row tdesign Selects: pdf defaults to builtin (first supporting
  // engine), office family prefers anydoc (Vue pickDefaultEngineName), the
  // default option carries the (Default) suffix in the trigger label.
  assert.equal(parserTriggerValue('pdf'), 'Built-in (Default)');
  assert.equal(parserTriggerValue('office'), 'anydoc (Default)', 'the office family defaults to anydoc like the Vue pickDefaultEngineName');

  // xlsx first-row checkbox only shows while the Excel family is builtin.
  const checkboxLabel = (candidate: Element): string => {
    const control = candidate as HTMLInputElement;
    return control.labels?.[0]?.textContent ?? '';
  };
  assert.equal([...document.body.querySelectorAll('input[type="checkbox"]')].some((candidate) => checkboxLabel(candidate).includes('first row as column context')), false);
  await pickParserEngine('excel', 'Built-in');
  const xlsxToggle = [...document.body.querySelectorAll('input[type="checkbox"]')].find((candidate) => checkboxLabel(candidate).includes('first row as column context'));
  assert.ok(xlsxToggle, 'the xlsx first-row checkbox renders while the Excel family is builtin');
  await act(async () => { xlsxToggle!.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });

  // Switch pdf to mineru and save: buildCompleteRules persists one rule per
  // group — pdf→mineru, office→anydoc, excel→builtin(+header), text→builtin.
  await pickParserEngine('pdf', 'MinerU');
  await act(async () => { buttonByText('Save and Close').dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })); });
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const put = calls.requests.find((request) => request.path === '/api/v1/initialization/config/kb-1');
  assert.ok(put, 'the config PUT ran');
  const rules = ((put!.body as Record<string, any>).documentSplitting.parserEngineRules ?? []) as Array<Record<string, unknown>>;
  const byEngine = (engine: string) => rules.filter((rule) => rule.engine === engine);
  assert.deepEqual(byEngine('mineru').map((rule) => rule.file_types), [['pdf']], 'the pdf group carries the selected engine');
  const office = byEngine('anydoc').find((rule) => Array.isArray(rule.file_types) && (rule.file_types as string[]).includes('docx'));
  assert.ok(office, 'untouched groups persist their resolved default engine');
  const excel = byEngine('builtin').find((rule) => Array.isArray(rule.file_types) && (rule.file_types as string[]).includes('xlsx'));
  assert.ok(excel, 'the excel group persists the builtin selection');
  assert.equal(excel!.xlsx_first_row_as_header, true, 'the xlsx first-row flag persists on the excel rule');
  assert.match(document.body.textContent ?? '', /Configuration saved successfully/);
});

// Vue DataSourceSettings inside the drawer renders its own header through the
// tab heading; the embedded React DataSourcesPage must not repeat the
// standalone page header (「知识库 · {kbId}」 eyebrow + h1) — the drawer shows
// the Vue empty-state add card instead of the React overview tile copy.
test('datasource tab renders the Vue empty state without the duplicated standalone page header', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, { dataSources: [] }));
  await openSection('datasource');

  const bodyText = document.body.textContent ?? '';
  assert.equal(bodyText.includes('Knowledge bases · kb-1'), false, 'the standalone DataSourcesPage eyebrow must not render inside the drawer');
  assert.equal([...document.body.querySelectorAll('h1')].length, 0, 'the standalone page h1 must not render inside the drawer');
  const titleHeadings = [...document.body.querySelectorAll('h1,h2,h3')].filter((candidate) => (candidate.textContent ?? '').trim() === 'Data Sources');
  assert.equal(titleHeadings.length, 1, 'exactly one Data Sources heading (the tab header)');
  assert.ok(bodyText.includes('Add Data Source'), 'the Vue empty-state add card renders');
});

// Vue KnowledgeBaseActivitySettings: icon-only refresh trigger
// (suggested-questions-refresh) and the audit-end hint
// 「没有更早的记录了」(knowledgeEditor.activity.end) once the cursor is
// exhausted — no load-more button, no text refresh button.
test('activity tab shows the Vue end hint with an icon-only refresh and no duplicated panel header', async () => {
  const calls: UiCalls = { requests: [] };
  await renderPage(clientFor(calls, {
    activity: async () => ({ data: [{ id: 'a1', action: 'kb.updated', outcome: 'success', target_type: 'knowledge_base', actor_username: 'parity', created_at: '2026-09-20T01:02:03Z' }], next_cursor: undefined }),
  }));
  await openSection('activity');

  const bodyText = document.body.textContent ?? '';
  assert.ok(bodyText.includes('No earlier activity'), 'the exhausted cursor renders the Vue end hint');
  assert.equal(bodyText.includes('Load more'), false, 'no load-more button once the cursor is exhausted');
  const refresh = [...document.body.querySelectorAll('button')].find((candidate) => candidate.getAttribute('aria-label') === 'Refresh');
  assert.ok(refresh, 'the refresh trigger keeps its accessible name');
  assert.doesNotMatch(refresh!.textContent ?? '', /Refresh/, 'the refresh trigger carries no visible text like the Vue icon-only title-row button');
  const headings = [...document.body.querySelectorAll('h1,h2,h3')].filter((candidate) => (candidate.textContent ?? '').trim() === 'Activity');
  assert.equal(headings.length, 1, 'the panel header is not duplicated behind the tab heading');
});
