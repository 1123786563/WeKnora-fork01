/**
 * Pure logic for the agent editor modal — a port of the behaviour living in
 * frontend/src/views/agent/AgentEditorModal.vue (Vue baseline, read-only).
 * Every export cites the Vue source line it mirrors so parity reviews can
 * diff the two implementations line by line.
 */
import { formatMessage, type Locale, type MessageValues } from '@weknora/i18n';
import { agentEditorFallback } from './agent-editor-fallback.ts';
import {
  AGENT_TYPE_PRESETS,
  applyAgentTypePreset,
  findAgentTypePreset,
  presetDefaultDescription,
  presetDefaultName,
  type AgentPromptTemplateOption,
  type AgentTypePreset,
} from './agent-type-presets.ts';

// Re-exported as the module surface for the modal + tests (the preset catalog
// is a static port of config/agent_type_presets.yaml, see the source file).
export {
  AGENT_TYPE_PRESETS,
  agentTypePresetDescription,
  agentTypePresetLabel,
  applyAgentTypePreset,
  findAgentTypePreset,
  isDescriptionSystemGenerated,
  isNameSystemGenerated,
  presetDefaultDescription,
  presetDefaultName,
  type AgentTypePreset,
  type AgentTypePresetConfig,
} from './agent-type-presets.ts';
// R486 D4 — 使用模板/恢复默认 selector surface (vendored agent_system_prompt.yaml)
export {
  AGENT_SYSTEM_PROMPT_TEMPLATE_LIST,
  builtinAgentSystemPromptTemplates,
  resolveAgentSystemPromptResetTemplate,
  type AgentPromptTemplateOption,
  type AgentSystemPromptTemplateOption,
} from './agent-type-presets.ts';

export type Translate = (key: string, values?: Record<string, string | number>) => string;

/** formatMessage + the agentEditor.* fallback table (packages/i18n lacks the block). */
export function editorT(locale: Locale, key: string, values?: MessageValues): string {
  const resolved = formatMessage(locale, key, values);
  if (resolved !== key) return resolved;
  const fallback = agentEditorFallback[locale]?.[key] ?? agentEditorFallback['en-US']?.[key];
  if (fallback === undefined) return resolved;
  return fallback.replace(/\{(\w+)\}/g, (_match, name: string) => String(values?.[name] ?? `{${name}}`));
}

export function makeEditorT(locale: Locale): Translate {
  return (key, values) => editorT(locale, key, values);
}

// --- form model (AgentEditorModal.vue:2716-2825 defaultFormData) ------------------

export type AgentMode = 'quick-answer' | 'smart-reasoning';
export type ScopeSelectionMode = 'all' | 'selected' | 'none';

export interface QuestionSuggestionsForm {
  starters: { enabled: boolean; mode: 'curated' | 'knowledge' | 'hybrid'; items: string[]; count: number };
  follow_ups: {
    enabled: boolean; mode: 'generated' | 'knowledge' | 'hybrid'; count: number; model_id: string;
    additional_instruction: string; categories: string[]; max_context_turns: number;
    suppress_on_fallback: boolean; suppress_when_answer_asks_question: boolean;
    knowledge_fallback: boolean; allow_regenerate: boolean;
  };
}

/**
 * Provenance stamp an expert-template instantiation writes onto the created
 * agent (internal/types/custom_agent.go ExpertSourceStruct): which expert
 * manifest it came from, where that template lives ("builtin" for shipped
 * experts), and the slug reserved for future community/custom sources.
 * Read-only for the editor — it survives hydrate/save round-trips untouched.
 */
export interface ExpertSourceForm {
  expert_id: string;
  source: string;
  slug: string;
}

export interface AgentConfigForm {
  agent_mode: AgentMode;
  system_prompt: string;
  context_template: string;
  model_id: string;
  rerank_model_id: string;
  temperature: number;
  max_completion_tokens: number;
  thinking: boolean;
  citation_enabled: boolean;
  max_iterations: number;
  llm_call_timeout: number;
  allowed_tools: string[];
  reflection_enabled: boolean;
  mcp_selection_mode: ScopeSelectionMode;
  mcp_services: string[];
  mcp_auth_wait_timeout: number;
  skills_selection_mode: ScopeSelectionMode;
  selected_skills: string[];
  sandbox_config_id: string;
  kb_selection_mode: ScopeSelectionMode;
  knowledge_bases: string[];
  retrieve_kb_only_when_mentioned: boolean;
  agent_type: string;
  system_prompt_id: string;
  supported_file_types: string[];
  data_analysis_enabled: boolean;
  faq_priority_enabled: boolean;
  faq_direct_answer_threshold: number;
  faq_score_boost: number;
  web_search_enabled: boolean;
  web_search_provider_id: string;
  web_search_max_results: number;
  web_fetch_enabled: boolean;
  web_fetch_top_n: number;
  multi_turn_enabled: boolean;
  history_turns: number;
  retain_retrieval_history: boolean;
  memory_enabled: boolean;
  enable_query_expansion: boolean;
  enable_rewrite: boolean;
  query_understand_model_id: string;
  rewrite_prompt_system: string;
  rewrite_prompt_user: string;
  fallback_strategy: 'fixed' | 'model';
  fallback_response: string;
  fallback_prompt: string;
  embedding_top_k: number;
  keyword_threshold: number;
  vector_threshold: number;
  rerank_top_k: number;
  rerank_threshold: number;
  question_suggestions: QuestionSuggestionsForm;
  // 附件上传（multimodal 分区，AgentEditorModal.vue:2758-2771 defaultFormData）
  image_upload_enabled: boolean;
  vlm_model_id: string;
  image_storage_provider: string;
  attachment_image_understanding: boolean;
  attachment_ocr_max_pages: number;
  attachment_parse_wait_timeout_sec: number;
  audio_upload_enabled: boolean;
  asr_model_id: string;
  chat_parser_engine_rules: unknown[];
  welcome_message: string;
  // Octop M1 persona (no Vue baseline); optional so legacy payloads stay valid
  persona_mbti?: string;
  persona_style?: string;
  // Octop M2 expert provenance (no Vue baseline); null = not expert-created.
  // Must exist in defaults or hydrateAgentForm drops it (trap at the merge loop)
  expert_source?: ExpertSourceForm | null;
  // Octop M3 delegation (no Vue baseline): catalog slugs installed on this
  // agent — empty means the subagent_delegate tool is not registered at all
  // (delegation off). Must exist in defaults or hydrate drops it (M1 trap).
  subagents: string[];
  [key: string]: unknown;
}

