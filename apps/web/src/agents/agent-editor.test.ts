import assert from 'node:assert/strict';
import test from 'node:test';
import {
  agentModeOf,
  applyAgentModeSwitch,
  applyKbSelectionMode,
  applyScopeSelectionMode,
  buildAgentPayload,
  buildNavGroups,
  catalogSkillRows,
  defaultAgentForm,
  editorT,
  evaluateToolRequirement,
  hydrateAgentForm,
  initKbSelectionMode,
  initScopeSelectionMode,
  kbOptionFromRecord,
  TOOL_CATALOG,
  validateAgentForm,
  type AgentEditorForm,
  type ToolCapabilityScope,
} from './agent-editor.ts';
import { agentEditorFallback } from './agent-editor-fallback.ts';

const fullScope: ToolCapabilityScope = { vector: true, keyword: true, wiki: true, graph: true, faq: true };
const noScope: ToolCapabilityScope = { vector: false, keyword: false, wiki: false, graph: false, faq: false };

const withConfig = (patch: { name?: string; description?: string; is_builtin?: boolean } & Partial<AgentEditorForm['config']>): AgentEditorForm => {
  const form = defaultAgentForm();
  const { name, description, is_builtin, ...config } = patch;
  if (name !== undefined) form.name = name;
  if (description !== undefined) form.description = description;
  if (is_builtin !== undefined) form.is_builtin = is_builtin;
  Object.assign(form.config, config);
  return form;
};

// --- defaults (AgentEditorModal.vue:2716-2825) --------------------------------------

test('default form mirrors the Vue defaultFormData field values', () => {
  const form = defaultAgentForm();
  assert.equal(form.name, '');
  assert.equal(form.is_builtin, false);
  assert.equal(form.config.agent_mode, 'smart-reasoning');
  assert.equal(form.config.kb_selection_mode, 'all');
  assert.equal(form.config.temperature, 0.7);
  assert.equal(form.config.citation_enabled, true);
  assert.equal(form.config.thinking, false);
  assert.equal(form.config.max_iterations, 10);
  assert.equal(form.config.llm_call_timeout, 120);
  assert.equal(form.config.multi_turn_enabled, false);
  assert.equal(form.config.history_turns, 5);
  assert.equal(form.config.enable_rewrite, true);
  assert.equal(form.config.enable_query_expansion, true);
  assert.equal(form.config.memory_enabled, true);
  assert.equal(form.config.fallback_strategy, 'model');
  assert.equal(form.config.faq_priority_enabled, true);
  assert.equal(form.config.faq_direct_answer_threshold, 0.9);
  assert.equal(form.config.faq_score_boost, 1.2);
  assert.equal(form.config.web_search_enabled, false);
  assert.equal(form.config.web_search_max_results, 5);
  assert.equal(form.config.embedding_top_k, 10);
  assert.equal(form.config.keyword_threshold, 0.3);
  assert.equal(form.config.vector_threshold, 0.5);
  assert.equal(form.config.rerank_top_k, 5);
  assert.equal(form.config.rerank_threshold, 0.5);
  assert.equal(form.config.skills_selection_mode, 'none');
  assert.equal(form.config.mcp_auth_wait_timeout, 600);
});

// --- hydration (AgentEditorModal.vue:3401-3471) --------------------------------------

test('hydrateAgentForm merges stored config over defaults', () => {
  const form = hydrateAgentForm({
    id: 'a-1',
    name: '我的助手',
    description: '描述',
    config: {
      agent_mode: 'quick-answer',
      system_prompt: '你是一个助手',
      model_id: 'm-1',
      knowledge_bases: ['kb-1'],
      temperature: 0.4,
    },
  });
  assert.equal(form.id, 'a-1');
  assert.equal(form.name, '我的助手');
  assert.equal(form.config.system_prompt, '你是一个助手');
  assert.equal(form.config.model_id, 'm-1');
  assert.equal(form.config.temperature, 0.4);
  assert.equal(form.config.knowledge_bases[0], 'kb-1');
  // untouched defaults survive the merge
  assert.equal(form.config.history_turns, 5);
  assert.equal(form.config.citation_enabled, true);
});

test('hydrateAgentForm infers agent_mode for legacy agents (Vue:3448-3452)', () => {
  const legacy = hydrateAgentForm({ id: 'a-2', name: 'x', config: { max_iterations: 5, allowed_tools: ['thinking'] } });
  assert.equal(legacy.config.agent_mode, 'smart-reasoning');
  const rag = hydrateAgentForm({ id: 'a-3', name: 'x', config: { max_iterations: 1, allowed_tools: [] } });
  assert.equal(rag.config.agent_mode, 'quick-answer');
});

