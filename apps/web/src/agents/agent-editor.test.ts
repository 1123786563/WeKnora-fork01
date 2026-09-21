import assert from 'node:assert/strict';
import test from 'node:test';
import {
  AGENT_FILE_TYPE_OPTIONS,
  AGENT_TYPE_PRESETS,
  agentModeOf,
  agentTypePresetDescription,
  agentTypePresetLabel,
  applyAgentModeSwitch,
  applyAgentTypePreset,
  applyCreateRetrievalDefaults,
  applyKbSelectionMode,
  applyScopeSelectionMode,
  buildAgentPayload,
  buildNavGroups,
  catalogSkillRows,
  defaultAgentForm,
  editorT,
  effectivePresetKbFilter,
  evaluateToolRequirement,
  findAgentTypePreset,
  hydrateAgentForm,
  incompatibleSelectedKbCount,
  initKbSelectionMode,
  initScopeSelectionMode,
  insertPlaceholderAtCursor,
  isNameSystemGenerated,
  kbOptionFromRecord,
  kbSatisfiesPresetFilter,
  makeEditorT,
  mcpOptionRows,
  needsRerankModel,
  promptPlaceholdersFor,
  seedCreateAgentForm,
  selectInitialModelId,
  tenantRetrievalDefaultsFromConfig,
  TOOL_CATALOG,
  validateAgentForm,
  type AgentEditorForm,
  type AgentTypePreset,
  type KbOption,
  type ToolCapabilityScope,
} from './agent-editor.ts';
import {
  AGENT_SYSTEM_PROMPT_TEMPLATE_LIST,
  resolveAgentSystemPromptResetTemplate,
} from './agent-type-presets.ts';
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
  assert.deepEqual(groups.map((group) => group.key), ['basic', 'knowledge', 'capability']);
  // quick-answer never renders persona (capability assembly is bypassed), so
  // the personalization section is gated by agent-mode like tools/skills
  assert.deepEqual(groups[0]!.items.map((item) => item.key), ['basic', 'prompts', 'model', 'conversation', 'suggestions']);
  assert.deepEqual(groups[1]!.items.map((item) => item.key), ['knowledge', 'websearch']);
  // multimodal stays available in quick-answer (Vue pushes it unconditionally)
  assert.deepEqual(groups[2]!.items.map((item) => item.key), ['multimodal']);
});

test('nav groups add retrieval with KB capability and tools/skills in agent mode', () => {
  const groups = buildNavGroups({ isAgentMode: true, hasKnowledgeBase: true });
  assert.deepEqual(groups.map((group) => group.key), ['basic', 'knowledge', 'capability']);
  // rail parity with Vue navItems 2657-2684: personalization (Octop persona)
  // and subagents (Octop delegation) stay off the rail in agent mode too —
  // both sections remain reachable via the initialSection deep link.
  assert.deepEqual(groups[0]!.items.map((item) => item.key), ['basic', 'prompts', 'model', 'conversation', 'suggestions']);
  assert.deepEqual(groups[1]!.items.map((item) => item.key), ['knowledge', 'retrieval', 'websearch']);
  assert.deepEqual(groups[2]!.items.map((item) => item.key), ['multimodal', 'tools', 'mcp', 'skills']);
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

// --- R484 D7: temperature description copy must stay Vue-identical ----------------------
// Vue zh-CN.ts desc.temperature (AgentEditorModal.vue:606) reads
// 「控制输出的随机性，0 最确定，1 最随机」— the R482 audit caught a React-side
// 「0 最稳定」 divergence; the lock below keeps the resolved copy on the Vue wording.
test('temperature description copy stays Vue-identical across locales (D7)', () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    const resolved = editorT(locale, 'agentEditor.desc.temperature');
    assert.ok(resolved.includes('确定') || locale !== 'zh-CN', locale + ' resolves');
    if (locale === 'zh-CN') {
      assert.equal(resolved, '控制输出的随机性，0 最确定，1 最随机');
      assert.ok(!resolved.includes('最稳定'), 'divergent 0-最稳定 wording must not resurface');
    }
  }
});