export interface AgentEditorForm {
  id?: string;
  name: string;
  description: string;
  is_builtin: boolean;
  config: AgentConfigForm;
}

const MODE_PRESET = {
  temperature: 0.7,
  embeddingTopK: 10,
  keywordThreshold: 0.3,
  vectorThreshold: 0.5,
  rerankTopK: 5,
  rerankThreshold: 0.5,
} as const;

export function defaultAgentConfig(): AgentConfigForm {
  return {
    agent_mode: 'smart-reasoning',
    system_prompt: '',
    context_template: '',
    model_id: '',
    rerank_model_id: '',
    temperature: MODE_PRESET.temperature,
    max_completion_tokens: 0,
    thinking: false,
    citation_enabled: true,
    max_iterations: 10,
    llm_call_timeout: 120,
    allowed_tools: [],
    reflection_enabled: false,
    mcp_selection_mode: 'none',
    mcp_services: [],
    mcp_auth_wait_timeout: 600,
    skills_selection_mode: 'none',
    selected_skills: [],
    sandbox_config_id: '',
    // 新建智能体默认选择 "全部知识库" (AgentEditorModal.vue:2747-2749)
    kb_selection_mode: 'all',
    knowledge_bases: [],
    retrieve_kb_only_when_mentioned: false,
    agent_type: 'rag-qa',
    system_prompt_id: '',
    supported_file_types: [],
    data_analysis_enabled: false,
    faq_priority_enabled: true,
    faq_direct_answer_threshold: 0.9,
    faq_score_boost: 1.2,
    web_search_enabled: false,
    web_search_provider_id: '',
    web_search_max_results: 5,
    web_fetch_enabled: false,
    web_fetch_top_n: 3,
    multi_turn_enabled: false,
    history_turns: 5,
    retain_retrieval_history: false,
    memory_enabled: true,
    enable_query_expansion: true,
    enable_rewrite: true,
    query_understand_model_id: '',
    rewrite_prompt_system: '',
    rewrite_prompt_user: '',
    fallback_strategy: 'model',
    fallback_response: '',
    fallback_prompt: '',
    embedding_top_k: MODE_PRESET.embeddingTopK,
    keyword_threshold: MODE_PRESET.keywordThreshold,
    vector_threshold: MODE_PRESET.vectorThreshold,
    rerank_top_k: MODE_PRESET.rerankTopK,
    rerank_threshold: MODE_PRESET.rerankThreshold,
    question_suggestions: {
      starters: { enabled: true, mode: 'hybrid', items: [], count: 6 },
      follow_ups: {
        enabled: false, mode: 'hybrid', count: 3, model_id: '', additional_instruction: '',
        categories: ['clarify', 'deepen', 'action'], max_context_turns: 2,
        suppress_on_fallback: true, suppress_when_answer_asks_question: true,
        knowledge_fallback: true, allow_regenerate: false,
      },
    },
    // 附件上传默认值（Vue defaultFormData 2758-2771：全部关闭/空/0）
    image_upload_enabled: false,
    vlm_model_id: '',
    image_storage_provider: '',
    attachment_image_understanding: false,
    attachment_ocr_max_pages: 0,
    attachment_parse_wait_timeout_sec: 0,
    audio_upload_enabled: false,
    asr_model_id: '',
    chat_parser_engine_rules: [],
    welcome_message: '',
    // must exist in defaults or hydrateAgentForm drops them (trap at :199)
    persona_mbti: '',
    persona_style: '',
    // expert provenance stamp: null unless the agent came from an expert
    // template; hydrate only copies a present object over this default
    expert_source: null,
    // delegation off by default — subagents must be installed explicitly
    subagents: [],
  };
}

export function defaultAgentForm(): AgentEditorForm {
  return { name: '', description: '', is_builtin: false, config: defaultAgentConfig() };
}

const asRecord = (value: unknown): Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
const asStringArray = (value: unknown): string[] => (Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []);
const asNumber = (value: unknown, fallback: number): number => (typeof value === 'number' && Number.isFinite(value) ? value : fallback);
const asBool = (value: unknown, fallback: boolean): boolean => (typeof value === 'boolean' ? value : fallback);

/**
 * Edit-mode hydration (AgentEditorModal.vue:3401-3471): deep-merge the stored
 * agent over the defaults so every field the payload needs is present.
 */
