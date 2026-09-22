import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain';

// R430 kb-create dialog anatomy slice (Vue KnowledgeBaseEditorModal.vue authority):
//   (a) type selector keeps the joined TDesign outline radio-group frame
//       (buttons carry the 1px #e7e7e7 border + 3px end radius; checked tab
//       is the brand-green fill with the green shared divider; arrow keys move
//       the selection like t-radio-group)
//   (b) the Wiki indexing card carries the NEW badge (brand-light pill,
//       10px/600, 16px tall, 3px radius, 6px x-padding)
//   (c) the description textarea carries the TDesign "0/200" limit counter
//       (right-aligned, 12px/20px placeholder gray, native maxlength=200)
// R463 kb-editor submit-structure slice (same Vue authority):
//   (d) the editor dialog renders NO wrapping <form> — the Vue modal has
//       zero native form elements (footer buttons are plain @click handlers,
//       :448-455), so the wk-form grid is carried by a plain <div> and the
//       nested-form hydration error source is gone at the root
//   (e) the save button submits through onClick into the same save pipeline
//       (Vue :451 `<t-button @click="handleSubmit">`), not type="submit"
//   (f) Enter in the name input does NOT submit (Vue has no form → no
//       implicit-submission contract) and the name input carries no native
//       `required` attribute (Vue t-input :165-169 is maxlength-only; blank
//       names are blocked by the JS pipeline, `if (!name.trim()) return`)
// R466 loading-disable slice (same Vue authority :451-454):
//   (g) the footer save button is `:disabled="loading"` — while the editor's
//       data (models/storage/vector/parser options) is still loading the save
//       button is disabled with the SAME label (Vue only disables, the label
//       stays saveButtonLabel), and the cancel button (:448) carries no
//       loading disable so it stays clickable
//   (h) `saving` keeps its own semantics independent of `loading`: once the
//       editor data has settled, save is enabled; during the submit request
//       the button shows the saving disable+spinner (aria-busy) and the
//       double-submit guard keeps exactly one create call

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg?raw') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/knowledge-bases' });
/* tdesign 运行时依赖的 DOM 构造器全局补齐（pilot agents 同款）。
 * renderAdapter 注入 createRoot，保证 MessagePlugin 等命令式 API 与组件同一
 * React 实例（main.tsx 同款 react-19 adapter，node/tsx 下直接注入）。 */
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  DocumentFragment: dom.window.DocumentFragment,
  Event: dom.window.Event,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  IS_REACT_ACT_ENVIRONMENT: true,
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
// openContextualGuide builds `new CustomEvent(...)` from the Node global and
// hands it to jsdom's window.dispatchEvent, which rejects foreign-realm
// events; route the Node global through the jsdom window for this harness.
(globalThis as { CustomEvent?: unknown }).CustomEvent = dom.window.CustomEvent;
// The live parity environment runs zh-CN (locale seeded via localStorage); the
// page resolves its locale from navigator.language, so pin it for assertions.
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });

const { createRoot } = await import('react-dom/client');
/* tdesign 命令式 API（MessagePlugin）的 React19 render adapter（main.tsx 同款）。 */
{
  const { renderAdapter } = await import('tdesign-react/lib/_util/react-render.js');
  renderAdapter(createRoot);
}
const { KnowledgeBasesPage } = await import('../App.tsx');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.history.replaceState(null, '', '/platform/knowledge-bases');
  dom.window.localStorage.clear();
});

/** Default tenant model rows: one default chat model, one spare chat, one embedding. */
const defaultModelRows = (): Array<Record<string, unknown>> => [
  { id: 'm-chat-default', name: 'Default Chat', display_name: '', type: 'KnowledgeQA', status: 'active', is_default: true },
  { id: 'm-chat-2', name: 'Second Chat', display_name: '', type: 'KnowledgeQA', status: 'active', is_default: false },
  { id: 'm-embed', name: 'Mock Embed', display_name: '', type: 'Embedding', status: 'active', is_default: false },
];

