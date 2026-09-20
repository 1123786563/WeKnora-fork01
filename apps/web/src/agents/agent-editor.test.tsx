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
const { resetAgentEditorResourcesCache } = await import('./agent-editor-resources.ts');
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
  { id: 'cat-2', name: 'Pending skill', description: '', installations: [] },
];

// R486 — GET /system/parser-engines fixture mirroring ParserEngineInfo
// (capitalized Go field names): builtin covers every chat-attachment family,
// anydoc covers the office families, mineru is registered but unavailable.
const PARSER_ENGINES = [
  {
    Name: 'builtin',
    Description: 'DocReader 内置解析引擎',
    FileTypes: ['pdf', 'docx', 'doc', 'xlsx', 'xls', 'pptx', 'ppt', 'epub', 'mhtml', 'csv', 'md', 'markdown', 'txt', 'json', 'jpg', 'jpeg', 'png', 'gif', 'bmp', 'tiff', 'webp'],
    Available: true,
  },
  { Name: 'anydoc', Description: '进程内 Office 文档解析', FileTypes: ['docx', 'doc', 'xlsx', 'xls', 'pptx', 'ppt', 'epub', 'mhtml'], Available: true },
  { Name: 'mineru', Description: 'MinerU 自部署服务', FileTypes: ['pdf'], Available: false },
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

// Sub-agent catalog fixture (Task 8): two divisions, three entries — the
// code-reviewer entry doubles as the pre-installed role of the EDIT agents.
const SUBAGENT_CATALOG = {
  divisions: [
    { slug: 'pm', label: '产品', icon: 'box', color: '#1050d8', count: 2 },
    { slug: 'dev', label: '研发', icon: 'code', color: '#2ba471', count: 1 },
  ],
  total: 3,
  entries: [
    { slug: 'product-manager', division: 'pm', name_zh: '产品经理', name_en: 'Product Manager', emoji: '📦', color: '#1050d8', installed: true },
    { slug: 'ux-researcher', division: 'pm', name_zh: '用户研究员', name_en: 'UX Researcher', emoji: '🔍', color: '#1050d8', installed: false },
    { slug: 'code-reviewer', division: 'dev', name_zh: '代码评审员', name_en: 'Code Reviewer', emoji: '🧑‍💻', color: '#2ba471', installed: true },
  ],
};

interface SubagentCall { kind: 'install' | 'remove'; agentId: string; slug: string; locale?: string }

// R491 — editor runtime catalogs (GET /agents/type-presets, /agents/placeholders,
// /tenants/kv/prompt-templates). undefined = endpoint absent on the stub client
// -> the editor degrades to the vendored static catalogs (R485/R486 behaviour);
// null = endpoint present but failing; a value = a successful payload.
function makeClient(options: {
  createReject?: Error;
  initialSubagents?: string[];
  retrievalConfig?: Record<string, unknown> | null;
  parserEngines?: Array<Record<string, unknown>> | null;
  typePresets?: Array<Record<string, unknown>> | null;
  placeholders?: Record<string, Array<Record<string, unknown>>> | null;
  promptTemplates?: Record<string, Array<Record<string, unknown>>> | null;
} = {}): { client: WeKnoraClient; requests: RoutedRequest[]; mbtiSubmits: Array<Record<string, 'A' | 'B'>>; subagentCalls: SubagentCall[] } {
  const requests: RoutedRequest[] = [];
  const mbtiSubmits: Array<Record<string, 'A' | 'B'>> = [];
  const subagentCalls: SubagentCall[] = [];
  // server-side installed list the install/remove endpoints mutate + return;
  // seeded to match the agent fixture so responses mirror the stored config
  let installedSubagents = options.initialSubagents ?? ['code-reviewer'];
  const client = {
    mbti: {
      types: async () => MBTI_TYPES,
      questions: async () => MBTI_QUESTIONS,
      submit: async (answers: Record<string, 'A' | 'B'>) => {
        mbtiSubmits.push(answers);
        return MBTI_SCORE;
      },
    },
    subagents: {
      catalog: async () => SUBAGENT_CATALOG,
      get: async (slug: string) => {
        const entry = SUBAGENT_CATALOG.entries.find((row) => row.slug === slug)!;
        return { ...entry, vibe: '认真', tools_raw: 'knowledge_search', body_zh: '**职责**：负责' + entry.name_zh, body_en: 'Owns ' + entry.name_en };
      },
      listAgent: async () => [...installedSubagents],
      install: async (agentId: string, slug: string, locale?: string) => {
        subagentCalls.push({ kind: 'install', agentId, slug, ...(locale === undefined ? {} : { locale }) });
        if (!installedSubagents.includes(slug)) installedSubagents = [...installedSubagents, slug];
        return [...installedSubagents];
      },
      remove: async (agentId: string, slug: string) => {
        subagentCalls.push({ kind: 'remove', agentId, slug });
        installedSubagents = installedSubagents.filter((row) => row !== slug);
        return [...installedSubagents];
      },
    },
    configuration: {
      models: { list: async () => MODELS },
      agents: {
        typePresets: options.typePresets === undefined
          ? undefined
          : async () => (options.typePresets === null ? Promise.reject(new Error('no presets')) : { data: options.typePresets, success: true }),
        placeholders: options.placeholders === undefined
          ? undefined
          : async () => (options.placeholders === null ? Promise.reject(new Error('no placeholders')) : { data: options.placeholders, success: true }),
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
      mcp: {
        list: async () => [
          { id: 'mcp-a', name: '服务A', enabled: true },
          { id: 'mcp-b', name: '服务B', enabled: false },
        ],
      },
    },
    knowledgeBases: {
      list: async () => KBS,
      settings: {
        // R486 parser-rules editor dep (GET /system/parser-engines); null
        // simulates the endpoint being absent so the block degrades to the
        // no-engine hint instead of blanking the section.
        parserEngines: async () =>
          options.parserEngines === null
            ? Promise.reject(new Error('no engines'))
            : { data: options.parserEngines ?? PARSER_ENGINES },
      },
    },
    sandboxConfigurations: { list: async () => ({ items: SANDBOXES, workspaceScriptsDisabled: false }) },
    settings: {
      webSearch: { providers: { list: async () => [] } },
      storage: { legacy: { status: async () => ({ storage_engine_status: [{ name: 'local', available: true }, { name: 'minio', available: false }] }) } },
      // R486 D9 tenant retrieval-config (GET /tenants/kv/retrieval-config);
      // undefined = endpoint absent -> built-in defaults stay in place.
      retrieval: {
        get: async () =>
          options.retrievalConfig === undefined
            ? Promise.reject(new Error('no retrieval config'))
            : options.retrievalConfig,
      },
      // R491 prompt-template catalog (GET /tenants/kv/prompt-templates);
      // undefined = absent -> static builtin template list stays in place.
      promptTemplates: options.promptTemplates === undefined
        ? undefined
        : {
            get: async () =>
              options.promptTemplates === null
                ? Promise.reject(new Error('no prompt templates'))
                : { data: options.promptTemplates, success: true },
          },
    },
  };
  return { client: client as unknown as WeKnoraClient, requests, mbtiSubmits, subagentCalls };
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

// --- subagents section (Task 8) ----------------------------------------------------------

/** Smart-reasoning agent fixture with the given delegation slugs installed. */
const subagentAgent = (subagents: string[]) => ({
  ...EDIT_AGENT,
  config: { ...EDIT_AGENT.config, agent_mode: 'smart-reasoning' as const, subagents },
});

/** Navigate to the subagents section and flush the catalog load. */
async function mountSubagents(agent: Record<string, unknown>, client: WeKnoraClient) {
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'subagents');
  // flush the client.subagents.catalog() resolution
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
  return root;
}

test('subagents section renders the installed list, catalog cards, division chips and zh names', async () => {
  const { client } = makeClient();
  const root = await mountSubagents(subagentAgent(['code-reviewer']), client);

  // installed row from config.subagents (slug fallback name → catalog entry)
  const row = $('[data-subagent-installed-row="code-reviewer"]', root);
  assert.ok(row, 'installed row missing for config.subagents slug');
  assert.match(row!.textContent!, /代码评审员/, 'installed row shows the zh catalog name');

  // catalog cards from the mock: 3 entries, zh name at the zh locale
  const slugs = $$(root, '[data-subagent-slug]').map((el) => el.getAttribute('data-subagent-slug'));
  assert.deepEqual(slugs, ['product-manager', 'ux-researcher', 'code-reviewer']);
  assert.match($('[data-subagent-slug="product-manager"]', root)!.textContent!, /产品经理/);

  // division chips: all + the two catalog divisions with counts
  const chips = $$('[data-subagent-division]', root).map((el) => el.getAttribute('data-subagent-division'));
  assert.deepEqual(chips, ['', 'pm', 'dev']);
  assert.match($('[data-subagent-division="pm"]', root)!.textContent!, /产品 · 2/);

  // installed checkmark only on the configured slug
  assert.ok($('[data-subagent-installed-tag="code-reviewer"]', root));
  assert.equal($('[data-subagent-installed-tag="product-manager"]', root), null);
  // install button only for entries the agent does not have
  assert.ok($('[data-subagent-install="product-manager"]', root));
  assert.equal($('[data-subagent-install="code-reviewer"]', root), null);

  // detail peek: clicking the name expands the role body rendered as markdown
  await click(root, '[data-subagent-name="product-manager"]');
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });
  const detail = $('[data-subagent-detail="product-manager"]', root);
  assert.ok(detail, 'expanded entry renders the detail block');
  assert.match(detail!.textContent!, /负责产品经理/, 'zh body at the zh locale through renderChatMarkdown');
  await click(root, '[data-subagent-name="product-manager"]');
  assert.equal($('[data-subagent-detail="product-manager"]', root), null, 'second click collapses');
});