export function hydrateAgentForm(agent: Record<string, unknown>): AgentEditorForm {
  const stored = asRecord(agent.config);
  const config = { ...defaultAgentConfig() };
  for (const [key, value] of Object.entries(stored)) {
    if (value !== undefined && value !== null && key in config) (config as Record<string, unknown>)[key] = value;
  }
  // Vue:3412-3413 thinking null-coalesce; Vue:3430-3446 array/number defaults
  config.thinking = asBool(stored.thinking, false);
  config.knowledge_bases = asStringArray(stored.knowledge_bases);
  config.allowed_tools = asStringArray(stored.allowed_tools);
  config.mcp_services = asStringArray(stored.mcp_services);
  config.selected_skills = asStringArray(stored.selected_skills);
  config.supported_file_types = asStringArray(stored.supported_file_types);
  // M3 delegation slugs — string-array like the tool/skill lists above
  config.subagents = asStringArray(stored.subagents);
  // multimodal block (Vue:2758-2771 defaults; rules kept verbatim when present)
  config.image_upload_enabled = asBool(stored.image_upload_enabled, false);
  config.vlm_model_id = typeof stored.vlm_model_id === 'string' ? stored.vlm_model_id : '';
  config.image_storage_provider = typeof stored.image_storage_provider === 'string' ? stored.image_storage_provider : '';
  config.attachment_image_understanding = asBool(stored.attachment_image_understanding, false);
  config.attachment_ocr_max_pages = asNumber(stored.attachment_ocr_max_pages, 0);
  config.attachment_parse_wait_timeout_sec = asNumber(stored.attachment_parse_wait_timeout_sec, 0);
  config.audio_upload_enabled = asBool(stored.audio_upload_enabled, false);
  config.asr_model_id = typeof stored.asr_model_id === 'string' ? stored.asr_model_id : '';
  config.chat_parser_engine_rules = Array.isArray(stored.chat_parser_engine_rules) ? stored.chat_parser_engine_rules : [];
  const waitTimeout = asNumber(stored.mcp_auth_wait_timeout, 0);
  config.mcp_auth_wait_timeout = waitTimeout <= 0 ? 600 : waitTimeout;
  config.max_completion_tokens = asNumber(stored.max_completion_tokens, 0);
  config.memory_enabled = asBool(stored.memory_enabled, true);
  // Vue:3448-3452 legacy agents without agent_mode infer it from tools/iterations
  if (typeof stored.agent_mode !== 'string' || stored.agent_mode === '') {
    const iterations = asNumber(stored.max_iterations, 1);
    const isAgent = iterations < 0 || iterations > 1 || config.allowed_tools.length > 0;
    config.agent_mode = isAgent ? 'smart-reasoning' : 'quick-answer';
  }
  // Vue:3416-3428 question_suggestions nested merge
  const qs = asRecord(stored.question_suggestions);
  const starters = asRecord(qs.starters);
  const followUps = asRecord(qs.follow_ups);
  const defaults = defaultAgentConfig().question_suggestions;
  config.question_suggestions = {
    starters: {
      enabled: asBool(starters.enabled, defaults.starters.enabled),
      mode: (typeof starters.mode === 'string' ? starters.mode : defaults.starters.mode) as QuestionSuggestionsForm['starters']['mode'],
      items: asStringArray(starters.items),
      count: asNumber(starters.count, defaults.starters.count),
    },
    follow_ups: {
      enabled: asBool(followUps.enabled, defaults.follow_ups.enabled),
      mode: (typeof followUps.mode === 'string' ? followUps.mode : defaults.follow_ups.mode) as QuestionSuggestionsForm['follow_ups']['mode'],
      count: asNumber(followUps.count, defaults.follow_ups.count),
      model_id: typeof followUps.model_id === 'string' ? followUps.model_id : '',
      additional_instruction: typeof followUps.additional_instruction === 'string' ? followUps.additional_instruction : '',
      categories: asStringArray(followUps.categories).length > 0 ? asStringArray(followUps.categories) : [...defaults.follow_ups.categories],
      max_context_turns: asNumber(followUps.max_context_turns, defaults.follow_ups.max_context_turns),
      suppress_on_fallback: asBool(followUps.suppress_on_fallback, defaults.follow_ups.suppress_on_fallback),
      suppress_when_answer_asks_question: asBool(followUps.suppress_when_answer_asks_question, defaults.follow_ups.suppress_when_answer_asks_question),
      knowledge_fallback: asBool(followUps.knowledge_fallback, defaults.follow_ups.knowledge_fallback),
      allow_regenerate: asBool(followUps.allow_regenerate, defaults.follow_ups.allow_regenerate),
    },
  };
  return {
    id: typeof agent.id === 'string' ? agent.id : undefined,
    name: typeof agent.name === 'string' ? agent.name : '',
    description: typeof agent.description === 'string' ? agent.description : '',
    is_builtin: agent.is_builtin === true,
    config,
  };
}

// --- selection modes (AgentEditorModal.vue:3564-3602 + watches 3640-3711) ---------

export function initKbSelectionMode(form: AgentEditorForm): ScopeSelectionMode {
  const stored = form.config.kb_selection_mode;
  if (stored === 'all' || stored === 'selected' || stored === 'none') return stored;
  if (form.config.knowledge_bases.length > 0) return 'selected';
  return 'none';
}

export function initScopeSelectionMode(mode: unknown, items: string[]): ScopeSelectionMode {
  if (mode === 'all' || mode === 'selected' || mode === 'none') return mode;
  if (items.length > 0) return 'selected';
  return 'none';
}

/** Watch(kbSelectionMode) Vue:3640-3650 — all/none clear the explicit list. */
export function applyKbSelectionMode(form: AgentEditorForm, mode: ScopeSelectionMode): void {
  form.config.kb_selection_mode = mode;
  if (mode === 'none' || mode === 'all') form.config.knowledge_bases = [];
}

export function applyScopeSelectionMode(form: AgentEditorForm, field: 'mcp' | 'skills', mode: ScopeSelectionMode): void {
  if (field === 'mcp') {
    form.config.mcp_selection_mode = mode;
    if (mode === 'none' || mode === 'all') form.config.mcp_services = [];
  } else {
    form.config.skills_selection_mode = mode;
    if (mode === 'none' || mode === 'all') form.config.selected_skills = [];
  }
}

export const agentModeOf = (config: Record<string, unknown> | undefined): AgentMode =>
  config?.agent_mode === 'smart-reasoning' ? 'smart-reasoning' : 'quick-answer';

// --- validation (AgentEditorModal.vue handleSave 4736-4799) ------------------------

export type AgentSectionKey =
  | 'basic' | 'prompts' | 'model' | 'conversation' | 'suggestions' | 'knowledge' | 'retrieval'
  | 'websearch' | 'multimodal' | 'tools' | 'mcp' | 'skills' | 'personalization' | 'subagents';

export type AgentFieldError =
  | 'name' | 'system_prompt' | 'context_template' | 'model_id'
  | 'rewrite_prompt_user' | 'fallback_prompt';

export interface AgentFormIssue {
  field: AgentFieldError;
  section: AgentSectionKey;
  messageKey: string;
}

export const hasQueryPlaceholder = (text: string | undefined): boolean => !!text && text.includes('{{query}}');

/**
 * Ordered port of handleSave validation. Empty output means the payload may
 * be submitted; each issue maps to a per-field error shown under the input
 * and to the section the UI should jump to.
 */
