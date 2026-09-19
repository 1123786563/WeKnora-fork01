import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

// --- harness (same pattern as SandboxSettingsPanel.test.tsx) -------------------------

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { AgentEditorModal } = await import('./AgentEditorModal.tsx');
const { makeEditorT } = await import('./agent-editor.ts');
import type { WeKnoraClient } from '@weknora/api-client';

const t = makeEditorT('zh-CN');

// --- fixtures ------------------------------------------------------------------------

const MODELS = [
  { id: 'm-chat', name: 'GPT-4o', type: 'KnowledgeQA' },
  { id: 'm-rerank', name: 'BGE Reranker', type: 'Rerank' },
];

const KBS = [
  { id: 'kb-1', name: '产品文档', type: 'document', knowledge_count: 12, indexing_strategy: { vector_enabled: true, keyword_enabled: true } },
  { id: 'kb-2', name: 'FAQ 库', type: 'faq', chunk_count: 4 },
];

const SANDBOXES = [
  { id: 'sb-1', name: 'Docker dev', description: '', sandbox_type: 'docker', config: { docker: { image: 'python:3.12' } }, created_at: '', updated_at: '' },
];

const CATALOG = [
  { id: 'cat-1', name: 'PDF skill', description: 'Read PDFs', installations: [{ skillId: 'x', sandboxConfigId: 'sb-1', status: 'ready', enabled: true }] },
  { id: 'cat-2', name: 'Pending skill', installations: [] },
];

const EDIT_AGENT = {
  id: 'a-1',
  name: '我的助手',
  description: '帮我写周报',
  is_builtin: false,
  config: {
    agent_mode: 'quick-answer',
    system_prompt: '你是周报助手',
    context_template: '上下文：{{query}}',
    model_id: 'm-chat',
    kb_selection_mode: 'selected',
    knowledge_bases: ['kb-1'],
    multi_turn_enabled: true,
  },
};

interface RoutedRequest { method?: string; path?: string; body?: unknown }

// MBTI catalog fixture — INTJ carries mixed-pole dimensions so the dominant
// label sides can be asserted (ei/jp right-vs-left pole dominance mix).
const mbtiProfile = (code: string) => ({
  code, name_zh: '类型' + code, name_en: 'Type ' + code, nickname_zh: '昵称' + code,
  summary_zh: '中文综述', summary_en: 'English summary',
  descriptors_zh: '描述', descriptors_en: 'descriptor',
  dimensions: code === 'INTJ'
    ? { ei: { pole: 'I', percent: 85 }, sn: { pole: 'S', percent: 62 }, tf: { pole: 'F', percent: 71 }, jp: { pole: 'J', percent: 55 } }
    : { ei: { pole: 'E', percent: 60 }, sn: { pole: 'N', percent: 55 }, tf: { pole: 'T', percent: 65 }, jp: { pole: 'P', percent: 70 } },
  behavior: {
    answer_style: '', casual_chat: '', conflict: '', creativity: '', emotion: '', planning: '',
    answer_style_zh: '', casual_chat_zh: '', conflict_zh: '', creativity_zh: '', emotion_zh: '', planning_zh: '',
  },
  color: '#123456', symbol: '*',
});
const MBTI_TYPES = ['INTJ', 'INTP', 'ENTJ', 'ENTP', 'INFJ', 'INFP', 'ENFJ', 'ENFP', 'ISTJ', 'ISFJ', 'ESTJ', 'ESFJ', 'ISTP', 'ISFP', 'ESTP', 'ESFP'].map(mbtiProfile);

// Test fixture: a 4-question subset of the 28-question API shape is enough to
// drive the all-answered submit gate (the gate is count-relative, not 28-hardcoded).
const MBTI_QUESTIONS = [1, 2, 3, 4].map((id) => ({
  id, dimension: 'EI' as const, a_pole: 'E', b_pole: 'I',
  question_zh: '问题' + id, option_a_zh: '选项A' + id, option_b_zh: '选项B' + id,
  question_en: 'Question ' + id, option_a_en: 'Option A' + id, option_b_en: 'Option B' + id,
}));
const MBTI_SCORE = {
  code: 'ENTP',
  dimensions: { ei: { dominant: 'E', percent: 72 }, sn: { dominant: 'N', percent: 58 }, tf: { dominant: 'T', percent: 64 }, jp: { dominant: 'P', percent: 66 } },
  profile: mbtiProfile('ENTP'),
};