function makeClient(calls: string[] = [], modelRows: () => Array<Record<string, unknown>> = defaultModelRows): WeKnoraClient {
  return {
    auth: {
      me: async () => ({ user: { id: 'u-1', is_system_admin: false }, memberships: [{ tenant_id: 't-1', role: 'contributor' }] }),
    },
    knowledgeBases: {
      list: async () => [
        { id: 'kb-doc', name: 'Parity KB Demo', description: 'parity test data', type: 'document', knowledge_count: 2, creator_id: 'u-1', summary_model_id: 'm-1', embedding_model_id: 'm-2' },
      ],
      togglePin: async () => ({ is_pinned: true }),
      duplicate: async () => ({}),
      remove: async () => ({}),
      create: async () => { calls.push('create'); return { id: 'kb-new' }; },
      update: async () => { calls.push('update'); return {}; },
      settings: {
        parserEngines: async () => ({ data: [] }),
        storageBackends: async () => ({ data: [] }),
        vectorStores: async () => ({ data: [] }),
      },
    },
    configuration: {
      models: {
        // R488 summary_model_id slice: the Vue editor pulls GET /api/v1/models
        // (KnowledgeBaseEditorModal.vue:680-693) to feed the ModelSelector
        // dropdowns and prefill the create-time defaults.
        list: async () => modelRows(),
      },
    },
    identity: { organizations: { knowledgeBaseShares: { listShared: async () => [] } } },
  } as unknown as WeKnoraClient;
}

async function mountPageWithClient(client: WeKnoraClient): Promise<void> {
  const scopeController = createScopeController({ origin: 'https://weknora.test', userId: 'u-1', tenantId: 't-1' });
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<KnowledgeBasesPage client={client} scopeController={scopeController} />);
  });
  await act(async () => {});
  await act(async () => {
    document.body.querySelector<HTMLButtonElement>('[data-guide="kb-list-create"]')?.click();
  });
  await act(async () => {});
}

async function mountPage(calls: string[] = []): Promise<void> {
  await mountPageWithClient(makeClient(calls));
}

/* Task 11a：类型选择器是 tdesign RadioGroup（Vue t-radio-group 默认 outline
   连体按钮组，DOM 与 Vue 逐字一致：div.t-radio-group > label.t-radio-button）。 */
function getTypeFrame(): HTMLElement | null {
  return document.body.querySelector<HTMLElement>('[data-guide="kb-create-type"] .t-radio-group');
}

test('(a) the type selector keeps the joined TDesign outline frame with the brand-green checked tab', async () => {
  await mountPage();
  const frame = getTypeFrame();
  assert.ok(frame, 'expected the .t-radio-group inside [data-guide="kb-create-type"]');
  assert.match(frame.className, /t-radio-group__outline/, 'default outline variant = the joined TDesign frame (Vue t-radio-group)');

  const radios = Array.from(frame.querySelectorAll<HTMLLabelElement>('label.t-radio-button'));
  assert.deepEqual(radios.map((radio) => radio.textContent), ['文档', '问答']);
  const [documentRadio, faqRadio] = radios;
  assert.ok(documentRadio.classList.contains('t-is-checked'), 'document is the create default');
  assert.equal(faqRadio.classList.contains('t-is-checked'), false, 'faq unchecked');
  assert.equal(faqRadio.tabIndex, 0, 'both radio buttons stay in the tab order (tdesign roving contract)');
  assert.equal(documentRadio.tabIndex, 0);
});

test('(a) keyboard check on a radio button moves the type selection like the TDesign radio group', async () => {
  await mountPage();
  const frame = getTypeFrame();
  assert.ok(frame);
  /* tdesign-react RadioGroup 的键盘契约是 Enter/Space 选中聚焦项
     （useKeyboard CHECKED_CODE_REG）；tdesign-vue-next 的方向键导航是库间
     行为差异（无 DOM/像素影响，不页面补偿——playbook §3.4）。 */
  const faqLabel = Array.from(frame.querySelectorAll<HTMLLabelElement>('label.t-radio-button'))
    .find((label) => label.textContent === '问答');
  assert.ok(faqLabel, 'faq radio button rendered');
  await act(async () => {
    faqLabel.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  });
  await act(async () => {});
  // Selecting FAQ flips the section content (问答配置) exactly like clicking the tab.
  const faqHeading = Array.from(document.body.querySelectorAll('h3')).find((el) => (el.textContent ?? '') === '问答');
  assert.ok(faqHeading, 'keyboard selection landed on the FAQ section');
});