test('subagents nav item is gated by agent mode like tools/skills', async () => {
  const { client } = makeClient();
  // EDIT_AGENT is quick-answer — delegation never registers the tool
  const root = await mountModal({ client, mode: 'edit', agent: EDIT_AGENT });
  assert.equal($('[data-section-key="subagents"]', root), null);
  const agentRoot = await mountSubagents(subagentAgent([]), client);
  assert.ok($('[data-section-key="subagents"]', agentRoot), 'smart-reasoning exposes the subagents section');
});

test('subagents empty installed list shows the delegation-off hint', async () => {
  const { client } = makeClient();
  const root = await mountSubagents(subagentAgent([]), client);
  const hint = $('[data-subagent-empty-hint]', root);
  assert.ok(hint, 'delegation-off hint missing for an empty installed list');
  assert.match(hint!.textContent!, /委派能力处于关闭状态/);
});

test('installing a catalog role patches config.subagents from the response and reaches the save payload', async () => {
  const { client, requests, subagentCalls } = makeClient({ initialSubagents: [] });
  const root = await mountSubagents(subagentAgent([]), client);

  await click(root, '[data-subagent-install="product-manager"]');
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });

  // agent-scoped install at the app locale ('zh-CN' normalizes onto zh)
  assert.deepEqual(subagentCalls, [{ kind: 'install', agentId: 'a-1', slug: 'product-manager', locale: 'zh-CN' }]);
  // config.subagents follows the response list, not a client-side append
  assert.ok($('[data-subagent-installed-row="product-manager"]', root), 'installed row appears from the response');
  assert.ok($('[data-subagent-installed-tag="product-manager"]', root), 'catalog card gains the installed tag');

  await click(root, '[data-editor-save]');
  const config = (requests[0]!.body as { config: Record<string, unknown> }).config;
  assert.deepEqual(config.subagents, ['product-manager']);
});

test('removing an installed role calls the endpoint, patches config and shows the hint when emptied', async () => {
  const { client, requests, subagentCalls } = makeClient();
  const root = await mountSubagents(subagentAgent(['code-reviewer']), client);

  await click(root, '[data-subagent-remove="code-reviewer"]');
  await act(async () => { await Promise.resolve(); });
  await act(async () => { await Promise.resolve(); });

  assert.deepEqual(subagentCalls, [{ kind: 'remove', agentId: 'a-1', slug: 'code-reviewer' }]);
  assert.equal($('[data-subagent-installed-row="code-reviewer"]', root), null, 'row removed from the response list');
  assert.ok($('[data-subagent-empty-hint]', root), 'empty list flips to the delegation-off hint');
  // the catalog card regains its install button
  assert.ok($('[data-subagent-install="code-reviewer"]', root));

  await click(root, '[data-editor-save]');
  const config = (requests[0]!.body as { config: Record<string, unknown> }).config;
  assert.deepEqual(config.subagents, []);
});