function makeClient(options: { createReject?: Error } = {}): { client: WeKnoraClient; requests: RoutedRequest[]; mbtiSubmits: Array<Record<string, 'A' | 'B'>> } {
  const requests: RoutedRequest[] = [];
  const mbtiSubmits: Array<Record<string, 'A' | 'B'>> = [];
  const client = {
    mbti: {
      types: async () => MBTI_TYPES,
      questions: async () => MBTI_QUESTIONS,
      submit: async (answers: Record<string, 'A' | 'B'>) => {
        mbtiSubmits.push(answers);
        return MBTI_SCORE;
      },
    },
    configuration: {
      models: { list: async () => MODELS },
      agents: {
        create: async (input: Record<string, unknown>) => {
          requests.push({ method: 'POST', path: '/api/v1/agents', body: input });
          if (options.createReject) throw options.createReject;
          return { id: 'new-1', name: (input as { name?: string }).name, is_builtin: false, config: (input as { config?: unknown }).config };
        },
        update: async (id: string, input: Record<string, unknown>) => {
          requests.push({ method: 'PUT', path: '/api/v1/agents/' + id, body: input });
          return { id, ...input };
        },
      },
      skills: {
        catalog: {
          list: async () => CATALOG,
          install: async () => ({ installs: {} }),
        },
      },
    },
    knowledgeBases: { list: async () => KBS },
    sandboxConfigurations: { list: async () => ({ items: SANDBOXES, workspaceScriptsDisabled: false }) },
    settings: { webSearch: { providers: { list: async () => [] } } },
  };
  return { client: client as unknown as WeKnoraClient, requests, mbtiSubmits };
}

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

async function mountModal(props: { client: WeKnoraClient; mode?: 'create' | 'edit'; agent?: Record<string, unknown> | null; initialSection?: string; initialHighlightField?: string; readOnly?: boolean; onClose?: () => void }) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  const { client, mode = 'create', agent = null, onClose = () => {}, ...initial } = props;
  await act(async () => {
    mountedRoot?.render(React.createElement(AgentEditorModal, { open: true, mode, client, t, onClose, ...initial, ...(agent ? { agent } : {}) }));
  });
  // let the async dependency load + form hydration settle
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
  return container;
}

test('edit deep-link inputs select the requested section and highlight', async () => {
  const { client } = makeClient();
  const agentMode = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config, agent_mode: 'smart-reasoning' } };
  const root = await mountModal({ client, mode: 'edit', agent: agentMode, initialSection: 'sandbox', initialHighlightField: 'allowed_tools' });
  assert.ok($('[data-section-key="skills"]', root), 'sandbox alias should select skills');
  assert.equal($('[data-section-key="skills"]', root)?.className.includes('text-[var(--td-brand-color'), true);
  // The tools highlight is only rendered when its section is active, so verify
  // the valid field is retained when the deep-link selects that section.
  const toolsRoot = await mountModal({ client, mode: 'edit', agent: agentMode, initialSection: 'tools', initialHighlightField: 'allowed_tools' });
  assert.equal($('[data-editor-section="tools"]', toolsRoot)?.getAttribute('data-highlighted-field'), 'allowed_tools');
});

test('read-only edit hides the save mutation control', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'edit', agent: EDIT_AGENT, readOnly: true });
  assert.equal($('[data-editor-save]', root), null);
  // quick-answer never renders persona (capability assembly is bypassed), so
  // the personalization nav item is gated like tools/skills
  assert.equal($('[data-section-key="personalization"]', root), null);
  assert.equal($('[data-section-key="tools"]', root), null);
});