// --- R484 D8: supported file type options mirror Vue availableFileTypes ------------------
// Vue AgentEditorModal.vue:2564-2570 offers 7 options (pdf/docx/txt/md/csv/xlsx/jpg).
test('AGENT_FILE_TYPE_OPTIONS mirror the Vue availableFileTypes list (D8)', () => {
  assert.deepEqual(AGENT_FILE_TYPE_OPTIONS.map((option) => option.value), ['pdf', 'docx', 'txt', 'md', 'csv', 'xlsx', 'jpg']);
  assert.equal(editorT('zh-CN', 'agentEditor.fileTypes.label'), '支持的文件类型');
  assert.equal(editorT('zh-CN', 'agentEditor.fileTypes.desc'), '限制可选择的文件类型，留空表示支持所有类型');
  assert.equal(editorT('zh-CN', 'agentEditor.fileTypes.allTypes'), '全部类型');
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    assert.ok(agentEditorFallback[locale]?.['agentEditor.fileTypes.label'], 'fileTypes fallback missing for ' + locale);
  }
});

// --- R485 D1: nav rail carries the three missing Vue sections --------------------------
// Vue AgentEditorModal.vue navItems 2657-2684 registers suggestions (basic group,
// after conversation), multimodal + mcp (capability group; mcp gated by agent
// mode like tools/skills). The React rail omitted all three (audit D1).
test('nav groups register suggestions/multimodal/mcp like the Vue rail (D1)', () => {
  const quick = buildNavGroups({ isAgentMode: false, hasKnowledgeBase: false });
  assert.deepEqual(quick.map((group) => group.key), ['basic', 'knowledge', 'capability']);
  assert.deepEqual(quick[0]!.items.map((item) => item.key), ['basic', 'prompts', 'model', 'conversation', 'suggestions']);
  assert.deepEqual(quick[1]!.items.map((item) => item.key), ['knowledge', 'websearch']);
  // multimodal is NOT gated by agent mode in Vue (navItems push is unconditional)
  assert.deepEqual(quick[2]!.items.map((item) => item.key), ['multimodal']);

  const agent = buildNavGroups({ isAgentMode: true, hasKnowledgeBase: true });
  // rail parity: personalization/subagents stay off the rail in agent mode too
  assert.deepEqual(agent[0]!.items.map((item) => item.key), ['basic', 'prompts', 'model', 'conversation', 'suggestions']);
  assert.deepEqual(agent[1]!.items.map((item) => item.key), ['knowledge', 'retrieval', 'websearch']);
  // Vue capability order: multimodal, tools, mcp, skills
  assert.deepEqual(agent[2]!.items.map((item) => item.key), ['multimodal', 'tools', 'mcp', 'skills']);
});

// --- R485 D1: multimodal/attachment config fields mirror Vue defaultFormData ------------
// Vue AgentEditorModal.vue:2758-2771 defaults the attachment block.
test('default form carries the Vue multimodal/attachment defaults (D1)', () => {
  const form = defaultAgentForm();
  assert.equal(form.config.image_upload_enabled, false);
  assert.equal(form.config.vlm_model_id, '');
  assert.equal(form.config.image_storage_provider, '');
  assert.equal(form.config.attachment_image_understanding, false);
  assert.equal(form.config.attachment_ocr_max_pages, 0);
  assert.equal(form.config.attachment_parse_wait_timeout_sec, 0);
  assert.equal(form.config.audio_upload_enabled, false);
  assert.equal(form.config.asr_model_id, '');
  assert.deepEqual(form.config.chat_parser_engine_rules, []);
});

test('hydrateAgentForm keeps stored multimodal config over the defaults', () => {
  const form = hydrateAgentForm({
    id: 'a-9',
    name: '多模态',
    description: '',
    is_builtin: false,
    config: {
      agent_mode: 'smart-reasoning',
      image_upload_enabled: true,
      vlm_model_id: 'm-vlm',
      attachment_ocr_max_pages: 12,
      audio_upload_enabled: true,
      asr_model_id: 'm-asr',
      attachment_parse_wait_timeout_sec: 120,
      image_storage_provider: 'minio',
      chat_parser_engine_rules: [{ extensions: ['.pdf'], engine: 'mineru' }],
    },
  });
  assert.equal(form.config.image_upload_enabled, true);
  assert.equal(form.config.vlm_model_id, 'm-vlm');
  assert.equal(form.config.attachment_ocr_max_pages, 12);
  assert.equal(form.config.audio_upload_enabled, true);
  assert.equal(form.config.asr_model_id, 'm-asr');
  assert.equal(form.config.attachment_parse_wait_timeout_sec, 120);
  assert.equal(form.config.image_storage_provider, 'minio');
  assert.deepEqual(form.config.chat_parser_engine_rules, [{ extensions: ['.pdf'], engine: 'mineru' }]);
});