test('hydrateAgentForm backfills memory_enabled and mcp_auth_wait_timeout (Vue:3434-3446)', () => {
  const form = hydrateAgentForm({ id: 'a-4', name: 'x', config: {} });
  assert.equal(form.config.memory_enabled, true);
  assert.equal(form.config.mcp_auth_wait_timeout, 600);
  const stale = hydrateAgentForm({ id: 'a-5', name: 'x', config: { mcp_auth_wait_timeout: -1 } });
  assert.equal(stale.config.mcp_auth_wait_timeout, 600);
});

// Octop M2: the expert provenance stamp must survive the editor round-trip —
// the hydrate merge only copies keys that exist in defaultAgentConfig (the M1
// persona trap), so expert_source defaults to null and a stored stamp is kept.
test('hydrateAgentForm keeps the expert_source provenance stamp and defaults it to null', () => {
  assert.equal(defaultAgentForm().config.expert_source, null);

  const stamp = { expert_id: 'stock-assistant', source: 'builtin', slug: '' };
  const fromExpert = hydrateAgentForm({ id: 'a-6', name: '股票助手', config: { expert_source: stamp } });
  assert.deepEqual(fromExpert.config.expert_source, stamp);
  const savedConfig = buildAgentPayload(fromExpert).config as AgentEditorForm['config'];
  assert.deepEqual(savedConfig.expert_source, stamp, 'save must carry the stamp back');

  const plain = hydrateAgentForm({ id: 'a-7', name: 'x', config: {} });
  assert.equal(plain.config.expert_source, null);
  // a null stamp (backend omits unset) never resurrects as an object
  const nullStamp = hydrateAgentForm({ id: 'a-8', name: 'x', config: { expert_source: null } });
  assert.equal(nullStamp.config.expert_source, null);
});

// Octop M3: delegation slugs must survive the editor round-trip — same trap as
// expert_source: a key missing from defaultAgentConfig is dropped by hydrate.
test('hydrateAgentForm keeps the subagents delegation list and defaults it to empty', () => {
  assert.deepEqual(defaultAgentForm().config.subagents, []);
  const stored = hydrateAgentForm({ id: 'a-9', name: 'x', config: { subagents: ['code-reviewer'] } });
  assert.deepEqual(stored.config.subagents, ['code-reviewer']);
  const savedConfig = buildAgentPayload(stored).config as AgentEditorForm['config'];
  assert.deepEqual(savedConfig.subagents, ['code-reviewer'], 'save must carry the delegation list back');
  // legacy agents without the key hydrate to delegation-off, and garbage values
  // (a string, an object) never leak through as a non-string-array
  const legacy = hydrateAgentForm({ id: 'a-10', name: 'x', config: {} });
  assert.deepEqual(legacy.config.subagents, []);
  const dirty = hydrateAgentForm({ id: 'a-11', name: 'x', config: { subagents: ['ok', 7, null] } });
  assert.deepEqual(dirty.config.subagents, ['ok']);
});

// --- selection modes (AgentEditorModal.vue:3564-3602, 3640-3711) ---------------------

test('initKbSelectionMode prefers the stored mode, then knowledge_bases, then none', () => {
  assert.equal(initKbSelectionMode(withConfig({ kb_selection_mode: 'all', knowledge_bases: [] })), 'all');
  const noStoredMode = withConfig({ knowledge_bases: ['kb-1'] });
  noStoredMode.config.kb_selection_mode = undefined as never;
  assert.equal(initKbSelectionMode(noStoredMode), 'selected');
  assert.equal(initKbSelectionMode(withConfig({ kb_selection_mode: 'none', knowledge_bases: [] })), 'none');
  const empty = defaultAgentForm();
  empty.config.kb_selection_mode = undefined as never;
  assert.equal(initKbSelectionMode(empty), 'none');
});

test('initScopeSelectionMode derives selected from populated lists', () => {
  assert.equal(initScopeSelectionMode(undefined, []), 'none');
  assert.equal(initScopeSelectionMode(undefined, ['mcp-1']), 'selected');
  assert.equal(initScopeSelectionMode('all', ['mcp-1']), 'all');
});