test('personalization section renders the 16-type grid and dominant-side percent labels', async () => {
  const { client } = makeClient();
  // persona is only offered in smart-reasoning mode: the quick-answer pipeline
  // never renders it, so the nav section is gated by agent-mode
  const agent = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config, agent_mode: 'smart-reasoning', persona_mbti: 'INTJ', persona_style: '语气轻松' } };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'personalization');
  // flush the client.mbti.types() resolution
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });

  // 16 type cards + the None card that clears persona_mbti
  const codes = $$(document.body, '[data-mbti-code]').map((el) => el.getAttribute('data-mbti-code'));
  assert.equal(codes.length, 17);
  assert.ok(codes.includes(''));
  assert.equal($('[data-mbti-code="INTJ"]', document.body)?.getAttribute('aria-pressed'), 'true');

  // dimension rows: [left span, track (aria-label), right span]
  const dims = $('[data-mbti-dimensions="INTJ"]', document.body);
  assert.ok(dims, 'dimension block missing for the selected type');
  const rows = Array.from(dims.children);
  assert.equal(rows.length, 4);
  const leftOf = (row: number) => rows[row]!.children[0]!.textContent;
  const rightOf = (row: number) => rows[row]!.children[2]!.textContent;
  const trackOf = (row: number) => rows[row]!.children[1]!.getAttribute('aria-label');
  // percent is the DOMINANT pole's strength — the dominant side prints it verbatim,
  // whichever side that is (regression: right-dominant used to print 100-percent)
  assert.equal(leftOf(0), 'E');
  assert.equal(rightOf(0), 'I 85%');
  assert.equal(trackOf(0), 'E / I: I 85%');
  assert.equal(leftOf(1), 'S 62%');
  assert.equal(rightOf(1), 'N');
  assert.equal(leftOf(2), 'T');
  assert.equal(rightOf(2), 'F 71%');
  assert.equal(leftOf(3), 'J 55%');
  assert.equal(rightOf(3), 'P');

  // style textarea mirrors the stored persona_style; take-test opens the test modal (Task 9)
  const styleEl = $('#wk-ae-persona-style', document.body) as HTMLTextAreaElement | null;
  assert.ok(styleEl, 'persona_style textarea missing');
  assert.equal(styleEl.value, '语气轻松');
  const takeTest = $('[data-mbti-take-test]', document.body) as HTMLButtonElement | null;
  assert.ok(takeTest, 'take-the-test button missing');
  assert.equal(takeTest.disabled, false);
});

// --- MBTI test modal (Task 9) ------------------------------------------------------------

test('test modal: intro disclaimer, all-answered submit gate, submit payload, apply writes persona_mbti', async () => {
  const { client, requests, mbtiSubmits } = makeClient();
  const agent = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config, agent_mode: 'smart-reasoning' } };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'personalization');
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });

  await click(root, '[data-mbti-take-test]');
  // the dialog portals to document.body; intro carries the for-entertainment disclaimer
  assert.ok($('[data-mbti-stage="intro"]', document.body), 'intro stage rendered');
  assert.match($('[data-mbti-disclaimer]', document.body)!.textContent!, /仅供娱乐/);

  await click(document.body, '[data-mbti-start]');
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
  assert.equal($$('[data-mbti-q]', document.body).length, 4, 'all questions rendered');
  assert.match($('[data-mbti-progress]', document.body)!.textContent!, /已答 0 \/ 4/);
  assert.equal(($('[data-mbti-submit]', document.body) as HTMLButtonElement).disabled, true, 'submit disabled with 0/4');

  await click(document.body, '[data-mbti-q="1"] [data-mbti-option="A"]');
  await click(document.body, '[data-mbti-q="2"] [data-mbti-option="B"]');
  await click(document.body, '[data-mbti-q="3"] [data-mbti-option="A"]');
  assert.match($('[data-mbti-progress]', document.body)!.textContent!, /已答 3 \/ 4/);
  assert.equal(($('[data-mbti-submit]', document.body) as HTMLButtonElement).disabled, true, 'submit still disabled with 3/4');

  await click(document.body, '[data-mbti-q="4"] [data-mbti-option="B"]');
  assert.equal(($('[data-mbti-submit]', document.body) as HTMLButtonElement).disabled, false, 'submit enabled once every question is answered');
  await click(document.body, '[data-mbti-submit]');
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });

  // the full answers map rides the submit call (question-id string keys)
  assert.deepEqual(mbtiSubmits, [{ 1: 'A', 2: 'B', 3: 'A', 4: 'B' }]);

  // result stage: code, four dominant-pole bars (Task 8 label semantics), apply
  assert.ok($('[data-mbti-stage="result"]', document.body), 'result stage rendered');
  assert.equal($('[data-mbti-result-code]', document.body)?.textContent, 'ENTP');
  const bars = $('[data-mbti-result-bars]', document.body)!;
  assert.equal(bars.children.length, 4);
  assert.equal(bars.children[0]!.children[1]!.getAttribute('aria-label'), 'E / I: E 72%');
  assert.equal(bars.children[3]!.children[1]!.getAttribute('aria-label'), 'J / P: P 66%');

  await click(document.body, '[data-mbti-apply]');
  assert.equal($('[data-mbti-stage="result"]', document.body), null, 'apply closes the modal');
  assert.equal($('[data-mbti-code="ENTP"]', document.body)?.getAttribute('aria-pressed'), 'true', 'grid selection follows the applied code');
  // the applied code reaches the save payload through persona_mbti
  await click(root, '[data-editor-save]');
  const savedConfig = (requests[0]!.body as { config: Record<string, unknown> }).config;
  assert.equal(savedConfig.persona_mbti, 'ENTP');
});