test('division chips and the search box filter the catalog client-side', async () => {
  const { client } = makeClient();
  const root = await mountSubagents(subagentAgent([]), client);
  const visible = () => $$(root, '[data-subagent-slug]').map((el) => el.getAttribute('data-subagent-slug'));

  await click(root, '[data-subagent-division="pm"]');
  assert.deepEqual(visible(), ['product-manager', 'ux-researcher'], 'pm chip filters to the pm division');

  // search narrows within the active division; no match shows the hint
  await setValue(root, '[data-subagent-search]', 'code');
  assert.deepEqual(visible(), [], 'search + division leave no entries');
  assert.ok($('[data-subagent-no-match]', root));

  // back to all divisions: the same query matches slug + en name
  await click(root, '[data-subagent-division=""]');
  assert.deepEqual(visible(), ['code-reviewer']);
  await setValue(root, '[data-subagent-search]', 'research'); // en name of ux-researcher
  assert.deepEqual(visible(), ['ux-researcher']);
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
  // R485 D3/D5: the create form opens with the rag-qa preset + default models
  // prefilled — clear the seeded fields to drive the required-field path
  await setValue(root, '[data-field="name"]', '');
  await goto(root, 'prompts');
  await setValue(root, '[data-field="system_prompt"]', '');
  await goto(root, 'model');
  await setValue(root, '[data-field="model_id"]', '');
  await goto(root, 'basic');
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

// --- R484 D8: knowledge section offers the supported file types picker -------------------

test('knowledge section renders the supported-file-types dropdown with the 7 Vue options (D8/P3-2)', async () => {
  const { client, requests } = makeClient();
  const agent = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config } };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'knowledge');
  const section = $('[data-editor-section="knowledge"]', root);
  assert.ok(section, 'knowledge section missing');

  // Vue AgentEditorModal.vue:1523-1536 — row only when a KB scope is configured
  assert.ok(section!.textContent!.includes('支持的文件类型'), 'supported file types label missing');
  assert.ok(section!.textContent!.includes('限制可选择的文件类型，留空表示支持所有类型'), 'file types desc missing');
  // P3-2: dropdown form — the options live behind the trigger
  await click(section, '[data-file-types-trigger]');
  const panel = $('[data-file-types-panel]', root)!;
  const boxes = $$('[data-file-type]', panel) as HTMLInputElement[];
  assert.deepEqual(boxes.map((box) => box.getAttribute('data-file-type')), ['pdf', 'docx', 'txt', 'md', 'csv', 'xlsx', 'jpg']);

  // toggling writes through to config.supported_file_types on save
  await checkCheckbox(panel, 'input[data-file-type="pdf"]');
  await checkCheckbox(panel, 'input[data-file-type="csv"]');
  await click(root, '[data-editor-save]');
  const payload = (requests[0]!.body as { config: Record<string, unknown> }).config;
  assert.deepEqual(payload.supported_file_types, ['pdf', 'csv']);
});

test('knowledge section hides the file-types picker when the KB scope is none (D8)', async () => {
  const { client } = makeClient();
  const agent = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config, kb_selection_mode: 'none', knowledge_bases: [] } };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'knowledge');
  const section = $('[data-editor-section="knowledge"]', root);
  assert.ok(section);
  assert.equal($('[data-file-types-trigger]', section), null, 'picker must be hidden without a KB scope');
  assert.equal($('[data-file-type]', section), null, 'no stray options without a KB scope');
});

// --- R484 D11: skills section carries the manage-sandboxes link --------------------------

test('skills section renders the manage-sandboxes link navigating to settings?sandbox (D11)', async () => {
  const { client } = makeClient();
  const agent = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config, agent_mode: 'smart-reasoning' as const } };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'skills');
  const section = $('[data-editor-section="skills"]', root);
  assert.ok(section, 'skills section missing');
  const link = $('[data-go-sandbox-settings]', section) as HTMLAnchorElement | null;
  assert.ok(link, 'manage-sandboxes link missing');
  assert.equal(link!.textContent, '管理沙箱');
  await act(async () => {
    link!.dispatchEvent(new window.MouseEvent('click', { bubbles: true, cancelable: true }));
  });
  assert.equal(window.location.pathname + window.location.search, '/platform/settings?section=sandbox');
});

// --- R485 D1: the three Vue sections the React rail omitted ------------------------------

test('create rail registers suggestions/multimodal/mcp and the capability group (D1)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  const keys = $$(document.body, '[data-section-key]').map((el) => el.getAttribute('data-section-key'));
  assert.ok(keys.includes('suggestions'), '问题推荐 nav item missing');
  assert.ok(keys.includes('multimodal'), '附件上传 nav item missing');
  assert.ok(keys.includes('mcp'), 'MCP 服务 nav item missing');
  const bodyText = document.body.textContent ?? '';
  assert.match(bodyText, /问题推荐/);
  assert.match(bodyText, /附件上传/);
  assert.match(bodyText, /MCP 服务/);
});

test('suggestions section renders starters + follow-ups tabs with the Vue rows (D1)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await goto(root, 'suggestions');
  const section = $('[data-editor-section="suggestions"]', root);
  assert.ok(section, 'suggestions section missing');
  const text = section!.textContent ?? '';
  assert.match(text, /对话问题推荐/);
  assert.match(text, /开场推荐/);
  assert.match(text, /回答后推荐/);
  assert.match(text, /展示开场问题/);
  assert.match(text, /内容来源/);
  // starters enabled by default -> mode select + count visible
  const modeSelect = $('[data-field="question_suggestions.starters.mode"]', section) as HTMLSelectElement;
  assert.ok(modeSelect, 'starters mode select missing');
  assert.equal(modeSelect.value, 'hybrid');
  // switch to the follow-ups tab (follow-ups disabled by default -> switch row only)
  await click(section, '[data-suggestion-tab="followUps"]');
  const followSection = $('[data-editor-section="suggestions"]', root)!;
  const followText = followSection.textContent ?? '';
  assert.match(followText, /生成回答后推荐/);
  assert.equal(followText.includes('高级生成设置'), false, 'advanced rows hidden while follow-ups are off');
  // enabling follow-ups reveals the model row + advanced generation block
  await click(followSection, '[data-switch="question_suggestions.follow_ups.enabled"]');
  const enabledText = $('[data-editor-section="suggestions"]', root)!.textContent ?? '';
  assert.match(enabledText, /高级生成设置/);
  assert.match(enabledText, /附加生成要求/);
  assert.match(enabledText, /展示与兜底规则/);
});

