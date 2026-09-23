/**
 * React port of frontend/src/views/agent/AgentEditorModal.vue (Vue baseline).
 * TDesign 同构迁移（Task 9 pilot）：DOM/类名按 Vue template 1:1 复刻
 * （settings-overlay / settings-modal / setting-row / setting-info /
 * setting-control …，样式平移在 agents.td.css），表单控件全部换 tdesign-react
 * 具名组件，图标用 tdesign-icons-react 的 Icon（= Vue `Icon as TIcon`，本地
 * sprite `<use>` 渲染）。纯逻辑仍在 agent-editor.ts / agent-type-presets.ts /
 * agent-editor-resources.ts。wk-* / data-* 测试 hook 按迁移前锚点保留。
 *
 * Stage scope notes（迁移前注释保留）：create/edit shell + grouped rail、
 * basic info（含 agent type presets）、prompts（R486 占位符标签条 + 模板控件）、
 * model config、conversation、question suggestions、personalization（MBTI）、
 * knowledge、retrieval（租户默认值）、web search、attachment upload（嵌入式
 * chat parser 规则）、tools、MCP、skills。R491 —— presets / 模板 / 占位符
 * 走运行时拉取（agents/type-presets 等），失败回退 vendored 静态目录。
 */
import React, { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import { createPortal } from 'react-dom';
import type { ModelConfiguration, SandboxConfigRecord, SkillCatalog, WeKnoraClient } from '@weknora/api-client';
import {
  Button,
  Checkbox,
  Input,
  InputNumber,
  Loading,
  Radio,
  RadioGroup,
  Select,
  Slider,
  Switch,
  Tabs,
  Textarea,
  Tooltip,
  MessagePlugin,
} from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import {
  AGENT_FILE_TYPE_OPTIONS,
  agentTypePresetDescription,
  agentTypePresetLabel,
  applyAgentModeSwitch,
  applyCreateRetrievalDefaults,
  applyKbSelectionMode,
  applyAgentTypePreset,
  applyScopeSelectionMode,
  buildAgentPayload,
  buildNavGroups,
  catalogSkillRows,
  defaultAgentForm,
  findAgentTypePreset,
  hydrateAgentForm,
  incompatibleSelectedKbCount,
  initKbSelectionMode,
  initScopeSelectionMode,
  insertPlaceholderAtCursor,
  isDescriptionSystemGenerated,
  isNameSystemGenerated,
  isNamedSandboxBackend,
  kbOptionFromRecord,
  mcpOptionRows,
  needsRerankModel,
  presetDefaultDescription,
  presetDefaultName,
  promptPlaceholdersFor,
  resolveAgentSystemPromptResetTemplate,
  seedCreateAgentForm,
  tenantRetrievalDefaultsFromConfig,
  TOOL_CATALOG,
  TOOL_GROUPS,
  validateAgentForm,
  type AgentEditorForm,
  type AgentFieldError,
  type AgentFormIssue,
  type AgentSectionKey,
  type KbOption,
  type McpServiceLike,
  type QuestionSuggestionsForm,
  type ScopeSelectionMode,
  type ToolCapabilityScope,
  type Translate,
} from './agent-editor.ts';
import { loadAgentEditorResources, resolveAgentEditorResources, type AgentEditorRuntimeData } from './agent-editor-resources.ts';
import { AgentParserRules, chatParserGroups, ensureCompleteParserRules } from './AgentParserRules.tsx';
import type { ParserEngineInfo, ParserEngineRule } from '../knowledge-settings/parserSettings.tsx';
import { PersonaSection } from './PersonaSection.tsx';
import { SubagentsSection } from './SubagentsSection.tsx';
import { navigate } from '../platform/navigation.ts';
import { usePreferredLocale } from '../locale.ts';
import { createAgentMarketplaceApi } from '../agent-marketplace/agent-marketplace-api.ts';
import { AgentVersionActions } from '../agent-marketplace/AgentVersionActions.tsx';
import './agents-u.css';

export interface AgentEditorModalProps {
  open: boolean;
  mode: 'create' | 'edit';
  agent?: Record<string, unknown> | null;
  initialSection?: string;
  initialHighlightField?: string;
  readOnly?: boolean;
  client: WeKnoraClient;
  t: Translate;
  onClose: () => void;
  onSaved?: (agent: Record<string, unknown>, mode: 'create' | 'edit') => void;
}

interface EditorDeps {
  models: ModelConfiguration[];
  kbOptions: KbOption[];
  providers: Array<{ id: string; name: string; is_default?: boolean }>;
  sandboxConfigs: SandboxConfigRecord[];
  catalog: SkillCatalog[];
  mcpServices: McpServiceLike[];
  storageStatus: Record<string, boolean>;
  // R486 — GET /system/parser-engines registry feeding the embedded chat
  // attachment parser rules editor (Vue editorResources.ensureParserEngines).
  parserEngines: ParserEngineInfo[];
}

const EMPTY_DEPS: EditorDeps = { models: [], kbOptions: [], providers: [], sandboxConfigs: [], catalog: [], mcpServices: [], storageStatus: {}, parserEngines: [] };
const EMPTY_SCOPE: ToolCapabilityScope = { vector: false, keyword: false, wiki: false, graph: false, faq: false };

const missReasonKey = (missKind: string): string =>
  missKind === 'needsWiki' ? 'agentEditor.tools.requiresWikiKb'
    : missKind === 'needsKb' ? 'agentEditor.tools.requiresKb'
      : 'agentEditor.tools.requiresRagKb';

/** Vue navItems SKILL_ICON = 'system-code'（types/mention.ts）；React 侧
 * buildNavGroups 的 'skills' 图标名在 tdesign sprite 无对应 symbol，视图层
 * 校正为 Vue 同名图标。 */
function navIconName(icon: string): string {
  return icon === 'skills' ? 'system-code' : icon;
}

// --- AgentAvatar.vue 端口（编辑器内局部副本，避免与 AgentsPage 循环依赖） -------

const AVATAR_GRADIENTS = [
  { from: '#667eea', to: '#764ba2' },
  { from: '#4facfe', to: '#00f2fe' },
  { from: '#43e97b', to: '#38f9d7' },
  { from: '#11998e', to: '#38ef7d' },
  { from: '#5ee7df', to: '#b490ca' },
  { from: '#48c6ef', to: '#6f86d6' },
  { from: '#a8edea', to: '#fed6e3' },
  { from: '#667db6', to: '#0082c8' },
  { from: '#36d1dc', to: '#5b86e5' },
  { from: '#56ab2f', to: '#a8e063' },
  { from: '#614385', to: '#516395' },
  { from: '#02aab0', to: '#00cdac' },
  { from: '#6a82fb', to: '#fc5c7d' },
  { from: '#834d9b', to: '#d04ed6' },
  { from: '#4776e6', to: '#8e54e9' },
  { from: '#00b09b', to: '#96c93d' },
];

function avatarGradient(name: string) {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    const char = name.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash;
  }
  return AVATAR_GRADIENTS[Math.abs(hash) % AVATAR_GRADIENTS.length]!;
}

function AgentAvatarChip({ name }: { name: string }) {
  const g = avatarGradient(name || '?');
  const letter = (() => {
    const first = (name || '').trim().charAt(0);
    if (!first) return '?';
    return /[a-zA-Z]/.test(first) ? first.toUpperCase() : first;
  })();
  return (
    <div
      className="agent-avatar"
      style={{ background: `linear-gradient(135deg, ${g.from} 0%, ${g.to} 100%)` }}
      data-agent-avatar
    >
      <svg className="agent-sparkles" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
        <path d="M24 5L24.4 6.6C24.45 6.85 24.65 7.05 24.9 7.1L26.5 7.5L24.9 7.9C24.65 7.95 24.45 8.15 24.4 8.4L24 10L23.6 8.4C23.55 8.15 23.35 7.95 23.1 7.9L21.5 7.5L23.1 7.1C23.35 7.05 23.55 6.85 23.6 6.6L24 5Z" fill="rgba(255,255,255,0.6)" />
        <path d="M7 22L7.4 23.6C7.45 23.85 7.65 24.05 7.9 24.1L9.5 24.5L7.9 24.9C7.65 24.95 7.45 25.15 7.4 25.4L7 27L6.6 25.4C6.55 25.15 6.35 24.95 6.1 24.9L4.5 24.5L6.1 24.1C6.35 24.05 6.55 23.85 6.6 23.6L7 22Z" fill="rgba(255,255,255,0.5)" />
      </svg>
      <span className="agent-avatar-letter" style={{ textShadow: `0 1px 2px ${g.to}80, 0 0 8px ${g.from}30` }}>{letter}</span>
    </div>
  );
}

// --- Vue setting-row 结构（AgentEditorModal.vue 5218-5381 平移 CSS 配套） --------

function Row({ label, required = false, desc, hint, descHint, htmlFor, error, extra, emphasize = false, vertical = false, highlight = false, dataGuide, dataAgentField, children }: {
  label?: string; required?: boolean; desc?: React.ReactNode; hint?: string; descHint?: string; htmlFor?: string;
  error?: string; extra?: React.ReactNode; emphasize?: boolean; vertical?: boolean; highlight?: boolean;
  dataGuide?: string; dataAgentField?: string; children: React.ReactNode;
}) {
  return (
    <div
      className={[
        'setting-row',
        vertical ? 'setting-row-vertical' : '',
        emphasize ? 'setting-row--emphasize' : '',
        highlight ? 'setting-row--field-highlight' : '',
      ].filter(Boolean).join(' ')}
      data-guide={dataGuide}
      data-agent-field={dataAgentField}
    >
      <div className="setting-info">
        {label ? (
        <label htmlFor={htmlFor}>
          {label}
          {/* Vue template: `{{ label }} <span class="required">*</span>` —— 空格是
              文本节点、星号独占 span；空格位置影响后续字形亚像素相位。 */}
          {required ? <> <span className="required" aria-hidden="true">*</span></> : null}
        </label>
        ) : null}
        {desc ? <p className="desc">{desc}{hint ? <><br /><span className="hint">{hint}</span></> : null}</p> : null}
        {descHint ? <p className="desc-hint">{descHint}</p> : null}
        {extra}
      </div>
      <div className="setting-control">
        {children}
        {error ? <p className="field-error" data-field-error={htmlFor ? htmlFor.replace('wk-ae-', '').replace(/-/g, '_') : undefined}>{error}</p> : null}
      </div>
    </div>
  );
}

/** Vue slider-wrapper：t-slider + 数值（AgentEditorModal.vue:609-613）。 */
function SliderRow({ value, min, max, step, ariaLabel, display, onChange }: {
  value: number; min: number; max: number; step: number; ariaLabel: string; display: string; onChange: (next: number) => void;
}) {
  return (
    <div className="slider-wrapper">
      <Slider value={value} min={min} max={max} step={step} aria-label={ariaLabel} onChange={(next) => onChange(Number(next))} />
      <span className="slider-value">{display}</span>
    </div>
  );
}

/** section-header（Vue 5154-5185）。 */
function SectionHeader({ title, desc, compact = false }: { title: string; desc: string; compact?: boolean }) {
  if (compact) {
    return (
      <div className="section-header section-header--compact">
        <h2>{title}</h2>
        <p className="section-description">{desc}</p>
      </div>
    );
  }
  return (
    <div className="section-header">
      <div className="section-header-title"><h2>{title}</h2></div>
      <p className="section-description">{desc}</p>
    </div>
  );
}

// --- main component --------------------------------------------------------------------

const VALID_INITIAL_HIGHLIGHTS = new Set(['summary_model', 'rerank_model', 'allowed_tools']);