export function validateAgentForm(form: AgentEditorForm): AgentFormIssue[] {
  const issues: AgentFormIssue[] = [];
  if (!form.is_builtin) {
    if (!form.name.trim()) issues.push({ field: 'name', section: 'basic', messageKey: 'agent.editor.nameRequired' });
    if (!form.config.system_prompt.trim()) {
      issues.push({ field: 'system_prompt', section: 'prompts', messageKey: 'agent.editor.systemPromptRequired' });
    } else if (form.config.agent_mode !== 'smart-reasoning' && !form.config.context_template.trim()) {
      // Vue:4752-4757 quick-answer mode requires the context template too
      issues.push({ field: 'context_template', section: 'prompts', messageKey: 'agent.editor.contextTemplateRequired' });
    }
  }
  const quickAnswer = form.config.agent_mode !== 'smart-reasoning';
  if (quickAnswer && form.config.multi_turn_enabled && form.config.enable_rewrite
    && form.config.rewrite_prompt_user.trim() && !hasQueryPlaceholder(form.config.rewrite_prompt_user)) {
    issues.push({ field: 'rewrite_prompt_user', section: 'prompts', messageKey: 'agent.editor.queryMissingInRewrite' });
  }
  if (quickAnswer && form.config.fallback_strategy === 'model'
    && form.config.fallback_prompt.trim() && !hasQueryPlaceholder(form.config.fallback_prompt)) {
    issues.push({ field: 'fallback_prompt', section: 'prompts', messageKey: 'agent.editor.queryMissingInFallback' });
  }
  if (!form.config.model_id) issues.push({ field: 'model_id', section: 'model', messageKey: 'agent.editor.modelRequired' });
  return issues;
}

/**
 * Trim starter suggestion items before submit (AgentEditorModal.vue:4804-4807)
 * and return the wire payload exactly as the Vue formData shape.
 */
export function buildAgentPayload(form: AgentEditorForm): Record<string, unknown> {
  const config: AgentConfigForm = {
    ...form.config,
    question_suggestions: {
      ...form.config.question_suggestions,
      starters: {
        ...form.config.question_suggestions.starters,
        items: form.config.question_suggestions.starters.items.map((item) => item.trim()).filter(Boolean),
      },
    },
  };
  const payload: Record<string, unknown> = {
    name: form.name.trim(),
    description: form.description,
    is_builtin: form.is_builtin,
    config,
  };
  if (form.id) payload.id = form.id;
  return payload;
}

// --- nav rail (AgentEditorModal.vue navItems 2657-2684 + navGroups 2687-2713) ------

export interface AgentNavItem { key: AgentSectionKey; icon: string; labelKey: string }
export interface AgentNavGroup { key: string; labelKey: string; items: AgentNavItem[] }

export function buildNavGroups(options: { isAgentMode: boolean; hasKnowledgeBase: boolean }): AgentNavGroup[] {
  const items: AgentNavItem[] = [
    { key: 'basic', icon: 'info-circle', labelKey: 'agent.editor.basicInfo' },
    { key: 'prompts', icon: 'file-paste', labelKey: 'agent.editor.promptsConfig' },
    { key: 'model', icon: 'control-platform', labelKey: 'agent.editor.modelConfig' },
    // R485 D1 — Vue navItems 2657-2684: suggestions rides the basic group
    { key: 'suggestions', icon: 'help-circle', labelKey: 'agentEditor.questionSuggestions.navLabel' },
    { key: 'conversation', icon: 'chat', labelKey: 'agent.editor.conversationSettings' },
    { key: 'knowledge', icon: 'folder', labelKey: 'agent.editor.knowledgeConfig' },
  ];
  if (options.hasKnowledgeBase) {
    items.push({ key: 'retrieval', icon: 'search', labelKey: 'agent.editor.retrievalStrategy' });
  }
  items.push({ key: 'websearch', icon: 'internet', labelKey: 'agent.editor.webSearchConfig' });
  // R485 D1 — multimodal is NOT agent-mode gated in Vue (navItems push is unconditional)
  items.push({ key: 'multimodal', icon: 'attach', labelKey: 'agentEditor.imageUpload.navLabel' });
  if (options.isAgentMode) {
    // Octop M1 persona section (no Vue baseline). Persona renders only in the
    // smart-reasoning pipeline (capability assembly); quick-answer runs never
    // read it, so gate the section by agent-mode exactly like tools/skills
    // instead of offering a control that silently does nothing.
    items.push({ key: 'personalization', icon: 'user', labelKey: 'agentEditor.personalization.title' });
    items.push({ key: 'tools', icon: 'tools', labelKey: 'agent.editor.toolsConfig' });
    // R485 D1 — Vue places mcp between tools and skills in agent mode
    items.push({ key: 'mcp', icon: 'server', labelKey: 'agentEditor.mcp.label' });
    items.push({ key: 'skills', icon: 'skills', labelKey: 'agent.editor.skillsConfig' });
    // Octop M3 delegation (no Vue baseline): sub-agent roles only register the
    // delegate tool in the smart-reasoning pipeline, so the same agent-mode
    // gate as tools/skills applies.
    items.push({ key: 'subagents', icon: 'app-link', labelKey: 'agentEditor.subagents.title' });
  }
  const byKey = new Map(items.map((item) => [item.key, item]));
  const pick = (keys: AgentSectionKey[]): AgentNavItem[] =>
    keys.map((key) => byKey.get(key)).filter((item): item is AgentNavItem => item !== undefined);
  return [
    // Vue navGroups 2687-2713: basic group picks suggestions after conversation
    { key: 'basic', labelKey: 'agentEditor.navGroups.basic', items: pick(['basic', 'prompts', 'model', 'conversation', 'suggestions', 'personalization']) },
    { key: 'knowledge', labelKey: 'agentEditor.navGroups.knowledge', items: pick(['knowledge', 'retrieval', 'websearch']) },
    // Vue capability order: multimodal, tools, mcp, skills (subagents rides last)
    { key: 'capability', labelKey: 'agentEditor.navGroups.capability', items: pick(['multimodal', 'tools', 'mcp', 'skills', 'subagents']) },
  ].filter((group) => group.items.length > 0);
}

// --- tool catalog (AgentEditorModal.vue allTools 2379-2416 + tool-capabilities) ----

export interface ToolCapabilityScope { vector: boolean; keyword: boolean; wiki: boolean; graph: boolean; faq: boolean }
export type RequirementMissKind = 'none' | 'needsKb' | 'needsRag' | 'needsWiki';

export interface ToolDefinition {
  value: string;
  labelKey: string;
  descriptionKey: string;
  group: 'base' | 'rag' | 'wiki_read' | 'wiki_edit' | 'wiki_issue' | 'data';
  danger?: boolean;
  anyOf?: string[];
  allOf?: string[];
}