test('test modal: Escape closes only the dialog and the editor stays open', async () => {
  const { client, requests } = makeClient();
  const agent = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config, agent_mode: 'smart-reasoning' } };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'personalization');
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
  await click(root, '[data-mbti-take-test]');
  assert.ok($('[data-mbti-stage="intro"]', document.body), 'dialog open');

  await act(async () => {
    document.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  });
  assert.equal($('[data-mbti-stage="intro"]', document.body), null, 'test dialog closed on Escape');
  assert.ok($('[data-testid="agent-editor-modal"]', root), 'editor stays open behind it');
  assert.equal(requests.length, 0, 'closing never saves');
});

const asNode = (a: ParentNode | string, b: ParentNode | string): ParentNode => (typeof a === 'string' ? (b as ParentNode) : (a as ParentNode));
const asSelector = (a: ParentNode | string, b: ParentNode | string): string => (typeof a === 'string' ? a : (b as string));
const $ = (a: ParentNode | string, b: ParentNode | string): Element | null => asNode(a, b).querySelector(asSelector(a, b));
const $$ = (a: ParentNode | string, b: ParentNode | string): Element[] => Array.from(asNode(a, b).querySelectorAll(asSelector(a, b)));

type FormEl = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

async function setValue(root: ParentNode, selector: string, value: string) {
  const el = $(root, selector) as FormEl | null;
  assert.ok(el, 'element missing for selector ' + selector);
  await act(async () => {
    const prototype = el instanceof HTMLSelectElement ? HTMLSelectElement.prototype
      : el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    const setter = Object.getOwnPropertyDescriptor(prototype, 'value')?.set;
    setter?.call(el, value);
    el.dispatchEvent(new window.Event('input', { bubbles: true }));
    el.dispatchEvent(new window.Event('change', { bubbles: true }));
  });
}

async function click(root: ParentNode, selector: string) {
  const el = $(root, selector);
  assert.ok(el, 'element missing for selector ' + selector);
  await act(async () => {
    el.dispatchEvent(new window.MouseEvent('click', { bubbles: true, cancelable: true }));
  });
}

async function checkRadio(section: ParentNode, selector: string) {
  const el = $(section, selector) as HTMLInputElement;
  assert.ok(el, 'radio missing for ' + selector);
  await act(async () => {
    // React wires radio onChange through click; a plain change event is ignored
    el.click();
    el.dispatchEvent(new window.Event('change', { bubbles: true }));
  });
}

async function checkCheckbox(section: ParentNode, selector: string) {
  const el = $(section, selector) as HTMLInputElement;
  assert.ok(el, 'checkbox missing for ' + selector);
  await act(async () => {
    el.click();
    el.dispatchEvent(new window.Event('change', { bubbles: true }));
  });
}

/** Navigate to a section through the rail. */
async function goto(root: ParentNode, key: string) {
  await click(root, '[data-section-key="' + key + '"]');
}

// --- create mode: rail + sections ------------------------------------------------------

test('create mode renders the grouped section rail and the create footer button', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  const html = document.body.innerHTML;
  console.log('DEBUG root:', typeof root, (root as unknown as { constructor?: { name?: string } }).constructor?.name, 'qs:', typeof (root as unknown as { querySelector?: unknown }).querySelector);
  const keys = $$(document.body, '[data-section-key]').map((el) => el.getAttribute('data-section-key'));
  assert.ok(keys.includes('basic'));
  assert.ok(keys.includes('prompts'));
  assert.ok(keys.includes('model'));
  assert.ok(keys.includes('conversation'));
  assert.ok(keys.includes('knowledge'));
  assert.ok(keys.includes('websearch'));
  assert.ok(keys.includes('retrieval'), 'kb_selection_mode=all exposes the retrieval section');
  assert.ok(keys.includes('tools'), 'default smart-reasoning mode exposes tools');
  assert.ok(keys.includes('skills'), 'default smart-reasoning mode exposes skills');
  assert.match(html, /基础/);
  assert.match(html, /知识检索/);
  assert.match(html, /能力扩展/);
  assert.match(html, /创建智能体/, 'sidebar title');
  assert.match($('[data-editor-save]', root)!.textContent!, /创建智能体/);
  assert.match($('[data-editor-section="basic"]', root)!.textContent!, /运行模式/);
  assert.match($('[data-editor-section="basic"]', root)!.textContent!, /快速问答/); // agent.type.normal (Vue:108)
});