export function AgentEditorModal({ open, mode, agent, initialSection, initialHighlightField, readOnly = false, client, t, onClose, onSaved }: AgentEditorModalProps) {
  const locale = usePreferredLocale();
  const [initializing, setInitializing] = useState(false);
  const [deps, setDeps] = useState<EditorDeps>(EMPTY_DEPS);
  const [form, setForm] = useState<AgentEditorForm>(defaultAgentForm);
  const [kbMode, setKbMode] = useState<ScopeSelectionMode>('all');
  const [mcpMode, setMcpMode] = useState<ScopeSelectionMode>('none');
  const [skillsMode, setSkillsMode] = useState<ScopeSelectionMode>('none');
  const [section, setSection] = useState<AgentSectionKey>('basic');
  const [highlightedField, setHighlightedField] = useState<string | null>(null);
  const [issues, setIssues] = useState<AgentFormIssue[]>([]);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [postCreate, setPostCreate] = useState(false);
  const [maxIterationsMode, setMaxIterationsMode] = useState<'limit' | 'unlimited'>('limit');
  const [maxTokensMode, setMaxTokensMode] = useState<'default' | 'custom'>('default');
  const [installingId, setInstallingId] = useState('');
  // R485 D1 — question suggestions tab (Vue suggestionTab, starters default)
  const [suggestionTab, setSuggestionTab] = useState<'starters' | 'followUps'>('starters');
  // R486 D4 — 使用模板 popup (Vue PromptTemplateSelector popupVisible)
  const [templatePanelOpen, setTemplatePanelOpen] = useState(false);
  // R486 KB warn — agent-type switch KB-conflict toast（Vue MessagePlugin.warning 4000ms）
  const kbWarnTimer = useRef<number | null>(null);
  // R491 — runtime editor catalogs (Vue editorResources prefetchAgentEditorDeps:
  // type-presets / prompt-templates / placeholders). null until the fetch
  // settles; resolve() merges whatever arrived over the vendored static
  // fallback so the editor renders before and after a failed fetch.
  const [runtimeResources, setRuntimeResources] = useState<AgentEditorRuntimeData | null>(null);
  const resources = useMemo(() => resolveAgentEditorResources(runtimeResources, locale), [runtimeResources, locale]);
  const marketplaceApi = useMemo(() => createAgentMarketplaceApi(client), [client]);
  // R486 D4 — caret insert targets (Vue promptTextareaRef / contextTemplateTextareaRef;
  // tdesign Textarea ref 暴露 { textareaElement })
  const systemPromptNodeRef = useRef<{ textareaElement: HTMLTextAreaElement | null } | null>(null);
  const contextTemplateNodeRef = useRef<{ textareaElement: HTMLTextAreaElement | null } | null>(null);
  const formRef = useRef(form);
  formRef.current = form;

  useEffect(() => () => { if (kbWarnTimer.current !== null) window.clearTimeout(kbWarnTimer.current); }, []);

  const patch = useCallback((mutate: (draft: AgentEditorForm) => void) => {
    setForm((current) => {
      const draft: AgentEditorForm = {
        ...current,
        config: { ...current.config, question_suggestions: { ...current.config.question_suggestions } },
      };
      mutate(draft);
      return draft;
    });
  }, []);

  const patchConfig = useCallback(<K extends keyof AgentEditorForm['config']>(key: K, value: AgentEditorForm['config'][K]) => {
    patch((draft) => { draft.config[key] = value; });
  }, [patch]);

  // Open lifecycle: dependency load + form hydration (Vue watch(visible) 3390-3562).
  useEffect(() => {
    if (!open) return;
    let active = true;
    setInitializing(true);
    setSaveError(null);
    setIssues([]);
    setSection(initialSection === 'sandbox' ? 'skills' : (initialSection && ['basic', 'prompts', 'model', 'conversation', 'suggestions', 'personalization', 'knowledge', 'retrieval', 'websearch', 'multimodal', 'tools', 'mcp', 'skills', 'subagents'].includes(initialSection) ? initialSection as AgentSectionKey : 'basic'));
    const highlight = initialHighlightField && VALID_INITIAL_HIGHLIGHTS.has(initialHighlightField) ? initialHighlightField : null;
    setHighlightedField(highlight);
    setPostCreate(false);
    setSuggestionTab('starters');
    setTemplatePanelOpen(false);
    void (async () => {
      const next: EditorDeps = { ...EMPTY_DEPS };
      const [models, kbs, sandboxes, catalog, providers, mcpServices, storageStatus, parserEngines, retrievalConfig, editorResources] = await Promise.allSettled([
        client.configuration.models.list(),
        client.knowledgeBases.list(),
        client.sandboxConfigurations.list(),
        client.configuration.skills.catalog.list(),
        client.settings.webSearch.providers.list(),
        client.configuration.mcp.list(),
        client.settings.storage.legacy.status(),
        // R486 — parser registry + tenant retrieval-config (Vue prefetchAgent-
        // EditorDeps ensureParserEngines / ensureTenantRetrievalConfig); the
        // Promise.resolve wrapper also catches a synchronously missing client
        // method so one absent endpoint cannot take the whole load down.
        Promise.resolve().then(() => client.knowledgeBases.settings.parserEngines()),
        Promise.resolve().then(() => client.settings.retrieval.get()),
        // R491 — type-presets / prompt-templates / placeholders (Vue prefetch-
        // AgentEditorDeps ensureAgentTypePresets / ensurePromptTemplates /
        // ensurePlaceholders); module-cached 60s + inflight de-dup inside, and
        // a failed fetch resolves to null fields (static fallback downstream).
        loadAgentEditorResources(client),
      ]);
      if (!active) return;
      if (models.status === 'fulfilled') next.models = Array.isArray(models.value) ? models.value as ModelConfiguration[] : [];
      if (kbs.status === 'fulfilled') {
        next.kbOptions = (Array.isArray(kbs.value) ? kbs.value : []).map((kb) => kbOptionFromRecord(kb as Record<string, unknown>));
      }
      if (sandboxes.status === 'fulfilled') next.sandboxConfigs = sandboxes.value.items ?? [];
      if (catalog.status === 'fulfilled') next.catalog = Array.isArray(catalog.value) ? catalog.value : [];
      if (providers.status === 'fulfilled') next.providers = Array.isArray(providers.value) ? providers.value as EditorDeps['providers'] : [];
      if (mcpServices.status === 'fulfilled') {
        next.mcpServices = (Array.isArray(mcpServices.value) ? mcpServices.value : []).map((row) => {
          const record = row as Record<string, unknown>;
          return { id: String(record.id ?? ''), name: String(record.name ?? record.id ?? ''), enabled: record.enabled !== false };
        });
      }
      if (storageStatus.status === 'fulfilled') {
        // GET /system/storage-engine-status -> { storage_engine_status: [{name, available}] }
        const rows = (storageStatus.value as Record<string, unknown>)?.storage_engine_status;
        if (Array.isArray(rows)) {
          const map: Record<string, boolean> = {};
          for (const row of rows) {
            if (row !== null && typeof row === 'object') {
              const record = row as Record<string, unknown>;
              if (typeof record.name === 'string') map[record.name] = record.available === true;
            }
          }
          next.storageStatus = map;
        }
      }
      if (parserEngines.status === 'fulfilled') {
        // GET /system/parser-engines -> { data: [{Name, Description, FileTypes, Available}] }
        const rows = (parserEngines.value as unknown as Record<string, unknown> | undefined)?.data;
        if (Array.isArray(rows)) {
          next.parserEngines = rows
            .filter((row): row is Record<string, unknown> => row !== null && typeof row === 'object')
            .map((row) => ({
              Name: String(row.Name ?? ''),
              ...(typeof row.Description === 'string' ? { Description: row.Description } : {}),
              ...(Array.isArray(row.FileTypes) ? { FileTypes: row.FileTypes.filter((ext): ext is string => typeof ext === 'string') } : {}),
              ...(row.Available === undefined ? {} : { Available: row.Available === true }),
            }));
        }
      }
      setDeps(next);
      // R491 — publish the fetched catalogs before the create prefill so the
      // seeded preset/template body matches what the dropdown renders (Vue
      // awaits loadDependencies before initializing the form).
      const runtimeData = editorResources.status === 'fulfilled' ? editorResources.value : null;
      if (active) setRuntimeResources(runtimeData);
      const resourcesNow = resolveAgentEditorResources(runtimeData, locale);

      if (mode === 'edit' && agent) {
        const hydrated = hydrateAgentForm(agent);
        setForm(hydrated);
        setKbMode(initKbSelectionMode(hydrated));
        setMcpMode(initScopeSelectionMode(hydrated.config.mcp_selection_mode, hydrated.config.mcp_services));
        setSkillsMode(initScopeSelectionMode(hydrated.config.skills_selection_mode, hydrated.config.selected_skills));
        setMaxIterationsMode(hydrated.config.max_iterations === -1 ? 'unlimited' : 'limit');
        setMaxTokensMode(hydrated.config.max_completion_tokens > 0 ? 'custom' : 'default');
      } else {
        // Vue:3474-3536 create defaults; KB scope starts at 全部, MCP/Skills off.
        // R485 D3: the default agent_type preset ('rag-qa') is applied on open
        // so name/description/system prompt/tools match the type dropdown.
        // R485 D5: chat/rerank models prefilled when empty.
        // R491: the runtime-fetched preset catalog + template bodies win over
        // the vendored fallback (backend config drives the prefill).
        const seeded = seedCreateAgentForm(t, locale, next.models, resourcesNow.typePresets, resourcesNow.promptTemplates);
        // R486 D9 — create-mode retrieval knobs open at the tenant-configured
        // values (Vue 3477-3481 newFormData defaults + 3912-3917 tenant reads;
        // the `|| / !== undefined` semantics live in tenantRetrievalDefaultsFromConfig)
        if (retrievalConfig.status === 'fulfilled') {
          applyCreateRetrievalDefaults(seeded, tenantRetrievalDefaultsFromConfig(retrievalConfig.value as Record<string, unknown>));
        }
        setForm(seeded);
        setKbMode('all');
        setMcpMode('none');
        setSkillsMode('none');
        setMaxIterationsMode('limit');
        setMaxTokensMode('default');
      }
      setInitializing(false);
    })();
    return () => { active = false; };
  }, [open, mode, agent, client, initialHighlightField, initialSection, t, locale]);

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      // A portaled project Dialog (e.g. the MBTI test modal) is stacked above
      // the editor and owns the Escape key while open; it closes itself.
      if (event.key === 'Escape' && document.querySelectorAll('.wk-dialog-backdrop').length === 0) onClose();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [open, onClose]);

  // R486 — Vue KBParserSettings ensureCompleteRules (onMounted → loadEngines):
  // once the parser registry is loaded, materialise a rule per chat-relevant
  // family so the stored config covers defaults too, not only touched rows.
  useEffect(() => {
    if (!open || deps.parserEngines.length === 0) return;
    const current = Array.isArray(form.config.chat_parser_engine_rules) ? form.config.chat_parser_engine_rules as ParserEngineRule[] : [];
    const complete = ensureCompleteParserRules(current, chatParserGroups(t, deps.parserEngines), deps.parserEngines);
    if (complete.length > current.length) patchConfig('chat_parser_engine_rules', complete);
  }, [open, deps.parserEngines, form.config.chat_parser_engine_rules, t, patchConfig]);

  // R486 D4 — Vue handlePlaceholderClick('system'|'context') → insertPlaceholder:
  // splice the full {{name}} token at the textarea caret and park the caret
  // after it.
  const insertPromptPlaceholder = useCallback((field: 'system_prompt' | 'context_template', name: string) => {
    const node = field === 'system_prompt' ? systemPromptNodeRef.current : contextTemplateNodeRef.current;
    const textarea = node?.textareaElement ?? null;
    const value = field === 'system_prompt' ? formRef.current.config.system_prompt : formRef.current.config.context_template;
    const caret = textarea ? textarea.selectionStart : value.length;
    const next = insertPlaceholderAtCursor(value, caret, name);
    patchConfig(field, next.value);
    if (textarea) {
      textarea.setSelectionRange(next.cursorPos, next.cursorPos);
      textarea.focus();
    }
  }, [patchConfig]);

  const isAgentMode = form.config.agent_mode === 'smart-reasoning';
  const hasKnowledgeBase = kbMode !== 'none';
  const quickAnswer = !isAgentMode;
  // R485 D2 — Vue activeAgentTypePreset 3118-3123 (agent-mode gated); R491 —
  // looked up in the runtime catalog with the static table as the fallback.
  const activeAgentTypePreset = isAgentMode && form.config.agent_type && form.config.agent_type !== 'custom'
    ? findAgentTypePreset(form.config.agent_type, resources.typePresets)
    : null;

  const scope = useMemo<ToolCapabilityScope>(() => {
    if (!hasKnowledgeBase) return EMPTY_SCOPE;
    const inScope = kbMode === 'all'
      ? deps.kbOptions
      : deps.kbOptions.filter((kb) => form.config.knowledge_bases.includes(kb.value));
    const next: ToolCapabilityScope = { ...EMPTY_SCOPE };
    for (const kb of inScope) {
      const caps = kb.capabilities;
      if (caps) {
        next.vector = next.vector || caps.vector === true;
        next.keyword = next.keyword || caps.keyword === true;
        next.wiki = next.wiki || caps.wiki === true;
        next.graph = next.graph || caps.graph === true;
        next.faq = next.faq || caps.faq === true;
      } else {
        if (kb.ragEnabled) { next.vector = true; next.keyword = true; }
        if (kb.wikiEnabled) next.wiki = true;
        if (kb.type === 'faq') next.faq = true;
      }
    }
    return next;
  }, [deps.kbOptions, form.config.knowledge_bases, hasKnowledgeBase, kbMode]);

  const hasFaqKnowledgeBase = useMemo(() => {
    if (!hasKnowledgeBase) return false;
    if (kbMode === 'all') return deps.kbOptions.some((kb) => kb.type === 'faq');
    return deps.kbOptions.some((kb) => form.config.knowledge_bases.includes(kb.value) && kb.type === 'faq');
  }, [deps.kbOptions, form.config.knowledge_bases, hasKnowledgeBase, kbMode]);

  // R485 D6 — Vue needsRerankModel 3373-3388: required while a RAG KB is reachable
  const rerankRequired = useMemo(
    () => needsRerankModel(kbMode, deps.kbOptions, form.config.knowledge_bases),
    [deps.kbOptions, form.config.knowledge_bases, kbMode],
  );

  const navGroups = useMemo(
    () => buildNavGroups({ isAgentMode, hasKnowledgeBase }),
    [hasKnowledgeBase, isAgentMode],
  );

  const evaluateTool = useCallback((tool: typeof TOOL_CATALOG[number]): { ok: boolean; missKind: string } => {
    const has = (capability: string): boolean => (scope as unknown as Record<string, boolean>)[capability] === true;
    if (!tool.anyOf?.length && !tool.allOf?.length) return { ok: true, missKind: 'none' };
    if (!hasKnowledgeBase) return { ok: false, missKind: 'needsKb' };
    if (tool.allOf?.length && !tool.allOf.every(has)) {
      return { ok: false, missKind: tool.allOf.includes('wiki') ? 'needsWiki' : 'needsRag' };
    }
    if (tool.anyOf?.length && !tool.anyOf.some(has)) return { ok: false, missKind: 'needsRag' };
    return { ok: true, missKind: 'none' };
  }, [hasKnowledgeBase, scope]);

  const errorMessage = useCallback((field: AgentFieldError): string | undefined => {
    const issue = issues.find((candidate) => candidate.field === field);
    return issue ? t(issue.messageKey) : undefined;
  }, [issues, t]);

  const onKbModeChange = useCallback((next: ScopeSelectionMode) => {
    setKbMode(next);
    setForm((current) => {
      const draft = { ...current, config: { ...current.config } };
      applyKbSelectionMode(draft, next);
      return draft;
    });
  }, []);

  const onAgentModeChange = useCallback((next: 'quick-answer' | 'smart-reasoning') => {
    setForm((current) => {
      const draft = { ...current, config: { ...current.config, allowed_tools: [...current.config.allowed_tools] } };
      applyAgentModeSwitch(draft, next, scope, hasKnowledgeBase);
      return draft;
    });
    if (next === 'smart-reasoning') setMaxIterationsMode('limit');
  }, [hasKnowledgeBase, scope]);

  const onScopeModeChange = useCallback((field: 'mcp' | 'skills', next: ScopeSelectionMode) => {
    if (field === 'mcp') setMcpMode(next); else setSkillsMode(next);
    setForm((current) => {
      const draft = { ...current, config: { ...current.config } };
      applyScopeSelectionMode(draft, field, next);
      return draft;
    });
  }, []);

  // R485 D2 + R486 KB warn — Vue onAgentTypeChange 3334-3361: system-generated
  // name/description refresh (user-edited values survive), preset application,
  // KB-mode sync (the Vue watch clears the explicit list on all/none), and a
  // soft warning when the post-switch preset/mode disables selected KBs
  // (MessagePlugin.warning kbIncompatibleWarn, 4000ms).
  const onAgentTypeChange = useCallback((value: string) => {
    const current = formRef.current;
    const canOverrideName = isNameSystemGenerated(current.name, t, locale, resources.typePresets);
    const canOverrideDesc = isDescriptionSystemGenerated(current.description, locale, resources.typePresets);
    const preset = findAgentTypePreset(value, resources.typePresets);
    const draft: AgentEditorForm = {
      ...current,
      config: {
        ...current.config,
        allowed_tools: [...current.config.allowed_tools],
        supported_file_types: [...current.config.supported_file_types],
      },
    };
    draft.config.agent_type = value;
    if (value !== 'custom') applyAgentTypePreset(draft, preset, resources.promptTemplates);
    if (canOverrideName) draft.name = presetDefaultName(preset, t, locale);
    if (canOverrideDesc) draft.description = presetDefaultDescription(preset, locale);
    const presetKbMode = preset?.config?.kb_selection_mode;
    if (presetKbMode === 'all' || presetKbMode === 'selected' || presetKbMode === 'none') {
      // Vue applyAgentTypePreset 3316-3318 writes both the config and the UI
      // radio; the kbSelectionMode watch (3639-3650) clears the explicit list
      // for all/none so residue never leaks into the payload.
      draft.config.kb_selection_mode = presetKbMode;
      if (presetKbMode !== 'selected') draft.config.knowledge_bases = [];
      setKbMode(presetKbMode);
    }
    setForm(draft);
    // Vue 3342-3347 — soft warning on incompatible selections (no forced removal)
    const nextMode = draft.config.kb_selection_mode;
    const count = incompatibleSelectedKbCount(nextMode, draft.config.knowledge_bases, deps.kbOptions, value === 'custom' ? null : preset, draft.config.agent_mode);
    if (kbWarnTimer.current !== null) window.clearTimeout(kbWarnTimer.current);
    if (count > 0) {
      void MessagePlugin.warning(t('agentEditor.agentType.kbIncompatibleWarn', { count }));
      kbWarnTimer.current = window.setTimeout(() => { /* 4s 后自然消失（MessagePlugin 自身超时） */ }, 4000);
    }
  }, [t, locale, deps.kbOptions, resources.typePresets, resources.promptTemplates]);

  // Deep-patch helper for the nested question_suggestions block (Vue v-model
  // binds straight into the nested objects; React needs a fresh copy).
  const patchQs = useCallback((mutate: (qs: QuestionSuggestionsForm) => void) => {
    patch((draft) => {
      const qs = structuredClone(draft.config.question_suggestions);
      mutate(qs);
      draft.config.question_suggestions = qs;
    });
  }, [patch]);

  const skillRows = useMemo(
    () => catalogSkillRows(deps.catalog, form.config.sandbox_config_id),
    [deps.catalog, form.config.sandbox_config_id],
  );
  const namedSandboxes = useMemo(
    () => deps.sandboxConfigs.filter((cfg) => isNamedSandboxBackend(cfg.sandbox_type)),
    [deps.sandboxConfigs],
  );
  const sandboxOptions = useMemo(() => {
    const selected = form.config.sandbox_config_id;
    if (!selected || namedSandboxes.some((cfg) => cfg.id === selected)) return namedSandboxes;
    // keep a deleted config visible instead of silently dropping it (Vue:2274-2284)
    return [...namedSandboxes, { id: selected, name: t('agent.editor.sandboxBackendMissing'), sandbox_type: '', config: {}, created_at: '', updated_at: '' } as SandboxConfigRecord];
  }, [form.config.sandbox_config_id, namedSandboxes, t]);

  const handleSave = async () => {
    if (saving || initializing) return; // duplicate-submit protection (Vue saving flag)
    const current = { ...formRef.current, config: { ...formRef.current.config } };
    const found = validateAgentForm(current);
    setIssues(found);
    if (found.length > 0) {
      setSection(found[0]!.section); // section jump mirrors the Vue handleSave flow
      return;
    }
    // prune skills that are not selectable on the current sandbox (Vue:4813)
    const selectableNames = new Set(skillRows.filter((row) => row.selectable).map((row) => row.name));
    current.config.selected_skills = current.config.selected_skills.filter((name) => selectableNames.has(name));
    const payload = buildAgentPayload(current);
    setSaving(true);
    setSaveError(null);
    try {
      if (mode === 'edit' && current.id) {
        const updated = await client.configuration.agents.update(current.id, payload);
        onSaved?.(updated as Record<string, unknown>, 'edit');
        onClose();
      } else if (postCreate) {
        const updated = await client.configuration.agents.update(current.id ?? '', payload);
        onSaved?.(updated as Record<string, unknown>, 'edit');
        onClose();
      } else {
        const created = await client.configuration.agents.create(payload) as Record<string, unknown>;
        // first save stays in the modal as a post-create edit session (Vue:4817-4829)
        setPostCreate(true);
        const hydrated = hydrateAgentForm(created);
        setForm(hydrated);
        setKbMode(initKbSelectionMode(hydrated));
        setSection('basic');
        onSaved?.(created, 'create');
      }
    } catch (failure) {
      setSaveError(failure instanceof Error ? failure.message : t('agent.messages.saveFailed'));
    } finally {
      setSaving(false);
    }
  };

  const installSkill = async (catalogId: string) => {
    if (!form.config.sandbox_config_id || installingId) return;
    setInstallingId(catalogId);
    try {
      await client.configuration.skills.catalog.install(catalogId, [form.config.sandbox_config_id]);
      const refreshed = await client.configuration.skills.catalog.list();
      setDeps((currentDeps) => ({ ...currentDeps, catalog: Array.isArray(refreshed) ? refreshed : currentDeps.catalog }));
    } catch {
      setSaveError(t('settings.sandbox.skillUploadFailed'));
    } finally {
      setInstallingId('');
    }
  };

  if (!open) return null;

  const editorMode: 'create' | 'edit' = postCreate ? 'edit' : mode;
  const chatModels = deps.models.filter((model) => String(model.type ?? '') === 'KnowledgeQA');
  const rerankModels = deps.models.filter((model) => String(model.type ?? '') === 'Rerank');
  const rewriteActive = quickAnswer && form.config.multi_turn_enabled && form.config.enable_rewrite;
  const showFallback = quickAnswer && hasKnowledgeBase;
  const hasSandbox = !!form.config.sandbox_config_id;
  const saveLabel = editorMode === 'create' ? t('agent.editor.buttons.create') : t('agent.editor.buttons.saveAndClose');

  // R486 D4 — the placeholder tag strip Vue renders under each prompt desc
  // (AgentEditorModal.vue:221-231): 可用变量 label, one {{name}} chip per
  // definition (tooltip = description + click-to-insert) and the hint.
  const renderPlaceholderTags = (field: 'agent_system_prompt' | 'system_prompt' | 'context_template', target: 'system_prompt' | 'context_template', tagBlock: string) => (
    <div className="placeholder-tags" data-placeholder-tags={tagBlock}>
      <span className="placeholder-label">{t('agentEditor.placeholders.available')}</span>
      {promptPlaceholdersFor(field, resources.placeholders).map((def) => (
        <Tooltip key={def.name} content={`${def.description}${t('agentEditor.placeholders.clickToInsert')}`} placement="top">
          <span
            className="placeholder-tag"
            data-placeholder-tag={def.name}
            onClick={() => insertPromptPlaceholder(target, def.name)}
          >{`{{${def.name}}}`}</span>
        </Tooltip>
      ))}
      <span className="placeholder-hint">{t('agentEditor.placeholders.hint')}</span>
    </div>
  );

  // R486 D4 — PromptTemplateSelector corner controls（Vue 组件为 body 弹层，
  // 此处保留 React 版下拉；类名走 agents.td.css 的 wk-ae-tpl 段）。
  const renderTemplateControls = () => (
    <div className="wk-ae-tpl-controls" data-template-controls>
      <button
        type="button"
        data-prompt-reset-default
        className="wk-ae-tpl-reset"
        onClick={() => {
          const template = resolveAgentSystemPromptResetTemplate(form.config.agent_type, resources.typePresets, resources.promptTemplates);
          if (!template) return;
          patch((draft) => {
            draft.config.system_prompt = template.content;
            draft.config.system_prompt_id = template.id;
          });
        }}
      >⟲ {t('promptTemplate.resetDefault')}</button>
      <div className="wk-ae-tpl-wrap">
        <button
          type="button"
          data-prompt-template-toggle
          aria-haspopup="listbox"
          aria-expanded={templatePanelOpen ? 'true' : 'false'}
          className="wk-ae-tpl-toggle"
          onClick={() => setTemplatePanelOpen((current) => !current)}
        >▦ {t('promptTemplate.useTemplate')}</button>
        {templatePanelOpen ? (
          <div className="wk-ae-tpl-panel" role="listbox" data-prompt-template-panel>
            <p className="wk-ae-tpl-panel-title">{t('promptTemplate.selectTemplate')}</p>
            <div className="wk-ae-tpl-panel-body">
              {resources.promptTemplates.length === 0 ? (
                <p className="wk-ae-tpl-empty">{t('promptTemplate.noTemplates')}</p>
              ) : resources.promptTemplates.map((template) => (
                <button
                  key={template.id}
                  type="button"
                  data-prompt-template={template.id}
                  className="wk-ae-tpl-item"
                  onClick={() => {
                    // Vue handleSystemPromptTemplateSelect (4683-4686): the
                    // selection replaces the body only; system_prompt_id is
                    // written by 恢复默认, not by a manual pick.
                    patchConfig('system_prompt', template.content);
                    setTemplatePanelOpen(false);
                  }}
                >
                  <span className="wk-ae-tpl-item-name">
                    {template.name}
                    {template.default ? <span className="wk-ae-tpl-default" data-template-default>{t('promptTemplate.default')}</span> : null}
                  </span>
                  <span className="wk-ae-tpl-item-desc">{template.description}</span>
                </button>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );

  function renderBasic() {
    const editId = editorMode === 'edit' ? form.id : undefined;
    return (
      <div className="section" data-editor-section="basic">
        <SectionHeader title={t('agent.editor.basicInfo')} desc={t('agent.editor.basicInfoDesc')} />
        <div className="settings-group">
          {editId ? (
            <Row label={t('agent.editor.agentId')} desc={t('agent.editor.agentIdDesc')}>
              <div className="agent-id-field">
                <code className="agent-id-value" title={editId} data-agent-id-copy>{editId}</code>
                <Tooltip content={t('common.copy')} placement="top">
                  <Button theme="default" size="small" variant="text" className="agent-id-copy" onClick={() => { void navigator.clipboard?.writeText(editId); }}>
                    <TIcon name="file-copy" />
                  </Button>
                </Tooltip>
              </div>
            </Row>
          ) : null}
          <Row label={t('agent.editor.mode')} required desc={isAgentMode ? t('agent.editor.agentDesc') : t('agent.editor.normalDesc')}>
            <RadioGroup value={form.config.agent_mode} disabled={form.is_builtin} onChange={(value) => onAgentModeChange(value as 'quick-answer' | 'smart-reasoning')} data-guide="agent-create-mode">
              <Radio.Button value="quick-answer">{t('agent.type.normal')}</Radio.Button>
              <Radio.Button value="smart-reasoning">{t('agent.type.agent')}</Radio.Button>
            </RadioGroup>
          </Row>
          {isAgentMode && resources.typePresets.length > 0 ? (
            // R485 D2 — Vue 118-137: agent type dropdown (smart-reasoning only),
            // setting-row--emphasize（3px 品牌条 + 加粗 label），preset 描述
            // echo 在 .agent-type-preset-desc；选项双行（agent-type-popup 弹层）。
            <Row
              label={t('agentEditor.agentType.label')}
              desc={t('agentEditor.agentType.desc')}
              emphasize
              dataGuide="agent-create-agent-type"
              extra={activeAgentTypePreset ? (
                <p className="desc agent-type-preset-desc" data-agent-type-desc>{agentTypePresetDescription(activeAgentTypePreset, locale)}</p>
              ) : null}
            >
              <Select
                data-field="agent_type"
                className="agent-type-select"
                value={form.config.agent_type || 'custom'}
                disabled={form.is_builtin}
                placeholder={t('agentEditor.agentType.label')}
                popupProps={{ overlayClassName: 'agent-type-popup' } as never}
                onChange={(value) => onAgentTypeChange(String(value))}
              >
                {resources.typePresets.map((preset) => (
                  <Select.Option key={preset.id} value={preset.id} label={agentTypePresetLabel(preset, locale)} title={agentTypePresetLabel(preset, locale)} disabled={preset.id === form.config.agent_type}>
                    <div className="agent-type-option">
                      <span className="agent-type-option-label">{agentTypePresetLabel(preset, locale)}</span>
                      <span className="agent-type-option-desc">{agentTypePresetDescription(preset, locale)}</span>
                    </div>
                  </Select.Option>
                ))}
              </Select>
            </Row>
          ) : null}
          <Row label={t('agent.editor.name')} required={!form.is_builtin} desc={t('agentEditor.desc.name')} htmlFor="wk-ae-name" error={errorMessage('name')}>
            {/* Vue 148-157: avatar chip (AgentAvatar) + input share the control
                column; builtin agents render a plain icon square instead. */}
            <div className="name-input-wrapper" data-guide="agent-create-name">
              {form.is_builtin ? (
                <div className={`builtin-avatar${isAgentMode ? 'agent' : 'normal'}`}>
                  <TIcon name={isAgentMode ? 'control-platform' : 'chat'} size="24px" />
                </div>
              ) : (
                <AgentAvatarChip name={form.name || '?'} />
              )}
              <Input
                data-field="name"
                className="name-input"
                value={form.name}
                placeholder={t('agent.editor.namePlaceholder')}
                disabled={form.is_builtin}
                onChange={(value) => patch((draft) => { draft.name = String(value); })}
              />
            </div>
          </Row>
          <Row label={t('agent.editor.description')} desc={t('agentEditor.desc.description')} htmlFor="wk-ae-description">
            <Textarea
              data-field="description"
              value={form.description}
              placeholder={t('agent.editor.descriptionPlaceholder')}
              autosize={{ minRows: 2, maxRows: 4 }}
              disabled={form.is_builtin}
              onChange={(value) => patch((draft) => { draft.description = String(value); })}
            />
          </Row>
          <Row label={t('agent.editor.memoryEnabled')} desc={t('agentEditor.desc.memoryEnabled')}>
            <Switch data-switch="memory_enabled" value={form.config.memory_enabled} onChange={(value) => patchConfig('memory_enabled', value === true)} />
          </Row>
        </div>
      </div>
    );
  }

  function renderPrompts() {
    return (
      <div className="section section--prompts" data-editor-section="prompts">
        <div className="prompts-panel">
          <div className="prompts-panel__header">
            <SectionHeader compact title={t('agent.editor.promptsConfig')} desc={t('agent.editor.promptsConfigDesc')} />
          </div>
          <div className="prompts-panel__body">
            <div className="settings-group">
              <Row
                label={t('agent.editor.systemPrompt')}
                required={!form.is_builtin}
                desc={`${t('agentEditor.desc.systemPrompt')}${form.is_builtin ? t('agentEditor.desc.leaveEmptyDefault') : ''}`}
                vertical
                htmlFor="wk-ae-system-prompt"
                error={errorMessage('system_prompt')}
                extra={renderPlaceholderTags(isAgentMode ? 'agent_system_prompt' : 'system_prompt', 'system_prompt', 'system')}
              >
                <div className="setting-control-full" style={{ position: 'relative' } as CSSProperties}>
                  <div className="textarea-with-template">
                    {isAgentMode ? renderTemplateControls() : null}
                    <Textarea
                      data-field="system_prompt"
                      ref={systemPromptNodeRef as never}
                      className="system-prompt-textarea"
                      value={form.config.system_prompt}
                      disabled={form.is_builtin}
                      autosize={{ minRows: 10, maxRows: 25 }}
                      onChange={(value) => patchConfig('system_prompt', String(value))}
                    />
                  </div>
                </div>
              </Row>
              {quickAnswer ? (
                <Row
                  label={t('agent.editor.contextTemplate')}
                  required={!form.is_builtin}
                  desc={`${t('agentEditor.desc.contextTemplate')}${form.is_builtin ? t('agentEditor.desc.leaveEmptyDefault') : ''}`}
                  vertical
                  htmlFor="wk-ae-context-template"
                  error={errorMessage('context_template')}
                  extra={renderPlaceholderTags('context_template', 'context_template', 'context')}
                >
                  <div className="setting-control-full" style={{ position: 'relative' } as CSSProperties}>
                    <div className="textarea-with-template">
                      <Textarea
                        data-field="context_template"
                        ref={contextTemplateNodeRef as never}
                        className="system-prompt-textarea"
                        value={form.config.context_template}
                        disabled={form.is_builtin}
                        autosize={{ minRows: 8, maxRows: 20 }}
                        onChange={(value) => patchConfig('context_template', String(value))}
                      />
                    </div>
                  </div>
                </Row>
              ) : null}
              {rewriteActive ? (
                <Row label={t('agent.editor.rewritePromptUser')} desc={t('agentEditor.desc.rewriteUserPrompt')} htmlFor="wk-ae-rewrite-user" error={errorMessage('rewrite_prompt_user')} vertical>
                  <div className="setting-control-full">
                    <div className="textarea-with-template">
                      <Textarea
                        data-field="rewrite_prompt_user"
                        value={form.config.rewrite_prompt_user}
                        autosize={{ minRows: 4, maxRows: 10 }}
                        onChange={(value) => patchConfig('rewrite_prompt_user', String(value))}
                      />
                    </div>
                  </div>
                </Row>
              ) : null}
              {showFallback ? (
                <Row label={t('agent.editor.fallbackStrategy')} desc={t('agentEditor.desc.fallbackStrategy')}>
                  <RadioGroup value={form.config.fallback_strategy} onChange={(value) => patchConfig('fallback_strategy', value as 'fixed' | 'model')}>
                    <Radio.Button value="fixed">{t('agentEditor.fallback.fixed')}</Radio.Button>
                    <Radio.Button value="model">{t('agentEditor.fallback.model')}</Radio.Button>
                  </RadioGroup>
                </Row>
              ) : null}
              {showFallback && form.config.fallback_strategy === 'fixed' ? (
                <Row label={t('agent.editor.fallbackResponse')} desc={t('agentEditor.desc.fallbackResponse')} htmlFor="wk-ae-fallback-response" vertical>
                  <div className="setting-control-full">
                    <div className="textarea-with-template">
                      <Textarea
                        data-field="fallback_response"
                        value={form.config.fallback_response}
                        autosize={{ minRows: 2, maxRows: 6 }}
                        onChange={(value) => patchConfig('fallback_response', String(value))}
                      />
                    </div>
                  </div>
                </Row>
              ) : null}
              {showFallback && form.config.fallback_strategy === 'model' ? (
                <Row label={t('agent.editor.fallbackPrompt')} desc={t('agentEditor.desc.fallbackPrompt')} htmlFor="wk-ae-fallback-prompt" error={errorMessage('fallback_prompt')} vertical>
                  <div className="setting-control-full" style={{ position: 'relative' } as CSSProperties}>
                    <div className="textarea-with-template">
                      <Textarea
                        data-field="fallback_prompt"
                        value={form.config.fallback_prompt}
                        autosize={{ minRows: 4, maxRows: 10 }}
                        onChange={(value) => patchConfig('fallback_prompt', String(value))}
                      />
                    </div>
                  </div>
                </Row>
              ) : null}
            </div>
          </div>
        </div>
      </div>
    );
  }

  function renderModel() {
    return (
      <div className="section" data-editor-section="model" data-highlighted-field={highlightedField === 'summary_model' || highlightedField === 'rerank_model' ? highlightedField : undefined}>
        <SectionHeader title={t('agent.editor.modelConfig')} desc={t('agent.editor.modelConfigDesc')} />
        <div className="settings-group">
          <Row
            label={t('agent.editor.model')}
            required
            desc={t('agentEditor.desc.model')}
            htmlFor="wk-ae-model-id"
            error={errorMessage('model_id')}
            dataGuide="agent-create-model"
            dataAgentField="summary_model"
            highlight={highlightedField === 'summary_model'}
          >
            <Select
              data-field="model_id"
              className="wk-ae-sel-model"
              value={form.config.model_id}
              clearable
              placeholder={t('agent.editor.modelPlaceholder')}
              onChange={(value) => patchConfig('model_id', String(value ?? ''))}
            >
              {chatModels.map((model) => <Select.Option key={model.id} value={model.id} label={model.name} title={model.name} />)}
            </Select>
          </Row>
          <Row label={t('agent.editor.temperature')} desc={t('agentEditor.desc.temperature')}>
            <SliderRow ariaLabel={t('agent.editor.temperature')} min={0} max={1} step={0.1} value={form.config.temperature}
              display={`${form.config.temperature}`} onChange={(next) => patchConfig('temperature', next)} />
          </Row>
          <Row label={t('agent.editor.maxCompletionTokens')} desc={isAgentMode ? t('agentEditor.desc.maxTokensAgent') : t('agentEditor.desc.maxTokens')}>
            <div className="max-tokens-control">
              <RadioGroup
                value={maxTokensMode}
                onChange={(value) => {
                  setMaxTokensMode(value as 'default' | 'custom');
                  patchConfig('max_completion_tokens', value === 'custom' ? 2048 : 0);
                }}
              >
                <Radio.Button value="default">{t('agent.editor.maxCompletionTokensDefault')}</Radio.Button>
                <Radio.Button value="custom">{t('agent.editor.maxCompletionTokensCustom')}</Radio.Button>
              </RadioGroup>
              {maxTokensMode === 'default' ? (
                /* Vue effectiveDefaultMaxCompletionTokens（2345-2347 常量：
                 * quick-answer 2048 / smart-reasoning 4096 / 沙箱写入 24576）。 */
                <span className="max-tokens-value">{isAgentMode ? (form.config.sandbox_config_id ? 24576 : 4096) : 2048}</span>
              ) : (
                <InputNumber
                  className="wk-ae-num-max_completion_tokens"
                  inputProps={{ "data-field": "max_completion_tokens" } as never}
                  theme="column"
                  min={100}
                  max={100000}
                  step={100}
                  value={form.config.max_completion_tokens}
                  onChange={(value) => patchConfig('max_completion_tokens', Number(value) || 0)}
                />
              )}
            </div>
          </Row>
          <Row label={t('agent.editor.thinking')} desc={t('agentEditor.desc.thinking')}>
            <Switch data-switch="thinking" value={form.config.thinking} onChange={(value) => patchConfig('thinking', value === true)} />
          </Row>
          <Row label={t('agent.editor.citationEnabled')} desc={t('agent.editor.citationEnabledDesc')}>
            <Switch data-switch="citation_enabled" value={form.config.citation_enabled} onChange={(value) => patchConfig('citation_enabled', value === true)} />
          </Row>
          {hasKnowledgeBase ? (
            // R485 D6 — Vue 671-689: the rerank row is required (star, no
            // optional hint, no clear option) while a RAG KB is in scope.
            <Row
              label={t('agent.editor.rerankModel')}
              desc={t('agent.editor.rerankModelDesc')}
              required={rerankRequired}
              hint={rerankRequired ? undefined : t('agent.editor.rerankModelOptionalHint')}
              dataAgentField="rerank_model"
              highlight={highlightedField === 'rerank_model'}
            >
              <Select
                data-field="rerank_model_id"
                className="wk-ae-sel-rerank"
                value={form.config.rerank_model_id}
                clearable={!rerankRequired}
                placeholder={t('agent.editor.rerankModelPlaceholder')}
                onChange={(value) => patchConfig('rerank_model_id', String(value ?? ''))}
              >
                {rerankModels.map((model) => <Select.Option key={model.id} value={model.id} label={model.name} title={model.name} />)}
              </Select>
            </Row>
          ) : null}
          {quickAnswer && form.config.multi_turn_enabled && form.config.enable_rewrite ? (
            <Row label={t('agent.editor.queryUnderstandModel')} desc={t('agentEditor.desc.queryUnderstandModel')}>
              <Select
                data-field="query_understand_model_id"
                className="wk-ae-sel-query-understand"
                value={form.config.query_understand_model_id}
                clearable
                placeholder={t('agent.editor.queryUnderstandModelPlaceholder')}
                onChange={(value) => patchConfig('query_understand_model_id', String(value ?? ''))}
              >
                {chatModels.map((model) => <Select.Option key={model.id} value={model.id} label={model.name} title={model.name} />)}
              </Select>
            </Row>
          ) : null}
          {isAgentMode ? (
            <Row label={t('agent.editor.maxIterations')} desc={t('agentEditor.desc.maxIterations')}>
              <div className="max-tokens-control">
                <RadioGroup
                  value={maxIterationsMode}
                  onChange={(value) => {
                    setMaxIterationsMode(value as 'limit' | 'unlimited');
                    patchConfig('max_iterations', value === 'unlimited' ? -1 : 10);
                  }}
                >
                  <Radio.Button value="limit">{t('agent.editor.maxIterationsLimit')}</Radio.Button>
                  <Radio.Button value="unlimited">{t('agent.editor.maxIterationsUnlimited')}</Radio.Button>
                </RadioGroup>
                {maxIterationsMode === 'limit' ? (
                  <InputNumber
                    className="wk-ae-num-max_iterations"
                    inputProps={{ "data-field": "max_iterations" } as never}
                    theme="column"
                    min={2}
                    max={50}
                    value={form.config.max_iterations}
                    onChange={(value) => patchConfig('max_iterations', Number(value) || 1)}
                  />
                ) : null}
              </div>
            </Row>
          ) : null}
          {isAgentMode ? (
            <Row label={t('agentEditor.llmCallTimeout.label')} desc={t('agentEditor.llmCallTimeout.desc')} descHint={t('agentEditor.llmCallTimeout.hint')}>
              <InputNumber
                className="wk-ae-num-llm_call_timeout"
                inputProps={{ "data-field": "llm_call_timeout" } as never}
                theme="column"
                min={0}
                max={3600}
                value={form.config.llm_call_timeout}
                placeholder={t('agentEditor.llmCallTimeout.placeholder')}
                onChange={(value) => patchConfig('llm_call_timeout', Number(value) || 0)}
              />
            </Row>
          ) : null}
        </div>
      </div>
    );
  }

  function renderMultimodal() {
    const vlmModels = deps.models.filter((model) => String(model.type ?? '') === 'VLLM');
    const asrModels = deps.models.filter((model) => String(model.type ?? '') === 'ASR');
    const storageOptions: Array<{ value: string; label: string; disabled?: boolean }> = [
      { value: 'local', label: t('agentEditor.imageUpload.engineLocal') },
      { value: 'minio', label: 'MinIO', ...(deps.storageStatus.minio === false ? { disabled: true } : {}) },
      { value: 'cos', label: t('agentEditor.imageUpload.engineCos'), ...(deps.storageStatus.cos === false ? { disabled: true } : {}) },
      { value: 'tos', label: t('agentEditor.imageUpload.engineTos'), ...(deps.storageStatus.tos === false ? { disabled: true } : {}) },
      { value: 's3', label: 'Amazon S3', ...(deps.storageStatus.s3 === false ? { disabled: true } : {}) },
      { value: 'oss', label: t('agentEditor.imageUpload.engineOss'), ...(deps.storageStatus.oss === false ? { disabled: true } : {}) },
    ];
    return (
      <div className="section" data-editor-section="multimodal">
        <SectionHeader title={t('agentEditor.imageUpload.sectionTitle')} desc={t('agentEditor.imageUpload.sectionDesc')} />
        <div className="settings-group">
          <Row label={t('agentEditor.imageUpload.label')} desc={t('agentEditor.imageUpload.desc')} dataGuide="agent-create-multimodal">
            <Switch data-switch="image_upload_enabled" value={form.config.image_upload_enabled} onChange={(value) => patchConfig('image_upload_enabled', value === true)} />
          </Row>
          {form.config.image_upload_enabled ? (
            <Row label={t('agentEditor.imageUpload.vlmModel')} required desc={t('agentEditor.imageUpload.vlmModelDesc')}>
              <Select
                data-field="vlm_model_id"
                className="wk-ae-sel-vlm"
                value={form.config.vlm_model_id}
                clearable
                placeholder={t('agentEditor.imageUpload.vlmModelPlaceholder')}
                onChange={(value) => patchConfig('vlm_model_id', String(value ?? ''))}
              >
                {vlmModels.map((model) => <Select.Option key={model.id} value={model.id} label={model.name} title={model.name} />)}
              </Select>
            </Row>
          ) : null}
          {form.config.image_upload_enabled ? (
            <Row label={t('agentEditor.imageUpload.imageUnderstandingLabel')} desc={t('agentEditor.imageUpload.imageUnderstandingDesc')}>
              <Switch data-switch="attachment_image_understanding" value={form.config.attachment_image_understanding} onChange={(value) => patchConfig('attachment_image_understanding', value === true)} />
            </Row>
          ) : null}
          {form.config.image_upload_enabled && form.config.attachment_image_understanding ? (
            <Row label={t('agentEditor.imageUpload.ocrMaxPagesLabel')} desc={t('agentEditor.imageUpload.ocrMaxPagesDesc')}>
              <InputNumber
                className="wk-ae-num-attachment_ocr_max_pages"
                inputProps={{ "data-field": "attachment_ocr_max_pages" } as never}
                theme="normal"
                min={0}
                max={64}
                step={1}
                style={{ width: '160px' } as CSSProperties}
                value={form.config.attachment_ocr_max_pages}
                placeholder={t('agentEditor.imageUpload.useGlobalDefault')}
                onChange={(value) => patchConfig('attachment_ocr_max_pages', Number(value) || 0)}
              />
            </Row>
          ) : null}
          {form.config.image_upload_enabled ? (
            <Row label={t('agentEditor.imageUpload.storageProvider')} desc={t('agentEditor.imageUpload.storageProviderDesc')}>
              <div className="setting-control" style={{ flexDirection: 'column', alignItems: 'flex-end' } as CSSProperties}>
                <Select
                  data-field="image_storage_provider"
                  className="wk-ae-sel-storage"
                  style={{ width: '280px' } as CSSProperties}
                  value={form.config.image_storage_provider}
                  clearable
                  placeholder={t('agentEditor.imageUpload.storageProviderPlaceholder')}
                  onChange={(value) => patchConfig('image_storage_provider', String(value ?? ''))}
                >
                  <Select.Option value="" label={t('agentEditor.imageUpload.storageDefault')} />
                  {storageOptions.map((option) => (
                    <Select.Option key={option.value} value={option.value} label={option.label} disabled={option.disabled}>
                      <span className="select-option-with-tag">
                        <span>{option.label}</span>
                      </span>
                    </Select.Option>
                  ))}
                </Select>
                <a href="javascript:void(0)" className="go-settings-link" data-go-storage-settings onClick={(event) => { event.preventDefault(); navigate('/platform/settings?section=storage'); }}>
                  {t('agentEditor.imageUpload.goStorageSettings')}
                </a>
              </div>
            </Row>
          ) : null}
          <Row label={t('agentEditor.audioUpload.label')} desc={t('agentEditor.audioUpload.desc')}>
            <Switch data-switch="audio_upload_enabled" value={form.config.audio_upload_enabled} onChange={(value) => patchConfig('audio_upload_enabled', value === true)} />
          </Row>
          {form.config.audio_upload_enabled ? (
            <Row label={t('agentEditor.audioUpload.asrModel')} desc={t('agentEditor.audioUpload.asrModelDesc')}>
              <Select
                data-field="asr_model_id"
                className="wk-ae-sel-asr"
                value={form.config.asr_model_id}
                clearable
                placeholder={t('agentEditor.audioUpload.asrModelPlaceholder')}
                onChange={(value) => patchConfig('asr_model_id', String(value ?? ''))}
              >
                {asrModels.map((model) => <Select.Option key={model.id} value={model.id} label={model.name} title={model.name} />)}
              </Select>
            </Row>
          ) : null}
          <Row label={t('agentEditor.chatParser.waitTimeoutLabel')} desc={t('agentEditor.chatParser.waitTimeoutDesc')}>
            <InputNumber
              className="wk-ae-num-attachment_parse_wait_timeout_sec"
              inputProps={{ "data-field": "attachment_parse_wait_timeout_sec" } as never}
              theme="normal"
              min={0}
              max={600}
              step={10}
              style={{ width: '160px' } as CSSProperties}
              value={form.config.attachment_parse_wait_timeout_sec}
              placeholder={t('agentEditor.imageUpload.useGlobalDefault')}
              onChange={(value) => patchConfig('attachment_parse_wait_timeout_sec', Number(value) || 0)}
            />
          </Row>
          {/* R486 — Vue 867-878 <KBParserSettings embedded>：chat 附件解析规则。 */}
          <div className="parser-policy-block" data-parser-policy-block>
            <div className="parser-policy-block__header">
              <label>{t('agentEditor.chatParser.label')}</label>
              <p className="desc">{t('agentEditor.chatParser.desc')}</p>
            </div>
            <AgentParserRules
              engines={deps.parserEngines}
              rules={Array.isArray(form.config.chat_parser_engine_rules) ? form.config.chat_parser_engine_rules as ParserEngineRule[] : []}
              onChange={(rules) => patchConfig('chat_parser_engine_rules', rules)}
              t={t}
            />
          </div>
        </div>
      </div>
    );
  }

  function renderConversation() {
    return (
      <div className="section" data-editor-section="conversation">
        <SectionHeader title={t('agent.editor.conversationSettings')} desc={isAgentMode ? t('agentEditor.desc.conversationSectionAgent') : t('agentEditor.desc.conversationSection')} />
        <div className="settings-group">
          {quickAnswer ? (
            <Row label={t('agent.editor.multiTurn')} desc={t('agentEditor.desc.multiTurn')}>
              <Switch data-switch="multi_turn_enabled" value={form.config.multi_turn_enabled} onChange={(value) => patchConfig('multi_turn_enabled', value === true)} />
            </Row>
          ) : null}
          {form.config.multi_turn_enabled || isAgentMode ? (
            <Row label={t('agent.editor.historyTurns')} desc={t('agentEditor.desc.historyRounds')}>
              <InputNumber
                className="wk-ae-num-history_turns"
                inputProps={{ "data-field": "history_turns" } as never}
                theme="column"
                min={1}
                max={100}
                value={form.config.history_turns}
                onChange={(value) => patchConfig('history_turns', Number(value) || 1)}
              />
            </Row>
          ) : null}
          {isAgentMode && hasKnowledgeBase ? (
            <Row label={t('agent.editor.retainRetrievalHistory')} desc={t('agentEditor.desc.retainRetrievalHistory')}>
              <Switch data-switch="retain_retrieval_history" value={form.config.retain_retrieval_history} onChange={(value) => patchConfig('retain_retrieval_history', value === true)} />
            </Row>
          ) : null}
          {quickAnswer && form.config.multi_turn_enabled ? (
            <Row label={t('agent.editor.enableRewrite')} desc={t('agentEditor.desc.rewrite')}>
              <Switch data-switch="enable_rewrite" value={form.config.enable_rewrite} onChange={(value) => patchConfig('enable_rewrite', value === true)} />
            </Row>
          ) : null}
        </div>
      </div>
    );
  }

  function renderSuggestions() {
    const qs = form.config.question_suggestions;
    const starters = qs.starters;
    const followUps = qs.follow_ups;
    return (
      <div className="section" data-editor-section="suggestions">
        <SectionHeader title={t('agentEditor.questionSuggestions.title')} desc={t('agentEditor.questionSuggestions.description')} />
        {/* Vue 948-953：t-tabs 只做导航（内容自渲染在下方，t-tabs__content 隐藏）。 */}
        <Tabs value={suggestionTab} onChange={(value) => setSuggestionTab(value as 'starters' | 'followUps')} className="suggestion-tabs">
          <Tabs.TabPanel value="starters" label={t('agentEditor.questionSuggestions.startersTitle')} />
          <Tabs.TabPanel value="followUps" label={t('agentEditor.questionSuggestions.followUpsTitle')} />
        </Tabs>
        <div className="settings-group" style={suggestionTab === 'starters' ? undefined : { display: 'none' }}>
          <Row label={t('agentEditor.questionSuggestions.enableStarters')} desc={t('agentEditor.questionSuggestions.enableStartersDesc')}>
            <Switch
              data-switch="question_suggestions.starters.enabled"
              value={starters.enabled}
              onChange={(value) => patchQs((draft) => { draft.starters.enabled = value === true; })}
            />
          </Row>
          {starters.enabled ? (
            <Row label={t('agentEditor.questionSuggestions.sourceMode')}>
              <Select
                data-field="question_suggestions.starters.mode"
                className="wk-ae-sel-starters-mode"
                value={starters.mode}
                onChange={(value) => patchQs((draft) => { draft.starters.mode = String(value) as typeof draft.starters.mode; })}
                options={[
                  { label: t('agentEditor.questionSuggestions.modeCurated'), value: 'curated', title: t('agentEditor.questionSuggestions.modeCurated') },
                  { label: t('agentEditor.questionSuggestions.modeKnowledge'), value: 'knowledge', title: t('agentEditor.questionSuggestions.modeKnowledge') },
                  { label: t('agentEditor.questionSuggestions.modeHybrid'), value: 'hybrid', title: t('agentEditor.questionSuggestions.modeHybrid') },
                ]}
              />
            </Row>
          ) : null}
          {starters.enabled ? (
            <Row label={t('agentEditor.questionSuggestions.count')}>
              <InputNumber
                className="wk-ae-num-question_suggestions.starters.count"
                inputProps={{ "data-field": "question_suggestions.starters.count" } as never}
                theme="column"
                min={1}
                max={8}
                value={starters.count}
                onChange={(value) => patchQs((draft) => { draft.starters.count = Number(value) || 1; })}
              />
            </Row>
          ) : null}
          {starters.enabled && (starters.mode === 'curated' || starters.mode === 'hybrid') ? (
            <Row label={t('agentEditor.questionSuggestions.curatedItems')} desc={t('agentEditor.questionSuggestions.curatedItemsDesc')} vertical>
              <div className="setting-control-full">
                <div className="suggested-prompts-list" data-field-group="question_suggestions.starters.items">
                  {starters.items.map((item, index) => (
                    <div className="prompt-item" key={index}>
                      <Input
                        data-starter-index={index}
                        maxlength={200}
                        value={item}
                        onChange={(value) => patchQs((draft) => { draft.starters.items[index] = String(value); })}
                      />
                      <Button variant="text" theme="danger" shape="square" aria-label={t('common.delete')} data-remove-starter={index} onClick={() => patchQs((draft) => { draft.starters.items.splice(index, 1); })}>
                        <TIcon name="delete" />
                      </Button>
                    </div>
                  ))}
                  <Button variant="dashed" disabled={starters.items.length >= 8} data-add-starter icon={<TIcon name="add" />} onClick={() => patchQs((draft) => { if (draft.starters.items.length < 8) draft.starters.items.push(''); })}>
                    {t('agentEditor.questionSuggestions.addItem')}
                  </Button>
                </div>
              </div>
            </Row>
          ) : null}
        </div>
        <div className="settings-group" style={suggestionTab === 'followUps' ? undefined : { display: 'none' }}>
          <Row label={t('agentEditor.questionSuggestions.enableFollowUps')} desc={t('agentEditor.questionSuggestions.enableFollowUpsDesc')}>
            <Switch
              data-switch="question_suggestions.follow_ups.enabled"
              value={followUps.enabled}
              onChange={(value) => patchQs((draft) => { draft.follow_ups.enabled = value === true; })}
            />
          </Row>
          {followUps.enabled ? (
            <>
              <Row label={t('agentEditor.questionSuggestions.sourceMode')}>
                <Select
                  data-field="question_suggestions.follow_ups.mode"
                  className="wk-ae-sel-followups-mode"
                  value={followUps.mode}
                  onChange={(value) => patchQs((draft) => { draft.follow_ups.mode = String(value) as typeof draft.follow_ups.mode; })}
                  options={[
                    { label: t('agentEditor.questionSuggestions.modeGenerated'), value: 'generated', title: t('agentEditor.questionSuggestions.modeGenerated') },
                    { label: t('agentEditor.questionSuggestions.modeKnowledge'), value: 'knowledge', title: t('agentEditor.questionSuggestions.modeKnowledge') },
                    { label: t('agentEditor.questionSuggestions.modeHybrid'), value: 'hybrid', title: t('agentEditor.questionSuggestions.modeHybrid') },
                  ]}
                />
              </Row>
              <Row label={t('agentEditor.questionSuggestions.count')}>
                <InputNumber
                  className="wk-ae-num-question_suggestions.follow_ups.count"
                  inputProps={{ "data-field": "question_suggestions.follow_ups.count" } as never}
                  theme="column"
                  min={1}
                  max={5}
                  value={followUps.count}
                  onChange={(value) => patchQs((draft) => { draft.follow_ups.count = Number(value) || 1; })}
                />
              </Row>
              {followUps.mode !== 'knowledge' ? (
                <Row label={t('agentEditor.questionSuggestions.model')} desc={t('agentEditor.questionSuggestions.modelDesc')}>
                  <Select
                    data-field="question_suggestions.follow_ups.model_id"
                    className="wk-ae-sel-followups-model"
                    value={followUps.model_id}
                    clearable
                    placeholder={t('agentEditor.modelPlaceholder')}
                    onChange={(value) => patchQs((draft) => { draft.follow_ups.model_id = String(value ?? ''); })}
                  >
                    {chatModels.map((model) => <Select.Option key={model.id} value={model.id} label={model.name} title={model.name} />)}
                  </Select>
                </Row>
              ) : null}
              <div className="suggestion-advanced-divider"><span>{t('agentEditor.questionSuggestions.advancedSettings')}</span></div>
              <Row label={t('agentEditor.questionSuggestions.contextTurns')}>
                <InputNumber
                  className="wk-ae-num-question_suggestions.follow_ups.max_context_turns"
                  inputProps={{ "data-field": "question_suggestions.follow_ups.max_context_turns" } as never}
                  theme="column"
                  min={1}
                  max={5}
                  value={followUps.max_context_turns}
                  onChange={(value) => patchQs((draft) => { draft.follow_ups.max_context_turns = Number(value) || 1; })}
                />
              </Row>
              <Row label={t('agentEditor.questionSuggestions.categories')} vertical>
                <div className="setting-control-full">
                  <Checkbox.Group
                    data-field-group="question_suggestions.follow_ups.categories"
                    value={followUps.categories}
                    onChange={(values) => patchQs((draft) => { draft.follow_ups.categories = (values as string[]).map(String); })}
                  >
                    <Checkbox value="clarify" data-category="clarify">{t('agentEditor.questionSuggestions.categoryClarify')}</Checkbox>
                    <Checkbox value="deepen" data-category="deepen">{t('agentEditor.questionSuggestions.categoryDeepen')}</Checkbox>
                    <Checkbox value="action" data-category="action">{t('agentEditor.questionSuggestions.categoryAction')}</Checkbox>
                  </Checkbox.Group>
                </div>
              </Row>
              <Row label={t('agentEditor.questionSuggestions.instruction')} vertical>
                <div className="setting-control-full">
                  <Textarea
                    data-field="question_suggestions.follow_ups.additional_instruction"
                    value={followUps.additional_instruction}
                    placeholder={t('agentEditor.questionSuggestions.instructionPlaceholder')}
                    maxlength={2000}
                    autosize={{ minRows: 3, maxRows: 8 }}
                    onChange={(value) => patchQs((draft) => { draft.follow_ups.additional_instruction = String(value); })}
                  />
                </div>
              </Row>
              <Row label={t('agentEditor.questionSuggestions.displayRules')} vertical>
                <div className="setting-control-full">
                  <div className="suggestion-checkboxes">
                    <Checkbox data-display-rule="suppress_on_fallback" checked={followUps.suppress_on_fallback === true} onChange={(checked) => patchQs((draft) => { draft.follow_ups.suppress_on_fallback = checked === true; })}>{t('agentEditor.questionSuggestions.suppressFallback')}</Checkbox>
                    <Checkbox data-display-rule="suppress_when_answer_asks_question" checked={followUps.suppress_when_answer_asks_question === true} onChange={(checked) => patchQs((draft) => { draft.follow_ups.suppress_when_answer_asks_question = checked === true; })}>{t('agentEditor.questionSuggestions.suppressQuestion')}</Checkbox>
                    <Checkbox data-display-rule="knowledge_fallback" checked={followUps.knowledge_fallback === true} onChange={(checked) => patchQs((draft) => { draft.follow_ups.knowledge_fallback = checked === true; })}>{t('agentEditor.questionSuggestions.knowledgeFallback')}</Checkbox>
                    <Checkbox data-display-rule="allow_regenerate" checked={followUps.allow_regenerate === true} onChange={(checked) => patchQs((draft) => { draft.follow_ups.allow_regenerate = checked === true; })}>{t('agentEditor.questionSuggestions.allowRegenerate')}</Checkbox>
                  </div>
                </div>
              </Row>
            </>
          ) : null}
        </div>
      </div>
    );
  }

  function renderTools() {
    const activeTools = TOOL_CATALOG.filter((tool) => form.config.allowed_tools.includes(tool.value));
    const evaluations = activeTools.map((tool) => ({ tool, ...evaluateTool(tool) }));
    const inactiveToolCount = evaluations.filter((item) => !item.ok).length;
    return (
      <div className="section" data-editor-section="tools" data-highlighted-field={highlightedField === 'allowed_tools' ? 'allowed_tools' : undefined}>
        <SectionHeader title={t('agent.editor.toolsConfig')} desc={t('agent.editor.toolsConfigDesc')} />
        {/* 合并面板：能力状态（Vue tools-overview 1132-1154）。 */}
        <div className="tools-overview">
          <div className="tools-overview-row">
            <div className="tools-status-chip">
              <TIcon name="folder" />
              {hasKnowledgeBase ? (
                <>
                  <span className="tools-status-metric">
                    <strong>{deps.kbOptions.filter((kb) => kb.ragEnabled).length}</strong> {t('agentEditor.tools.kbMetricRag')}
                  </span>
                  <span className="tools-status-sep">·</span>
                  <span className="tools-status-metric">
                    <strong>{deps.kbOptions.filter((kb) => kb.wikiEnabled).length}</strong> {t('agentEditor.tools.kbMetricWiki')}
                  </span>
                </>
              ) : (
                <span>{t('agentEditor.tools.statusNoKb')}</span>
              )}
            </div>
            {inactiveToolCount > 0 ? (
              <div className="tools-status-chip tools-status-chip--warn">
                <TIcon name="error-circle" />
                <span>{t('agentEditor.tools.statusInactive', { count: inactiveToolCount })}</span>
              </div>
            ) : null}
          </div>
        </div>
        <div className="settings-group">
          {/* R485 D10 — Vue 1158-1201: the allowed-tools block renders per-group
              section headers + a two-column grid of tool cards（t-checkbox 卡片）。 */}
          <Row label={t('agent.editor.allowedTools')} desc={t('agentEditor.desc.selectTools')} vertical dataAgentField="allowed_tools" highlight={highlightedField === 'allowed_tools'}>
            <div className="setting-control-full">
              <Checkbox.Group
                className="tool-groups"
                value={form.config.allowed_tools}
                onChange={(values) => patchConfig('allowed_tools', (values as string[]).map(String))}
              >
                {TOOL_GROUPS.map((group) => {
                  const tools = TOOL_CATALOG.filter((tool) => tool.group === group.key);
                  if (tools.length === 0) return null;
                  return (
                    <section key={group.key} className={`tool-group tool-group--${group.key}`}>
                      <header className="tool-group-header">
                        <span className="tool-group-bar" />
                        <span className="tool-group-title">{t(group.labelKey)}</span>
                        <span className="tool-group-count">{tools.length}</span>
                        {group.key === 'wiki_edit' ? (
                          <span className="tool-group-warning">
                            <TIcon name="error-circle" />
                            {t('agentEditor.tools.writeWarning')}
                          </span>
                        ) : null}
                      </header>
                      <div className="tool-grid">
                        {tools.map((tool) => {
                          const evaluation = evaluateTool(tool);
                          return (
                            <Checkbox
                              key={tool.value}
                              value={tool.value}
                              data-tool={tool.value}
                              disabled={!evaluation.ok}
                              className={`tool-card${!evaluation.ok ? ' tool-card--disabled' : ''}${tool.danger ? ' tool-card--danger' : ''}`}
                            >
                              <div className="tool-card-body">
                                <div className="tool-card-head">
                                  <span className="tool-card-name">{t(tool.labelKey)}</span>
                                  {tool.danger ? <span className="tool-card-badge">{t('agentEditor.tools.dangerTag')}</span> : null}
                                </div>
                                <span className="tool-card-desc">{t(tool.descriptionKey)}</span>
                                {!evaluation.ok ? <span className="tool-card-hint">{t(missReasonKey(evaluation.missKind))}</span> : null}
                              </div>
                            </Checkbox>
                          );
                        })}
                      </div>
                    </section>
                  );
                })}
              </Checkbox.Group>
            </div>
          </Row>
          <Row label={t('agentEditor.tools.effectiveLabel')} desc={t('agentEditor.tools.effectiveDesc')} vertical>
            <div className="setting-control-full">
              <div className="effective-tools">
                {evaluations.length === 0 && !form.config.web_search_enabled ? (
                  <div className="effective-tools-empty">{t('agentEditor.tools.effectiveEmpty')}</div>
                ) : evaluations.map(({ tool, ok, missKind }) => (
                  <span key={tool.value} className={`effective-chip${ok ? '' : ' effective-chip--inactive'}`} title={ok ? '' : t(missReasonKey(missKind))}>
                    <span className="effective-chip-label">{t(tool.labelKey)}</span>
                    {!ok ? <span className="effective-chip-reason">{t(missReasonKey(missKind))}</span> : null}
                  </span>
                ))}
                {form.config.web_search_enabled ? (
                  <>
                    <span className="effective-chip"><span className="effective-chip-label">{t('agentEditor.tools.webSearch')}</span></span>
                    <span className="effective-chip"><span className="effective-chip-label">{t('agentEditor.tools.webFetch')}</span></span>
                  </>
                ) : null}
              </div>
            </div>
          </Row>
        </div>
      </div>
    );
  }

  function renderMcp() {
    const serviceRows = mcpOptionRows(deps.mcpServices, form.config.mcp_services, t);
    const showServiceSelect = serviceRows.length > 0 || form.config.mcp_services.length > 0;
    return (
      <div className="section" data-editor-section="mcp">
        <SectionHeader title={t('agentEditor.mcp.label')} desc={t('agentEditor.mcp.desc')} />
        <div className="settings-group">
          <Row label={t('agentEditor.mcp.label')} desc={t('agentEditor.mcp.desc')}>
            <RadioGroup name="mcp-mode" value={mcpMode} onChange={(value) => onScopeModeChange('mcp', value as ScopeSelectionMode)}>
              <Radio.Button value="all">{t('agentEditor.selection.all')}</Radio.Button>
              <Radio.Button value="selected">{t('agentEditor.selection.selected')}</Radio.Button>
              <Radio.Button value="none">{t('agentEditor.selection.disabled')}</Radio.Button>
            </RadioGroup>
          </Row>
          {mcpMode === 'selected' && showServiceSelect ? (
            <Row label={t('agentEditor.mcp.selectLabel')} desc={t('agentEditor.mcp.selectDesc')}>
              <Select
                data-field="mcp_services"
                className="wk-ae-sel-mcp"
                multiple
                filterable
                value={form.config.mcp_services}
                placeholder={t('agentEditor.mcp.selectPlaceholder')}
                onChange={(values) => patchConfig('mcp_services', (values as string[]).map(String))}
              >
                {serviceRows.map((row) => (
                  <Select.Option key={row.value} value={row.value} label={row.label} title={row.label} disabled={row.disabled} data-mcp-service={row.value} />
                ))}
              </Select>
            </Row>
          ) : null}
          {mcpMode !== 'none' ? (
            <Row label={t('agentEditor.mcp.authWaitTimeout')} desc={t('agentEditor.mcp.authWaitTimeoutDesc')}>
              <InputNumber
                className="wk-ae-num-mcp_auth_wait_timeout"
                inputProps={{ "data-field": "mcp_auth_wait_timeout" } as never}
                theme="column"
                min={5}
                max={3600}
                value={form.config.mcp_auth_wait_timeout}
                placeholder={t('agentEditor.mcp.authWaitTimeoutPlaceholder')}
                onChange={(value) => patchConfig('mcp_auth_wait_timeout', Number(value) || 0)}
              />
            </Row>
          ) : null}
        </div>
      </div>
    );
  }

  function renderSkills() {
    const readyRows = skillRows.filter((row) => row.selectable);
    const pendingRows = skillRows.filter((row) => !row.selectable);
    return (
      <div className="section" data-editor-section="skills">
        <SectionHeader title={t('agent.editor.skillsConfig')} desc={t('agent.editor.skillsConfigDesc')} />
        <div className="settings-group">
          <Row label={t('agent.editor.sandboxBackend')} desc={t('agent.editor.sandboxBackendHint')}>
            <div className="sandbox-select-control">
              <Select
                data-field="sandbox_config_id"
                className="sandbox-config-select"
                value={form.config.sandbox_config_id}
                placeholder={t('agent.editor.sandboxBackendDefault')}
                filterable
                popupProps={{ overlayClassName: 'sandbox-config-select-popup' } as never}
                onChange={(value) => patchConfig('sandbox_config_id', String(value ?? ''))}
              >
                <Select.Option value="" label={t('agent.editor.sandboxBackendDefault')} />
                {sandboxOptions.map((cfg) => <Select.Option key={cfg.id} value={cfg.id} label={cfg.name} />)}
              </Select>
              {/* R484 D11 — Vue 1320-1338 sandbox-select-links：管理沙箱跳转。 */}
              <div className="sandbox-select-links">
                <a href="javascript:void(0)" className="go-settings-link" data-go-sandbox-settings onClick={(event) => { event.preventDefault(); navigate('/platform/settings?section=sandbox'); }}>
                  {t('agent.editor.goSandboxSettings')}
                </a>
              </div>
              {sandboxOptions.length === 0 ? <p className="desc empty-hint">{t('agent.editor.sandboxNoConfigs')}</p> : null}
            </div>
          </Row>
          <Row label={t('agent.editor.skillsSelection')} desc={t('agent.editor.skillsSelectionDesc')}>
            <div className="sandbox-select-control">
              <RadioGroup name="skills-mode" value={skillsMode} onChange={(value) => onScopeModeChange('skills', value as ScopeSelectionMode)}>
                <Radio.Button value="all" disabled={!hasSandbox && namedSandboxes.length !== 1}>{t('agent.editor.skillsAll')}</Radio.Button>
                <Radio.Button value="selected" disabled={!hasSandbox && namedSandboxes.length !== 1}>{t('agent.editor.skillsSelected')}</Radio.Button>
                <Radio.Button value="none">{t('agent.editor.skillsNone')}</Radio.Button>
              </RadioGroup>
              {!hasSandbox && sandboxOptions.length > 1 ? <p className="desc empty-hint">{t('agent.editor.skillsNeedSandbox')}</p> : null}
            </div>
          </Row>
          {skillsMode !== 'none' && hasSandbox && skillRows.length > 0 ? (
            <Row vertical>
              <div className="setting-control-full">
                <Checkbox.Group className="skill-pick-list" value={form.config.selected_skills} onChange={(values) => patchConfig('selected_skills', (values as string[]).map(String))}>
                  {readyRows.length > 0 ? (
                    <section className="skill-pick-group">
                      <header className="skill-pick-group__header">
                        <span className="skill-pick-group__bar" />
                        <span className="skill-pick-group__title">{t('agent.editor.skillsGroupAvailable')}</span>
                        <span className="skill-pick-group__count">{readyRows.length}</span>
                      </header>
                      {readyRows.map(renderSkillRow)}
                    </section>
                  ) : null}
                  {pendingRows.length > 0 ? (
                    <section className="skill-pick-group skill-pick-group--pending">
                      <header className="skill-pick-group__header">
                        <span className="skill-pick-group__bar" />
                        <span className="skill-pick-group__title">{t('agent.editor.skillsGroupUnavailable')}</span>
                        <span className="skill-pick-group__count">{pendingRows.length}</span>
                      </header>
                      {pendingRows.map(renderSkillRow)}
                    </section>
                  ) : null}
                </Checkbox.Group>
              </div>
            </Row>
          ) : null}
        </div>
      </div>
    );
  }

  function renderKnowledge() {
    const myKbs = deps.kbOptions.filter((kb) => !kb.shared);
    const sharedKbs = deps.kbOptions.filter((kb) => kb.shared);
    return (
      <div className="section" data-editor-section="knowledge">
        <SectionHeader title={t('agent.editor.knowledgeConfig')} desc={t('agent.editor.knowledgeConfigDesc')} />
        <div className="settings-group">
          <Row label={t('agent.editor.knowledgeBases')} desc={t('agentEditor.desc.kbScope')} dataGuide="agent-create-knowledge">
            <RadioGroup name="kb-mode" value={kbMode} onChange={(value) => onKbModeChange(value as ScopeSelectionMode)}>
              <Radio.Button value="all">{t('agent.editor.allKnowledgeBases')}</Radio.Button>
              <Radio.Button value="selected">{t('agent.editor.selectedKnowledgeBases')}</Radio.Button>
              <Radio.Button value="none">{t('agent.editor.noKnowledgeBase')}</Radio.Button>
            </RadioGroup>
          </Row>
          {kbMode === 'selected' ? (
            <Row label={t('agent.editor.selectKnowledgeBases')} desc={t('agent.editor.selectKnowledgeBasesDesc')}>
              <Select
                data-field="knowledge_bases"
                className="wk-ae-sel-kbs"
                multiple
                filterable
                minCollapsedNum={3}
                value={form.config.knowledge_bases}
                placeholder={t('agent.editor.selectKnowledgeBases')}
                onChange={(values) => patchConfig('knowledge_bases', (values as string[]).map(String))}
              >
                {/* tdesign 内部按数组遍历 children（null 项会 crash）——条件组用展开拼数组 */}
                {[
                  ...(myKbs.length > 0
                    ? [<Select.OptionGroup key="mine" label={t('agent.editor.myKnowledgeBases')}>{myKbs.map(renderKbOption)}</Select.OptionGroup>]
                    : []),
                  ...(sharedKbs.length > 0
                    ? [<Select.OptionGroup key="shared" label={t('agent.editor.sharedKnowledgeBases')}>{sharedKbs.map(renderKbOption)}</Select.OptionGroup>]
                    : []),
                ]}
              </Select>
            </Row>
          ) : null}
          {hasKnowledgeBase ? (
            // R486 P3-2 — Vue 1529-1536: 支持的文件类型（t-select multiple，全部类型占位）。
            <Row label={t('agentEditor.fileTypes.label')} desc={t('agentEditor.fileTypes.desc')}>
              <Select
                data-field="supported_file_types"
                className="wk-ae-sel-file-types"
                multiple
                clearable
                minCollapsedNum={3}
                value={form.config.supported_file_types}
                placeholder={t('agentEditor.fileTypes.allTypes')}
                onChange={(values) => patchConfig('supported_file_types', (values as string[]).map(String))}
              >
                {AGENT_FILE_TYPE_OPTIONS.map((option) => (
                  <Select.Option
                    key={option.value}
                    value={option.value}
                    label={option.labelKey ? t(option.labelKey) : option.label}
                    title={option.labelKey ? t(option.labelKey) : option.label}
                    data-file-type={option.value}
                  />
                ))}
              </Select>
            </Row>
          ) : null}
          {hasKnowledgeBase ? (
            <Row label={t('agent.editor.retrieveKBOnlyWhenMentioned')} desc={t('agent.editor.retrieveKBOnlyWhenMentionedDesc')}>
              <Switch data-switch="retrieve_kb_only_when_mentioned" value={form.config.retrieve_kb_only_when_mentioned} onChange={(value) => patchConfig('retrieve_kb_only_when_mentioned', value === true)} />
            </Row>
          ) : null}
        </div>
      </div>
    );
  }

  /** Vue 1486-1517 kb-option-item：图标 + 名称 + RAG/Wiki 标签 + 数量。 */
  function renderKbOption(kb: KbOption) {
    return (
      <Select.Option key={kb.value} value={kb.value} label={kb.label} title={kb.label} data-kb-id={kb.value}>
        <div className="kb-option-item">
          <span className={`kb-option-icon${kb.type === 'faq' ? 'faq-icon' : 'doc-icon'}`}>
            <TIcon name={kb.type === 'faq' ? 'chat-bubble-help' : 'folder'} />
          </span>
          <span className="kb-option-label">{kb.label}</span>
          {kb.ragEnabled ? <span className="kb-option-tag tag-rag">RAG</span> : null}
          {kb.wikiEnabled ? <span className="kb-option-tag tag-wiki">Wiki</span> : null}
          {kb.shared && kb.orgName ? <span className="kb-option-org">{kb.orgName}</span> : null}
          <span className="kb-option-count">{kb.count || 0}</span>
        </div>
      </Select.Option>
    );
  }

  function renderWebSearch() {
    return (
      <div className="section" data-editor-section="websearch">
        <SectionHeader title={t('agent.editor.webSearchConfig')} desc={t('agent.editor.webSearchConfigDesc')} />
        <div className="settings-group">
          <Row label={t('agent.editor.webSearch')} desc={t('agentEditor.desc.webSearch')}>
            <Switch data-switch="web_search_enabled" value={form.config.web_search_enabled} onChange={(value) => patchConfig('web_search_enabled', value === true)} />
          </Row>
          {form.config.web_search_enabled ? (
            <Row label={t('agent.editor.webSearchProvider')} desc={t('agentEditor.desc.webSearchProvider')}>
              <Select
                data-field="web_search_provider_id"
                className="wk-ae-sel-provider"
                style={{ width: '240px' } as CSSProperties}
                value={form.config.web_search_provider_id}
                clearable
                placeholder={t('agent.editor.webSearchProviderPlaceholder')}
                onChange={(value) => patchConfig('web_search_provider_id', String(value ?? ''))}
              >
                {deps.providers.map((provider) => (
                  <Select.Option key={provider.id} value={provider.id} label={provider.name} title={provider.name}>
                    <span>{provider.name}</span>
                  </Select.Option>
                ))}
              </Select>
            </Row>
          ) : null}
          {form.config.web_search_enabled ? (
            <Row label={t('agent.editor.webSearchMaxResults')} desc={t('agentEditor.desc.webSearchMaxResults')}>
              <SliderRow ariaLabel={t('agent.editor.webSearchMaxResults')} min={1} max={10} step={1} value={form.config.web_search_max_results}
                display={`${form.config.web_search_max_results}`} onChange={(next) => patchConfig('web_search_max_results', next)} />
            </Row>
          ) : null}
          {form.config.web_search_enabled ? (
            <Row label={t('agent.editor.webFetchEnabled')} desc={t('agentEditor.desc.webFetchEnabled')}>
              <Switch data-switch="web_fetch_enabled" value={form.config.web_fetch_enabled} onChange={(value) => patchConfig('web_fetch_enabled', value === true)} />
            </Row>
          ) : null}
          {form.config.web_search_enabled && form.config.web_fetch_enabled ? (
            <Row label={t('agent.editor.webFetchTopN')} desc={t('agentEditor.desc.webFetchTopN')}>
              <SliderRow ariaLabel={t('agent.editor.webFetchTopN')} min={1} max={10} step={1} value={form.config.web_fetch_top_n}
                display={`${form.config.web_fetch_top_n}`} onChange={(next) => patchConfig('web_fetch_top_n', next)} />
            </Row>
          ) : null}
        </div>
      </div>
    );
  }

  function renderRetrieval() {
    return (
      <div className="section" data-editor-section="retrieval">
        <SectionHeader title={t('agent.editor.retrievalStrategy')} desc={t('agentEditor.desc.retrievalSection')} />
        <div className="settings-group">
          {quickAnswer ? (
            <Row label={t('agent.editor.enableQueryExpansion')} desc={t('agentEditor.desc.queryExpansion')}>
              <Switch data-switch="enable_query_expansion" value={form.config.enable_query_expansion} onChange={(value) => patchConfig('enable_query_expansion', value === true)} />
            </Row>
          ) : null}
          <Row label={t('agent.editor.embeddingTopK')} desc={t('agentEditor.desc.embeddingTopK')}>
            <InputNumber className="wk-ae-num-embedding_top_k" inputProps={{ "data-field": "embedding_top_k" } as never} theme="column" min={1} max={50} value={form.config.embedding_top_k}
              onChange={(value) => patchConfig('embedding_top_k', Number(value) || 1)} />
          </Row>
          <Row label={t('agent.editor.keywordThreshold')} desc={t('agentEditor.desc.keywordThreshold')}>
            <SliderRow ariaLabel={t('agent.editor.keywordThreshold')} min={0} max={1} step={0.01} value={form.config.keyword_threshold}
              display={form.config.keyword_threshold.toFixed(2)} onChange={(next) => patchConfig('keyword_threshold', next)} />
          </Row>
          <Row label={t('agent.editor.vectorThreshold')} desc={t('agentEditor.desc.vectorThreshold')}>
            <SliderRow ariaLabel={t('agent.editor.vectorThreshold')} min={0} max={1} step={0.01} value={form.config.vector_threshold}
              display={form.config.vector_threshold.toFixed(2)} onChange={(next) => patchConfig('vector_threshold', next)} />
          </Row>
          {form.config.rerank_model_id ? (
            <>
              <Row label={t('agent.editor.rerankTopK')} desc={t('agentEditor.desc.rerankTopK')}>
                <InputNumber className="wk-ae-num-rerank_top_k" inputProps={{ "data-field": "rerank_top_k" } as never} theme="column" min={1} max={20} value={form.config.rerank_top_k}
                  onChange={(value) => patchConfig('rerank_top_k', Number(value) || 1)} />
              </Row>
              <Row label={t('agent.editor.rerankThreshold')} desc={t('agentEditor.desc.rerankThreshold')}>
                <SliderRow ariaLabel={t('agent.editor.rerankThreshold')} min={-10} max={10} step={0.01} value={form.config.rerank_threshold}
                  display={form.config.rerank_threshold.toFixed(1)} onChange={(next) => patchConfig('rerank_threshold', next)} />
              </Row>
            </>
          ) : null}
          {hasFaqKnowledgeBase ? (
            <>
              <Row label={t('agentEditor.faq.enableLabel')} desc={t('agentEditor.faq.enableDesc')}>
                <Switch data-switch="faq_priority_enabled" value={form.config.faq_priority_enabled} onChange={(value) => patchConfig('faq_priority_enabled', value === true)} />
              </Row>
              {form.config.faq_priority_enabled ? (
                <>
                  <Row label={t('agentEditor.faq.thresholdLabel')} desc={t('agentEditor.faq.thresholdDesc')}>
                    <SliderRow ariaLabel={t('agentEditor.faq.thresholdLabel')} min={0.7} max={1} step={0.05} value={form.config.faq_direct_answer_threshold}
                      display={form.config.faq_direct_answer_threshold.toFixed(2)} onChange={(next) => patchConfig('faq_direct_answer_threshold', next)} />
                  </Row>
                  <Row label={t('agentEditor.faq.boostLabel')} desc={t('agentEditor.faq.boostDesc')}>
                    <SliderRow ariaLabel={t('agentEditor.faq.boostLabel')} min={1} max={2} step={0.1} value={form.config.faq_score_boost}
                      display={`${form.config.faq_score_boost.toFixed(1)}x`} onChange={(next) => patchConfig('faq_score_boost', next)} />
                  </Row>
                </>
              ) : null}
            </>
          ) : null}
          {quickAnswer ? (
            <Row label={t('agentEditor.dataAnalysis.enableLabel')} desc={t('agentEditor.dataAnalysis.enableDesc')}>
              <Switch data-switch="data_analysis_enabled" value={form.config.data_analysis_enabled} onChange={(value) => patchConfig('data_analysis_enabled', value === true)} />
            </Row>
          ) : null}
        </div>
      </div>
    );
  }

  function renderPersonalization() {
    return <PersonaSection config={form.config} patchConfig={patchConfig} client={client} t={t} />;
  }

  function renderSubagents() {
    // agentId is empty in a create session before the first save — install/
    // remove need the persisted agent, so the section gates its mutations
    return <SubagentsSection config={form.config} patchConfig={patchConfig} client={client} t={t} agentId={form.id ?? ''} />;
  }

  function renderSkillRow(row: ReturnType<typeof catalogSkillRows>[number]) {
    const busy = row.installStatus === 'installing' || row.installStatus === 'removing';
    const canInstall = !busy && (!row.installed || row.installStatus === 'failed')
      && hasSandbox && namedSandboxes.some((cfg) => cfg.id === form.config.sandbox_config_id);
    return (
      <article
        key={row.id}
        className={`skill-pick skill-pick--${row.selectable ? 'ready' : 'pending'}`}
        data-skill-row={row.id}
      >
        {skillsMode === 'selected' ? (
          <Checkbox className="skill-pick__check" value={row.name} disabled={!row.selectable} data-skill-id={row.id} />
        ) : null}
        <div className="skill-pick__badge" aria-hidden="true"><TIcon name="system-code" size="16px" /></div>
        <div className="skill-pick__body">
          <div className="skill-pick__title-row">
            <span className="skill-name" title={row.name}>{row.name}</span>
            {!row.selectable ? (
              <span className={`skill-pick__hint${busy ? ' skill-pick__hint--busy' : ''}`}>
                <TIcon name={busy ? 'loading' : 'info-circle'} size="14px" />
                {row.installed ? row.installStatus : t('agent.editor.skillNotInstalled')}
              </span>
            ) : null}
          </div>
          {row.description ? <p className="skill-desc" title={row.description}>{row.description}</p> : null}
        </div>
        {canInstall ? (
          <Button size="small" variant="text" theme="primary" loading={installingId === row.id} title={t('agent.editor.installToThisSandbox')} data-skill-install={row.id} onClick={() => void installSkill(row.id)}>
            {t('agent.editor.installShort')}
          </Button>
        ) : busy ? (
          <Button size="small" variant="text" theme="primary" title={t('agent.editor.viewInstallProgress')} onClick={() => navigate('/platform/settings?section=skills')}>
            {t('agent.editor.viewInstallProgress')}
          </Button>
        ) : null}
      </article>
    );
  }

  const renderSection = () => {
    switch (section) {
      case 'prompts': return renderPrompts();
      case 'model': return renderModel();
      case 'conversation': return renderConversation();
      case 'suggestions': return renderSuggestions();
      case 'knowledge': return renderKnowledge();
      case 'retrieval': return renderRetrieval();
      case 'websearch': return renderWebSearch();
      case 'multimodal': return renderMultimodal();
      case 'tools': return renderTools();
      case 'mcp': return renderMcp();
      case 'skills': return renderSkills();
      case 'personalization': return renderPersonalization();
      case 'subagents': return renderSubagents();
      default: return renderBasic();
    }
  };

  /* Vue <Teleport to="body">：overlay 挂 body，脱离 shell 的层叠/合成上下文。 */
  return createPortal(
    <div className="settings-overlay" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }} data-editor-overlay>
      <div className="settings-modal" role="dialog" aria-modal="true" aria-label={editorMode === 'create' ? t('agent.editor.createTitle') : t('agent.editor.editTitle')} data-testid="agent-editor-modal">
        {initializing ? (
          <div className="editor-initializing" role="status" aria-label={t('common.loading')}>
            <Loading size="medium" text={t('common.loading')} />
          </div>
        ) : null}
        {/* 关闭按钮（Vue close-btn：20×20 stroke-2 X） */}
        <button className="close-btn" aria-label={t('common.close')} onClick={onClose}>
          <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
            <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          </svg>
        </button>
        <div className="settings-container">
          {/* 左侧导航 */}
          <div className="settings-sidebar">
            <div className="sidebar-header">
              <h2 className="sidebar-title">{editorMode === 'create' ? t('agent.editor.createTitle') : t('agent.editor.editTitle')}</h2>
            </div>
            <nav className="settings-nav" data-guide="agent-editor-sidebar">
              {/* Vue template v-for 无包装层：group title / nav-item 是 .settings-nav
                  直接子元素（.nav-group-title:first/:not(:first-child) 依赖该结构） */}
              {navGroups.map((group) => (
                <React.Fragment key={group.key}>
                  <div className="nav-group-title">{t(group.labelKey)}</div>
                  {group.items.map((item) => (
                    <div
                      key={item.key}
                      className={`nav-item${section === item.key ? ' active' : ''}`}
                      data-guide={`agent-editor-nav-${item.key}`}
                      data-section-key={item.key}
                      onClick={() => setSection(item.key)}
                    >
                      <TIcon name={navIconName(item.icon)} className="nav-icon" />
                      <span className="nav-label">{t(item.labelKey)}</span>
                    </div>
                  ))}
                </React.Fragment>
              ))}
            </nav>
          </div>
          {/* 右侧内容区域 */}
          <div className="settings-content">
            <div className={`content-wrapper${section === 'prompts' ? ' content-wrapper--prompts' : ''}`}>
              {renderSection()}
              {editorMode === 'edit' && form.id && !readOnly ? (
                <div className="wk-ae-version-actions">
                  <AgentVersionActions api={marketplaceApi} agentId={form.id} agentName={form.name} />
                </div>
              ) : null}
            </div>
            {/* 底部操作栏 */}
            <div className="settings-footer">
              {postCreate ? (
                <p className="settings-footer-note" data-post-create>
                  <TIcon name="check-circle-filled" className="settings-footer-note__icon" />
                  <span>
                    <strong>{t('agent.editor.postCreateHint.title')}</strong>
                    {t('agent.editor.postCreateHint.footer')}
                  </span>
                </p>
              ) : null}
              {saveError ? <p className="footer-note" data-tone="error" role="alert">{saveError}</p> : null}
              <div className="settings-footer-actions">
                <Button variant="outline" data-editor-cancel onClick={onClose}>{readOnly ? t('common.close') : t('common.cancel')}</Button>
                {!readOnly ? (
                  <Button
                    theme="primary"
                    data-guide="agent-create-submit"
                    data-editor-save
                    loading={saving}
                    disabled={initializing}
                    onClick={() => void handleSave()}
                  >{saveLabel}</Button>
                ) : null}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