test('(b) the Wiki indexing card carries the NEW badge and the RAG card does not', async () => {
  await mountPage();
  const cards = Array.from(document.body.querySelectorAll('[data-guide="kb-create-indexing"] .indexing-check-item'));
  assert.equal(cards.length, 2, 'RAG + Wiki cards render');
  const wikiCard = cards.find((card) => (card.textContent ?? '').includes('Wiki 知识库'));
  const ragCard = cards.find((card) => (card.textContent ?? '').includes('RAG 检索'));
  assert.ok(wikiCard && ragCard, 'both indexing cards found');
  const ragChecked = ragCard.classList.contains('is-checked');
  assert.equal(ragChecked, true, 'RAG card renders checked by default (Vue vectorEnabled=true)');
  assert.equal(wikiCard.classList.contains('is-checked'), false, 'Wiki card unchecked');

  const badge = wikiCard.querySelector('.indexing-new-badge');
  assert.ok(badge, 'wiki card renders the NEW badge (Vue .indexing-new-badge, 样式走 kb-list.td.css)');
  assert.equal(badge.textContent, 'NEW');
  assert.equal(ragCard.querySelector('.indexing-new-badge'), null, 'RAG card carries no NEW badge');
});

test('(c) the description textarea carries the live 0/200 limit counter', async () => {
  await mountPage();
  const textarea = document.body.querySelector<HTMLTextAreaElement>('[data-editor-section="basic"] textarea[placeholder="请输入知识库描述（可选）"]');
  assert.ok(textarea, 'description textarea rendered');

  /* tdesign-react 的 maxlength 走 JS 截断（limitUnicodeMaxLength），不落原生
     属性——与 tdesign-vue-next 的属性渲染差异无视觉影响（台账外，无 DOM
     补偿，playbook §3.4）。计数器是 .t-textarea__limit。 */
  const counter = document.body.querySelector('[data-editor-section="basic"] .t-textarea__limit');
  assert.ok(counter, 'limit counter rendered by t-textarea');
  assert.equal(counter.textContent, '0/200');

  const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLTextAreaElement.prototype, 'value')?.set;
  await act(async () => {
    setValue?.call(textarea, '测试描述');
    textarea.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {});
  assert.equal(counter.textContent, '4/200', 'counter follows the input value');
});

test('(d) the editor dialog renders no wrapping form — Vue KnowledgeBaseEditorModal has zero <form> elements', async () => {
  await mountPage();
  assert.equal(document.body.querySelectorAll('form').length, 0, 'no native form wraps the editor sections (removes the form-in-form hydration error source)');
  const modal = document.body.querySelector('.wk-kb-editor-dialog');
  assert.ok(modal, 'the settings-modal renders (Vue .settings-modal, wk-kb-editor-dialog 为测试锚点类)');
  assert.equal(modal.classList.contains('settings-modal'), true);
  assert.ok(document.body.querySelector('.settings-overlay'), 'overlay teleported to body (Vue Teleport)');
});

test('(d-edit) edit mode: visiting every section (share included) keeps the DOM form-free', async () => {
  await mountPage();
  // Card settings menu → 知识库设置 opens the editor in edit mode. Task 11a：
  // 三点菜单是 t-popup（.more-wrap 触发，内容 portal 到 body 的 .popup-menu）。
  await act(async () => {
    document.body.querySelector<HTMLElement>('.kb-card .more-wrap')?.click();
  });
  await act(async () => {});
  const settingsItem = Array.from(document.body.querySelectorAll('.card-more-popup .popup-menu-item'))
    .find((el) => (el.textContent ?? '') === '设置');
  assert.ok(settingsItem, 'card settings menu item rendered');
  await act(async () => {
    (settingsItem as HTMLElement).click();
  });
  await act(async () => {});
  const navButtons = Array.from(document.body.querySelectorAll<HTMLElement>('[data-guide^="kb-editor-nav-"]'))
    // The datasource section mounts DataSourcesPage, whose client mock surface
    // (datasource types etc.) is out of scope for this anatomy slice.
    .filter((button) => button.dataset.guide !== 'kb-editor-nav-datasource');
  assert.ok(navButtons.length > 0, 'editor sidebar nav rendered');
  assert.ok(navButtons.some((button) => button.dataset.guide === 'kb-editor-nav-share'), 'the share section (the R462 nested-form site) is visitable');
  for (const button of navButtons) {
    await act(async () => {
      button.click();
    });
    await act(async () => {});
    assert.equal(document.body.querySelectorAll('form').length, 0, `section ${button.dataset.guide} renders no form — nested-form hydration errors are structurally impossible`);
  }
});