const TOOL = (value: string, name: string, group: ToolDefinition['group'], requirement: { anyOf?: string[]; allOf?: string[] } = {}, danger = false): ToolDefinition => ({
  value,
  labelKey: `agentEditor.tools.${name}`,
  descriptionKey: `agentEditor.tools.${name}Desc`,
  group,
  ...(danger ? { danger } : {}),
  ...(requirement.anyOf ? { anyOf: requirement.anyOf } : {}),
  ...(requirement.allOf ? { allOf: requirement.allOf } : {}),
});

// R484 D8 — supported file types offered on the knowledge section, mirroring
// Vue AgentEditorModal.vue:2564-2570 availableFileTypes (pdf/docx/txt/md/csv/
// xlsx/jpg). Values are stored verbatim in config.supported_file_types; the
// localized labels/descriptions resolve through agentEditor.fileTypes.*
// (label stays literal where Vue keeps a proper noun: PDF/Word/Markdown/CSV/Excel).
export interface SupportedFileTypeOption { value: string; label: string; labelKey?: string; descriptionKey: string }
export const AGENT_FILE_TYPE_OPTIONS: SupportedFileTypeOption[] = [
  { value: 'pdf', label: 'PDF', descriptionKey: 'agentEditor.fileTypes.pdf' },
  { value: 'docx', label: 'Word', descriptionKey: 'agentEditor.fileTypes.word' },
  { value: 'txt', label: '文本', labelKey: 'agentEditor.fileTypes.textLabel', descriptionKey: 'agentEditor.fileTypes.text' },
  { value: 'md', label: 'Markdown', descriptionKey: 'agentEditor.fileTypes.markdown' },
  { value: 'csv', label: 'CSV', descriptionKey: 'agentEditor.fileTypes.csv' },
  { value: 'xlsx', label: 'Excel', descriptionKey: 'agentEditor.fileTypes.excel' },
  { value: 'jpg', label: '图片', labelKey: 'agentEditor.fileTypes.imageLabel', descriptionKey: 'agentEditor.fileTypes.image' },
];

// Order and grouping mirror AgentEditorModal.vue:2379-2406.
export const TOOL_CATALOG: ToolDefinition[] = [
  TOOL('thinking', 'thinking', 'base'),
  TOOL('todo_write', 'todoWrite', 'base'),
  TOOL('grep_chunks', 'grepChunks', 'rag', { anyOf: ['vector', 'keyword'] }),
  TOOL('knowledge_search', 'knowledgeSearch', 'rag', { anyOf: ['vector', 'keyword'] }),
  TOOL('list_knowledge_chunks', 'listChunks', 'rag', { anyOf: ['vector', 'keyword'] }),
  TOOL('query_knowledge_graph', 'queryGraph', 'rag', { anyOf: ['vector', 'keyword'] }),
  TOOL('get_document_info', 'getDocInfo', 'rag', { anyOf: ['vector', 'keyword'] }),
  TOOL('database_query', 'dbQuery', 'rag', { anyOf: ['vector', 'keyword'] }),
  TOOL('wiki_search', 'wikiSearch', 'wiki_read', { allOf: ['wiki'] }),
  TOOL('wiki_read_page', 'wikiReadPage', 'wiki_read', { allOf: ['wiki'] }),
  TOOL('wiki_read_source_doc', 'wikiReadSourceDoc', 'wiki_read', { allOf: ['wiki'] }),
  TOOL('wiki_flag_issue', 'wikiFlagIssue', 'wiki_read', { allOf: ['wiki'] }),
  TOOL('wiki_write_page', 'wikiWritePage', 'wiki_edit', { allOf: ['wiki'] }, true),
  TOOL('wiki_replace_text', 'wikiReplaceText', 'wiki_edit', { allOf: ['wiki'] }, true),
  TOOL('wiki_rename_page', 'wikiRenamePage', 'wiki_edit', { allOf: ['wiki'] }, true),
  TOOL('wiki_delete_page', 'wikiDeletePage', 'wiki_edit', { allOf: ['wiki'] }, true),
  TOOL('wiki_read_issue', 'wikiReadIssue', 'wiki_issue', { allOf: ['wiki'] }),
  TOOL('wiki_update_issue', 'wikiUpdateIssue', 'wiki_issue', { allOf: ['wiki'] }),
  TOOL('data_analysis', 'dataAnalysis', 'data', { anyOf: ['vector', 'keyword'] }),
  TOOL('data_schema', 'dataSchema', 'data', { anyOf: ['vector', 'keyword'] }),
];

export const TOOL_GROUPS: Array<{ key: ToolDefinition['group']; labelKey: string }> = [
  { key: 'base', labelKey: 'agentEditor.tools.groupBase' },
  { key: 'rag', labelKey: 'agentEditor.tools.groupRag' },
  { key: 'wiki_read', labelKey: 'agentEditor.tools.groupWikiRead' },
  { key: 'wiki_edit', labelKey: 'agentEditor.tools.groupWikiEdit' },
  { key: 'wiki_issue', labelKey: 'agentEditor.tools.groupWikiIssue' },
  { key: 'data', labelKey: 'agentEditor.tools.groupData' },
];

/** Port of frontend/src/utils/tool-capabilities.ts evaluateToolRequirement. */
export function evaluateToolRequirement(tool: ToolDefinition, scope: ToolCapabilityScope, hasAnyKb: boolean): { ok: boolean; missKind: RequirementMissKind } {
  if (!tool.anyOf?.length && !tool.allOf?.length) return { ok: true, missKind: 'none' };
  if (!hasAnyKb) return { ok: false, missKind: 'needsKb' };
  const has = (capability: string): boolean => (scope as unknown as Record<string, boolean>)[capability] === true;
  if (tool.allOf?.length && !tool.allOf.every(has)) {
    return { ok: false, missKind: tool.allOf.includes('wiki') ? 'needsWiki' : 'needsRag' };
  }
  if (tool.anyOf?.length && !tool.anyOf.some(has)) return { ok: false, missKind: 'needsRag' };
  return { ok: true, missKind: 'none' };
}

export interface KbOption {
  label: string;
  value: string;
  type: 'document' | 'faq';
  count: number;
  shared: boolean;
  orgName?: string;
  ragEnabled: boolean;
  wikiEnabled: boolean;
  capabilities?: Partial<ToolCapabilityScope>;
}