test('multimodal section renders image/audio/timeout rows and gates on image_upload_enabled (D1)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await goto(root, 'multimodal');
  const section = $('[data-editor-section="multimodal"]', root);
  assert.ok(section, 'multimodal section missing');
  let text = section!.textContent ?? '';
  assert.match(text, /附件上传/);
  assert.match(text, /配置对话中图片、文档、音频等附件的上传、解析及对应模型/);
  assert.match(text, /图片上传/);
  assert.match(text, /语音上传/);
  assert.match(text, /附件解析等待超时（秒）/);
  // VLM row only after enabling image upload
  assert.equal($('[data-field="vlm_model_id"]', section), null);
  await click(section, '[data-switch="image_upload_enabled"]');
  text = $('[data-editor-section="multimodal"]', root)!.textContent ?? '';
  assert.match(text, /VLM 模型/);
  assert.match(text, /附件图片理解 \/ 扫描件 OCR/);
});

test('mcp section renders scope radios, service checklist and auth timeout (D1)', async () => {
  const { client } = makeClient();
  const agent = {
    ...EDIT_AGENT,
    config: { ...EDIT_AGENT.config, agent_mode: 'smart-reasoning' as const, mcp_services: ['mcp-b'] },
  };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'mcp');
  const section = $('[data-editor-section="mcp"]', root);
  assert.ok(section, 'mcp section missing');
  const text = section!.textContent ?? '';
  assert.match(text, /选择 Agent 可以调用的 MCP 服务/);
  assert.match(text, /全部/);
  // scope=none by default -> auth timeout row hidden (Vue v-if mcpSelectionMode !== 'none')
  assert.equal($('[data-field="mcp_auth_wait_timeout"]', section), null);
  await checkRadio(section, 'input[name="mcp-mode"][value="selected"]');
  const after = $('[data-editor-section="mcp"]', root)!;
  assert.match(after.textContent ?? '', /授权等待超时（秒）/);
  assert.ok($('[data-field="mcp_auth_wait_timeout"]', after), 'auth timeout visible once a scope is chosen');
  const checklist = $('[data-field="mcp_services"]', after);
  assert.ok(checklist, 'service checklist missing');
  // enabled service + disabled ghost entries flow through mcpOptionRows
  assert.match(checklist!.textContent ?? '', /服务A/);
  assert.match(checklist!.textContent ?? '', /服务B \(已禁用\)/);
});

// --- R485 D3+D10: create opens with the rag-qa preset applied ---------------------------

test('create mode prefills name/description/system prompt from the rag-qa preset (D3)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  const nameInput = $('[data-field="name"]', root) as HTMLInputElement;
  assert.equal(nameInput.value, '我的RAG 问答');
  const descInput = $('[data-field="description"]', root) as HTMLTextAreaElement;
  assert.equal(descInput.value, '基于文档分块的检索式问答，适合未启用 Wiki 的文档 / FAQ 知识库。');
  await goto(root, 'prompts');
  const prompt = $('[data-field="system_prompt"]', root) as HTMLTextAreaElement;
  assert.ok(prompt.value.startsWith('You are WeKnora'), 'system prompt body prefilled');
});

test('create mode seeds the four RAG tools into the effective-tools preview (D10)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await goto(root, 'tools');
  const section = $('[data-editor-section="tools"]', root)!;
  assert.match(section.textContent!, /允许的工具/);
  assert.match(section.textContent!, /选择 Agent 可以使用的工具/);
  assert.match(section.textContent!, /最终启用的工具/);
  // the four preset RAG tools are enabled (audit D10: React used to show the
  // "degraded to plain model Q&A" empty state)
  const checked = $$('[data-tool]', section).filter((el) => (el as HTMLInputElement).checked).map((el) => el.getAttribute('data-tool'));
  assert.deepEqual(checked.sort(), ['get_document_info', 'grep_chunks', 'knowledge_search', 'list_knowledge_chunks']);
  assert.equal(section.textContent!.includes('当前没有可用工具'), false, 'empty state must not show');
});

// --- R485 D2: agent type dropdown on the basic section ----------------------------------

test('basic section offers the agent type dropdown in agent mode (D2)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  const section = $('[data-editor-section="basic"]', root)!;
  assert.match(section.textContent!, /智能体类型/);
  assert.match(section.textContent!, /选择一个预设会自动填充系统提示词、工具列表和推荐的知识库范围。/);
  assert.match(section.textContent!, /基于文档分块的检索式问答/);
  const select = $('[data-field="agent_type"]', section) as HTMLSelectElement;
  assert.ok(select, 'agent type select missing');
  assert.deepEqual($$(select, 'option').map((option) => option.value), ['rag-qa', 'wiki-qa', 'hybrid-rag-wiki', 'data-analysis', 'custom']);
  assert.equal(select.value, 'rag-qa');
});

test('switching agent type applies the preset and refreshes system-generated fields (D2)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await setValue(root, '[data-field="agent_type"]', 'wiki-qa');
  const nameInput = $('[data-field="name"]', root) as HTMLInputElement;
  assert.equal(nameInput.value, '我的Wiki 问答');
  await goto(root, 'tools');
  const section = $('[data-editor-section="tools"]', root)!;
  const checked = $$('[data-tool]', section).filter((el) => (el as HTMLInputElement).checked).map((el) => el.getAttribute('data-tool'));
  assert.deepEqual(checked.sort(), ['wiki_flag_issue', 'wiki_read_page', 'wiki_read_source_doc', 'wiki_search']);
  // user-edited names survive a type switch
  await goto(root, 'basic');
  await setValue(root, '[data-field="name"]', '我自己的名字');
  await setValue(root, '[data-field="agent_type"]', 'rag-qa');
  assert.equal(($('[data-field="name"]', root) as HTMLInputElement).value, '我自己的名字');
});

test('quick-answer mode hides the agent type dropdown (D2)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await checkRadio(root, 'input[name="agent-mode"][value="quick-answer"]');
  assert.equal($('[data-field="agent_type"]', root), null, 'type dropdown is agent-mode only (Vue isAgentMode gate)');
});