test('(e) the save button submits the create pipeline via onClick like Vue @click="handleSubmit"', async () => {
  const calls: string[] = [];
  await mountPage(calls);
  const button = document.body.querySelector<HTMLButtonElement>('[data-guide="kb-create-submit"]');
  assert.ok(button, 'save button rendered');
  assert.notEqual(button.type, 'submit', 'Vue footer buttons are plain @click handlers, not type="submit"');

  const nameInput = document.body.querySelector<HTMLInputElement>('[data-guide="kb-create-name"] input');
  assert.ok(nameInput, 'name input rendered on the default basic section (t-input inner input)');
  const setInputValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  await act(async () => {
    setInputValue?.call(nameInput, 'R463 提交结构');
    nameInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {});
  // The model fields live on the models section — navigate there like a user
  // would. R488: they are ModelSelector dropdowns prefilled with the tenant's
  // default chat + embedding models (Vue KnowledgeBaseEditorModal.vue:687-693),
  // so no manual typing is needed — exactly the Vue flow K2 had to fake by
  // hand-typing builtin-llm-mock into a free-text input.
  await act(async () => {
    document.body.querySelector<HTMLButtonElement>('[data-guide="kb-editor-nav-models"]')?.click();
  });
  await act(async () => {});
  const summaryTrigger = document.body.querySelector<HTMLElement>('[data-guide="kb-create-llm"] [role="combobox"]');
  assert.ok(summaryTrigger, 'summary model selector rendered on the models section');
  assert.equal(summaryTrigger.getAttribute('data-value'), 'm-chat-default', 'create prefill picks the default chat model (selectInitialModelId)');
  const embeddingTrigger = document.body.querySelector<HTMLElement>('[data-guide="kb-create-embedding"] [role="combobox"]');
  assert.ok(embeddingTrigger, 'embedding model selector rendered on the models section');
  assert.equal(embeddingTrigger.getAttribute('data-value'), 'm-embed', 'embedding prefill picks the first active Embedding row');
  await act(async () => {
    button.click();
  });
  await act(async () => {});
  assert.deepEqual(calls, ['create'], 'the save pipeline reaches knowledgeBases.create exactly once');
});

test('(f) Enter in the name input does not submit and the input carries no native required attribute', async () => {
  const calls: string[] = [];
  await mountPage(calls);
  const nameInput = document.body.querySelector<HTMLInputElement>('[data-guide="kb-create-name"] input');
  assert.ok(nameInput);
  assert.equal(nameInput.required, false, 'Vue t-input (:165-169) is maxlength-only; blank names are blocked in the JS pipeline');
  /* tdesign-react 的 maxlength 是 JS 截断（不落原生属性，与 vue-next 的
     属性渲染差异无视觉影响）。 */
  assert.equal(nameInput.getAttribute('required'), null);

  const setInputValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  await act(async () => {
    setInputValue?.call(nameInput, 'R463 回车不提交');
    nameInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {
    nameInput.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  });
  await act(async () => {});
  assert.deepEqual(calls, [], 'no implicit Enter submission contract — Vue has no form to submit');
});

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => { resolve = res; });
  return { promise, resolve };
}

function getFooterCancelButton(saveButton: HTMLElement): HTMLElement | undefined {
  return Array.from(saveButton.closest('.settings-footer-actions')?.querySelectorAll<HTMLElement>('button, div[class*="t-button"]') ?? [])
    .find((button) => (button.textContent ?? '') === '取消');
}