test('applyKbSelectionMode clears knowledge_bases for all/none but keeps it for selected', () => {
  const form = withConfig({ knowledge_bases: ['kb-1', 'kb-2'] });
  applyKbSelectionMode(form, 'all');
  assert.deepEqual(form.config.knowledge_bases, []);
  assert.equal(form.config.kb_selection_mode, 'all');
  form.config.knowledge_bases = ['kb-1'];
  applyKbSelectionMode(form, 'selected');
  assert.deepEqual(form.config.knowledge_bases, ['kb-1']);
  applyKbSelectionMode(form, 'none');
  assert.deepEqual(form.config.knowledge_bases, []);
});

test('applyScopeSelectionMode clears the service/skill lists for all/none', () => {
  const form = withConfig({ mcp_services: ['m-1'], selected_skills: ['pdf'] });
  applyScopeSelectionMode(form, 'mcp', 'all');
  assert.deepEqual(form.config.mcp_services, []);
  form.config.mcp_services = ['m-1'];
  applyScopeSelectionMode(form, 'mcp', 'selected');
  assert.deepEqual(form.config.mcp_services, ['m-1']);
  applyScopeSelectionMode(form, 'skills', 'none');
  assert.deepEqual(form.config.selected_skills, []);
  assert.equal(form.config.skills_selection_mode, 'none');
});

// --- validation (AgentEditorModal.vue handleSave 4736-4799) ---------------------------

test('validation blocks on missing name, system prompt, context template and model with section jumps', () => {
  const empty = defaultAgentForm();
  empty.config.agent_mode = 'quick-answer';
  const issues = validateAgentForm(empty);
  assert.deepEqual(issues.map((issue) => issue.field), ['name', 'system_prompt', 'model_id']);
  assert.deepEqual(issues.map((issue) => issue.section), ['basic', 'prompts', 'model']);
  assert.equal(issues[0]!.messageKey, 'agent.editor.nameRequired');
  assert.equal(issues[1]!.messageKey, 'agent.editor.systemPromptRequired');
  assert.equal(issues[2]!.messageKey, 'agent.editor.modelRequired');

  const withPrompt = withConfig({ name: 'n', agent_mode: 'quick-answer', system_prompt: 'x', context_template: '', model_id: '' });
  const issues2 = validateAgentForm(withPrompt);
  assert.deepEqual(issues2.map((issue) => issue.field), ['context_template', 'model_id']);
  assert.equal(issues2[0]!.messageKey, 'agent.editor.contextTemplateRequired');
});

test('smart-reasoning mode drops the context-template requirement (Vue:4752-4757)', () => {
  const form = withConfig({ name: 'n', agent_mode: 'smart-reasoning', system_prompt: 'x', model_id: 'm-1' });
  assert.deepEqual(validateAgentForm(form), []);
});

test('builtin agents skip the name/system-prompt requirements but still need a model', () => {
  const builtin = withConfig({ model_id: '' });
  builtin.is_builtin = true;
  const issues = validateAgentForm(builtin);
  assert.deepEqual(issues.map((issue) => issue.field), ['model_id']);
});

test('rewrite and fallback prompts must keep the {{query}} placeholder (Vue:4764-4786)', () => {
  const form = withConfig({
    agent_mode: 'quick-answer',
    name: 'n', system_prompt: 'x', context_template: 'ctx', model_id: 'm',
    multi_turn_enabled: true, enable_rewrite: true,
    rewrite_prompt_user: 'answer the question without placeholder',
    fallback_strategy: 'model',
    fallback_prompt: 'no placeholder here',
  });
  const issues = validateAgentForm(form);
  assert.deepEqual(issues.map((issue) => issue.field), ['rewrite_prompt_user', 'fallback_prompt']);
  assert.equal(issues[0]!.messageKey, 'agent.editor.queryMissingInRewrite');
  assert.equal(issues[1]!.messageKey, 'agent.editor.queryMissingInFallback');
  form.config.rewrite_prompt_user = 'rewrite {{query}} please';
  form.config.fallback_prompt = 'fallback for {{query}}';
  assert.deepEqual(validateAgentForm(form), []);
  // empty prompts are fine — only custom prompts are checked (Vue:4767-4768)
  form.config.rewrite_prompt_user = '';
  form.config.fallback_prompt = '';
  assert.deepEqual(validateAgentForm(form), []);
});

// --- payload (AgentEditorModal.vue formData -> createAgent/updateAgent) --------------