// --- R485 D5+D6: creation-time model prefill + rerank required derivation ----------------

test('create mode prefills chat/rerank models and rerank is required with a RAG KB (D5+D6)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await goto(root, 'model');
  const model = $('[data-field="model_id"]', root) as HTMLSelectElement;
  assert.equal(model.value, 'm-chat', 'chat model prefilled (Vue applyDefaultModelsIfEmpty)');
  const rerank = $('[data-field="rerank_model_id"]', root) as HTMLSelectElement;
  assert.equal(rerank.value, 'm-rerank', 'rerank model prefilled');
  // kb=all with kb-1 RAG -> required star on the rerank label, optional hint hidden
  const section = $('[data-editor-section="model"]', root)!;
  const rerankLabel = Array.from(section.querySelectorAll('label')).find((label) => label.textContent?.includes('ReRank 模型') ?? label.textContent?.includes('重排'));
  assert.ok(rerankLabel, 'rerank label rendered');
  assert.ok((rerankLabel?.textContent ?? '').includes('*'), 'required star rendered while a RAG KB is in scope');
  assert.equal((section.textContent ?? '').includes('可不填'), false, 'optional hint hidden while required');
});

test('rerank stays optional when no RAG kb is in scope (D6)', async () => {
  const { client } = makeClient();
  // a wiki-only KB (no vector/keyword) keeps the section visible but rag-free
  const wikiOnly = { id: 'kb-w', name: 'Wiki 库', type: 'document', knowledge_count: 3, indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true } };
  const richClient = client as unknown as { knowledgeBases: { list: () => Promise<unknown[]> } };
  richClient.knowledgeBases = { list: async () => [wikiOnly] };
  const agent = {
    ...EDIT_AGENT,
    config: { ...EDIT_AGENT.config, agent_mode: 'smart-reasoning' as const, kb_selection_mode: 'selected' as const, knowledge_bases: ['kb-w'] },
  };
  const root = await mountModal({ client: richClient as unknown as WeKnoraClient, mode: 'edit', agent });
  await goto(root, 'model');
  const section = $('[data-editor-section="model"]', root)!;
  const rerankLabel = Array.from(section.querySelectorAll('label')).find((label) => (label.textContent ?? '').includes('ReRank') || (label.textContent ?? '').includes('重排'));
  assert.ok(rerankLabel, 'rerank label rendered');
  assert.equal((rerankLabel?.textContent ?? '').includes('*'), false, 'no required star without a RAG KB');
  assert.ok((section.textContent ?? '').length > 0);
});

// --- R486 D4: prompt placeholder tags + reset-default / use-template (Vue 220-270, 4689-4712) ---

test('prompts section lists the agent-mode placeholders and inserts on tag click (D4)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await goto(root, 'prompts');
  const section = $('[data-editor-section="prompts"]', root)!;
  const tagNodes = $$('[data-placeholder-tags="system"] [data-placeholder-tag]', section);
  assert.deepEqual(
    tagNodes.map((node) => node.getAttribute('data-placeholder-tag')),
    ['knowledge_bases', 'web_search_status', 'current_time', 'language'],
    'agent-mode system prompt placeholder set',
  );
  assert.match($('[data-placeholder-tags="system"]', section)!.textContent ?? '', /可用变量：/);
  // clicking a tag splices {{name}} at the caret (caret 0 in jsdom -> prefix)
  await click(section, '[data-placeholder-tag="knowledge_bases"]');
  const textarea = $('[data-field="system_prompt"]', root) as HTMLTextAreaElement;
  assert.ok(textarea.value.startsWith('{{knowledge_bases}}'), 'placeholder inserted at the caret');
});

test('quick-answer prompts carry the system + context placeholder sets (D4)', async () => {
  const { client } = makeClient();
  const agent = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config } };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'prompts');
  const section = $('[data-editor-section="prompts"]', root)!;
  assert.deepEqual(
    $$('[data-placeholder-tags="system"] [data-placeholder-tag]', section).map((node) => node.getAttribute('data-placeholder-tag')),
    ['query', 'contexts', 'current_time', 'current_week', 'language'],
    'normal-mode system prompt placeholder set',
  );
  assert.deepEqual(
    $$('[data-placeholder-tags="context"] [data-placeholder-tag]', section).map((node) => node.getAttribute('data-placeholder-tag')),
    ['query', 'contexts', 'current_time', 'current_week', 'language'],
    'context template placeholder set',
  );
  // context tag click inserts into context_template at the caret
  await click(section, '[data-placeholder-tags="context"] [data-placeholder-tag="contexts"]');
  const contextArea = $('[data-field="context_template"]', root) as HTMLTextAreaElement;
  assert.ok(contextArea.value.startsWith('{{contexts}}'), 'context placeholder inserted at the caret');
});

test('agent-mode system prompt exposes 恢复默认 + 使用模板 with all 7 yaml templates (D4 + R486 verify DIFF-A)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await goto(root, 'prompts');
  const section = $('[data-editor-section="prompts"]', root)!;
  assert.ok($('[data-prompt-reset-default]', section), 'reset-default button missing');
  const toggle = $('[data-prompt-template-toggle]', section);
  assert.ok(toggle, 'use-template trigger missing');
  assert.match(toggle!.textContent ?? '', /使用模板/);
  // panel opens on demand and lists the vendored builtin entries
  assert.equal($('[data-prompt-template-panel]', section), null, 'panel closed before opening');
  await click(section, '[data-prompt-template-toggle]');
  const panel = $('[data-prompt-template-panel]', root)!;
  const items = $$('[data-prompt-template]', panel);
  assert.deepEqual(
    items.map((item) => item.getAttribute('data-prompt-template')),
    ['pure_agent', 'progressive_rag_agent', 'data_analyst', 'wiki_researcher', 'wiki_fixer', 'hybrid_rag_wiki_agent', 'skill_installer'],
  );
  assert.match(panel.textContent ?? '', /渐进式 RAG 智能体/, 'zh template name rendered');
  assert.ok($('[data-template-default]', panel), 'default tag on the global default entry');
  // selecting a template writes the body into the textarea (Vue handleSystemPromptTemplateSelect)
  await click(panel, '[data-prompt-template="wiki_researcher"]');
  const textarea = $('[data-field="system_prompt"]', root) as HTMLTextAreaElement;
  assert.ok(textarea.value.startsWith('<role>'), 'wiki template body applied');
  // reset-default resolves the preset-bound template: create form is rag-qa
  await click(section, '[data-prompt-reset-default]');
  const afterReset = $('[data-field="system_prompt"]', root) as HTMLTextAreaElement;
  assert.ok(afterReset.value.startsWith('You are WeKnora'), 'reset restores the preset-bound body');
});