// --- R485 D1: MCP service option rows (Vue mcpOptions 2039-2068) ------------------------
test('mcpOptionRows flags enabled services, disabled selections and ghost ids (D1)', () => {
  const t = makeEditorT('zh-CN');
  const rows = mcpOptionRows(
    [
      { id: 'mcp-a', name: '服务A', enabled: true },
      { id: 'mcp-b', name: '服务B', enabled: false },
    ],
    ['mcp-a', 'mcp-b', 'mcp-ghost'],
    t,
  );
  assert.deepEqual(rows[0], { label: '服务A', value: 'mcp-a' });
  assert.equal(rows[1]!.label, '服务B (已禁用)');
  assert.equal(rows[1]!.disabled, true);
  assert.equal(rows[2]!.label, '不可用服务');
  assert.equal(rows[2]!.disabled, true);
});

// --- R485 D2: agent type preset options mirror config/agent_type_presets.yaml ----------
test('agent type preset options mirror the shipped YAML catalog (D2)', () => {
  assert.deepEqual(AGENT_TYPE_PRESETS.map((preset) => preset.id), ['rag-qa', 'wiki-qa', 'hybrid-rag-wiki', 'data-analysis', 'custom']);
  assert.equal(agentTypePresetLabel(findAgentTypePreset('rag-qa')!, 'zh-CN'), 'RAG 问答');
  assert.equal(agentTypePresetLabel(findAgentTypePreset('custom')!, 'zh-CN'), '自定义');
  // en-US is absent from the YAML i18n maps -> falls back to the default block
  assert.equal(agentTypePresetLabel(findAgentTypePreset('rag-qa')!, 'en-US'), 'RAG Q&A');
  assert.equal(agentTypePresetDescription(findAgentTypePreset('rag-qa')!, 'zh-CN'), '基于文档分块的检索式问答，适合未启用 Wiki 的文档 / FAQ 知识库。');
  assert.equal(findAgentTypePreset('custom')!.config, undefined, 'custom preset carries no auto-fill config');
  assert.equal(findAgentTypePreset('nope'), null);
});

test('isNameSystemGenerated treats preset default names and blanks as safe to override', () => {
  const t = makeEditorT('zh-CN');
  assert.equal(isNameSystemGenerated('', t, 'zh-CN'), true);
  assert.equal(isNameSystemGenerated('我的RAG 问答', t, 'zh-CN'), true);
  assert.equal(isNameSystemGenerated('自己起的名字', t, 'zh-CN'), false);
});

// --- R485 D3+D10: create-mode rag-qa prefill (Vue 3514-3536) ----------------------------
// Vue opens the create modal with the default agent_type ('rag-qa') preset
// applied: name/description/system prompt/tools prefilled so the modal matches
// the type dropdown from the first paint (audit D3), which also seeds the
// effective-tools preview with 4 RAG tools (audit D10).
test('seedCreateAgentForm applies the rag-qa preset to the create form (D3+D10)', () => {
  const t = makeEditorT('zh-CN');
  const form = seedCreateAgentForm(t);
  assert.equal(form.config.agent_type, 'rag-qa');
  assert.equal(form.name, '我的RAG 问答');
  assert.equal(form.description, '基于文档分块的检索式问答，适合未启用 Wiki 的文档 / FAQ 知识库。');
  assert.equal(form.config.system_prompt_id, 'progressive_rag_agent');
  assert.ok(form.config.system_prompt.startsWith('You are WeKnora'), 'system prompt body must be resolved from the template');
  assert.deepEqual(form.config.allowed_tools, ['knowledge_search', 'grep_chunks', 'list_knowledge_chunks', 'get_document_info']);
  assert.equal(form.config.max_iterations, 30);
  assert.equal(form.config.temperature, 0.7);
  assert.equal(form.config.faq_priority_enabled, true);
  assert.equal(form.config.retain_retrieval_history, false);
  assert.equal(form.config.kb_selection_mode, 'all');
});

