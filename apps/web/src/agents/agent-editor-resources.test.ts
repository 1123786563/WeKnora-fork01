/**
 * R491 — runtime fetch of the agent editor catalogs, aligning the React editor
 * with the Vue baseline (frontend/src/stores/editorResources.ts):
 *   - GET /api/v1/agents/type-presets   (ensureAgentTypePresets)
 *   - GET /api/v1/tenants/kv/prompt-templates (ensurePromptTemplates, agent_
 *     system_prompt section) via client.settings.promptTemplates.get()
 *   - GET /api/v1/agents/placeholders   (ensurePlaceholders)
 * Vue keeps empty locals when a fetch fails (no static fallback). The React
 * client intentionally keeps the vendored static catalogs as the fallback so a
 * failed/empty fetch degrades to the R485/R486 behaviour instead of an empty
 * dropdown — divergence decision recorded in the R491 report.
 */
import assert from 'node:assert/strict';
import test from 'node:test';
import type { WeKnoraClient } from '@weknora/api-client';
import {
  fallbackAgentEditorResources,
  loadAgentEditorResources,
  resetAgentEditorResourcesCache,
  resolveAgentEditorResources,
  type AgentEditorRuntimeData,
} from './agent-editor-resources.ts';
import { AGENT_SYSTEM_PROMPT_TEMPLATE_LIST, AGENT_TYPE_PRESETS } from './agent-type-presets.ts';
import { promptPlaceholdersFor } from './agent-editor.ts';

function makeClient(handlers: {
  typePresets?: () => unknown;
  placeholders?: () => unknown;
  promptTemplates?: () => unknown;
}): { client: WeKnoraClient; calls: string[] } {
  const calls: string[] = [];
  const client = {
    configuration: {
      agents: {
        typePresets: handlers.typePresets
          ? async () => { calls.push('typePresets'); return handlers.typePresets!(); }
          : undefined,
        placeholders: handlers.placeholders
          ? async () => { calls.push('placeholders'); return handlers.placeholders!(); }
          : undefined,
      },
    },
    settings: {
      promptTemplates: handlers.promptTemplates
        ? { get: async () => { calls.push('promptTemplates'); return handlers.promptTemplates!(); } }
        : undefined,
    },
  };
  return { client: client as unknown as WeKnoraClient, calls };
}

const BACKEND_PRESETS = [
  {
    id: 'rag-qa',
    i18n: {
      default: { label: 'RAG Q&A', description: 'Evidence-based retrieval.' },
      'zh-CN': { label: 'RAG 问答(后端)', description: '后端描述' },
    },
    config: { system_prompt_id: 'progressive_rag_agent', temperature: 0.5, kb_selection_mode: 'all' },
  },
];

const BACKEND_TEMPLATES = {
  agent_system_prompt: [
    { id: 'progressive_rag_agent', name: '渐进式 RAG 智能体', description: '后端模板', content: 'backend body', default: true, mode: 'rag' },
    { id: 'extra_agent', name: '租户自定义', description: '租户加的', content: 'tenant body' },
  ],
};

const BACKEND_PLACEHOLDERS = {
  agent_system_prompt: [
    { name: 'knowledge_bases', label: '知识库列表', description: '自动格式化的知识库列表' },
    { name: 'extra_var', label: '新增变量', description: '后端新增的变量' },
  ],
  system_prompt: [{ name: 'query', label: '用户问题', description: '用户当前的问题' }],
};

test('loadAgentEditorResources fetches all three catalogs and prefers backend data', async () => {
  resetAgentEditorResourcesCache();
  const { client, calls } = makeClient({
    typePresets: () => BACKEND_PRESETS,
    placeholders: () => BACKEND_PLACEHOLDERS,
    promptTemplates: () => BACKEND_TEMPLATES,
  });

  const runtime = await loadAgentEditorResources(client);
  assert.deepEqual(calls.sort(), ['placeholders', 'promptTemplates', 'typePresets']);
  assert.equal(runtime.typePresets?.[0]?.id, 'rag-qa');
  assert.equal(runtime.typePresets?.[0]?.config?.temperature, 0.5);
  assert.equal(runtime.promptTemplates?.length, 2);
  assert.equal(runtime.promptTemplates?.[0]?.content, 'backend body');
  assert.equal(runtime.placeholders?.agent_system_prompt?.length, 2);

  const resolved = resolveAgentEditorResources(runtime, 'zh-CN');
  assert.equal(resolved.typePresets[0]!.i18n['zh-CN']!.label, 'RAG 问答(后端)');
  assert.equal(resolved.promptTemplates[0]!.name, '渐进式 RAG 智能体');
  assert.equal(promptPlaceholdersFor('agent_system_prompt', resolved.placeholders)[1]!.name, 'extra_var');
});