test('quick-answer system prompt keeps the bare textarea (template selector is agent-mode only)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'edit', agent: { ...EDIT_AGENT } });
  await goto(root, 'prompts');
  const section = $('[data-editor-section="prompts"]', root)!;
  assert.equal($('[data-prompt-template-toggle]', section), null, 'no agent templates under quick-answer');
  assert.equal($('[data-prompt-reset-default]', section), null, 'no reset-default under quick-answer');
});

// --- R486: embedded KBParserSettings rows in the multimodal section (Vue 869-878) ---------

test('multimodal embeds the per-file-type parser rows and persists engine changes', async () => {
  const { client, requests } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await goto(root, 'multimodal');
  const section = $('[data-editor-section="multimodal"]', root)!;
  const block = $('[data-parser-policy-block]', section)!;
  assert.match(block.textContent ?? '', /聊天附件解析策略/);
  // one row per chat-relevant family (pdf first, audio families filtered out)
  const rows = $$('[data-parser-row]', block);
  const rowKeys = rows.map((row) => row.getAttribute('data-parser-row'));
  assert.ok(rowKeys.includes('pdf'), 'pdf family row missing');
  assert.ok(rowKeys.includes('excel'), 'excel family row missing');
  assert.ok(!rowKeys.includes('audiovisual'), 'audio families are not chat-attachment relevant');
  assert.ok(rows.length >= 8, 'expected the chat-relevant families to render');
  // each row shows its extension tags and an engine select with available engines
  const pdfRow = rows.find((row) => row.getAttribute('data-parser-row') === 'pdf')!;
  assert.match(pdfRow.textContent ?? '', /\.pdf/);
  const pdfSelect = $('[data-parser-engine="pdf"]', block) as HTMLSelectElement;
  assert.ok(pdfSelect, 'pdf engine select missing');
  const pdfOptions = Array.from(pdfSelect.options).map((option) => option.value);
  assert.ok(pdfOptions.includes('builtin'), 'builtin engine option missing');
  assert.ok(!pdfOptions.includes('mineru'), 'unavailable engines are hidden');
  // the office family additionally offers anydoc (its only other supporter)
  const officeSelect = $('[data-parser-engine="office"]', block) as HTMLSelectElement;
  const officeOptions = Array.from(officeSelect.options).map((option) => option.value);
  assert.ok(officeOptions.includes('builtin') && officeOptions.includes('anydoc'), 'office engines missing');
  // pick builtin for pdf -> the save payload carries a per-group rule
  await setValue(block, '[data-parser-engine="pdf"]', 'builtin');
  await click(root, '[data-editor-save]');
  const payload = (requests[0]!.body as { config: { chat_parser_engine_rules?: Array<{ file_types: string[]; engine: string }> } }).config;
  const pdfRule = payload.chat_parser_engine_rules?.find((rule) => rule.file_types.includes('pdf'));
  assert.ok(pdfRule, 'pdf rule missing from payload');
  assert.equal(pdfRule!.engine, 'builtin');
});

test('parser block degrades to the no-engine hint when the registry is empty', async () => {
  const { client } = makeClient({ parserEngines: null });
  const root = await mountModal({ client, mode: 'create' });
  await goto(root, 'multimodal');
  const block = $('[data-parser-policy-block]', $('[data-editor-section="multimodal"]', root)!)!;
  assert.ok($('[data-parser-empty]', block), 'no-engine hint element missing');
  assert.match(block.textContent ?? '', /暂无可用解析引擎/, 'no-engine-available hint rendered');
  assert.equal($('[data-parser-row]', block), null, 'no rows without engines');
});

// --- R486 P3-2: supported file types as a dropdown multi-select (Vue 1529-1536) ----------

test('supported file types render as a dropdown multi-select writing the same set (P3-2)', async () => {
  const { client, requests } = makeClient();
  const agent = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config } };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'knowledge');
  const section = $('[data-editor-section="knowledge"]', root)!;
  // closed state: a single trigger showing the placeholder, no loose checkboxes
  const trigger = $('[data-file-types-trigger]', section) as HTMLElement;
  assert.ok(trigger, 'multi-select trigger missing');
  assert.match(trigger.textContent ?? '', /全部类型/, 'empty selection shows the all-types placeholder');
  assert.equal($('[data-file-type]', section), null, 'options hidden until the dropdown opens');
  // open -> the 7 Vue options, selection writes back the same set semantics
  await click(section, '[data-file-types-trigger]');
  const panel = $('[data-file-types-panel]', root)!;
  assert.deepEqual(
    $$('[data-file-type]', panel).map((node) => node.getAttribute('data-file-type')),
    ['pdf', 'docx', 'txt', 'md', 'csv', 'xlsx', 'jpg'],
  );
  await checkCheckbox(panel, 'input[data-file-type="pdf"]');
  await checkCheckbox(panel, 'input[data-file-type="csv"]');
  const triggerAfter = $('[data-file-types-trigger]', section)!;
  assert.match(triggerAfter.textContent ?? '', /PDF/, 'selected labels surface on the trigger');
  await click(root, '[data-editor-save]');
  const payload = (requests[0]!.body as { config: Record<string, unknown> }).config;
  assert.deepEqual(payload.supported_file_types, ['pdf', 'csv']);
});

test('file-types dropdown closes on outside click and hydrates stored selections (P3-2)', async () => {
  const { client } = makeClient();
  const agent = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config, supported_file_types: ['md', 'xlsx'] } };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'knowledge');
  const section = $('[data-editor-section="knowledge"]', root)!;
  const trigger = $('[data-file-types-trigger]', section)!;
  assert.match(trigger.textContent ?? '', /Markdown/, 'hydrated selection rendered on the trigger');
  await click(section, '[data-file-types-trigger]');
  assert.ok($('[data-file-types-panel]', root), 'panel opens');
  await act(async () => {
    document.body.dispatchEvent(new window.MouseEvent('click', { bubbles: true }));
  });
  assert.equal($('[data-file-types-panel]', document.body), null, 'panel closes on outside click');
});