test('editor close control uses the Vue close accessible name', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  const closeButton = $('button[aria-label="关闭"]', root);
  assert.ok(closeButton, 'the top-right close control should announce close, not cancel');
});

// --- create mode: validation blocks submit with per-field errors ----------------------

test('empty submit shows per-field required errors, jumps sections and fires no request', async () => {
  const { client, requests } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await click(root, '[data-editor-save]');
  const nameError = $('[data-field-error="name"]', root);
  assert.ok(nameError, 'name error rendered under the field');
  assert.match(nameError!.textContent!, /请输入智能体名称/);
  assert.equal(requests.length, 0, 'no POST before the form is valid');
  // fix the name; the next save jumps to prompts where the system prompt is required
  await setValue(root, '[data-field="name"]', '有名字了');
  await click(root, '[data-editor-save]');
  assert.ok($('[data-editor-section="prompts"]', root), 'validation jumped to the prompts section');
  const promptError = $('[data-field-error="system_prompt"]', root);
  assert.ok(promptError);
  assert.match(promptError!.textContent!, /请输入系统提示词/);
  await setValue(root, '[data-field="system_prompt"]', '你是助手');
  await click(root, '[data-editor-save]');
  const modelError = $('[data-field-error="model_id"]', root);
  assert.ok(modelError);
  assert.match(modelError!.textContent!, /请选择模型/);
  assert.ok($('[data-editor-section="model"]', root), 'validation jumped to the model section');
  assert.equal(requests.length, 0, 'no POST before the form is valid');
});

// --- create mode: valid save posts the Vue payload and stays open (post-create) -------

test('valid create posts the Vue payload shape then shows the post-create hint', async () => {
  const { client, requests } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await setValue(root, '[data-field="name"]', '新助手');
  await setValue(root, '[data-field="description"]', '说明');
  await goto(root, 'prompts');
  await setValue(root, '[data-field="system_prompt"]', '你是助手');
  await goto(root, 'model');
  await setValue(root, '[data-field="model_id"]', 'm-chat');
  await click(root, '[data-editor-save]');

  assert.equal(requests.length, 1);
  const request = requests[0]!;
  assert.equal(request.method, 'POST');
  assert.equal(request.path, '/api/v1/agents');
  const payload = request.body as Record<string, unknown>;
  assert.equal(payload.name, '新助手');
  assert.equal(payload.description, '说明');
  assert.equal(payload.is_builtin, false);
  const config = payload.config as Record<string, unknown>;
  assert.equal(config.agent_mode, 'smart-reasoning');
  assert.equal(config.system_prompt, '你是助手');
  assert.equal(config.model_id, 'm-chat');
  assert.equal(config.kb_selection_mode, 'all');
  assert.equal(config.temperature, 0.7);
  assert.equal(config.memory_enabled, true);
  assert.ok('history_turns' in config, 'config carries the full default set');
  assert.ok($('[data-post-create]', root), 'post-create hint note rendered');
  assert.match($('[data-editor-save]', root)!.textContent!, /保存并关闭/);
});

// --- edit mode: hydrate + PUT payload --------------------------------------------------

test('edit mode hydrates the agent and PUTs the merged config on save', async () => {
  const { client, requests } = makeClient();
  const root = await mountModal({ client, mode: 'edit', agent: EDIT_AGENT });
  const nameInput = $('[data-field="name"]', root) as HTMLInputElement;
  assert.equal(nameInput.value, '我的助手');
  assert.match(document.body.innerHTML, /编辑智能体/);
  assert.match($('[data-editor-save]', root)!.textContent!, /保存并关闭/);
  await click(root, '[data-editor-save]');

  assert.equal(requests.length, 1);
  const request = requests[0]!;
  assert.equal(request.method, 'PUT');
  assert.equal(request.path, '/api/v1/agents/a-1');
  const payload = request.body as Record<string, unknown>;
  assert.equal(payload.name, '我的助手');
  const config = payload.config as Record<string, unknown>;
  assert.equal(config.agent_mode, 'quick-answer');
  assert.equal(config.system_prompt, '你是周报助手');
  assert.equal(config.context_template, '上下文：{{query}}');
  assert.equal(config.model_id, 'm-chat');
  assert.equal(config.kb_selection_mode, 'selected');
  assert.deepEqual(config.knowledge_bases, ['kb-1']);
  assert.equal(config.memory_enabled, true);
  assert.equal(config.history_turns, 5);
});