test('applyAgentTypePreset strong-syncs supported_file_types and kb mode across types (D2)', () => {
  const form = defaultAgentForm();
  applyAgentTypePreset(form, findAgentTypePreset('data-analysis')!);
  assert.deepEqual(form.config.supported_file_types, ['csv', 'xlsx']);
  assert.equal(form.config.web_search_enabled, false);
  assert.equal(form.config.system_prompt_id, 'data_analyst');
  assert.ok(form.config.system_prompt.includes('DuckDB'));
  // switching back to rag-qa must clear the residue (Vue strong-sync comment)
  applyAgentTypePreset(form, findAgentTypePreset('rag-qa')!);
  assert.deepEqual(form.config.supported_file_types, []);
  // wiki preset swaps the tool list + system prompt template
  applyAgentTypePreset(form, findAgentTypePreset('wiki-qa')!);
  assert.deepEqual(form.config.allowed_tools, ['wiki_search', 'wiki_read_page', 'wiki_read_source_doc', 'wiki_flag_issue']);
  assert.equal(form.config.system_prompt_id, 'wiki_researcher');
});

// --- R485 D1/D2/D3: i18n fallback covers the new section keys ---------------------------
test('fallback table carries the three-section + agent-type keys (D1/D2)', () => {
  const t = makeEditorT('zh-CN');
  assert.equal(t('agentEditor.questionSuggestions.navLabel'), '问题推荐');
  assert.equal(t('agentEditor.imageUpload.navLabel'), '附件上传');
  assert.equal(t('agentEditor.mcp.label'), 'MCP 服务');
  assert.equal(t('agentEditor.agentType.label'), '智能体类型');
  assert.equal(t('agentEditor.agentType.defaultNamePattern', { label: 'RAG 问答' }), '我的RAG 问答');
  assert.equal(t('agentEditor.questionSuggestions.title'), '对话问题推荐');
  assert.equal(t('agentEditor.imageUpload.sectionTitle'), '附件上传');
  assert.equal(t('agentEditor.imageUpload.vlmModel'), 'VLM 模型');
  assert.equal(t('agentEditor.audioUpload.label'), '语音上传');
  assert.equal(t('agentEditor.chatParser.label'), '聊天附件解析策略');
  assert.equal(t('agentEditor.mcp.authWaitTimeout'), '授权等待超时（秒）');
  for (const locale of ['zh-CN', 'en-US'] as const) {
    assert.ok(agentEditorFallback[locale]?.['agentEditor.questionSuggestions.navLabel'], 'suggestions nav fallback missing for ' + locale);
    assert.ok(agentEditorFallback[locale]?.['agentEditor.imageUpload.navLabel'], 'multimodal nav fallback missing for ' + locale);
    assert.ok(agentEditorFallback[locale]?.['agentEditor.mcp.label'], 'mcp nav fallback missing for ' + locale);
    assert.ok(agentEditorFallback[locale]?.['agentEditor.agentType.label'], 'agentType fallback missing for ' + locale);
  }
});

// --- R485 D5: creation-time model prefill (Vue applyDefaultModelsIfEmpty 2854-2862) ------
test('selectInitialModelId prefers the declared default then the first active model (D5)', () => {
  const models = [
    { id: 'm1', type: 'KnowledgeQA' },
    { id: 'm2', type: 'KnowledgeQA', is_default: true },
    { id: 'm3', type: 'KnowledgeQA', is_default: true, status: 'inactive' },
    { id: '', type: 'KnowledgeQA' },
  ];
  assert.equal(selectInitialModelId(models, 'KnowledgeQA'), 'm2');
  assert.equal(selectInitialModelId(models, 'Rerank'), null);
  assert.equal(selectInitialModelId([{ id: 'r1', type: 'Rerank' }], 'Rerank'), 'r1');
});

test('seedCreateAgentForm prefills empty chat/rerank models without clobbering set ones (D5)', () => {
  const t = makeEditorT('zh-CN');
  const models = [
    { id: 'm-chat', type: 'KnowledgeQA' },
    { id: 'm-rerank', type: 'Rerank' },
  ];
  const form = seedCreateAgentForm(t, 'zh-CN', models);
  assert.equal(form.config.model_id, 'm-chat');
  assert.equal(form.config.rerank_model_id, 'm-rerank');
});