/** mapKbToOption (AgentEditorModal.vue:3833-3847) over the React KB record. */
export function kbOptionFromRecord(kb: Record<string, unknown>, shared = false, orgName?: string): KbOption {
  const strategy = asRecord(kb.indexing_strategy);
  const capabilities = asRecord(kb.capabilities);
  const caps: Partial<ToolCapabilityScope> | undefined = Object.keys(capabilities).length > 0
    ? {
        vector: capabilities.vector === true,
        keyword: capabilities.keyword === true,
        wiki: capabilities.wiki === true,
        graph: capabilities.graph === true,
        faq: capabilities.faq === true,
      }
    : undefined;
  const type = kb.type === 'faq' ? 'faq' : 'document';
  const ragEnabled = caps ? caps.vector === true || caps.keyword === true
    : !kb.indexing_strategy || strategy.vector_enabled === true || strategy.keyword_enabled === true;
  const wikiEnabled = caps ? caps.wiki === true : strategy.wiki_enabled === true;
  return {
    label: typeof kb.name === 'string' ? kb.name : String(kb.id ?? ''),
    value: typeof kb.id === 'string' ? kb.id : String(kb.id ?? ''),
    type,
    count: type === 'faq' ? asNumber(kb.chunk_count, 0) : asNumber(kb.knowledge_count, 0),
    shared,
    ...(orgName ? { orgName } : {}),
    ragEnabled,
    wikiEnabled,
    ...(caps ? { capabilities: caps } : {}),
  };
}

// --- mode switch (AgentEditorModal.vue watch(agentMode) 3713-3767) -----------------

const KNOWLEDGE_BASE_TOOLS = ['grep_chunks', 'knowledge_search', 'list_knowledge_chunks', 'get_document_info'];
const WIKI_READ_TOOLS = ['wiki_search', 'wiki_read_page', 'wiki_read_source_doc', 'wiki_flag_issue'];

/**
 * Seed tools/iterations when the running mode changes. The Vue watcher also
 * swaps default prompts; the React editor has no prompt-template defaults
 * endpoint yet, so only the tool/iteration side is reproduced.
 */
export function applyAgentModeSwitch(form: AgentEditorForm, mode: AgentMode, scope: ToolCapabilityScope, hasKb: boolean): void {
  form.config.agent_mode = mode;
  if (mode === 'smart-reasoning') {
    if (form.config.allowed_tools.length === 0) {
      const tools: string[] = [];
      if (hasKb && scope.vector) tools.push(...KNOWLEDGE_BASE_TOOLS);
      if (hasKb && scope.wiki) tools.push(...WIKI_READ_TOOLS);
      form.config.allowed_tools = tools;
    }
    if (form.config.max_iterations >= 0 && form.config.max_iterations <= 1) form.config.max_iterations = 10;
  } else {
    form.config.allowed_tools = [];
    form.config.max_iterations = 1;
  }
}

// --- skills (AgentEditorModal.vue catalogSkillRows 2090-2102) ----------------------

export interface SkillCatalogInstallLike { sandboxConfigId: string; status: string; enabled: boolean }
export interface SkillCatalogLike { id: string; name: string; description?: string; installations?: SkillCatalogInstallLike[] }

export interface CatalogSkillRow {
  id: string;
  name: string;
  description?: string;
  installed: boolean;
  selectable: boolean;
  installStatus: string;
  installEnabled: boolean;
}

export function catalogSkillRows(catalog: SkillCatalogLike[], sandboxConfigId: string): CatalogSkillRow[] {
  return catalog.map((item) => {
    const install = sandboxConfigId
      ? (item.installations ?? []).find((row) => row.sandboxConfigId === sandboxConfigId)
      : undefined;
    const installStatus = install?.status ?? '';
    const installEnabled = install?.enabled === true;
    const installed = install !== undefined && installStatus !== 'removed';
    const selectable = installStatus === 'ready' && installEnabled;
    return {
      id: item.id,
      name: item.name,
      ...(item.description ? { description: item.description } : {}),
      installed,
      selectable,
      installStatus,
      installEnabled,
    };
  });
}

export const isNamedSandboxBackend = (type: string): boolean => ['docker', 'e2b', 'cube'].includes(type);

// --- create prefill (AgentEditorModal.vue 3514-3536 + D3/D10) ------------------------

/** frontend/src/utils/modelDefaults.ts ModelDefaultCandidate (type optional to accept api-client rows). */
export interface ModelDefaultCandidate { id?: string; type?: string; status?: string; is_default?: boolean }

/**
 * Pick a creation-time model: prefer the declared default, fall back to the
 * first active model (frontend/src/utils/modelDefaults.ts selectInitialModelId).
 */
export function selectInitialModelId(models: readonly ModelDefaultCandidate[], modelType: string): string | null {
  const active = models.filter((model) =>
    Boolean(model.id?.trim()) && model.type === modelType && (!model.status || model.status === 'active'));
  return active.find((model) => model.is_default)?.id?.trim() ?? active[0]?.id?.trim() ?? null;
}

/**
 * The create form opens with the default agent_type preset applied so the
 * visible defaults match the type dropdown from the first paint (Vue 3514-3536
 * "否则用户在 modal 打开瞬间看到的默认表单和类型下拉显示的类型不一致").
 * Smart-reasoning is the default agent_mode, so the default type 'rag-qa'
 * seeds the system prompt, the 4 RAG tools, kb mode and the 我的<label> name.
 * Models are prefilled when empty (Vue applyDefaultModelsIfEmpty 2854-2862, D5).
 *
 * R491 — `presets` / `promptTemplates` default to the vendored catalogs; the
 * editor passes the runtime-fetched ones (Vue awaits loadDependencies before
 * the create prefill, so the backend rag-qa preset + template body win).
 */
export function seedCreateAgentForm(
  t: Translate,
  locale: Locale = 'zh-CN',
  models: readonly ModelDefaultCandidate[] = [],
  presets: readonly AgentTypePreset[] = AGENT_TYPE_PRESETS,
  promptTemplates?: readonly AgentPromptTemplateOption[] | Record<string, string>,
): AgentEditorForm {
  const form = defaultAgentForm();
  if (form.config.agent_mode === 'smart-reasoning') {
    const defaultTypeId = form.config.agent_type;
    const preset = findAgentTypePreset(defaultTypeId, presets);
    if (defaultTypeId && defaultTypeId !== 'custom') {
      applyAgentTypePreset(form, preset, promptTemplates);
    }
    if (!form.name) form.name = presetDefaultName(preset, t, locale);
    if (!form.description) form.description = presetDefaultDescription(preset, locale);
  }
  const chatModelId = selectInitialModelId(models, 'KnowledgeQA');
  const rerankModelId = selectInitialModelId(models, 'Rerank');
  if (!form.config.model_id && chatModelId) form.config.model_id = chatModelId;
  if (!form.config.rerank_model_id && rerankModelId) form.config.rerank_model_id = rerankModelId;
  return form;
}

