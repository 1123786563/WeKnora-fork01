/**
 * R490 B1 (R489 D12) — agent-scoped KB filtering for the chat `@` mention popup.
 *
 * Byte-parity port of the Vue sources:
 *   - frontend/src/utils/tool-capabilities.ts (TOOL_CAPABILITY_REQUIREMENTS,
 *     deriveKbFilterFromTools, deriveKbFilterForAgent,
 *     kbSatisfiesAgentRequirements, toolsConsumeFiles)
 *   - frontend/src/components/Input-field.vue (kbToScopeCaps + the mention KB
 *     scope pass at 1288-1315: kb_selection_mode 'none' empties the list,
 *     'selected' narrows to the configured ids, 'all' keeps only
 *     agent-compatible KBs; files load only when the mode keeps KBs alive and
 *     some tool can consume file ids, then get client-filtered to the same
 *     scope).
 *
 * The React chat page loaded `knowledgeBases.list({creator:'all'})` verbatim,
 * so a quick-answer agent (vector|keyword requirement) showed every KB even
 * though the Vue popup filtered them all out.
 */

/** Aggregate KB capability bits (backend capability set; all default false). */
export interface KbScopeCaps {
  vector: boolean;
  keyword: boolean;
  wiki: boolean;
  graph: boolean;
  faq: boolean;
}

/** frontend/src/utils/tool-capabilities.ts TOOL_CAPABILITY_REQUIREMENTS. */
const TOOL_CAPABILITY_REQUIREMENTS: Record<string, { anyOf?: string[]; allOf?: string[]; consumesFiles?: boolean }> = {
  // base / reasoning (no KB dependency)
  thinking: {},
  todo_write: {},
  // RAG / chunk retrieval (need at least one chunk-indexed KB)
  knowledge_search: { anyOf: ['vector', 'keyword'], consumesFiles: true },
  grep_chunks: { anyOf: ['vector', 'keyword'], consumesFiles: true },
  list_knowledge_chunks: { anyOf: ['vector', 'keyword'], consumesFiles: true },
  query_knowledge_graph: { anyOf: ['vector', 'keyword'], consumesFiles: true },
  get_document_info: { anyOf: ['vector', 'keyword'], consumesFiles: true },
  database_query: { anyOf: ['vector', 'keyword'], consumesFiles: true },
  // Wiki (wiki pages only; arbitrary user-picked file ids are meaningless)
  wiki_search: { allOf: ['wiki'] },
  wiki_read_page: { allOf: ['wiki'] },
  wiki_read_source_doc: { allOf: ['wiki'] },
  wiki_flag_issue: { allOf: ['wiki'] },
  wiki_write_page: { allOf: ['wiki'] },
  wiki_replace_text: { allOf: ['wiki'] },
  wiki_rename_page: { allOf: ['wiki'] },
  wiki_delete_page: { allOf: ['wiki'] },
  wiki_read_issue: { allOf: ['wiki'] },
  wiki_update_issue: { allOf: ['wiki'] },
  // Data analysis (reads table summary/column chunks from RAG ingest)
  data_analysis: { anyOf: ['vector', 'keyword'], consumesFiles: true },
  data_schema: { anyOf: ['vector', 'keyword'], consumesFiles: true },
};

/** tool-capabilities.ts deriveKbFilterFromTools: union of every capability
 *  named by any tool's anyOf/allOf; null when no tool has a KB requirement. */
export function deriveKbFilterFromTools(tools: readonly string[]): { any_of: string[] } | null {
  const caps = new Set<string>();
  for (const tool of tools) {
    const requirement = TOOL_CAPABILITY_REQUIREMENTS[tool];
    if (!requirement) continue;
    requirement.anyOf?.forEach((capability) => caps.add(capability));
    requirement.allOf?.forEach((capability) => caps.add(capability));
  }
  if (caps.size === 0) return null;
  return { any_of: [...caps] };
}

/** tool-capabilities.ts deriveKbFilterForAgent: tool-derived any_of unioned
 *  with the implicit quick-answer requirement (vector|keyword). Null when
 *  neither the mode nor the tools constrain capabilities. */
export function deriveKbFilterForAgent(agentMode: string | undefined | null, allowedTools: readonly string[] | undefined | null): { any_of: string[] } | null {
  const caps = new Set<string>();
  if (agentMode === 'quick-answer') {
    caps.add('vector');
    caps.add('keyword');
  }
  deriveKbFilterFromTools(allowedTools ?? [])?.any_of.forEach((capability) => caps.add(capability));
  if (caps.size === 0) return null;
  return { any_of: [...caps] };
}

/** tool-capabilities.ts kbSatisfiesAgentRequirements. */
export function kbSatisfiesAgentRequirements(
  kbCaps: Partial<KbScopeCaps> | undefined | null,
  agentMode: string | undefined | null,
  allowedTools: readonly string[] | undefined | null,
): boolean {
  const filter = deriveKbFilterForAgent(agentMode, allowedTools);
  if (!filter) return true;
  if (!kbCaps) return false;
  return filter.any_of.some((capability) => kbCaps[capability as keyof KbScopeCaps] === true);
}