// --- R485 D6: rerank required derivation (Vue needsRerankModel 3373-3388) ----------------
test('needsRerankModel derives from the KB scope rag capability (D6)', () => {
  const kbs: KbOption[] = [
    { label: 'A', value: 'kb-1', type: 'document', count: 0, shared: false, ragEnabled: true, wikiEnabled: false },
    { label: 'B', value: 'kb-2', type: 'document', count: 0, shared: false, ragEnabled: false, wikiEnabled: true },
  ];
  assert.equal(needsRerankModel('all', kbs, []), true, 'any RAG kb under all');
  assert.equal(needsRerankModel('selected', kbs, ['kb-2']), false, 'selected scope has no RAG kb');
  assert.equal(needsRerankModel('selected', kbs, ['kb-1', 'kb-2']), true, 'selected scope includes a RAG kb');
  assert.equal(needsRerankModel('none', kbs, []), false, 'no kb scope -> never required');
});

// --- R486 D4: prompt placeholder catalogue (internal/types/placeholder.go static port) ---
test('promptPlaceholdersFor mirrors the backend PlaceholdersByField sets (D4)', () => {
  assert.deepEqual(
    promptPlaceholdersFor('agent_system_prompt').map((p) => p.name),
    ['knowledge_bases', 'web_search_status', 'current_time', 'language'],
  );
  assert.deepEqual(
    promptPlaceholdersFor('system_prompt').map((p) => p.name),
    ['query', 'contexts', 'current_time', 'current_week', 'language'],
  );
  assert.deepEqual(
    promptPlaceholdersFor('context_template').map((p) => p.name),
    ['query', 'contexts', 'current_time', 'current_week', 'language'],
  );
  // every definition carries the backend label + description pair
  for (const def of promptPlaceholdersFor('agent_system_prompt')) {
    assert.ok(def.label, 'label missing for ' + def.name);
    assert.ok(def.description, 'description missing for ' + def.name);
  }
});

// --- R486 D4: cursor insert (Vue insertPlaceholder tag-click path 4100-4146) ------------
test('insertPlaceholderAtCursor splices {{name}} at the caret and returns the new caret (D4)', () => {
  // Vue tail: cursorPos + name.length + 4 (the {{ + }} braces)
  assert.deepEqual(insertPlaceholderAtCursor('ab', 1, 'query'), { value: 'a{{query}}b', cursorPos: 1 + 5 + 4 });
  assert.deepEqual(insertPlaceholderAtCursor('ab', 0, 'language'), { value: '{{language}}ab', cursorPos: 8 + 4 });
  assert.deepEqual(insertPlaceholderAtCursor('ab', 2, 'query'), { value: 'ab{{query}}', cursorPos: 2 + 5 + 4 });
});

// --- R486 D4: agent system prompt reset-default resolution (Vue 4689-4712) --------------
test('resolveAgentSystemPromptResetTemplate prefers the preset-bound template then the global default (D4)', () => {
  const wiki = resolveAgentSystemPromptResetTemplate('wiki-qa');
  assert.equal(wiki?.id, 'wiki_researcher');
  assert.ok(wiki!.content.startsWith('<role>'), 'wiki template body expected');
  // custom (or unknown) types fall back to the global default template
  const fallback = resolveAgentSystemPromptResetTemplate('custom');
  assert.equal(fallback?.id, 'progressive_rag_agent');
  assert.ok(fallback!.content.startsWith('You are WeKnora'), 'default template body expected');
  const analyst = resolveAgentSystemPromptResetTemplate('data-analysis');
  assert.equal(analyst?.id, 'data_analyst');
});

test('agent system prompt template list carries all 7 yaml entries in Vue order (D4 + R486 verify DIFF-A)', () => {
  assert.deepEqual(
    AGENT_SYSTEM_PROMPT_TEMPLATE_LIST.map((tpl) => tpl.id),
    ['pure_agent', 'progressive_rag_agent', 'data_analyst', 'wiki_researcher', 'wiki_fixer', 'hybrid_rag_wiki_agent', 'skill_installer'],
  );
  assert.equal(AGENT_SYSTEM_PROMPT_TEMPLATE_LIST.find((tpl) => tpl.default)?.id, 'progressive_rag_agent');
  for (const tpl of AGENT_SYSTEM_PROMPT_TEMPLATE_LIST) {
    assert.ok(tpl.name.zh, 'zh name missing for ' + tpl.id);
    assert.ok(tpl.description.zh, 'zh description missing for ' + tpl.id);
    assert.ok(tpl.content.length > 100, 'body missing for ' + tpl.id);
  }
});

