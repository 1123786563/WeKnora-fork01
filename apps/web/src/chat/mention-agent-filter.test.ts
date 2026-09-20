import assert from 'node:assert/strict';
import test from 'node:test';

import {
  deriveKbFilterForAgent,
  isKbModelReady,
  kbSatisfiesAgentRequirements,
  kbToScopeCaps,
  mergeSharedKbsForMention,
  resolveMentionAgentKbScope,
  toolsConsumeFiles,
} from './mention-agent-filter.ts';

// --- deriveKbFilterForAgent (frontend/src/utils/tool-capabilities.ts) ----------

test('quick-answer mode derives the implicit vector|keyword requirement', () => {
  assert.deepEqual(deriveKbFilterForAgent('quick-answer', []), { any_of: ['vector', 'keyword'] });
});

test('allowed tools union their capability requirements into the filter', () => {
  const filter = deriveKbFilterForAgent('smart-reasoning', ['wiki_search', 'knowledge_search']);
  assert.ok(filter);
  assert.deepEqual([...filter.any_of].sort(), ['keyword', 'vector', 'wiki']);
});

test('no mode requirement and no KB-dependent tools yield no filter', () => {
  assert.equal(deriveKbFilterForAgent('smart-reasoning', ['thinking', 'todo_write']), null);
  assert.equal(deriveKbFilterForAgent('', []), null);
});

// --- kbSatisfiesAgentRequirements ----------------------------------------------

test('a KB satisfies the agent when it exposes a required capability', () => {
  assert.equal(kbSatisfiesAgentRequirements({ vector: true }, 'quick-answer', []), true);
  assert.equal(kbSatisfiesAgentRequirements({ wiki: true }, 'quick-answer', ['wiki_search']), true);
  assert.equal(kbSatisfiesAgentRequirements(null, 'quick-answer', []), false);
  assert.equal(kbSatisfiesAgentRequirements({ wiki: true }, '', []), true);
});

// --- toolsConsumeFiles ----------------------------------------------------------

test('toolsConsumeFiles mirrors the Vue permissive fallback', () => {
  assert.equal(toolsConsumeFiles([]), true);
  assert.equal(toolsConsumeFiles(undefined), true);
  assert.equal(toolsConsumeFiles(['wiki_search', 'wiki_read_page']), false, 'pure wiki tools cannot consume @ file ids');
  assert.equal(toolsConsumeFiles(['wiki_search', 'knowledge_search']), true);
  assert.equal(toolsConsumeFiles(['some_unknown_mcp_tool']), true, 'unknown tools are treated as file-consuming');
});

// --- kbToScopeCaps (Input-field.vue 400-419) ------------------------------------

test('kbToScopeCaps prefers explicit capabilities, then indexing_strategy, then faq type', () => {
  assert.deepEqual(kbToScopeCaps({ capabilities: { vector: true } }), { vector: true, keyword: false, wiki: false, graph: false, faq: false });
  assert.deepEqual(kbToScopeCaps({ indexing_strategy: { keyword_enabled: true, wiki_enabled: true } }), { vector: false, keyword: true, wiki: true, graph: false, faq: false });
  assert.deepEqual(kbToScopeCaps({}), { vector: false, keyword: false, wiki: false, graph: false, faq: false });
  assert.deepEqual(kbToScopeCaps({ type: 'faq' }), { vector: false, keyword: false, wiki: false, graph: false, faq: true });
});

// --- resolveMentionAgentKbScope (Input-field.vue 1288-1399) ---------------------

const KB_ROWS = [
  { id: 'kb-plain', name: 'Plain KB' }, // no capability payload -> filtered under quick-answer
  { id: 'kb-rag', name: 'RAG KB', capabilities: { vector: true } },
  { id: 'kb-wiki', name: 'Wiki KB', capabilities: { wiki: true } },
];

test('no agent config leaves the KB list and file loading unrestricted', () => {
  const scope = resolveMentionAgentKbScope(undefined, KB_ROWS);
  assert.equal(scope.hasAgentConfig, false);
  assert.equal(scope.scopedKbs.length, 3);
  assert.equal(scope.allowedKbIds, null);
  assert.equal(scope.shouldLoadFiles, true);
});

test('quick-answer agent with kb_selection_mode all drops KBs without vector/keyword', () => {
  // R489 D12 repro shape: builtin quick-answer + KBs lacking capability bits.
  const scope = resolveMentionAgentKbScope({ agent_mode: 'quick-answer', kb_selection_mode: 'all' }, KB_ROWS);
  assert.equal(scope.kbMode, 'all');
  assert.deepEqual(scope.scopedKbs.map((kb) => kb.id), ['kb-rag']);
  assert.deepEqual([...scope.allowedKbIds ?? []], ['kb-rag']);
  assert.deepEqual(scope.kbFilter, { any_of: ['vector', 'keyword'] });
  assert.equal(scope.shouldLoadFiles, true);
});