// --- R486: agent-type switch KB-conflict warning (Vue 3342-3347) --------------------------

test('switching to an incompatible preset keeps the selected scope and warns (KB warn)', async () => {
  const { client } = makeClient();
  // tenant-style preset without kb_selection_mode: the selected KB list must
  // survive the switch, and its RAG tools derive a filter the wiki-only KB
  // cannot satisfy — the shipped-catalog path to the Vue warning
  const injected = { id: 'test-rag-strict', i18n: { default: { label: 'Strict RAG', description: 'test preset' } }, config: { system_prompt_id: 'progressive_rag_agent', allowed_tools: ['knowledge_search'] } };
  const presets = (await import('./agent-type-presets.ts')).AGENT_TYPE_PRESETS;
  presets.splice(presets.length - 1, 0, injected); // before 'custom'
  try {
    const wikiOnly = { id: 'kb-w', name: 'Wiki 库', type: 'document', knowledge_count: 3, indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true } };
    const base = client as unknown as { knowledgeBases: { list: () => Promise<unknown[]>; settings: unknown } };
    const kbSettings = base.knowledgeBases.settings;
    base.knowledgeBases = { list: async () => [wikiOnly], settings: kbSettings };
    const agent = {
      ...EDIT_AGENT,
      config: { ...EDIT_AGENT.config, agent_mode: 'smart-reasoning' as const, kb_selection_mode: 'selected' as const, knowledge_bases: ['kb-w'] },
    };
    const root = await mountModal({ client, mode: 'edit', agent });
    // switch the type dropdown to the injected strict-RAG preset
    await setValue(root, '[data-field="agent_type"]', 'test-rag-strict');
    const warn = $('[data-agent-type-warn]', root);
    assert.ok(warn, 'kb-incompatible warning rendered');
    assert.match(warn!.textContent ?? '', /已选的 1 个知识库不适用于当前类型/);
    // switching back to a compatible preset clears the warning
    await setValue(root, '[data-field="agent_type"]', 'wiki-qa');
    assert.equal($('[data-agent-type-warn]', root), null, 'warning cleared on the next compatible switch');
  } finally {
    const index = presets.indexOf(injected);
    if (index >= 0) presets.splice(index, 1);
  }
});

test('shipped presets reset the KB radio to all and clear the explicit list (Vue watch 3639-3650)', async () => {
  const { client, requests } = makeClient();
  const agent = {
    ...EDIT_AGENT,
    config: { ...EDIT_AGENT.config, agent_mode: 'smart-reasoning' as const, kb_selection_mode: 'selected' as const, knowledge_bases: ['kb-1'] },
  };
  const root = await mountModal({ client, mode: 'edit', agent });
  await setValue(root, '[data-field="agent_type"]', 'rag-qa');
  // the preset writes kb_selection_mode 'all' -> the radio mirrors it and the
  // explicit selection clears (so no stale-KB warning is owed)
  await goto(root, 'knowledge');
  const allRadio = $('input[name="kb-mode"][value="all"]', root) as HTMLInputElement;
  assert.ok(allRadio?.checked, 'kb radio reset to 全部知识库');
  await click(root, '[data-editor-save]');
  const payload = (requests[0]!.body as { config: Record<string, unknown> }).config;
  assert.deepEqual(payload.knowledge_bases, [], 'explicit list cleared with the all-mode reset');
  assert.equal(payload.kb_selection_mode, 'all');
});

// --- R486 D9: create-form retrieval defaults read the tenant retrieval-config --------------

test('create form seeds retrieval thresholds from the tenant retrieval-config (D9)', async () => {
  const { client } = makeClient({ retrievalConfig: { embedding_top_k: 50, keyword_threshold: 0, vector_threshold: 0.2, rerank_top_k: 8, rerank_threshold: 0.6 } });
  const root = await mountModal({ client, mode: 'create' });
  await goto(root, 'retrieval');
  const section = $('[data-editor-section="retrieval"]', root)!;
  const topK = $('[data-field="embedding_top_k"]', section) as HTMLInputElement;
  assert.equal(topK.value, '50', 'tenant embedding_top_k applied');
  // the keyword/vector thresholds ride on Range inputs carrying the numeric value
  const keyword = $('input[type="range"][aria-label*="关键词"], input[type="range"]', section) as HTMLInputElement;
  assert.ok(keyword, 'threshold slider rendered');
  assert.equal(keyword.value, '0', 'tenant keyword_threshold 0 overrides the 0.3 default (Vue !== undefined rule)');
  const vector = $$('input[type="range"]', section)[1] as HTMLInputElement;
  assert.equal(vector.value, '0.2', 'tenant vector_threshold applied');
});

test('create form keeps the built-in retrieval defaults when the tenant config is unreachable (D9)', async () => {
  const { client } = makeClient();
  const root = await mountModal({ client, mode: 'create' });
  await goto(root, 'retrieval');
  const section = $('[data-editor-section="retrieval"]', root)!;
  const topK = $('[data-field="embedding_top_k"]', section) as HTMLInputElement;
  assert.equal(topK.value, '10', 'built-in default kept');
  const keyword = $$('input[type="range"]', section)[0] as HTMLInputElement;
  assert.equal(keyword.value, '0.3', 'built-in keyword default kept');
});

test('edit form keeps the stored retrieval values over tenant defaults (D9)', async () => {
  const { client } = makeClient({ retrievalConfig: { embedding_top_k: 50, keyword_threshold: 0, vector_threshold: 0.2 } });
  const agent = { ...EDIT_AGENT, config: { ...EDIT_AGENT.config, embedding_top_k: 7, keyword_threshold: 0.4, vector_threshold: 0.9 } };
  const root = await mountModal({ client, mode: 'edit', agent });
  await goto(root, 'retrieval');
  const section = $('[data-editor-section="retrieval"]', root)!;
  assert.equal(($('[data-field="embedding_top_k"]', section) as HTMLInputElement).value, '7', 'stored value survives hydration');
  assert.equal(($$('input[type="range"]', section)[0] as HTMLInputElement).value, '0.4');
});

// --- R491: runtime catalogs drive the editor UI (Vue editorResources parity) -----------