test('(g) the save button is disabled (label unchanged) while editor data loads; the cancel button stays enabled', async () => {
  // Vue KnowledgeBaseEditorModal.vue watch(visible) sets loading=true for every
  // open and clears it in loadKBData's finally — the footer mirrors that with
  // :disabled="loading". React's counterpart is editorOptions.loading, armed by
  // loadEditorOptions() on both openCreate and openEdit.
  const settingsGate = deferred<void>();
  const client = makeClient();
  (client.knowledgeBases as unknown as { settings: unknown }).settings = {
    parserEngines: () => settingsGate.promise.then(() => ({ data: [] })),
    storageBackends: () => settingsGate.promise.then(() => ({ data: [] })),
    vectorStores: () => settingsGate.promise.then(() => ({ data: [] })),
  };
  await mountPageWithClient(client);

  const saveButton = document.body.querySelector<HTMLElement>('[data-guide="kb-create-submit"]');
  assert.ok(saveButton, 'save button rendered');
  /* 台账 #7：tdesign-react Button disabled 渲染 div.t-is-disabled（无原生
     disabled 属性）；断言走 classList。 */
  assert.equal(saveButton.classList.contains('t-is-disabled'), true, 'Vue :452 :disabled="loading" — editor data not ready blocks save');
  assert.equal(saveButton.classList.contains('t-is-loading'), false, 'loading-disable is a plain disable, not the saving spinner');
  assert.equal(saveButton.textContent, '创建知识库', 'Vue keeps saveButtonLabel while loading (disable-only, no relabel)');

  const cancelButton = getFooterCancelButton(saveButton);
  assert.ok(cancelButton, 'footer cancel button rendered');
  assert.equal(cancelButton.classList.contains('t-is-disabled'), false, 'Vue cancel (:448-450) has no loading binding — stays clickable while loading');

  await act(async () => { settingsGate.resolve(); });
  await act(async () => {});
  await act(async () => {});
  /* 台账 #7：disabled→enabled 时 tdesign Button 的根标签从 div 换回 button，
     旧元素引用失效——重查。 */
  const saveAfter = document.body.querySelector<HTMLElement>('[data-guide="kb-create-submit"]');
  assert.ok(saveAfter, 'save button still rendered after the gate settles');
  assert.equal(saveAfter.classList.contains('t-is-disabled'), false, 'save re-enables once the editor data settles (Vue loading=false)');
});