test('buildAgentPayload emits the Vue formData shape and trims starter items', () => {
  const form = hydrateAgentForm({ id: 'a-9', name: '  助手  ', description: 'd', is_builtin: false, config: { agent_mode: 'quick-answer' } });
  form.config.question_suggestions.starters.items = ['  hello ', '', 'world'];
  const payload = buildAgentPayload(form);
  assert.equal(payload.id, 'a-9');
  assert.equal(payload.name, '助手');
  assert.equal(payload.description, 'd');
  assert.equal(payload.is_builtin, false);
  const config = payload.config as AgentEditorForm['config'];
  assert.equal(config.agent_mode, 'quick-answer');
  assert.deepEqual(config.question_suggestions.starters.items, ['hello', 'world']);
});

test('buildAgentPayload omits id in create mode', () => {
  const payload = buildAgentPayload(defaultAgentForm());
  assert.equal('id' in payload, false);
});

// --- nav rail (AgentEditorModal.vue navItems 2657-2684 + navGroups 2687-2713) --------

test('nav groups follow the Vue section list and order for quick-answer without KB', () => {
  const groups = buildNavGroups({ isAgentMode: false, hasKnowledgeBase: false });
  assert.deepEqual(groups.map((group) => group.key), ['basic', 'knowledge']);
  // quick-answer never renders persona (capability assembly is bypassed), so
  // the personalization section is gated by agent-mode like tools/skills
  assert.deepEqual(groups[0]!.items.map((item) => item.key), ['basic', 'prompts', 'model', 'conversation']);
  assert.deepEqual(groups[1]!.items.map((item) => item.key), ['knowledge', 'websearch']);
});

test('nav groups add retrieval with KB capability and tools/skills in agent mode', () => {
  const groups = buildNavGroups({ isAgentMode: true, hasKnowledgeBase: true });
  assert.deepEqual(groups.map((group) => group.key), ['basic', 'knowledge', 'capability']);
  // personalization (Octop M1, no Vue baseline) rides the basic group after
  // conversation, offered only in smart-reasoning mode; subagents (Octop M3
  // delegation) rides the capability group under the same agent-mode gate
  assert.deepEqual(groups[0]!.items.map((item) => item.key), ['basic', 'prompts', 'model', 'conversation', 'personalization']);
  assert.deepEqual(groups[1]!.items.map((item) => item.key), ['knowledge', 'retrieval', 'websearch']);
  assert.deepEqual(groups[2]!.items.map((item) => item.key), ['tools', 'skills', 'subagents']);
});

// --- tool requirement evaluation (frontend/src/utils/tool-capabilities.ts) -----------

test('tool requirements: base tools always ok, RAG tools need a RAG KB, wiki tools need wiki', () => {
  const rag = TOOL_CATALOG.find((tool) => tool.value === 'knowledge_search')!;
  const wiki = TOOL_CATALOG.find((tool) => tool.value === 'wiki_write_page')!;
  const thinking = TOOL_CATALOG.find((tool) => tool.value === 'thinking')!;
  assert.deepEqual(evaluateToolRequirement(thinking, noScope, false), { ok: true, missKind: 'none' });
  assert.deepEqual(evaluateToolRequirement(rag, noScope, false), { ok: false, missKind: 'needsKb' });
  assert.deepEqual(evaluateToolRequirement(rag, { ...noScope, vector: true }, true), { ok: true, missKind: 'none' });
  assert.deepEqual(evaluateToolRequirement(wiki, { ...noScope, vector: true }, true), { ok: false, missKind: 'needsWiki' });
  assert.deepEqual(evaluateToolRequirement(wiki, { ...noScope, wiki: true }, true), { ok: true, missKind: 'none' });
});

test('tool catalog matches the Vue order and danger flags (AgentEditorModal.vue:2379-2406)', () => {
  assert.equal(TOOL_CATALOG.length, 20);
  assert.deepEqual(TOOL_CATALOG.slice(0, 2).map((tool) => tool.value), ['thinking', 'todo_write']);
  assert.deepEqual(TOOL_CATALOG.filter((tool) => tool.danger).map((tool) => tool.value), ['wiki_write_page', 'wiki_replace_text', 'wiki_rename_page', 'wiki_delete_page']);
});

// --- KB option mapping (AgentEditorModal.vue mapKbToOption 3833-3847) -----------------

test('kbOptionFromRecord maps counts, type and capabilities with strategy fallback', () => {
  const doc = kbOptionFromRecord({ id: 'kb-1', name: '文档库', type: 'document', knowledge_count: 7, indexing_strategy: { vector_enabled: true, keyword_enabled: false } });
  assert.equal(doc.type, 'document');
  assert.equal(doc.count, 7);
  assert.equal(doc.ragEnabled, true);
  assert.equal(doc.wikiEnabled, false);
  const faq = kbOptionFromRecord({ id: 'kb-2', name: '问答库', type: 'faq', chunk_count: 3 });
  assert.equal(faq.type, 'faq');
  assert.equal(faq.count, 3);
  assert.equal(faq.ragEnabled, true);
  const withCaps = kbOptionFromRecord({ id: 'kb-3', name: 'W', capabilities: { vector: false, keyword: false, wiki: true, graph: false, faq: false } });
  assert.equal(withCaps.ragEnabled, false);
  assert.equal(withCaps.wikiEnabled, true);
});