// --- R486 D9: tenant retrieval-config defaults (Vue 2340-2344 + 3912-3917) --------------
test('tenantRetrievalDefaultsFromConfig keeps the Vue || / !== undefined override semantics (D9)', () => {
  const base = tenantRetrievalDefaultsFromConfig(null);
  assert.deepEqual(base, { embeddingTopK: 10, keywordThreshold: 0.3, vectorThreshold: 0.5, rerankTopK: 5, rerankThreshold: 0.5 });
  // thresholds use !== undefined: an explicit 0 overrides the built-in default
  assert.deepEqual(
    tenantRetrievalDefaultsFromConfig({ keyword_threshold: 0, vector_threshold: 0 }),
    { embeddingTopK: 10, keywordThreshold: 0, vectorThreshold: 0, rerankTopK: 5, rerankThreshold: 0.5 },
  );
  // top-k fields use truthiness: 0 keeps the default
  assert.deepEqual(
    tenantRetrievalDefaultsFromConfig({ embedding_top_k: 50, rerank_top_k: 0 }),
    { embeddingTopK: 50, keywordThreshold: 0.3, vectorThreshold: 0.5, rerankTopK: 5, rerankThreshold: 0.5 },
  );
  assert.deepEqual(
    tenantRetrievalDefaultsFromConfig({ rerank_threshold: 1.5 }),
    { embeddingTopK: 10, keywordThreshold: 0.3, vectorThreshold: 0.5, rerankTopK: 5, rerankThreshold: 1.5 },
  );
});

test('applyCreateRetrievalDefaults writes the tenant defaults onto a fresh create form (D9)', () => {
  const t = makeEditorT('zh-CN');
  const form = seedCreateAgentForm(t);
  applyCreateRetrievalDefaults(form, { embeddingTopK: 50, keywordThreshold: 0, vectorThreshold: 0.2, rerankTopK: 8, rerankThreshold: 0.6 });
  assert.equal(form.config.embedding_top_k, 50);
  assert.equal(form.config.keyword_threshold, 0);
  assert.equal(form.config.vector_threshold, 0.2);
  assert.equal(form.config.rerank_top_k, 8);
  assert.equal(form.config.rerank_threshold, 0.6);
});

// --- R486 D2/KB warn: preset KB filter + incompatible selected count (Vue 3190-3288) ----
test('effectivePresetKbFilter derives any_of from the preset tools and merges yaml filters (KB warn)', () => {
  // rag-qa writes no kb_filter: the filter is derived from its RAG tools
  assert.deepEqual(effectivePresetKbFilter(findAgentTypePreset('rag-qa')), { any_of: ['vector', 'keyword'], all_of: [], none_of: [] });
  // data-analysis keeps the derived any_of plus the yaml none_of faq
  assert.deepEqual(effectivePresetKbFilter(findAgentTypePreset('data-analysis')), { any_of: ['vector', 'keyword'], all_of: [], none_of: ['faq'] });
  // custom applies nothing -> no filter
  assert.equal(effectivePresetKbFilter(findAgentTypePreset('custom')), null);
  assert.equal(effectivePresetKbFilter(null), null);
});

test('kbSatisfiesPresetFilter evaluates a KB against the derived filter (KB warn)', () => {
  const wikiOnly: KbOption = { label: 'w', value: 'kb-w', type: 'document', count: 0, shared: false, ragEnabled: false, wikiEnabled: true };
  const rag: KbOption = { label: 'r', value: 'kb-r', type: 'document', count: 0, shared: false, ragEnabled: true, wikiEnabled: false };
  const faq: KbOption = { label: 'f', value: 'kb-f', type: 'faq', count: 0, shared: false, ragEnabled: true, wikiEnabled: false };
  assert.equal(kbSatisfiesPresetFilter(rag, findAgentTypePreset('rag-qa')).ok, true);
  assert.equal(kbSatisfiesPresetFilter(wikiOnly, findAgentTypePreset('rag-qa')).ok, false);
  assert.equal(kbSatisfiesPresetFilter(wikiOnly, findAgentTypePreset('wiki-qa')).ok, true);
  assert.equal(kbSatisfiesPresetFilter(faq, findAgentTypePreset('data-analysis')).ok, false, 'faq is excluded by none_of');
  assert.equal(kbSatisfiesPresetFilter(rag, null).ok, true, 'no preset -> everything satisfies');
});