// --- ESC / cancel / overlay close ------------------------------------------------------

test('Escape key, cancel button and overlay click all close without saving', async () => {
  const { client, requests } = makeClient();
  let closed = 0;
  const root = await mountModal({ client, mode: 'edit', agent: EDIT_AGENT, onClose: () => { closed += 1; } });

  await act(async () => {
    document.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  });
  assert.equal(closed, 1, 'Escape closes the modal');

  await click(root, '[data-editor-cancel]');
  assert.equal(closed, 2, 'cancel button closes the modal');

  const overlay = $('[data-editor-overlay]', root)!;
  await act(async () => {
    overlay.dispatchEvent(new window.MouseEvent('click', { bubbles: true }));
  });
  assert.equal(closed, 3, 'overlay click closes the modal');
  assert.equal(requests.length, 0, 'closing never saves');
});

// --- KB selection toggling --------------------------------------------------------------

test('KB selection: none clears knowledge_bases, selected renders groups and checkbox toggles', async () => {
  const { client, requests } = makeClient();
  const root = await mountModal({ client, mode: 'edit', agent: EDIT_AGENT });
  await click(root, '[data-section-key="knowledge"]');
  const section = $('[data-editor-section="knowledge"]', root)!;
  assert.ok($('input[name="kb-mode"][value="all"]', section));
  assert.ok($('input[name="kb-mode"][value="selected"]', section));
  assert.ok($('input[name="kb-mode"][value="none"]', section));
  assert.ok(section.textContent!.includes('我的知识库'));
  const kb1 = $('input[data-kb-id="kb-1"]', section) as HTMLInputElement;
  assert.equal(kb1.checked, true, 'stored knowledge_bases hydrate as checked');
  const faq = $('input[data-kb-id="kb-2"]', section) as HTMLInputElement;
  assert.equal(faq.checked, false);

  await checkRadio(section, 'input[name="kb-mode"][value="none"]');
  await click(root, '[data-editor-save]');
  const nonePayload = (requests[0]!.body as { config: Record<string, unknown> }).config;
  assert.equal(nonePayload.kb_selection_mode, 'none');
  assert.deepEqual(nonePayload.knowledge_bases, []);

  await checkRadio(section, 'input[name="kb-mode"][value="selected"]');
  await checkCheckbox(section, 'input[data-kb-id="kb-2"]');
  await click(root, '[data-editor-save]');
  const selectedPayload = (requests[1]!.body as { config: Record<string, unknown> }).config;
  assert.equal(selectedPayload.kb_selection_mode, 'selected');
  assert.deepEqual(selectedPayload.knowledge_bases, ['kb-2']);
});

// --- duplicate submit protection ---------------------------------------------------------

test('double-clicking save only submits once while the request is in flight', async () => {
  let resolveCreate!: (value: unknown) => void;
  const { client, requests } = makeClient();
  const slow = client as WeKnoraClient & { configuration: { agents: { create: (input: Record<string, unknown>) => Promise<unknown> } } };
  slow.configuration.agents.create = (input: Record<string, unknown>) => {
    requests.push({ method: 'POST', path: '/api/v1/agents', body: input });
    return new Promise((resolve) => { resolveCreate = resolve; });
  };
  const root = await mountModal({ client, mode: 'create' });
  await setValue(root, '[data-field="name"]', 'x');
  await goto(root, 'prompts');
  await setValue(root, '[data-field="system_prompt"]', 's');
  await goto(root, 'model');
  await setValue(root, '[data-field="model_id"]', 'm-chat');
  await click(root, '[data-editor-save]');
  await click(root, '[data-editor-save]');
  assert.equal(requests.length, 1, 'in-flight save is not resubmitted');
  await act(async () => { resolveCreate({ id: 'new-2', name: 'x', is_builtin: false, config: {} }); });
  assert.equal(requests.length, 1);
});
