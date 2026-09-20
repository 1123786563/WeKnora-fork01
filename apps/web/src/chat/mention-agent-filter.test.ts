import assert from 'node:assert/strict';
import test from 'node:test';

import {
  deriveKbFilterForAgent,
  kbSatisfiesAgentRequirements,
  kbToScopeCaps,
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