test('loadAgentEditorResources caches for 60s and de-dupes concurrent loads (Vue runOnce parity)', async () => {
  resetAgentEditorResourcesCache();
  let typePresetCalls = 0;
  const { client } = makeClient({
    typePresets: () => { typePresetCalls += 1; return BACKEND_PRESETS; },
    placeholders: () => { throw new Error('down'); },
    promptTemplates: () => ({}),
  });

  const [a, b] = await Promise.all([loadAgentEditorResources(client), loadAgentEditorResources(client)]);
  assert.equal(typePresetCalls, 1, 'concurrent loads share one inflight request');
  assert.equal(a.typePresets, b.typePresets, 'same resolved raw object');

  await loadAgentEditorResources(client);
  assert.equal(typePresetCalls, 1, 'fresh cache skips the request');

  await loadAgentEditorResources(client, { force: true });
  assert.equal(typePresetCalls, 2, 'force bypasses the TTL cache');
});

test('failed or empty fetches degrade to null so resolve() keeps the static catalogs', async () => {
  resetAgentEditorResourcesCache();
  const { client } = makeClient({
    typePresets: () => { throw new Error('500'); },
    placeholders: () => ({ data: { system_prompt: [] } }), // empty section
    promptTemplates: () => ({ agent_system_prompt: [] }), // empty list
  });

  const runtime = await loadAgentEditorResources(client);
  assert.equal(runtime.typePresets, null);
  assert.equal(runtime.promptTemplates, null);
  assert.equal(runtime.placeholders, null, 'an empty placeholder section degrades to null');

  const resolved = resolveAgentEditorResources(runtime, 'zh-CN');
  assert.equal(resolved.typePresets, AGENT_TYPE_PRESETS, 'static preset table stays as the fallback');
  assert.equal(resolved.typePresets.length, 5);
  assert.equal(resolved.promptTemplates.length, AGENT_SYSTEM_PROMPT_TEMPLATE_LIST.length);
  assert.equal(resolved.placeholders, null);
  assert.equal(promptPlaceholdersFor('agent_system_prompt', resolved.placeholders)[0]!.name, 'knowledge_bases', 'static placeholder set still answers');
});

test('a missing client method (older stub) degrades instead of throwing', async () => {
  resetAgentEditorResourcesCache();
  const { client } = makeClient({});
  const runtime = await loadAgentEditorResources(client);
  assert.deepEqual(runtime, { typePresets: null, promptTemplates: null, placeholders: null });
});

test('resolveAgentEditorResources maps the static fallback per locale and prefers partial runtime data', async () => {
  const fallbackZh = fallbackAgentEditorResources('zh-CN');
  assert.equal(fallbackZh.promptTemplates[0]!.name, '纯智能体');
  const fallbackEn = fallbackAgentEditorResources('en-US');
  assert.equal(fallbackEn.promptTemplates[0]!.name, 'Pure Agent');

  // partial runtime: presets fetched, templates down -> mixed resolution
  const partial: AgentEditorRuntimeData = { typePresets: BACKEND_PRESETS as never, promptTemplates: null, placeholders: BACKEND_PLACEHOLDERS as never };
  const resolved = resolveAgentEditorResources(partial, 'zh-CN');
  assert.equal(resolved.typePresets[0]!.id, 'rag-qa');
  assert.equal(resolved.promptTemplates.length, AGENT_SYSTEM_PROMPT_TEMPLATE_LIST.length, 'templates fall back to the static list');
  assert.equal(resolved.placeholders?.system_prompt?.[0]!.name, 'query');
  // null runtime resolves entirely to the fallback
  const allFallback = resolveAgentEditorResources(null, 'zh-CN');
  assert.equal(allFallback.typePresets, AGENT_TYPE_PRESETS);
});