test('(h) saving keeps its own disable+aria-busy semantics once loading has settled', async () => {
  const calls: string[] = [];
  const createGate = deferred<void>();
  const client = makeClient(calls);
  (client.knowledgeBases as unknown as { create: unknown }).create = () => createGate.promise.then(() => { calls.push('create'); return { id: 'kb-new' }; });
  await mountPageWithClient(client);

  const saveButton = document.body.querySelector<HTMLElement>('[data-guide="kb-create-submit"]');
  assert.ok(saveButton, 'save button rendered');
  assert.equal(saveButton.classList.contains('t-is-disabled'), false, 'settings settled → loading no longer disables save');

  const setInputValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  const nameInput = document.body.querySelector<HTMLInputElement>('[data-guide="kb-create-name"] input');
  assert.ok(nameInput, 'name input rendered');
  await act(async () => {
    setInputValue?.call(nameInput, 'R466 保存态独立');
    nameInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {
    document.body.querySelector<HTMLButtonElement>('[data-guide="kb-editor-nav-models"]')?.click();
  });
  await act(async () => {});
  // R488: model fields are prefilled dropdowns (default chat + embedding), so
  // the save pipeline is ready without manual model typing.
  const embeddingTrigger = document.body.querySelector<HTMLElement>('[data-guide="kb-create-embedding"] [role="combobox"]');
  const summaryTrigger = document.body.querySelector<HTMLElement>('[data-guide="kb-create-llm"] [role="combobox"]');
  assert.ok(embeddingTrigger && summaryTrigger, 'model selectors rendered on the models section');
  assert.equal(summaryTrigger?.getAttribute('data-value'), 'm-chat-default', 'summary prefilled with the default chat model');
  await act(async () => {});

  await act(async () => {
    saveButton.click();
  });
  await act(async () => {});
  assert.equal(saveButton.classList.contains('t-is-loading'), true, 'saving carries the spinner affordance (Vue :loading="saving")');
  await act(async () => {
    saveButton.click();
  });
  await act(async () => {});

  await act(async () => { createGate.resolve(); });
  await act(async () => {});
  assert.deepEqual(calls, ['create'], 'double-submit guard holds: exactly one create while saving');
  assert.ok(!saveButton.isConnected || !saveButton.classList.contains('t-is-disabled'), 'a successful save closes the dialog (button gone or re-enabled)');
});

// R488 summary_model_id slice (Vue KBModelConfig.vue authority, K2 report
// "placeholder claims optional but the submit validation requires it"):
//   (i) the models section renders Vue's ModelSelector form — LLM row first,
//       Embedding row second, combobox triggers (no free-text inputs), the
//       required star on both labels (LLM always, Embedding while RAG runs),
//       the shared desc lines, and the add-model entry
//   (j) opening create prefills the defaults: the declared-default chat model
//       and the first active embedding model (KnowledgeBaseEditorModal.vue
//       :687-693 selectInitialModelId semantics)
//   (k) a missing summary model is blocked in the UI pipeline BEFORE the
//       request (Vue validateForm :1170-1174 summaryRequired + models jump);
//       with the prefill in place the create payload carries both model ids

async function openCreateAndGoToModels(): Promise<void> {
  await act(async () => {
    document.body.querySelector<HTMLButtonElement>('[data-guide="kb-list-create"]')?.click();
  });
  await act(async () => {});
  await act(async () => {
    document.body.querySelector<HTMLButtonElement>('[data-guide="kb-editor-nav-models"]')?.click();
  });
  await act(async () => {});
}

test('(i) the models section mirrors Vue KBModelConfig: LLM first, combobox model selectors, required stars, descs, add-model entry', async () => {
  await mountPage();
  await openCreateAndGoToModels();

  const section = document.body.querySelector('[data-editor-section="models"]');
  assert.ok(section, 'models section rendered (v-show section shell)');
  const rows = Array.from(section.querySelectorAll('[data-guide="kb-create-llm"], [data-guide="kb-create-embedding"]'));
  assert.equal(rows.length, 2, 'exactly two model rows render');
  assert.equal(rows[0].getAttribute('data-guide'), 'kb-create-llm', 'LLM row renders FIRST (KBModelConfig.vue:10 before :29)');
  assert.equal(rows[1].getAttribute('data-guide'), 'kb-create-embedding', 'Embedding row renders second');

  // Free-text inputs are gone — the Vue ModelSelector is a dropdown.
  assert.equal(document.body.querySelectorAll<HTMLInputElement>('input[data-guide="kb-create-llm"], input[data-guide="kb-create-embedding"]').length, 0,
    'no free-text model inputs remain (K2: placeholder said 可选 while the submit required the value)');

  const llmLabel = rows[0].querySelector('label') ?? rows[0];
  assert.ok((llmLabel.textContent ?? '').includes('LLM 大语言模型'), 'shared llmLabel key');
  assert.ok((llmLabel.textContent ?? '').includes('*'), 'LLM label carries the required star (KBModelConfig.vue:12)');
  assert.ok((rows[0].textContent ?? '').includes('用于总结和摘要的大语言模型'), 'shared llmDesc line renders');

  const embeddingLabel = rows[1].querySelector('label') ?? rows[1];
  assert.ok((embeddingLabel.textContent ?? '').includes('Embedding 嵌入模型'), 'shared embeddingLabel key');
  assert.ok((embeddingLabel.textContent ?? '').includes('*'), 'Embedding label carries the required star while RAG indexing is enabled (KBModelConfig.vue:33)');
  assert.ok((rows[1].textContent ?? '').includes('用于文本向量化的嵌入模型'), 'shared embeddingDesc line renders');

  // Dropdown options + the Vue add-model entry (ModelSelector.vue:46-56).
  const llmTrigger = rows[0].querySelector<HTMLElement>('[role="combobox"]');
  assert.ok(llmTrigger, 'LLM combobox trigger rendered');
  await act(async () => { llmTrigger?.click(); });
  await act(async () => {});
  const listbox = document.body.querySelector('[role="listbox"]');
  assert.ok(listbox, 'LLM dropdown opens');
  const optionTexts = Array.from(listbox?.querySelectorAll('button') ?? []).map((node) => node.textContent ?? '');
  assert.ok(optionTexts.some((text) => text.includes('Default Chat')), 'dropdown lists the tenant chat models');
  assert.ok(optionTexts.some((text) => text.includes('前往全局设置添加模型')), 'add-model entry renders at the dropdown bottom (ModelSelector.vue:54)');
});

test('(j) opening create prefills the default chat model and the first active embedding model', async () => {
  await mountPage();
  await openCreateAndGoToModels();

  const summaryTrigger = document.body.querySelector<HTMLElement>('[data-guide="kb-create-llm"] [role="combobox"]');
  const embeddingTrigger = document.body.querySelector<HTMLElement>('[data-guide="kb-create-embedding"] [role="combobox"]');
  assert.ok(summaryTrigger && embeddingTrigger, 'both selectors rendered');
  assert.equal(summaryTrigger.getAttribute('data-value'), 'm-chat-default', 'is_default chat row wins (selectInitialModelId)');
  assert.ok((summaryTrigger.textContent ?? '').includes('Default Chat'), 'trigger shows the default chat model name');
  assert.equal(embeddingTrigger.getAttribute('data-value'), 'm-embed', 'first active Embedding row prefills');
});

test('(k) a missing summary model blocks the save in the UI pipeline with the models-section jump, and the prefill lands in the payload', async () => {
  const calls: string[] = [];
  const payloads: Array<Record<string, unknown>> = [];
  // Tenant with an embedding model but NO chat model: the embedding prefill
  // succeeds so the save hits the summary-model gate alone (an empty tenant
  // would trip the embedding guard first).
  const client = makeClient(calls, () => [
    { id: 'm-embed', name: 'Mock Embed', display_name: '', type: 'Embedding', status: 'active', is_default: false },
  ]);
  (client.knowledgeBases as unknown as { create: unknown }).create = async (input: Record<string, unknown>) => {
    calls.push('create');
    payloads.push(input);
    return { id: 'kb-new' };
  };
  await mountPageWithClient(client);

  const saveButton = document.body.querySelector<HTMLButtonElement>('[data-guide="kb-create-submit"]');
  assert.ok(saveButton, 'save button rendered');
  const setInputValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  const nameInput = document.body.querySelector<HTMLInputElement>('[data-guide="kb-create-name"] input');
  assert.ok(nameInput, 'name input rendered');
  await act(async () => {
    setInputValue?.call(nameInput, 'R488 无模型租户');
    nameInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {});
  await act(async () => { saveButton.click(); });
  await act(async () => {});

  assert.deepEqual(calls, [], 'no create request leaves the page when the summary model is missing');
  assert.ok((document.body.textContent ?? '').includes('请选择 Summary 模型'), 'Vue summaryRequired message surfaces (KnowledgeBaseEditorModal.vue:1172)');
  const activeNav = document.body.querySelector<HTMLElement>('[data-guide="kb-editor-nav-models"]');
  assert.equal(activeNav?.classList.contains('active'), true, 'the editor jumps to the models section like Vue currentSection="models"');

  // With models available the prefill rides along in the payload.
  const calls2: string[] = [];
  const payloads2: Array<Record<string, unknown>> = [];
  const client2 = makeClient(calls2);
  (client2.knowledgeBases as unknown as { create: unknown }).create = async (input: Record<string, unknown>) => {
    calls2.push('create');
    payloads2.push(input);
    return { id: 'kb-new-2' };
  };
  // Mount the second page on a clean body — otherwise the first app (the
  // summary-blocked one) still owns the query-selected buttons.
  if (mountedRoot) await act(async () => { await mountedRoot?.unmount(); });
  mountedRoot = undefined;
  document.body.replaceChildren();
  await mountPageWithClient(client2);
  const saveButton2 = document.body.querySelector<HTMLButtonElement>('[data-guide="kb-create-submit"]');
  const nameInput2 = document.body.querySelector<HTMLInputElement>('[data-guide="kb-create-name"] input');
  assert.ok(nameInput2, 'second mount rendered the name input');
  await act(async () => {
    setInputValue?.call(nameInput2, 'R488 预填默认模型');
    nameInput2.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {});
  await act(async () => { saveButton2?.click(); });
  await act(async () => {});
  assert.deepEqual(calls2, ['create'], 'prefilled defaults let the create through');
  assert.equal(payloads2[0]?.summary_model_id, 'm-chat-default', 'payload carries the prefilled summary model');
  assert.equal(payloads2[0]?.embedding_model_id, 'm-embed', 'payload carries the prefilled embedding model');
});