/** tool-capabilities.ts toolsConsumeFiles: whether any allowed tool can use
 *  user-@'d file ids. Unknown tools are treated as file-consuming; empty → true. */
export function toolsConsumeFiles(allowedTools: readonly string[] | undefined | null): boolean {
  if (!allowedTools || allowedTools.length === 0) return true;
  for (const tool of allowedTools) {
    const requirement = TOOL_CAPABILITY_REQUIREMENTS[tool];
    if (!requirement) return true;
    if (requirement.consumesFiles) return true;
  }
  return false;
}

/** Input-field.vue kbToScopeCaps (400-419): capability bits from the explicit
 *  backend `capabilities` field, else indexing_strategy, else kb.type==='faq'. */
export function kbToScopeCaps(kb: Record<string, unknown>): Partial<KbScopeCaps> {
  const capabilities = kb.capabilities as Record<string, unknown> | undefined;
  if (capabilities && typeof capabilities === 'object') {
    return {
      vector: capabilities.vector === true,
      keyword: capabilities.keyword === true,
      wiki: capabilities.wiki === true,
      graph: capabilities.graph === true,
      faq: capabilities.faq === true,
    };
  }
  const strategy = kb.indexing_strategy as Record<string, unknown> | undefined;
  return {
    vector: strategy ? strategy.vector_enabled === true : false,
    keyword: strategy ? strategy.keyword_enabled === true : false,
    wiki: strategy ? strategy.wiki_enabled === true : false,
    graph: strategy ? strategy.graph_enabled === true : false,
    faq: kb.type === 'faq',
  };
}

/** Resolved agent scope for one `@` popup session (Input-field.vue 1288-1315
 *  + the file gates at 1392-1399). */
export interface MentionAgentKbScope {
  /** Vue hasAgentConfig — falsy agent config leaves the popup unrestricted. */
  hasAgentConfig: boolean;
  /** config.kb_selection_mode || 'all' (raw string; Vue does not normalize it). */
  kbMode: string;
  /** KBs surviving the scope pass; the untouched input when no agent config. */
  scopedKbs: Record<string, unknown>[];
  /** Stringified ids of scopedKbs; null = unrestricted (no agent config). */
  allowedKbIds: Set<string> | null;
  /** File gate: kb mode keeps KBs alive AND some tool consumes file ids. */
  shouldLoadFiles: boolean;
  /** Agent-derived capability filter, for the noCompatibleKbForAgent hint. */
  kbFilter: { any_of: string[] } | null;
  /** config.supported_file_types (file-type gate for the document search). */
  fileTypes: string[];
}

function agentConfigList(config: Record<string, unknown> | undefined | null, key: string): string[] {
  const value = config?.[key];
  return Array.isArray(value) ? value.map((entry) => String(entry)) : [];
}

/**
 * Apply the Vue mention KB scope pass to the raw `knowledgeBases.list` rows.
 * `agentConfig` is the selected agent's config object (undefined when no
 * agent is resolved); an empty object counts as configured, matching Vue's
 * `!!config` truthiness.
 */
export function resolveMentionAgentKbScope(
  agentConfig: Record<string, unknown> | undefined | null,
  kbRecords: readonly Record<string, unknown>[],
): MentionAgentKbScope {
  const hasAgentConfig = agentConfig != null;
  const kbMode = hasAgentConfig ? String(agentConfig?.kb_selection_mode ?? '') || 'all' : '';
  const allowedTools = hasAgentConfig ? agentConfigList(agentConfig, 'allowed_tools') : [];
  const agentMode = hasAgentConfig && typeof agentConfig?.agent_mode === 'string' ? agentConfig.agent_mode : '';
  const kbFilter = deriveKbFilterForAgent(agentMode, allowedTools);

  // Input-field.vue: 'none' empties the list; 'selected' narrows to the
  // configured ids (no second compatibility pass — the editor owns that);
  // 'all' keeps only agent-compatible KBs. An unrecognized mode falls through
  // every branch and keeps the full list, exactly like the Vue if/else chain.
  let scopedKbs = [...kbRecords];
  if (hasAgentConfig) {
    if (kbMode === 'none') {
      scopedKbs = [];
    } else if (kbMode === 'selected') {
      const configuredKbIds = new Set(agentConfigList(agentConfig, 'knowledge_bases'));
      scopedKbs = scopedKbs.filter((kb) => configuredKbIds.has(String(kb.id)));
    } else if (kbMode === 'all') {
      scopedKbs = scopedKbs.filter((kb) => kbSatisfiesAgentRequirements(kbToScopeCaps(kb), agentMode, allowedTools));
    }
  }

  const allowedKbIds = hasAgentConfig ? new Set(scopedKbs.map((kb) => String(kb.id))) : null;
  const kbModeAllowsFiles = !hasAgentConfig || kbMode !== 'none';
  const toolsAllowFiles = !hasAgentConfig || toolsConsumeFiles(allowedTools);
  return {
    hasAgentConfig,
    kbMode,
    scopedKbs,
    allowedKbIds,
    shouldLoadFiles: kbModeAllowsFiles && toolsAllowFiles,
    kbFilter,
    fileTypes: agentConfigList(agentConfig, 'supported_file_types'),
  };
}