/**
 * Rerank is required only when a RAG-capable KB is reachable: under "all" any
 * ragEnabled KB counts, under "selected" only the picked ones (Vue
 * needsRerankModel computed 3373-3388, audit D6).
 */
export function needsRerankModel(kbMode: ScopeSelectionMode, kbOptions: readonly KbOption[], selectedIds: readonly string[]): boolean {
  if (kbMode === 'none') return false;
  if (kbMode === 'all') return kbOptions.some((kb) => kb.ragEnabled);
  return kbOptions.some((kb) => selectedIds.includes(kb.value) && kb.ragEnabled);
}

// --- R486 D4: prompt placeholder catalogue (internal/types/placeholder.go) ----------------
//
// The Vue editor fetches these definitions from GET /api/v1/agents/placeholders
// (editorResources.ensurePlaceholders); the React api-client has no endpoint,
// so the backend's static PlaceholdersByField table is vendored here the same
// way agent-type-presets.ts vendors the YAML catalogs.

export interface PromptPlaceholderDef { name: string; label: string; description: string }

const PLACEHOLDER_DEFINITIONS: Record<string, PromptPlaceholderDef> = {
  query: { name: 'query', label: '用户问题', description: '用户当前的问题或查询内容' },
  contexts: { name: 'contexts', label: '检索内容', description: '从知识库检索到的相关内容列表' },
  current_time: { name: 'current_time', label: '当前时间', description: '当前日期（ISO 格式：2006-01-02）。只用日期、不用时钟，避免秒级变化打断 provider 前缀缓存。' },
  current_week: { name: 'current_week', label: '当前星期', description: '当前星期几（如：星期一、Monday）' },
  knowledge_bases: { name: 'knowledge_bases', label: '知识库列表', description: '自动格式化的知识库列表，包含名称、描述、文档数量等信息' },
  web_search_status: { name: 'web_search_status', label: '网络搜索状态', description: '网络搜索工具是否启用的状态（Enabled 或 Disabled）' },
  language: { name: 'language', label: '用户语言', description: '用户界面的语言偏好，如 Chinese (Simplified)、English、Korean 等，用于控制 LLM 回答语言' },
};

/** internal/types/placeholder.go PlaceholdersByField — order preserved per field.
 *
 * R491 — `runtime` is the fetched GET /api/v1/agents/placeholders catalog
 * (Vue editorResources.placeholders); when its section for `field` is present
 * and non-empty the backend definitions win, otherwise the vendored set
 * answers (failed/empty fetch fallback — Vue renders none instead). */
export function promptPlaceholdersFor(field: 'agent_system_prompt' | 'system_prompt' | 'context_template', runtime?: Readonly<Record<string, PromptPlaceholderDef[]>> | null): PromptPlaceholderDef[] {
  const fetched = runtime?.[field];
  if (fetched && fetched.length > 0) return [...fetched];
  if (field === 'agent_system_prompt') {
    return [PLACEHOLDER_DEFINITIONS.knowledge_bases!, PLACEHOLDER_DEFINITIONS.web_search_status!, PLACEHOLDER_DEFINITIONS.current_time!, PLACEHOLDER_DEFINITIONS.language!];
  }
  // system_prompt and context_template share the normal-mode set (Go switch)
  return [PLACEHOLDER_DEFINITIONS.query!, PLACEHOLDER_DEFINITIONS.contexts!, PLACEHOLDER_DEFINITIONS.current_time!, PLACEHOLDER_DEFINITIONS.current_week!, PLACEHOLDER_DEFINITIONS.language!];
}

/**
 * Vue insertPlaceholder tag-click tail (AgentEditorModal.vue:4140-4146): splice
 * the full {{name}} token at the caret and return the caret position that lands
 * right after the inserted token.
 */
export function insertPlaceholderAtCursor(value: string, cursorPos: number, name: string): { value: string; cursorPos: number } {
  const clamped = Math.max(0, Math.min(cursorPos, value.length));
  const token = `{{${name}}}`;
  return {
    value: value.slice(0, clamped) + token + value.slice(clamped),
    cursorPos: clamped + token.length,
  };
}

// --- R486 D9: tenant retrieval-config defaults (Vue 2340-2344 + 3912-3917) ---------------

export interface TenantRetrievalDefaults {
  embeddingTopK: number;
  keywordThreshold: number;
  vectorThreshold: number;
  rerankTopK: number;
  rerankThreshold: number;
}

/**
 * Vue defaultEmbeddingTopK & friends start at 10/0.3/0.5/5/0.5 and the
 * tenant retrieval-config overrides them on load. The override semantics differ
 * per field exactly as the Vue writes read (AgentEditorModal.vue:3912-3917):
 * top-k fields are truthiness-gated (`if (rc?.embedding_top_k)`) so a 0 keeps
 * the default, thresholds use `!== undefined` so an explicit 0 wins.
 */
export function tenantRetrievalDefaultsFromConfig(config: Record<string, unknown> | null | undefined): TenantRetrievalDefaults {
  const defaults: TenantRetrievalDefaults = { embeddingTopK: 10, keywordThreshold: 0.3, vectorThreshold: 0.5, rerankTopK: 5, rerankThreshold: 0.5 };
  if (!config) return defaults;
  if (typeof config.embedding_top_k === 'number' && config.embedding_top_k) defaults.embeddingTopK = config.embedding_top_k;
  if (typeof config.rerank_top_k === 'number' && config.rerank_top_k) defaults.rerankTopK = config.rerank_top_k;
  if (typeof config.keyword_threshold === 'number') defaults.keywordThreshold = config.keyword_threshold;
  if (typeof config.vector_threshold === 'number') defaults.vectorThreshold = config.vector_threshold;
  if (typeof config.rerank_threshold === 'number') defaults.rerankThreshold = config.rerank_threshold;
  return defaults;
}