// --- agent mode switch (AgentEditorModal.vue watch(agentMode) 3713-3767) -------------

test('switching to agent mode seeds KB tools and lifts max iterations', () => {
  const form = withConfig({ agent_mode: 'quick-answer', max_iterations: 1, allowed_tools: [] });
  applyAgentModeSwitch(form, 'smart-reasoning', fullScope, true);
  assert.deepEqual(form.config.allowed_tools, ['grep_chunks', 'knowledge_search', 'list_knowledge_chunks', 'get_document_info', 'wiki_search', 'wiki_read_page', 'wiki_read_source_doc', 'wiki_flag_issue']);
  assert.equal(form.config.max_iterations, 10);
  // vector-less scope seeds nothing
  const wikiOnly = withConfig({ allowed_tools: [] });
  applyAgentModeSwitch(wikiOnly, 'smart-reasoning', { ...noScope, wiki: true }, true);
  assert.deepEqual(wikiOnly.config.allowed_tools, ['wiki_search', 'wiki_read_page', 'wiki_read_source_doc', 'wiki_flag_issue']);
});

test('switching back to quick-answer clears tools and pins max_iterations to 1', () => {
  const form = withConfig({ agent_mode: 'smart-reasoning', allowed_tools: ['thinking'], max_iterations: 10 });
  applyAgentModeSwitch(form, 'quick-answer', fullScope, true);
  assert.deepEqual(form.config.allowed_tools, []);
  assert.equal(form.config.max_iterations, 1);
});

// --- skills rows (AgentEditorModal.vue catalogSkillRows 2090-2102) --------------------

test('catalogSkillRows classifies ready+enabled as selectable and missing installs as uninstalled', () => {
  const rows = catalogSkillRows([
    { id: 's1', name: 'PDF', installations: [{ sandboxConfigId: 'sb-1', status: 'ready', enabled: true }] },
    { id: 's2', name: 'Installing', installations: [{ sandboxConfigId: 'sb-1', status: 'installing', enabled: true }] },
    { id: 's3', name: 'Disabled', installations: [{ sandboxConfigId: 'sb-1', status: 'ready', enabled: false }] },
    { id: 's4', name: 'Removed', installations: [{ sandboxConfigId: 'sb-1', status: 'removed', enabled: true }] },
    { id: 's5', name: 'Elsewhere', installations: [{ sandboxConfigId: 'sb-2', status: 'ready', enabled: true }] },
  ], 'sb-1');
  assert.deepEqual(rows.map((row) => row.selectable), [true, false, false, false, false]);
  assert.deepEqual(rows.map((row) => row.installed), [true, true, true, false, false]);
});

// --- i18n fallback ----------------------------------------------------------------------

test('editorT resolves agent.* keys through formatMessage and agentEditor.* via the fallback table', () => {
  assert.equal(editorT('zh-CN', 'agent.editor.nameRequired'), '请输入智能体名称');
  assert.equal(editorT('zh-CN', 'agentEditor.navGroups.basic'), '基础');
  assert.equal(editorT('zh-CN', 'agentEditor.tools.statusInactive', { count: 2 }), '有 2 个已勾选工具在当前配置下无法生效');
  assert.equal(editorT('zh-CN', 'agentEditor.not.a.real.key'), 'agentEditor.not.a.real.key');
});

test('fallback table covers all five locales for nav/tool keys used by the modal', () => {
  const locales = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'];
  for (const locale of locales) {
    assert.ok(agentEditorFallback[locale], 'missing fallback locale ' + locale);
    assert.ok(agentEditorFallback[locale]!['agentEditor.navGroups.basic']);
    assert.ok(agentEditorFallback[locale]!['agentEditor.tools.knowledgeSearch']);
  }
});

// helper shared with the modal component
test('agentModeOf maps config values to the radio value', () => {
  assert.equal(agentModeOf({ agent_mode: 'smart-reasoning' }), 'smart-reasoning');
  assert.equal(agentModeOf({}), 'quick-answer');
  assert.equal(agentModeOf({ agent_mode: 'quick-answer' }), 'quick-answer');
});