test('incompatibleSelectedKbCount counts selected KBs the new preset disables (KB warn)', () => {
  const t = makeEditorT('zh-CN');
  const kbs: KbOption[] = [
    { label: 'wiki', value: 'kb-w', type: 'document', count: 0, shared: false, ragEnabled: false, wikiEnabled: true },
    { label: 'rag', value: 'kb-r', type: 'document', count: 0, shared: false, ragEnabled: true, wikiEnabled: false },
  ];
  // switching to rag-qa disables the wiki-only selection
  assert.equal(incompatibleSelectedKbCount('selected', ['kb-w', 'kb-r'], kbs, findAgentTypePreset('rag-qa'), 'smart-reasoning'), 1);
  assert.equal(incompatibleSelectedKbCount('selected', ['kb-r'], kbs, findAgentTypePreset('rag-qa'), 'smart-reasoning'), 0);
  // quick-answer mode has no preset but still disables wiki-only KBs
  assert.equal(incompatibleSelectedKbCount('selected', ['kb-w'], kbs, null, 'quick-answer'), 1);
  // outside the selected scope nothing counts
  assert.equal(incompatibleSelectedKbCount('all', ['kb-w'], kbs, findAgentTypePreset('rag-qa'), 'smart-reasoning'), 0);
});

// R490 B4 — the fallback table is static (no vue-i18n literal {'{{'} syntax),
// so the hint must carry the already-unescaped braces Vue renders at runtime.
test('agentEditor.placeholders.hint renders a clean {{ in every fallback locale', () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    const hint = agentEditorFallback[locale]['agentEditor.placeholders.hint'];
    assert.ok(hint.includes('{{'), `${locale} keeps the {{ trigger`);
    assert.ok(!hint.includes(`{'`), `${locale} drops the vue-i18n literal quoting`);
  }
});

// --- R491: runtime catalogs override the vendored static tables ----------------------
// The editor fetches type-presets / prompt-templates / placeholders at runtime
// (Vue editorResources parity); the pure helpers take the runtime data as an
// optional trailing argument and keep the static catalog as the default so
// pre-fetch renders and the R485/R486 tests keep working.

test('promptPlaceholdersFor prefers runtime placeholder definitions and keeps the static set otherwise', () => {
  const runtime = {
    agent_system_prompt: [
      { name: 'knowledge_bases', label: '知识库列表', description: '自动格式化的知识库列表' },
      { name: 'extra_var', label: '新增变量', description: '后端新增' },
    ],
    context_template: [],
  };
  const defs = promptPlaceholdersFor('agent_system_prompt', runtime);
  assert.deepEqual(defs.map((def) => def.name), ['knowledge_bases', 'extra_var']);
  // an empty runtime section is treated as "not fetched" -> static fallback
  assert.equal(promptPlaceholdersFor('context_template', runtime)[0]!.name, 'query');
  // no runtime data at all -> static catalog (R486 behaviour)
  assert.equal(promptPlaceholdersFor('agent_system_prompt', null)[0]!.name, 'knowledge_bases');
  assert.equal(promptPlaceholdersFor('system_prompt')[1]!.name, 'contexts');
});

test('findAgentTypePreset / isNameSystemGenerated accept a runtime preset list', () => {
  const t = makeEditorT('zh-CN');
  const runtimePresets = [
    { id: 'custom', i18n: { default: { label: 'Custom', description: '' }, 'zh-CN': { label: '自定义', description: '自定义描述' } } },
    { id: 'ops-agent', i18n: { default: { label: 'Ops', description: 'Ops preset' }, 'zh-CN': { label: '运维', description: '运维预设' } } },
  ];
  assert.equal(findAgentTypePreset('ops-agent', runtimePresets)!.id, 'ops-agent');
  assert.equal(findAgentTypePreset('rag-qa', runtimePresets), null, 'runtime list shadows the static one');
  assert.equal(findAgentTypePreset('rag-qa')!.id, 'rag-qa', 'static list stays the default');

  // 我的运维 is generated from the runtime preset -> safe to override
  assert.equal(isNameSystemGenerated('我的运维', t, 'zh-CN', runtimePresets), true);
  // the static 我的RAG 问答 is unknown to the runtime list -> user-owned
  assert.equal(isNameSystemGenerated('我的RAG 问答', t, 'zh-CN', runtimePresets), false);
});