/**
 * Create-mode only (Vue watch(visible) 3474-3481): the fresh form's retrieval
 * knobs open at the tenant-configured defaults; edit mode keeps stored values.
 */
export function applyCreateRetrievalDefaults(form: AgentEditorForm, defaults: TenantRetrievalDefaults): void {
  form.config.embedding_top_k = defaults.embeddingTopK;
  form.config.keyword_threshold = defaults.keywordThreshold;
  form.config.vector_threshold = defaults.vectorThreshold;
  form.config.rerank_top_k = defaults.rerankTopK;
  form.config.rerank_threshold = defaults.rerankThreshold;
}

// --- R486 KB warn: preset KB filter + incompatible selected count (Vue 3190-3288) --------

export interface PresetKbFilter { any_of: string[]; all_of: string[]; none_of: string[] }

/**
 * Vue effectiveKbFilter (AgentEditorModal.vue:3190-3208 + frontend/src/utils/
 * tool-capabilities.ts deriveKbFilterFromTools): the any_of set is derived
 * from the preset's tool requirements (union of anyOf+allOf capabilities,
 * resolved through TOOL_CATALOG), a yaml any_of overrides the derivation, and
 * all_of/none_of ride along from the yaml. Empty everywhere → null (no filter).
 */
export function effectivePresetKbFilter(preset: AgentTypePreset | null): PresetKbFilter | null {
  if (!preset) return null;
  const derived = new Set<string>();
  for (const toolId of preset.config?.allowed_tools ?? []) {
    const tool = TOOL_CATALOG.find((tool) => tool.value === toolId);
    for (const capability of tool?.anyOf ?? []) derived.add(capability);
    for (const capability of tool?.allOf ?? []) derived.add(capability);
  }
  const yamlAnyOf = preset.kb_filter?.any_of ?? [];
  const anyOf = yamlAnyOf.length > 0 ? yamlAnyOf : [...derived];
  const allOf = preset.kb_filter?.all_of ?? [];
  const noneOf = preset.kb_filter?.none_of ?? [];
  if (anyOf.length === 0 && allOf.length === 0 && noneOf.length === 0) return null;
  return { any_of: anyOf, all_of: allOf, none_of: noneOf };
}

const kbCapabilitiesOf = (kb: KbOption): Partial<ToolCapabilityScope> => kb.capabilities ?? {
  vector: kb.ragEnabled,
  keyword: kb.ragEnabled,
  wiki: kb.wikiEnabled,
  graph: false,
  faq: kb.type === 'faq',
};

/**
 * Vue kbSatisfiesPresetFilter (3211-3243): evaluate a KB option against the
 * effective filter. The `reason` string comes from presetKbMismatchReason and
 * is only used for option tooltips; the count logic reads `ok`.
 */
export function kbSatisfiesPresetFilter(kb: KbOption, preset: AgentTypePreset | null): { ok: boolean; reason: string } {
  const filter = effectivePresetKbFilter(preset);
  if (!preset || !filter) return { ok: true, reason: '' };
  const caps = kbCapabilitiesOf(kb);
  const has = (name: string): boolean => (caps as Record<string, boolean | undefined>)[name] === true;
  if (filter.all_of.length > 0 && !filter.all_of.every(has)) return { ok: false, reason: 'mismatch' };
  if (filter.any_of.length > 0 && !filter.any_of.some(has)) return { ok: false, reason: 'mismatch' };
  if (filter.none_of.length > 0 && filter.none_of.some(has)) return { ok: false, reason: 'mismatch' };
  return { ok: true, reason: '' };
}

/**
 * Vue kbSatisfiesQuickAnswerMode (3250-3258): quick-answer retrieval needs a
 * vector or keyword index; wiki-only KBs would come back empty, so they count
 * as incompatible even though quick-answer has no agent_type preset.
 */
export function kbSatisfiesQuickAnswerMode(agentMode: AgentMode, kb: KbOption): boolean {
  if (agentMode !== 'quick-answer') return true;
  const caps = kb.capabilities;
  return caps ? caps.vector === true || caps.keyword === true : kb.ragEnabled;
}

/**
 * Vue incompatibleSelectedKbCount (3276-3281): how many of the explicitly
 * selected KBs the post-switch preset / mode would disable. Zero outside the
 * "selected" scope — an all/none preset switch clears the list before this
 * can count (Vue watch 3639-3650), so the warning only fires for presets that
 * keep the selected scope.
 */
export function incompatibleSelectedKbCount(
  kbMode: ScopeSelectionMode,
  selectedIds: readonly string[],
  kbOptions: readonly KbOption[],
  preset: AgentTypePreset | null,
  agentMode: AgentMode,
): number {
  if (kbMode !== 'selected') return 0;
  const selected = new Set(selectedIds);
  return kbOptions.filter((kb) => {
    if (!selected.has(kb.value)) return false;
    return !kbSatisfiesPresetFilter(kb, preset).ok || !kbSatisfiesQuickAnswerMode(agentMode, kb);
  }).length;
}

// --- MCP service options (AgentEditorModal.vue mcpOptions 2039-2068) ------------------

export interface McpServiceLike { id: string; name: string; enabled?: boolean }
export interface McpOptionRow { label: string; value: string; disabled?: boolean }

/**
 * MCP select options: enabled services first, then selected-but-disabled ones
 * flagged "(已禁用)", then ghost ids (selected service deleted elsewhere) shown
 * as unavailable. Mirrors the Vue computed that keeps stale selections visible
 * instead of silently dropping them.
 */
export function mcpOptionRows(services: McpServiceLike[], selectedIds: string[], t: Translate): McpOptionRow[] {
  const serviceById = new Map(services.map((mcp) => [mcp.id, mcp]));
  const selected = new Set(selectedIds);
  const rows: McpOptionRow[] = [];
  for (const mcp of services) {
    if (mcp.enabled !== false) rows.push({ label: mcp.name, value: mcp.id });
  }
  for (const id of selected) {
    const mcp = serviceById.get(id);
    if (mcp && mcp.enabled === false) {
      rows.push({ label: `${mcp.name} (${t('mcpSettings.disabled')})`, value: id, disabled: true });
    } else if (!mcp) {
      rows.push({ label: t('agentEditor.mcp.unavailableService'), value: id, disabled: true });
    }
  }
  return rows;
}