test('default kb_selection_mode (missing) behaves as all', () => {
  const scope = resolveMentionAgentKbScope({ agent_mode: 'quick-answer' }, KB_ROWS);
  assert.equal(scope.kbMode, 'all');
  assert.deepEqual(scope.scopedKbs.map((kb) => kb.id), ['kb-rag']);
});

test('kb_selection_mode none empties the list and disables file loading', () => {
  const scope = resolveMentionAgentKbScope({ agent_mode: 'quick-answer', kb_selection_mode: 'none' }, KB_ROWS);
  assert.deepEqual(scope.scopedKbs, []);
  assert.equal(scope.allowedKbIds?.size, 0);
  assert.equal(scope.shouldLoadFiles, false);
});

test('kb_selection_mode selected narrows to the configured ids without a compatibility pass', () => {
  const scope = resolveMentionAgentKbScope(
    { agent_mode: 'quick-answer', kb_selection_mode: 'selected', knowledge_bases: ['kb-plain', 'kb-wiki'] },
    KB_ROWS,
  );
  // 'selected' trusts the editor picks: even incompatible KBs survive.
  assert.deepEqual(scope.scopedKbs.map((kb) => kb.id), ['kb-plain', 'kb-wiki']);
  assert.equal(scope.shouldLoadFiles, true);
});

test('wiki-only tools block the file gate and require wiki-capable KBs', () => {
  const scope = resolveMentionAgentKbScope(
    { agent_mode: 'smart-reasoning', kb_selection_mode: 'all', allowed_tools: ['wiki_search', 'wiki_read_page'] },
    KB_ROWS,
  );
  assert.deepEqual(scope.scopedKbs.map((kb) => kb.id), ['kb-wiki']);
  assert.equal(scope.shouldLoadFiles, false, 'no tool consumes @ file ids');
});

test('agent supported_file_types ride along for the document search', () => {
  const scope = resolveMentionAgentKbScope({ agent_mode: 'quick-answer', supported_file_types: ['pdf', 'docx'] }, KB_ROWS);
  assert.deepEqual(scope.fileTypes, ['pdf', 'docx']);
});

// --- isKbModelReady (chatResources.ts 23-29, the D12 initialization filter) ---

test('isKbModelReady drops KBs without a summary LLM', () => {
  assert.equal(isKbModelReady({ id: 'kb-1', name: 'KB', summary_model_id: '' }), false);
  assert.equal(isKbModelReady({ id: 'kb-1', name: 'KB' }), false);
});

test('isKbModelReady requires an embedding model for chunk-indexed KBs', () => {
  // The live parity tenant FAQ shape: summary LLM set, embedding missing.
  assert.equal(isKbModelReady({ summary_model_id: 'llm', embedding_model_id: '', indexing_strategy: { vector_enabled: true, keyword_enabled: true } }), false);
  assert.equal(isKbModelReady({ summary_model_id: 'llm', embedding_model_id: '', indexing_strategy: undefined }), false, 'no strategy defaults to chunk indexing');
  assert.equal(isKbModelReady({ summary_model_id: 'llm', embedding_model_id: 'emb', indexing_strategy: { vector_enabled: true } }), true);
});

test('isKbModelReady lets wiki-only KBs through without an embedding model', () => {
  assert.equal(isKbModelReady({ summary_model_id: 'llm', embedding_model_id: '', indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true } }), true);
});

// --- mergeSharedKbsForMention (Input-field.vue 1262-1284) ----------------------

test('mergeSharedKbsForMention appends shares after own rows, dedup by id, skips null', () => {
  const own = [
    { id: 'own-1', name: 'Own 1', summary_model_id: 'llm', embedding_model_id: 'emb' },
    { id: 'shared-dupe', name: 'Own Dup', summary_model_id: 'llm' },
  ];
  const merged = mergeSharedKbsForMention(own, [
    { knowledge_base: { id: 'shared-1', name: 'Shared 1', capabilities: { vector: true } }, permission: 'viewer', org_name: 'Org A' },
    { knowledge_base: { id: 'shared-dupe', name: 'Own Dup (shared)' }, permission: 'admin', org_name: 'Org A' },
    { knowledge_base: null, permission: 'viewer' },
    {},
  ]);
  assert.deepEqual(merged.map((kb) => String(kb.id)), ['own-1', 'shared-dupe', 'shared-1']);
  const shared = merged[2]!;
  assert.equal(shared.name, 'Shared 1');
  assert.equal(shared.org_name, 'Org A');
  assert.equal(shared.type, 'document', 'missing type defaults to document like Vue');
  assert.deepEqual(shared.capabilities, { vector: true }, 'capability payload rides along for the agent pass');
});

test('mergeSharedKbsForMention keeps shares even without readiness fields (Vue shares bypass the filter)', () => {
  const merged = mergeSharedKbsForMention([], [{ knowledge_base: { id: 'shared-1', name: 'Shared 1' }, permission: 'viewer' }]);
  assert.equal(merged.length, 1);
  assert.equal(isKbModelReady(merged[0]!), false, 'the share has no model ids — it survives via the merge path, not readiness');
});