const RUNTIME_PRESETS = [
  {
    id: 'rag-qa',
    i18n: {
      default: { label: 'RAG Q&A', description: 'Evidence-based retrieval.' },
      'zh-CN': { label: '后端问答', description: '后端预设描述' },
    },
    config: { system_prompt_id: 'backend_prompt', temperature: 0.42, allowed_tools: ['knowledge_search'], kb_selection_mode: 'all' },
  },
  { id: 'custom', i18n: { default: { label: 'Custom', description: '' }, 'zh-CN': { label: '自定义', description: '' } } },
];

const RUNTIME_TEMPLATES = {
  agent_system_prompt: [
    { id: 'backend_prompt', name: '后端模板', description: '来自租户 KV 的模板', content: 'backend prompt body', default: true, mode: 'rag' },
    { id: 'backend_extra', name: '后端追加模板', description: '租户自定义追加', content: 'extra body' },
  ],
};

const RUNTIME_PLACEHOLDERS = {
  agent_system_prompt: [
    { name: 'knowledge_bases', label: '知识库列表', description: '自动格式化的知识库列表' },
    { name: 'backend_var', label: '后端变量', description: '后端新增的变量' },
  ],
  system_prompt: [{ name: 'query', label: '用户问题', description: '用户当前的问题' }],
  context_template: [{ name: 'query', label: '用户问题', description: '用户当前的问题' }],
};

test('runtime catalogs render in the type dropdown, template panel and placeholder strip', async () => {
  resetAgentEditorResourcesCache();
  try {
    const { client } = makeClient({
      typePresets: RUNTIME_PRESETS as never,
      promptTemplates: RUNTIME_TEMPLATES as never,
      placeholders: RUNTIME_PLACEHOLDERS as never,
    });
    const root = await mountModal({ client, mode: 'create' });
    await act(async () => { await Promise.resolve(); });
    await act(async () => { await Promise.resolve(); });

    // type dropdown + description + 我的<label> prefill come from the backend
    const select = $('[data-field="agent_type"]', root) as HTMLSelectElement;
    assert.deepEqual($$(select, 'option').map((option) => option.value), ['rag-qa', 'custom']);
    assert.equal(select.value, 'rag-qa');
    assert.match(root.textContent!, /后端预设描述/);
    assert.equal(($('[data-field="name"]', root) as HTMLInputElement).value, '我的后端问答');

    // template panel lists the tenant-KV templates with backend strings; the
    // create prefill applied the backend preset body (prompts section field)
    await goto(root, 'prompts');
    const section = $('[data-editor-section="prompts"]', root)!;
    assert.equal(($('[data-field="system_prompt"]', root) as HTMLTextAreaElement).value, 'backend prompt body');
    await click(section, '[data-prompt-template-toggle]');
    const panel = $('[data-prompt-template-panel]', root)!;
    assert.deepEqual(
      $$('[data-prompt-template]', panel).map((item) => item.getAttribute('data-prompt-template')),
      ['backend_prompt', 'backend_extra'],
    );
    assert.match(panel.textContent!, /后端模板/);
    assert.match(panel.textContent!, /租户自定义追加/);

    // placeholder strip shows the backend variable set
    const tags = $$('[data-placeholder-tags="system"] [data-placeholder-tag]', section).map((node) => node.getAttribute('data-placeholder-tag'));
    assert.deepEqual(tags, ['knowledge_bases', 'backend_var']);
  } finally {
    resetAgentEditorResourcesCache();
  }
});

test('failing runtime catalog endpoints fall back to the vendored static catalogs', async () => {
  resetAgentEditorResourcesCache();
  try {
    const { client } = makeClient({ typePresets: null, promptTemplates: null, placeholders: null });
    const root = await mountModal({ client, mode: 'create' });
    await act(async () => { await Promise.resolve(); });
    await act(async () => { await Promise.resolve(); });

    // static preset table: the five shipped presets, rag-qa selected with its
    // vendored system prompt body (prompts section field)
    const select = $('[data-field="agent_type"]', root) as HTMLSelectElement;
    assert.deepEqual($$(select, 'option').map((option) => option.value), ['rag-qa', 'wiki-qa', 'hybrid-rag-wiki', 'data-analysis', 'custom']);

    // static builtin template list answers the template panel
    await goto(root, 'prompts');
    const section = $('[data-editor-section="prompts"]', root)!;
    assert.ok(($('[data-field="system_prompt"]', root) as HTMLTextAreaElement).value.startsWith('You are WeKnora, an assistant'));
    await click(section, '[data-prompt-template-toggle]');
    const panel = $('[data-prompt-template-panel]', root)!;
    assert.equal($$('[data-prompt-template]', panel).length, 7);

    // static placeholder catalogue answers the variable strip
    const tags = $$('[data-placeholder-tags="system"] [data-placeholder-tag]', section).map((node) => node.getAttribute('data-placeholder-tag'));
    assert.deepEqual(tags, ['knowledge_bases', 'web_search_status', 'current_time', 'language']);
  } finally {
    resetAgentEditorResourcesCache();
  }
});

test('restore-default prefers the preset-bound runtime template over the global default (R491)', async () => {
  resetAgentEditorResourcesCache();
  try {
    // wiki-qa preset bound to wiki_prompt while backend_prompt is the global default
    const presets = [
      ...RUNTIME_PRESETS,
      { id: 'wiki-qa', i18n: { default: { label: 'Wiki Q&A', description: '' }, 'zh-CN': { label: 'Wiki 问答', description: '' } }, config: { system_prompt_id: 'wiki_prompt' } },
    ];
    const templates = {
      agent_system_prompt: [
        { id: 'backend_prompt', name: '全局默认', description: '', content: 'global default body', default: true },
        { id: 'wiki_prompt', name: 'Wiki 模板', description: '', content: 'wiki preset body' },
      ],
    };
    const { client } = makeClient({ typePresets: presets as never, promptTemplates: templates as never, placeholders: null });
    const root = await mountModal({ client, mode: 'create' });
    await act(async () => { await Promise.resolve(); });
    await act(async () => { await Promise.resolve(); });
    await setValue(root, '[data-field="agent_type"]', 'wiki-qa');
    await goto(root, 'prompts');
    const section = $('[data-editor-section="prompts"]', root)!;
    await click(section, '[data-prompt-reset-default]');
    assert.equal(($('[data-field="system_prompt"]', root) as HTMLTextAreaElement).value, 'wiki preset body');
  } finally {
    resetAgentEditorResourcesCache();
  }
});