test('applyAgentTypePreset reads the system prompt body from the runtime template list', () => {
  const form = defaultAgentForm();
  const runtimePreset: AgentTypePreset = {
    id: 'rag-qa',
    i18n: { default: { label: 'RAG Q&A', description: '' } },
    config: {
      system_prompt_id: 'tenant_prompt',
      temperature: 0.4,
      allowed_tools: ['knowledge_search'],
      kb_selection_mode: 'all',
    },
  };
  const runtimeTemplates = [
    { id: 'tenant_prompt', name: '租户模板', description: '', content: 'tenant body', default: true },
  ];
  applyAgentTypePreset(form, runtimePreset, runtimeTemplates);
  assert.equal(form.config.system_prompt, 'tenant body');
  assert.equal(form.config.system_prompt_id, 'tenant_prompt');
  assert.equal(form.config.temperature, 0.4);

  // unknown template id in the runtime list -> clear so the change is visible
  const unknown = { ...form, config: { ...form.config } };
  applyAgentTypePreset(unknown, { ...runtimePreset, config: { system_prompt_id: 'missing' } }, runtimeTemplates);
  assert.equal(unknown.config.system_prompt, '');

  // default template source stays the vendored catalog
  const form2 = defaultAgentForm();
  applyAgentTypePreset(form2, findAgentTypePreset('rag-qa')!);
  assert.ok(form2.config.system_prompt.startsWith('You are WeKnora, an assistant'));
});

test('resolveAgentSystemPromptResetTemplate honours runtime presets and templates', () => {
  const runtimePresets: AgentTypePreset[] = [
    {
      id: 'wiki-qa',
      i18n: { default: { label: 'Wiki Q&A', description: '' } },
      config: { system_prompt_id: 'tenant_wiki' },
    },
  ];
  const runtimeTemplates = [
    { id: 'tenant_wiki', name: '租户Wiki模板', description: '', content: 'tenant wiki body' },
    { id: 'tenant_default', name: '租户默认', description: '', content: 'tenant default body', default: true },
  ];
  // preset-bound template wins over the global default (Vue 4691-4703)
  assert.equal(resolveAgentSystemPromptResetTemplate('wiki-qa', runtimePresets, runtimeTemplates)!.id, 'tenant_wiki');
  // custom / unknown type -> global default entry, then first row
  assert.equal(resolveAgentSystemPromptResetTemplate('custom', runtimePresets, runtimeTemplates)!.id, 'tenant_default');
  assert.equal(resolveAgentSystemPromptResetTemplate(undefined, runtimePresets, runtimeTemplates)!.id, 'tenant_default');
  // static defaults stay intact when nothing runtime is passed
  assert.equal(resolveAgentSystemPromptResetTemplate('wiki-qa')!.id, 'wiki_researcher');
});

test('seedCreateAgentForm applies the runtime default preset (backend rag-qa without retain_retrieval_history)', () => {
  const t = makeEditorT('zh-CN');
  const runtimePresets: AgentTypePreset[] = [
    {
      id: 'rag-qa',
      i18n: { default: { label: 'RAG Q&A', description: 'backend desc' }, 'zh-CN': { label: 'RAG 问答', description: '后端描述' } },
      config: {
        system_prompt_id: 'progressive_rag_agent',
        temperature: 0.55,
        allowed_tools: ['knowledge_search'],
        kb_selection_mode: 'all',
      },
    },
  ];
  const runtimeTemplates = [
    { id: 'progressive_rag_agent', name: '渐进式 RAG 智能体', description: '', content: 'backend rag body', default: true },
  ];
  const form = seedCreateAgentForm(t, 'zh-CN', [], runtimePresets, runtimeTemplates);
  assert.equal(form.config.system_prompt, 'backend rag body');
  assert.equal(form.config.temperature, 0.55);
  assert.equal(form.name, '我的RAG 问答');
  assert.equal(form.description, '后端描述');
  // retain_retrieval_history is untouched when the backend preset omits it
  assert.equal(form.config.retain_retrieval_history, false);
});
