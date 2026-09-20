/**
 * React port of frontend/src/views/agent/AgentEditorModal.vue (Vue baseline).
 * Behaviour parity notes cite the Vue source per block; the pure logic lives
 * in agent-editor.ts and agent-type-presets.ts. Stage scope: create/edit
 * shell + grouped rail, basic info (incl. agent type presets), prompts (with
 * the R486 placeholder tag strip + builtin template controls), model config,
 * conversation, question suggestions, personalization (MBTI persona),
 * knowledge (file-types dropdown multi-select), retrieval (tenant-configured
 * create defaults), web search, attachment upload (with the embedded chat
 * parser rules editor), tools, MCP services and skills. R491 — the agent
 * type presets, prompt template list and placeholder hints are fetched at
 * runtime (agents/type-presets, tenants/kv/prompt-templates,
 * agents/placeholders) exactly like the Vue editorResources store, with the
 * vendored static catalogs as the failed-fetch fallback. Out of scope
 * (recorded gaps): intent prompts, the `{{` caret autocomplete popup,
 * share settings and the normal-mode (quick-answer) template pickers.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ModelConfiguration, SandboxConfigRecord, SkillCatalog, WeKnoraClient } from '@weknora/api-client';
import { Checkbox, Input, Radio, Range, Select, Textarea } from '@weknora/ui';
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

// --- tiny form primitives (no UI library in the React client) -------------------------
// agent-editor.css converted to utilities; var(--td-*) theme hooks kept verbatim
// as arbitrary values (undefined in this client -> the fallback always applied).
// wk-ae-input / wk-ae-textarea / wk-ae-select stay as anchors for the Row error variant.
const FIELD_BASE = 'w-full rounded-md border border-[var(--td-component-stroke,#dcdcdc)] px-2.5 py-1.5 font-[family-name:inherit] text-[14px] text-inherit bg-[var(--td-bg-color-container,#fff)]';
const FIELD_WIDE = 'max-w-[460px]';
const FIELD_DISABLED_BG = 'disabled:bg-[var(--td-bg-color-secondarycontainer,#f2f3f5)]';
const FIELD_INPUT = `${FIELD_BASE} ${FIELD_WIDE} ${FIELD_DISABLED_BG}`;
const FIELD_TEXTAREA = `${FIELD_BASE} ${FIELD_WIDE} resize-y ${FIELD_DISABLED_BG}`;
const FIELD_SELECT = `${FIELD_BASE} ${FIELD_WIDE}`;
const FIELD_NUMBER = `${FIELD_BASE} max-w-40`;
const FIELD_TALL = 'min-h-[200px]';
const ROW_ERROR_FIELDS = '[&_.wk-ae-input]:border-[var(--td-error-color,#d54941)] [&_.wk-ae-textarea]:border-[var(--td-error-color,#d54941)] [&_.wk-ae-select]:border-[var(--td-error-color,#d54941)]';
const AE_BTN = 'cursor-pointer rounded-md px-4 py-1.5 text-[14px]';

function Row({ label, required = false, desc, hint, htmlFor, error, extra, children }: {
  label: string; required?: boolean; desc?: string; hint?: string; htmlFor?: string;
  error?: string; extra?: React.ReactNode; children: React.ReactNode;
}) {
  return (
    <div className={`flex items-start gap-6${error ? ` ${ROW_ERROR_FIELDS}` : ''}`}>
      <div className="w-[260px] shrink-0">
        <label className="text-sm font-medium" htmlFor={htmlFor}>
          {label}
          {required ? <span className="ml-1 text-[var(--td-error-color,#d54941)]" aria-hidden="true">*</span> : null}
        </label>
        {desc ? <p className="m-0 mt-1 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{desc}</p> : null}
        {hint ? <p className="m-0 mt-1 text-[12px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">{hint}</p> : null}
        {extra}
      </div>
      <div className="flex min-w-0 flex-1 flex-col items-start gap-1.5">
        {children}
        {error ? <p className="m-0 mt-0.5 text-[12px] text-[var(--td-error-color,#d54941)]" data-field-error={htmlFor ? htmlFor.replace('wk-ae-', '').replace(/-/g, '_') : undefined}>{error}</p> : null}
      </div>
    </div>
  );
}

/**
 * R486 P3-2 — the supported-file-types control as a dropdown multi-select,
 * mirroring the Vue t-select multiple (AgentEditorModal.vue:1529-1536): a
 * single trigger showing the joined labels (placeholder 全部类型 when empty)
 * that opens a checkbox panel; the selected set semantics are unchanged and
 * write straight back to config.supported_file_types.
 */
function FileTypeMultiSelect({ selected, options, onChange, placeholderLabel, t }: {
  selected: string[];
  options: typeof AGENT_FILE_TYPE_OPTIONS;
  onChange: (next: string[]) => void;
  placeholderLabel: string;
  t: Translate;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!open) return;
    const onDocClick = (event: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('click', onDocClick);
    return () => document.removeEventListener('click', onDocClick);
  }, [open]);
  const optionLabel = (option: typeof options[number]) => (option.labelKey ? t(option.labelKey) : option.label);
  const selectedLabels = options.filter((option) => selected.includes(option.value)).map(optionLabel);
  return (
    <div className="relative w-full max-w-[460px]" ref={rootRef} data-field-group="supported_file_types">
      <button
        type="button"
        data-file-types-trigger
        aria-haspopup="listbox"
        aria-expanded={open ? 'true' : 'false'}
        className={`flex h-8 w-full cursor-pointer items-center justify-between gap-2 rounded-md border border-[var(--td-component-stroke,#dcdcdc)] bg-[var(--td-bg-color-container,#fff)] px-2.5 text-left text-[14px] ${selected.length === 0 ? 'text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]' : ''}`}
        onClick={() => setOpen((current) => !current)}
      >
        <span className="truncate">{selectedLabels.length > 0 ? selectedLabels.join('、') : placeholderLabel}</span>
        <span aria-hidden="true" className="text-[10px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">▾</span>
      </button>
      {open ? (
        <div className="absolute top-[calc(100%+4px)] left-0 z-30 w-full rounded-md border border-[var(--td-component-stroke,#e7e7e7)] bg-[var(--td-bg-color-container,#fff)] p-1 shadow-[0_8px_32px_rgba(0,0,0,0.12)]" role="listbox" data-file-types-panel>
          {options.map((option) => (
            <label key={option.value} title={t(option.descriptionKey)} className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-[13px] hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)]">
              <Checkbox
                data-file-type={option.value}
                checked={selected.includes(option.value)}
                onChange={(event) => {
                  const set = new Set(selected);
                  if (event.target.checked) set.add(option.value); else set.delete(option.value);
                  onChange([...set]);
                }}
              />
              <span>{optionLabel(option)}</span>
            </label>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function Switch({ checked, onChange, label, field }: { checked: boolean; onChange: (next: boolean) => void; label: string; field: string }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked ? 'true' : 'false'}
      aria-label={label}
      className={`relative h-[22px] w-10 cursor-pointer rounded-[11px] border-none transition-[background] duration-200 ease-[ease] ${checked ? 'bg-[var(--td-brand-color,#0052d9)]' : 'bg-[var(--td-bg-color-secondarycontainer,#c9c9c9)]'}`}
      data-switch={field}
      onClick={() => onChange(!checked)}
    ><span className={`absolute top-[2px] h-[18px] w-[18px] rounded-full bg-white transition-[left] duration-200 ease-[ease] ${checked ? 'left-5' : 'left-[2px]'}`} /></button>
  );
}

function RadioGroup({ name, value, options, onChange }: {
  name: string;
  value: string;
  options: Array<{ value: string; label: string; disabled?: boolean }>;
  onChange: (value: string) => void;
}) {
  return (
    <div className="flex flex-wrap gap-2" role="radiogroup">
      {options.map((option) => (
        <label key={option.value} className={`inline-flex cursor-pointer items-center gap-1.5 rounded-md border px-3 py-[5px] text-[13px] [&>input]:accent-[var(--td-brand-color,#0052d9)] [&>input:disabled+span]:text-[var(--td-text-color-disabled,rgba(0,0,0,0.26))] ${option.value === value ? 'border-[var(--td-brand-color,#0052d9)] text-[var(--td-brand-color,#0052d9)]' : 'border-[var(--td-component-stroke,#dcdcdc)]'}`}>
          <Radio
            name={name}
            value={option.value}
            checked={option.value === value}
            disabled={option.disabled === true}
            onChange={() => onChange(option.value)}
          />
          <span>{option.label}</span>
        </label>
      ))}
    </div>
  );
}

function Slider({ value, min, max, step, ariaLabel, onChange }: {
  value: number; min: number; max: number; step: number; ariaLabel: string; onChange: (next: number) => void;
}) {
  return (
    <span className="inline-flex items-center gap-2.5 [&>input]:w-60">
      <Range min={min} max={max} step={step} value={value} aria-label={ariaLabel}
        onChange={(event) => onChange(Number(event.target.value))} />
      <span className="min-w-8 text-[13px]">{value}</span>
    </span>
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
  // R486 KB warn — agent-type switch KB-conflict toast (Vue MessagePlugin.warning
  // with a 4000ms duration; React renders it in-modal as a status region)
  const [kbWarn, setKbWarn] = useState<string | null>(null);
  const kbWarnTimer = useRef<number | null>(null);
  // R491 — runtime editor catalogs (Vue editorResources prefetchAgentEditorDeps:
  // type-presets / prompt-templates / placeholders). null until the fetch
  // settles; resolve() merges whatever arrived over the vendored static
  // fallback so the editor renders before and after a failed fetch.
  const [runtimeResources, setRuntimeResources] = useState<AgentEditorRuntimeData | null>(null);
  const resources = useMemo(() => resolveAgentEditorResources(runtimeResources, locale), [runtimeResources, locale]);
  const marketplaceApi = useMemo(() => createAgentMarketplaceApi(client), [client]);
  // R486 D4 — caret insert targets (Vue promptTextareaRef / contextTemplateTextareaRef)
  const systemPromptRef = useRef<HTMLTextAreaElement | null>(null);
  const contextTemplateRef = useRef<HTMLTextAreaElement | null>(null);
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
    setKbWarn(null);
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
    const textarea = field === 'system_prompt' ? systemPromptRef.current : contextTemplateRef.current;
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
  // (MessagePlugin.warning kbIncompatibleWarn, 4000ms — rendered in-modal).
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
      setKbWarn(t('agentEditor.agentType.kbIncompatibleWarn', { count }));
      kbWarnTimer.current = window.setTimeout(() => setKbWarn(null), 4000);
    } else {
      setKbWarn(null);
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
  const myKbs = deps.kbOptions.filter((kb) => !kb.shared);
  const sharedKbs = deps.kbOptions.filter((kb) => kb.shared);
  const hasSandbox = !!form.config.sandbox_config_id;
  const saveLabel = editorMode === 'create' ? t('agent.editor.buttons.create') : t('agent.editor.buttons.saveAndClose');

  const renderKbCheckbox = (kb: KbOption) => {
    const checked = form.config.knowledge_bases.includes(kb.value);
    return (
      <label key={kb.value} className="flex items-center gap-2 rounded-md border border-[var(--td-component-stroke,#e7e7e7)] px-2 py-1.5 text-[13px]">
        <Checkbox
          data-kb-id={kb.value}
          checked={checked}
          onChange={(event) => patch((draft) => {
            const set = new Set(draft.config.knowledge_bases);
            if (event.target.checked) set.add(kb.value); else set.delete(kb.value);
            draft.config.knowledge_bases = [...set];
          })}
        />
        <span className="wk-ae-kb-name">{kb.label}</span>
        <span className="ml-auto text-[12px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">{kb.type === 'faq' ? 'FAQ' : kb.ragEnabled ? 'RAG' : ''}{kb.wikiEnabled ? ' · Wiki' : ''} · {kb.count}</span>
      </label>
    );
  };

  function renderBasic() {
    const editId = editorMode === 'edit' ? form.id : undefined;
    return (
      <section className="wk-ae-section" data-editor-section="basic">
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agent.editor.basicInfo')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.basicInfoDesc')}</p>
        </header>
        <div className="flex flex-col gap-[18px]">
          {editId ? (
            <Row label={t('agent.editor.agentId')} desc={t('agent.editor.agentIdDesc')}>
              <code className="rounded-md bg-[var(--td-bg-color-secondarycontainer,#f2f3f5)] px-2 py-1 font-[family-name:monospace] text-[12px] [overflow-wrap:anywhere]" data-agent-id-copy title={editId}>{editId}</code>
            </Row>
          ) : null}
          <Row label={t('agent.editor.mode')} required desc={isAgentMode ? t('agent.editor.agentDesc') : t('agent.editor.normalDesc')}>
            <RadioGroup
              name="agent-mode"
              value={form.config.agent_mode}
              options={[
                { value: 'quick-answer', label: t('agent.type.normal') },
                { value: 'smart-reasoning', label: t('agent.type.agent') },
              ]}
              onChange={(value) => onAgentModeChange(value as 'quick-answer' | 'smart-reasoning')}
            />
          </Row>
          {isAgentMode && resources.typePresets.length > 0 ? (
            // R485 D2 — Vue 118-135: agent type dropdown (smart-reasoning only),
            // preset description echoed under the label. R491 — options come
            // from the runtime type-presets fetch (static catalog on failure).
            <Row label={t('agentEditor.agentType.label')} desc={t('agentEditor.agentType.desc')}>
              <Select
                data-field="agent_type"
                className={`wk-ae-select ${FIELD_SELECT}`}
                value={form.config.agent_type || 'custom'}
                disabled={form.is_builtin}
                onChange={(event) => onAgentTypeChange(event.target.value)}
              >
                {resources.typePresets.map((preset) => (
                  <option key={preset.id} value={preset.id}>{agentTypePresetLabel(preset, locale)}</option>
                ))}
              </Select>
              {activeAgentTypePreset ? (
                <p className="m-0 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" data-agent-type-desc>
                  {agentTypePresetDescription(activeAgentTypePreset, locale)}
                </p>
              ) : null}
            </Row>
          ) : null}
          <Row label={t('agent.editor.name')} required={!form.is_builtin} desc={t('agentEditor.desc.name')} htmlFor="wk-ae-name" error={errorMessage('name')}>
            <Input
              id="wk-ae-name"
              data-field="name"
              className={`wk-ae-input ${FIELD_INPUT}`}
              value={form.name}
              placeholder={t('agent.editor.namePlaceholder')}
              disabled={form.is_builtin}
              onChange={(event) => patch((draft) => { draft.name = event.target.value; })}
            />
          </Row>
          <Row label={t('agent.editor.description')} desc={t('agentEditor.desc.description')} htmlFor="wk-ae-description">
            <Textarea
              id="wk-ae-description"
              data-field="description"
              className={`wk-ae-textarea ${FIELD_TEXTAREA}`}
              rows={3}
              value={form.description}
              placeholder={t('agent.editor.descriptionPlaceholder')}
              disabled={form.is_builtin}
              onChange={(event) => patch((draft) => { draft.description = event.target.value; })}
            />
          </Row>
          <Row label={t('agent.editor.memoryEnabled')} desc={t('agentEditor.desc.memoryEnabled')}>
            <Switch field="memory_enabled" label={t('agent.editor.memoryEnabled')} checked={form.config.memory_enabled}
              onChange={(next) => patchConfig('memory_enabled', next)} />
          </Row>
        </div>
      </section>
    );
  }

  // R486 D4 — the placeholder tag strip Vue renders under each prompt desc
  // (AgentEditorModal.vue:221-231): 可用变量 label, one {{name}} chip per
  // definition (tooltip = description + click-to-insert) and the hint.
  // R491 — the definitions come from GET /agents/placeholders with the
  // vendored catalogue as the failed-fetch fallback (Vue: fetched only).
  const renderPlaceholderTags = (field: 'agent_system_prompt' | 'system_prompt' | 'context_template', target: 'system_prompt' | 'context_template', tagBlock: string) => (
    <div className="mt-1.5 flex flex-wrap items-center gap-1" data-placeholder-tags={tagBlock}>
      <span className="text-[12px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">{t('agentEditor.placeholders.available')}</span>
      {promptPlaceholdersFor(field, resources.placeholders).map((def) => (
        <button
          key={def.name}
          type="button"
          data-placeholder-tag={def.name}
          title={`${def.description}${t('agentEditor.placeholders.clickToInsert')}`}
          className="cursor-pointer rounded border border-[var(--td-component-stroke,#e7e7e7)] bg-[var(--td-bg-color-secondarycontainer,#f2f3f5)] px-1.5 py-[1px] font-[family-name:monospace] text-[11px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))] hover:border-[var(--td-brand-color,#0052d9)] hover:text-[var(--td-brand-color,#0052d9)]"
          onClick={() => insertPromptPlaceholder(target, def.name)}
        >{`{{${def.name}}}`}</button>
      ))}
      <span className="text-[12px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">{t('agentEditor.placeholders.hint')}</span>
    </div>
  );

  // R486 D4 — PromptTemplateSelector corner controls (frontend/src/components/
  // PromptTemplateSelector.vue): 恢复默认 + 使用模板 popup. R491 — the list
  // is fetched from GET /tenants/kv/prompt-templates (agent_system_prompt
  // section; backend single-language name/description strings rendered
  // verbatim like Vue), with the vendored builtin list as the failed-fetch
  // fallback. The normal-mode system/context template bodies still have no
  // React picker, so the controls stay agent-mode only (R486 gap).
  const renderTemplateControls = () => (
    <div className="relative flex items-center gap-1.5 self-end">
      <button
        type="button"
        data-prompt-reset-default
        className="flex cursor-pointer items-center gap-1 rounded-md border-none bg-transparent px-1.5 py-[3px] text-[12px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))] hover:text-[var(--td-brand-color,#0052d9)]"
        onClick={() => {
          const template = resolveAgentSystemPromptResetTemplate(form.config.agent_type, resources.typePresets, resources.promptTemplates);
          if (!template) return;
          patch((draft) => {
            draft.config.system_prompt = template.content;
            draft.config.system_prompt_id = template.id;
          });
        }}
      >⟲ {t('promptTemplate.resetDefault')}</button>
      <div className="relative">
        <button
          type="button"
          data-prompt-template-toggle
          aria-haspopup="listbox"
          aria-expanded={templatePanelOpen ? 'true' : 'false'}
          className="flex cursor-pointer items-center gap-1 rounded-md border border-[var(--td-component-stroke,#dcdcdc)] bg-[var(--td-bg-color-container,#fff)] px-2 py-[3px] text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))] hover:border-[var(--td-brand-color,#0052d9)] hover:text-[var(--td-brand-color,#0052d9)]"
          onClick={() => setTemplatePanelOpen((current) => !current)}
        >▦ {t('promptTemplate.useTemplate')}</button>
        {templatePanelOpen ? (
          <div className="absolute right-0 bottom-[calc(100%+4px)] z-30 w-[420px] rounded-md border border-[var(--td-component-stroke,#e7e7e7)] bg-[var(--td-bg-color-container,#fff)] p-1 shadow-[0_8px_32px_rgba(0,0,0,0.12)]" role="listbox" data-prompt-template-panel>
            <p className="m-0 border-b border-[var(--td-component-stroke,#e7e7e7)] px-3 py-2 text-[13px] font-medium">{t('promptTemplate.selectTemplate')}</p>
            <div className="max-h-[320px] overflow-y-auto p-1">
              {resources.promptTemplates.length === 0 ? (
                <p className="m-0 px-3 py-6 text-center text-[13px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">{t('promptTemplate.noTemplates')}</p>
              ) : resources.promptTemplates.map((template) => (
                <button
                  key={template.id}
                  type="button"
                  data-prompt-template={template.id}
                  className="mb-1 flex w-full cursor-pointer flex-col items-start gap-1 rounded-md border-none bg-transparent px-3 py-2.5 text-left last:mb-0 hover:bg-[var(--td-bg-color-secondarycontainer,#f2f3f5)]"
                  onClick={() => {
                    // Vue handleSystemPromptTemplateSelect (4683-4686): the
                    // selection replaces the body only; system_prompt_id is
                    // written by 恢复默认, not by a manual pick.
                    patchConfig('system_prompt', template.content);
                    setTemplatePanelOpen(false);
                  }}
                >
                  <span className="flex items-center gap-2 text-[13px] font-medium">
                    {template.name}
                    {template.default ? (
                      <span className="rounded bg-[var(--td-warning-color-light,#fdf1e3)] px-1.5 py-[1px] text-[11px] font-medium text-[var(--td-warning-color,#e37318)]" data-template-default>{t('promptTemplate.default')}</span>
                    ) : null}
                  </span>
                  <span className="text-left text-[12px] leading-[1.5] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{template.description}</span>
                </button>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );

  function renderPrompts() {
    return (
      <section className="wk-ae-section" data-editor-section="prompts">
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agent.editor.promptsConfig')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.promptsConfigDesc')}</p>
        </header>
        <div className="flex flex-col gap-[18px]">
          <Row
            label={t('agent.editor.systemPrompt')}
            required={!form.is_builtin}
            desc={t('agentEditor.desc.systemPrompt')}
            htmlFor="wk-ae-system-prompt"
            error={errorMessage('system_prompt')}
            extra={renderPlaceholderTags(isAgentMode ? 'agent_system_prompt' : 'system_prompt', 'system_prompt', 'system')}
          >
            {isAgentMode ? renderTemplateControls() : null}
            <Textarea
              id="wk-ae-system-prompt"
              data-field="system_prompt"
              className={`wk-ae-textarea ${FIELD_TEXTAREA} ${FIELD_TALL}`}
              rows={10}
              value={form.config.system_prompt}
              disabled={form.is_builtin}
              ref={systemPromptRef}
              onChange={(event) => patchConfig('system_prompt', event.target.value)}
            />
          </Row>
          {quickAnswer ? (
            <Row
              label={t('agent.editor.contextTemplate')}
              required={!form.is_builtin}
              desc={t('agentEditor.desc.contextTemplate')}
              htmlFor="wk-ae-context-template"
              error={errorMessage('context_template')}
              extra={renderPlaceholderTags('context_template', 'context_template', 'context')}
            >
              <Textarea
                id="wk-ae-context-template"
                data-field="context_template"
                className={`wk-ae-textarea ${FIELD_TEXTAREA}`}
                rows={8}
                value={form.config.context_template}
                disabled={form.is_builtin}
                ref={contextTemplateRef}
                onChange={(event) => patchConfig('context_template', event.target.value)}
              />
            </Row>
          ) : null}
          {rewriteActive ? (
            <Row label={t('agent.editor.rewritePromptUser')} desc={t('agentEditor.desc.rewriteUserPrompt')} htmlFor="wk-ae-rewrite-user" error={errorMessage('rewrite_prompt_user')}>
              <Textarea
                id="wk-ae-rewrite-user"
                data-field="rewrite_prompt_user"
                className={`wk-ae-textarea ${FIELD_TEXTAREA}`}
                rows={4}
                value={form.config.rewrite_prompt_user}
                onChange={(event) => patchConfig('rewrite_prompt_user', event.target.value)}
              />
            </Row>
          ) : null}
          {showFallback ? (
            <Row label={t('agent.editor.fallbackStrategy')} desc={t('agentEditor.desc.fallbackStrategy')}>
              <RadioGroup
                name="fallback-strategy"
                value={form.config.fallback_strategy}
                options={[
                  { value: 'fixed', label: t('agentEditor.fallback.fixed') },
                  { value: 'model', label: t('agentEditor.fallback.model') },
                ]}
                onChange={(value) => patchConfig('fallback_strategy', value as 'fixed' | 'model')}
              />
            </Row>
          ) : null}
          {showFallback && form.config.fallback_strategy === 'fixed' ? (
            <Row label={t('agent.editor.fallbackResponse')} desc={t('agentEditor.desc.fallbackResponse')} htmlFor="wk-ae-fallback-response">
              <Textarea
                id="wk-ae-fallback-response"
                data-field="fallback_response"
                className={`wk-ae-textarea ${FIELD_TEXTAREA}`}
                rows={3}
                value={form.config.fallback_response}
                onChange={(event) => patchConfig('fallback_response', event.target.value)}
              />
            </Row>
          ) : null}
          {showFallback && form.config.fallback_strategy === 'model' ? (
            <Row label={t('agent.editor.fallbackPrompt')} desc={t('agentEditor.desc.fallbackPrompt')} htmlFor="wk-ae-fallback-prompt" error={errorMessage('fallback_prompt')}>
              <Textarea
                id="wk-ae-fallback-prompt"
                data-field="fallback_prompt"
                className={`wk-ae-textarea ${FIELD_TEXTAREA}`}
                rows={4}
                value={form.config.fallback_prompt}
                onChange={(event) => patchConfig('fallback_prompt', event.target.value)}
              />
            </Row>
          ) : null}
        </div>
      </section>
    );
  }

  function renderModel() {
    return (
      <section className={`wk-ae-section ${highlightedField === 'summary_model' || highlightedField === 'rerank_model' ? 'ring-2 ring-[var(--td-brand-color,#0052d9)] ring-inset' : ''}`} data-editor-section="model" data-highlighted-field={highlightedField === 'summary_model' || highlightedField === 'rerank_model' ? highlightedField : undefined}>
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agent.editor.modelConfig')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.modelConfigDesc')}</p>
        </header>
        <div className="flex flex-col gap-[18px]">
          <Row label={t('agent.editor.model')} required desc={t('agentEditor.desc.model')} error={errorMessage('model_id')} htmlFor="wk-ae-model-id">
            <Select
              id="wk-ae-model-id"
              data-field="model_id"
              className={`wk-ae-select ${FIELD_SELECT}`}
              value={form.config.model_id}
              onChange={(event) => patchConfig('model_id', event.target.value)}
            >
              <option value="">{t('agent.editor.modelPlaceholder')}</option>
              {chatModels.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
            </Select>
          </Row>
          <Row label={t('agent.editor.temperature')} desc={t('agentEditor.desc.temperature')}>
            <Slider ariaLabel={t('agent.editor.temperature')} min={0} max={1} step={0.1} value={form.config.temperature}
              onChange={(next) => patchConfig('temperature', next)} />
          </Row>
          <Row label={t('agent.editor.maxCompletionTokens')} desc={isAgentMode ? t('agentEditor.desc.maxTokensAgent') : t('agentEditor.desc.maxTokens')}>
            <RadioGroup
              name="max-tokens-mode"
              value={maxTokensMode}
              options={[
                { value: 'default', label: t('agent.editor.maxCompletionTokensDefault') },
                { value: 'custom', label: t('agent.editor.maxCompletionTokensCustom') },
              ]}
              onChange={(value) => {
                setMaxTokensMode(value as 'default' | 'custom');
                patchConfig('max_completion_tokens', value === 'custom' ? 2048 : 0);
              }}
            />
            {maxTokensMode === 'custom' ? (
              <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={100} max={100000} step={100}
                data-field="max_completion_tokens"
                value={form.config.max_completion_tokens}
                onChange={(event) => patchConfig('max_completion_tokens', Number(event.target.value) || 0)} />
            ) : null}
          </Row>
          <Row label={t('agent.editor.thinking')} desc={t('agentEditor.desc.thinking')}>
            <Switch field="thinking" label={t('agent.editor.thinking')} checked={form.config.thinking}
              onChange={(next) => patchConfig('thinking', next)} />
          </Row>
          <Row label={t('agent.editor.citationEnabled')} desc={t('agent.editor.citationEnabledDesc')}>
            <Switch field="citation_enabled" label={t('agent.editor.citationEnabled')} checked={form.config.citation_enabled}
              onChange={(next) => patchConfig('citation_enabled', next)} />
          </Row>
          {hasKnowledgeBase ? (
            // R485 D6 — Vue 671-684: the rerank row is required (star, no
            // optional hint, no clear option) while a RAG KB is in scope.
            <Row
              label={t('agent.editor.rerankModel')}
              desc={t('agent.editor.rerankModelDesc')}
              required={rerankRequired}
              hint={rerankRequired ? undefined : t('agent.editor.rerankModelOptionalHint')}
              htmlFor="wk-ae-rerank-id"
            >
              <Select
                id="wk-ae-rerank-id"
                data-field="rerank_model_id"
                className={`wk-ae-select ${FIELD_SELECT}`}
                value={form.config.rerank_model_id}
                onChange={(event) => patchConfig('rerank_model_id', event.target.value)}
              >
                {rerankRequired ? null : <option value="">{t('agent.editor.rerankModelPlaceholder')}</option>}
                {rerankModels.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
              </Select>
            </Row>
          ) : null}
          {quickAnswer && form.config.multi_turn_enabled && form.config.enable_rewrite ? (
            <Row label={t('agent.editor.queryUnderstandModel')} desc={t('agentEditor.desc.queryUnderstandModel')}>
              <Select
                data-field="query_understand_model_id"
                className={`wk-ae-select ${FIELD_SELECT}`}
                value={form.config.query_understand_model_id}
                onChange={(event) => patchConfig('query_understand_model_id', event.target.value)}
              >
                <option value="">{t('agent.editor.queryUnderstandModelPlaceholder')}</option>
                {chatModels.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
              </Select>
            </Row>
          ) : null}
          {isAgentMode ? (
            <Row label={t('agent.editor.maxIterations')} desc={t('agentEditor.desc.maxIterations')}>
              <RadioGroup
                name="max-iterations-mode"
                value={maxIterationsMode}
                options={[
                  { value: 'limit', label: t('agent.editor.maxIterationsLimit') },
                  { value: 'unlimited', label: t('agent.editor.maxIterationsUnlimited') },
                ]}
                onChange={(value) => {
                  setMaxIterationsMode(value as 'limit' | 'unlimited');
                  patchConfig('max_iterations', value === 'unlimited' ? -1 : 10);
                }}
              />
              {maxIterationsMode === 'limit' ? (
                <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={2} max={50}
                  data-field="max_iterations"
                  value={form.config.max_iterations}
                  onChange={(event) => patchConfig('max_iterations', Number(event.target.value) || 1)} />
              ) : null}
            </Row>
          ) : null}
          {isAgentMode ? (
            <Row label={t('agentEditor.llmCallTimeout.label')} desc={t('agentEditor.llmCallTimeout.desc')} hint={t('agentEditor.llmCallTimeout.hint')}>
              <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={0} max={3600}
                data-field="llm_call_timeout"
                value={form.config.llm_call_timeout}
                onChange={(event) => patchConfig('llm_call_timeout', Number(event.target.value) || 0)} />
            </Row>
          ) : null}
        </div>
      </section>
    );
  }

  function renderConversation() {
    return (
      <section className="wk-ae-section" data-editor-section="conversation">
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agent.editor.conversationSettings')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{isAgentMode ? t('agentEditor.desc.conversationSectionAgent') : t('agentEditor.desc.conversationSection')}</p>
        </header>
        <div className="flex flex-col gap-[18px]">
          {quickAnswer ? (
            <Row label={t('agent.editor.multiTurn')} desc={t('agentEditor.desc.multiTurn')}>
              <Switch field="multi_turn_enabled" label={t('agent.editor.multiTurn')} checked={form.config.multi_turn_enabled}
                onChange={(next) => patchConfig('multi_turn_enabled', next)} />
            </Row>
          ) : null}
          {form.config.multi_turn_enabled || isAgentMode ? (
            <Row label={t('agent.editor.historyTurns')} desc={t('agentEditor.desc.historyRounds')}>
              <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={1} max={100}
                data-field="history_turns"
                value={form.config.history_turns}
                onChange={(event) => patchConfig('history_turns', Number(event.target.value) || 1)} />
            </Row>
          ) : null}
          {isAgentMode && hasKnowledgeBase ? (
            <Row label={t('agent.editor.retainRetrievalHistory')} desc={t('agentEditor.desc.retainRetrievalHistory')}>
              <Switch field="retain_retrieval_history" label={t('agent.editor.retainRetrievalHistory')} checked={form.config.retain_retrieval_history}
                onChange={(next) => patchConfig('retain_retrieval_history', next)} />
            </Row>
          ) : null}
          {quickAnswer && form.config.multi_turn_enabled ? (
            <Row label={t('agent.editor.enableRewrite')} desc={t('agentEditor.desc.rewrite')}>
              <Switch field="enable_rewrite" label={t('agent.editor.enableRewrite')} checked={form.config.enable_rewrite}
                onChange={(next) => patchConfig('enable_rewrite', next)} />
            </Row>
          ) : null}
        </div>
      </section>
    );
  }

  function renderKnowledge() {
    return (
      <section className="wk-ae-section" data-editor-section="knowledge">
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agent.editor.knowledgeConfig')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.knowledgeConfigDesc')}</p>
        </header>
        <div className="flex flex-col gap-[18px]">
          <Row label={t('agent.editor.knowledgeBases')} desc={t('agentEditor.desc.kbScope')}>
            <RadioGroup
              name="kb-mode"
              value={kbMode}
              options={[
                { value: 'all', label: t('agent.editor.allKnowledgeBases') },
                { value: 'selected', label: t('agent.editor.selectedKnowledgeBases') },
                { value: 'none', label: t('agent.editor.noKnowledgeBase') },
              ]}
              onChange={(value) => onKbModeChange(value as ScopeSelectionMode)}
            />
          </Row>
          {kbMode === 'selected' ? (
            <Row label={t('agent.editor.selectKnowledgeBases')} desc={t('agent.editor.selectKnowledgeBasesDesc')}>
              <div className="flex w-full max-w-[460px] flex-col gap-1">
                {myKbs.length > 0 ? <p className="mb-[2px] mt-[6px] text-[12px] font-semibold text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.myKnowledgeBases')}</p> : null}
                {myKbs.map(renderKbCheckbox)}
                {sharedKbs.length > 0 ? <p className="mb-[2px] mt-[6px] text-[12px] font-semibold text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.sharedKnowledgeBases')}</p> : null}
                {sharedKbs.map(renderKbCheckbox)}
              </div>
            </Row>
          ) : null}
          {hasKnowledgeBase ? (
            // R486 P3-2 — Vue AgentEditorModal.vue:1529-1536: 支持的文件类型 is
            // a t-select multiple (placeholder 全部类型, min-collapsed-num 3);
            // the dropdown multi-select keeps the selected-set semantics and
            // writes back config.supported_file_types unchanged.
            <Row label={t('agentEditor.fileTypes.label')} desc={t('agentEditor.fileTypes.desc')}>
              <FileTypeMultiSelect
                selected={form.config.supported_file_types}
                options={AGENT_FILE_TYPE_OPTIONS}
                placeholderLabel={t('agentEditor.fileTypes.allTypes')}
                t={t}
                onChange={(next) => patchConfig('supported_file_types', next)}
              />
            </Row>
          ) : null}
          {hasKnowledgeBase ? (
            <Row label={t('agent.editor.retrieveKBOnlyWhenMentioned')} desc={t('agent.editor.retrieveKBOnlyWhenMentionedDesc')}>
              <Switch field="retrieve_kb_only_when_mentioned" label={t('agent.editor.retrieveKBOnlyWhenMentioned')}
                checked={form.config.retrieve_kb_only_when_mentioned}
                onChange={(next) => patchConfig('retrieve_kb_only_when_mentioned', next)} />
            </Row>
          ) : null}
        </div>
      </section>
    );
  }

  function renderRetrieval() {
    return (
      <section className="wk-ae-section" data-editor-section="retrieval">
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agent.editor.retrievalStrategy')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agentEditor.desc.retrievalSection')}</p>
        </header>
        <div className="flex flex-col gap-[18px]">
          {quickAnswer ? (
            <Row label={t('agent.editor.enableQueryExpansion')} desc={t('agentEditor.desc.queryExpansion')}>
              <Switch field="enable_query_expansion" label={t('agent.editor.enableQueryExpansion')} checked={form.config.enable_query_expansion}
                onChange={(next) => patchConfig('enable_query_expansion', next)} />
            </Row>
          ) : null}
          <Row label={t('agent.editor.embeddingTopK')} desc={t('agentEditor.desc.embeddingTopK')}>
            <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={1} max={50} data-field="embedding_top_k"
              value={form.config.embedding_top_k}
              onChange={(event) => patchConfig('embedding_top_k', Number(event.target.value) || 1)} />
          </Row>
          <Row label={t('agent.editor.keywordThreshold')} desc={t('agentEditor.desc.keywordThreshold')}>
            <Slider ariaLabel={t('agent.editor.keywordThreshold')} min={0} max={1} step={0.01} value={form.config.keyword_threshold}
              onChange={(next) => patchConfig('keyword_threshold', next)} />
          </Row>
          <Row label={t('agent.editor.vectorThreshold')} desc={t('agentEditor.desc.vectorThreshold')}>
            <Slider ariaLabel={t('agent.editor.vectorThreshold')} min={0} max={1} step={0.01} value={form.config.vector_threshold}
              onChange={(next) => patchConfig('vector_threshold', next)} />
          </Row>
          {form.config.rerank_model_id ? (
            <>
              <Row label={t('agent.editor.rerankTopK')} desc={t('agentEditor.desc.rerankTopK')}>
                <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={1} max={20} data-field="rerank_top_k"
                  value={form.config.rerank_top_k}
                  onChange={(event) => patchConfig('rerank_top_k', Number(event.target.value) || 1)} />
              </Row>
              <Row label={t('agent.editor.rerankThreshold')} desc={t('agentEditor.desc.rerankThreshold')}>
                <Slider ariaLabel={t('agent.editor.rerankThreshold')} min={-10} max={10} step={0.01} value={form.config.rerank_threshold}
                  onChange={(next) => patchConfig('rerank_threshold', next)} />
              </Row>
            </>
          ) : null}
          {hasFaqKnowledgeBase ? (
            <>
              <Row label={t('agentEditor.faq.enableLabel')} desc={t('agentEditor.faq.enableDesc')}>
                <Switch field="faq_priority_enabled" label={t('agentEditor.faq.enableLabel')} checked={form.config.faq_priority_enabled}
                  onChange={(next) => patchConfig('faq_priority_enabled', next)} />
              </Row>
              {form.config.faq_priority_enabled ? (
                <>
                  <Row label={t('agentEditor.faq.thresholdLabel')} desc={t('agentEditor.faq.thresholdDesc')}>
                    <Slider ariaLabel={t('agentEditor.faq.thresholdLabel')} min={0.7} max={1} step={0.05} value={form.config.faq_direct_answer_threshold}
                      onChange={(next) => patchConfig('faq_direct_answer_threshold', next)} />
                  </Row>
                  <Row label={t('agentEditor.faq.boostLabel')} desc={t('agentEditor.faq.boostDesc')}>
                    <Slider ariaLabel={t('agentEditor.faq.boostLabel')} min={1} max={2} step={0.1} value={form.config.faq_score_boost}
                      onChange={(next) => patchConfig('faq_score_boost', next)} />
                  </Row>
                </>
              ) : null}
            </>
          ) : null}
          {quickAnswer ? (
            <Row label={t('agentEditor.dataAnalysis.enableLabel')} desc={t('agentEditor.dataAnalysis.enableDesc')}>
              <Switch field="data_analysis_enabled" label={t('agentEditor.dataAnalysis.enableLabel')} checked={form.config.data_analysis_enabled}
                onChange={(next) => patchConfig('data_analysis_enabled', next)} />
            </Row>
          ) : null}
        </div>
      </section>
    );
  }

  function renderWebSearch() {
    return (
      <section className="wk-ae-section" data-editor-section="websearch">
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agent.editor.webSearchConfig')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.webSearchConfigDesc')}</p>
        </header>
        <div className="flex flex-col gap-[18px]">
          <Row label={t('agent.editor.webSearch')} desc={t('agentEditor.desc.webSearch')}>
            <Switch field="web_search_enabled" label={t('agent.editor.webSearch')} checked={form.config.web_search_enabled}
              onChange={(next) => patchConfig('web_search_enabled', next)} />
          </Row>
          {form.config.web_search_enabled ? (
            <>
              <Row label={t('agent.editor.webSearchProvider')} desc={t('agentEditor.desc.webSearchProvider')}>
                <Select data-field="web_search_provider_id" className={`wk-ae-select ${FIELD_SELECT}`} value={form.config.web_search_provider_id}
                  onChange={(event) => patchConfig('web_search_provider_id', event.target.value)}>
                  <option value="">{t('agent.editor.webSearchProviderPlaceholder')}</option>
                  {deps.providers.map((provider) => (
                    <option key={provider.id} value={provider.id}>{provider.name}{provider.is_default ? ' · ' + t('common.default') : ''}</option>
                  ))}
                </Select>
              </Row>
              <Row label={t('agent.editor.webSearchMaxResults')} desc={t('agentEditor.desc.webSearchMaxResults')}>
                <Slider ariaLabel={t('agent.editor.webSearchMaxResults')} min={1} max={10} step={1} value={form.config.web_search_max_results}
                  onChange={(next) => patchConfig('web_search_max_results', next)} />
              </Row>
              <Row label={t('agent.editor.webFetchEnabled')} desc={t('agentEditor.desc.webFetchEnabled')}>
                <Switch field="web_fetch_enabled" label={t('agent.editor.webFetchEnabled')} checked={form.config.web_fetch_enabled}
                  onChange={(next) => patchConfig('web_fetch_enabled', next)} />
              </Row>
              {form.config.web_fetch_enabled ? (
                <Row label={t('agent.editor.webFetchTopN')} desc={t('agentEditor.desc.webFetchTopN')}>
                  <Slider ariaLabel={t('agent.editor.webFetchTopN')} min={1} max={10} step={1} value={form.config.web_fetch_top_n}
                    onChange={(next) => patchConfig('web_fetch_top_n', next)} />
                </Row>
              ) : null}
            </>
          ) : null}
        </div>
      </section>
    );
  }

  function renderTools() {
    const activeTools = TOOL_CATALOG.filter((tool) => form.config.allowed_tools.includes(tool.value));
    const evaluations = activeTools.map((tool) => ({ tool, ...evaluateTool(tool) }));
    const inactiveToolCount = evaluations.filter((item) => !item.ok).length;
    return (
      <section className={`wk-ae-section ${highlightedField === 'allowed_tools' ? 'ring-2 ring-[var(--td-brand-color,#0052d9)] ring-inset' : ''}`} data-editor-section="tools" data-highlighted-field={highlightedField === 'allowed_tools' ? 'allowed_tools' : undefined}>
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agent.editor.toolsConfig')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.toolsConfigDesc')}</p>
        </header>
        <p className="text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">
          {hasKnowledgeBase
            ? t('agentEditor.tools.kbMetricRag') + ' · ' + t('agentEditor.tools.kbMetricWiki')
            : t('agentEditor.tools.statusNoKb')}
          {inactiveToolCount > 0 ? ' · ' + t('agentEditor.tools.statusInactive', { count: inactiveToolCount }) : ''}
        </p>
        <div className="flex flex-col gap-[18px]">
          {/* R485 D10 — Vue 1163-1167: the allowed-tools block carries the
              「允许的工具」label + 「选择 Agent 可以使用的工具」desc and per-group
              counts, which the React port omitted. */}
          <Row label={t('agent.editor.allowedTools')} desc={t('agentEditor.desc.selectTools')}>
            <div className="flex w-full flex-col gap-3" data-agent-field="allowed_tools">
              {TOOL_GROUPS.map((group) => {
                const tools = TOOL_CATALOG.filter((tool) => tool.group === group.key);
                if (tools.length === 0) return null;
                return (
                  <div key={group.key} className="wk-ae-tool-group">
                    <p className="mb-1.5 mt-1 flex items-center gap-1.5 text-[13px] font-semibold">
                      <span className="inline-block h-3 w-[3px] rounded bg-[var(--td-brand-color,#0052d9)]" aria-hidden="true" />
                      <span>{t(group.labelKey)}</span>
                      <span className="text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">{tools.length}</span>
                      {group.key === 'wiki_edit' ? <span className="font-normal text-[var(--td-warning-color,#e37318)]">{t('agentEditor.tools.writeWarning')}</span> : null}
                    </p>
                    <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-2">
                      {tools.map((tool) => {
                        const evaluation = evaluateTool(tool);
                        const checked = form.config.allowed_tools.includes(tool.value);
                        return (
                          <label key={tool.value} className={`flex cursor-pointer flex-col gap-0.5 rounded-lg border px-2.5 py-2 text-[13px] ${evaluation.ok ? '' : 'cursor-not-allowed opacity-55'} ${tool.danger ? 'border-[var(--td-warning-color,#e37318)]' : 'border-[var(--td-component-stroke,#e7e7e7)]'}`}>
                            <Checkbox
                              data-tool={tool.value}
                              checked={checked}
                              disabled={!evaluation.ok}
                              onChange={(event) => {
                                const next = event.target.checked;
                                patch((draft) => {
                                  const set = new Set(draft.config.allowed_tools);
                                  if (next) set.add(tool.value); else set.delete(tool.value);
                                  draft.config.allowed_tools = [...set];
                                });
                              }}
                            />
                            <span className="font-medium">{t(tool.labelKey)}{tool.danger ? ' · ' + t('agentEditor.tools.dangerTag') : ''}</span>
                            <span className="text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t(tool.descriptionKey)}</span>
                            {!evaluation.ok ? <span className="text-[12px] text-[var(--td-warning-color,#e37318)]">{t(missReasonKey(evaluation.missKind))}</span> : null}
                          </label>
                        );
                      })}
                    </div>
                  </div>
                );
              })}
            </div>
          </Row>
          <Row label={t('agentEditor.tools.effectiveLabel')} desc={t('agentEditor.tools.effectiveDesc')}>
            <div className="flex flex-wrap gap-1.5">
              {evaluations.length === 0 && !form.config.web_search_enabled ? <span className="m-0 mt-1 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agentEditor.tools.effectiveEmpty')}</span> : null}
              {evaluations.map(({ tool, ok }) => (
                <span key={tool.value} className={ok ? 'rounded-[10px] bg-[var(--td-brand-color-light,#ecf2fe)] px-2.5 py-[2px] text-[12px] text-[var(--td-brand-color,#0052d9)]' : 'rounded-[10px] bg-[var(--td-bg-color-secondarycontainer,#f2f3f5)] px-2.5 py-[2px] text-[12px] text-[var(--td-text-color-disabled,rgba(0,0,0,0.26))]'}>{t(tool.labelKey)}</span>
              ))}
              {form.config.web_search_enabled ? (
                <>
                  <span className="rounded-[10px] bg-[var(--td-brand-color-light,#ecf2fe)] px-2.5 py-[2px] text-[12px] text-[var(--td-brand-color,#0052d9)]">{t('agentEditor.tools.webSearch')}</span>
                  <span className="rounded-[10px] bg-[var(--td-brand-color-light,#ecf2fe)] px-2.5 py-[2px] text-[12px] text-[var(--td-brand-color,#0052d9)]">{t('agentEditor.tools.webFetch')}</span>
                </>
              ) : null}
            </div>
          </Row>
        </div>
      </section>
    );
  }

  function renderSuggestions() {
    const qs = form.config.question_suggestions;
    const starters = qs.starters;
    const followUps = qs.follow_ups;
    return (
      <section className="wk-ae-section" data-editor-section="suggestions">
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agentEditor.questionSuggestions.title')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agentEditor.questionSuggestions.description')}</p>
        </header>
        <div className="mb-3 flex gap-1" role="tablist" aria-label={t('agentEditor.questionSuggestions.title')}>
          {(['starters', 'followUps'] as const).map((tab) => (
            <button
              key={tab}
              type="button"
              role="tab"
              data-suggestion-tab={tab}
              aria-selected={suggestionTab === tab ? 'true' : 'false'}
              className={`cursor-pointer rounded-t-md border-b-2 px-4 py-1.5 text-[14px] ${suggestionTab === tab ? 'border-[var(--td-brand-color,#0052d9)] font-medium text-[var(--td-brand-color,#0052d9)]' : 'border-transparent text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]'}`}
              onClick={() => setSuggestionTab(tab)}
            >
              {t(tab === 'starters' ? 'agentEditor.questionSuggestions.startersTitle' : 'agentEditor.questionSuggestions.followUpsTitle')}
            </button>
          ))}
        </div>
        {suggestionTab === 'starters' ? (
          <div className="flex flex-col gap-[18px]">
            <Row label={t('agentEditor.questionSuggestions.enableStarters')} desc={t('agentEditor.questionSuggestions.enableStartersDesc')}>
              <Switch field="question_suggestions.starters.enabled" label={t('agentEditor.questionSuggestions.enableStarters')}
                checked={starters.enabled}
                onChange={(next) => patchQs((draft) => { draft.starters.enabled = next; })} />
            </Row>
            {starters.enabled ? (
              <Row label={t('agentEditor.questionSuggestions.sourceMode')}>
                <Select data-field="question_suggestions.starters.mode" className={`wk-ae-select ${FIELD_SELECT}`} value={starters.mode}
                  onChange={(event) => patchQs((draft) => { draft.starters.mode = event.target.value as typeof draft.starters.mode; })}>
                  <option value="curated">{t('agentEditor.questionSuggestions.modeCurated')}</option>
                  <option value="knowledge">{t('agentEditor.questionSuggestions.modeKnowledge')}</option>
                  <option value="hybrid">{t('agentEditor.questionSuggestions.modeHybrid')}</option>
                </Select>
              </Row>
            ) : null}
            {starters.enabled ? (
              <Row label={t('agentEditor.questionSuggestions.count')}>
                <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={1} max={8}
                  data-field="question_suggestions.starters.count"
                  value={starters.count}
                  onChange={(event) => patchQs((draft) => { draft.starters.count = Number(event.target.value) || 1; })} />
              </Row>
            ) : null}
            {starters.enabled && (starters.mode === 'curated' || starters.mode === 'hybrid') ? (
              <Row
                label={t('agentEditor.questionSuggestions.curatedItems') + ` ${starters.items.length}/8`}
                desc={t('agentEditor.questionSuggestions.curatedItemsDesc')}
              >
                <div className="flex w-full max-w-[560px] flex-col gap-1.5" data-field-group="question_suggestions.starters.items">
                  {starters.items.map((item, index) => (
                    <div key={index} className="flex items-center gap-1.5">
                      <Input className={`wk-ae-input w-full`} maxLength={200} value={item}
                        data-starter-index={index}
                        onChange={(event) => patchQs((draft) => { draft.starters.items[index] = event.target.value; })} />
                      <button type="button" aria-label={t('common.delete')} className="cursor-pointer border-0 bg-transparent p-1 text-[var(--td-error-color,#d54941)]"
                        data-remove-starter={index}
                        onClick={() => patchQs((draft) => { draft.starters.items.splice(index, 1); })}>×</button>
                    </div>
                  ))}
                  <button type="button" disabled={starters.items.length >= 8}
                    className="w-fit cursor-pointer rounded-md border border-dashed border-[var(--td-component-stroke,#dcdcdc)] px-3 py-1.5 text-[13px] disabled:cursor-not-allowed disabled:opacity-50"
                    data-add-starter
                    onClick={() => patchQs((draft) => { if (draft.starters.items.length < 8) draft.starters.items.push(''); })}>
                    {t('agentEditor.questionSuggestions.addItem')}
                  </button>
                </div>
              </Row>
            ) : null}
          </div>
        ) : (
          <div className="flex flex-col gap-[18px]">
            <Row label={t('agentEditor.questionSuggestions.enableFollowUps')} desc={t('agentEditor.questionSuggestions.enableFollowUpsDesc')}>
              <Switch field="question_suggestions.follow_ups.enabled" label={t('agentEditor.questionSuggestions.enableFollowUps')}
                checked={followUps.enabled}
                onChange={(next) => patchQs((draft) => { draft.follow_ups.enabled = next; })} />
            </Row>
            {followUps.enabled ? (
              <>
                <Row label={t('agentEditor.questionSuggestions.sourceMode')}>
                  <Select data-field="question_suggestions.follow_ups.mode" className={`wk-ae-select ${FIELD_SELECT}`} value={followUps.mode}
                    onChange={(event) => patchQs((draft) => { draft.follow_ups.mode = event.target.value as typeof draft.follow_ups.mode; })}>
                    <option value="generated">{t('agentEditor.questionSuggestions.modeGenerated')}</option>
                    <option value="knowledge">{t('agentEditor.questionSuggestions.modeKnowledge')}</option>
                    <option value="hybrid">{t('agentEditor.questionSuggestions.modeHybrid')}</option>
                  </Select>
                </Row>
                <Row label={t('agentEditor.questionSuggestions.count')}>
                  <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={1} max={5}
                    data-field="question_suggestions.follow_ups.count"
                    value={followUps.count}
                    onChange={(event) => patchQs((draft) => { draft.follow_ups.count = Number(event.target.value) || 1; })} />
                </Row>
                {followUps.mode !== 'knowledge' ? (
                  <Row label={t('agentEditor.questionSuggestions.model')} desc={t('agentEditor.questionSuggestions.modelDesc')}>
                    <Select data-field="question_suggestions.follow_ups.model_id" className={`wk-ae-select ${FIELD_SELECT}`} value={followUps.model_id}
                      onChange={(event) => patchQs((draft) => { draft.follow_ups.model_id = event.target.value; })}>
                      <option value="">{t('agentEditor.modelPlaceholder')}</option>
                      {chatModels.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
                    </Select>
                  </Row>
                ) : null}
                <div className="my-1 flex items-center gap-2 text-[12px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">
                  <span className="h-px flex-1 bg-[var(--td-component-stroke,#e7e7e7)]" />
                  <span>{t('agentEditor.questionSuggestions.advancedSettings')}</span>
                  <span className="h-px flex-1 bg-[var(--td-component-stroke,#e7e7e7)]" />
                </div>
                <Row label={t('agentEditor.questionSuggestions.contextTurns')}>
                  <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={1} max={5}
                    data-field="question_suggestions.follow_ups.max_context_turns"
                    value={followUps.max_context_turns}
                    onChange={(event) => patchQs((draft) => { draft.follow_ups.max_context_turns = Number(event.target.value) || 1; })} />
                </Row>
                <Row label={t('agentEditor.questionSuggestions.categories')}>
                  <div className="flex flex-wrap gap-3" data-field-group="question_suggestions.follow_ups.categories">
                    {(['clarify', 'deepen', 'action'] as const).map((category) => (
                      <label key={category} className="inline-flex cursor-pointer items-center gap-1.5 text-[13px] [&>input]:accent-[var(--td-brand-color,#0052d9)]">
                        <Checkbox
                          data-category={category}
                          checked={followUps.categories.includes(category)}
                          onChange={(event) => patchQs((draft) => {
                            const set = new Set(draft.follow_ups.categories);
                            if (event.target.checked) set.add(category); else set.delete(category);
                            draft.follow_ups.categories = [...set];
                          })}
                        />
                        <span>{t(`agentEditor.questionSuggestions.category${category === 'clarify' ? 'Clarify' : category === 'deepen' ? 'Deepen' : 'Action'}`)}</span>
                      </label>
                    ))}
                  </div>
                </Row>
                <Row label={t('agentEditor.questionSuggestions.instruction')}>
                  <Textarea className={`wk-ae-textarea ${FIELD_TEXTAREA}`} rows={3} maxLength={2000}
                    data-field="question_suggestions.follow_ups.additional_instruction"
                    value={followUps.additional_instruction}
                    placeholder={t('agentEditor.questionSuggestions.instructionPlaceholder')}
                    onChange={(event) => patchQs((draft) => { draft.follow_ups.additional_instruction = event.target.value; })} />
                </Row>
                <Row label={t('agentEditor.questionSuggestions.displayRules')}>
                  <div className="flex w-full max-w-[560px] flex-col gap-1.5">
                    {([
                      ['suppress_on_fallback', 'suppressFallback'],
                      ['suppress_when_answer_asks_question', 'suppressQuestion'],
                      ['knowledge_fallback', 'knowledgeFallback'],
                      ['allow_regenerate', 'allowRegenerate'],
                    ] as const).map(([field, labelKey]) => (
                      <label key={field} className="inline-flex cursor-pointer items-center gap-1.5 text-[13px] [&>input]:accent-[var(--td-brand-color,#0052d9)]">
                        <Checkbox
                          data-display-rule={field}
                          checked={followUps[field] === true}
                          onChange={(event) => patchQs((draft) => { (draft.follow_ups[field] as boolean) = event.target.checked; })}
                        />
                        <span>{t(`agentEditor.questionSuggestions.${labelKey}`)}</span>
                      </label>
                    ))}
                  </div>
                </Row>
              </>
            ) : null}
          </div>
        )}
      </section>
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
      <section className="wk-ae-section" data-editor-section="multimodal">
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agentEditor.imageUpload.sectionTitle')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agentEditor.imageUpload.sectionDesc')}</p>
        </header>
        <div className="flex flex-col gap-[18px]">
          <Row label={t('agentEditor.imageUpload.label')} desc={t('agentEditor.imageUpload.desc')}>
            <Switch field="image_upload_enabled" label={t('agentEditor.imageUpload.label')} checked={form.config.image_upload_enabled}
              onChange={(next) => patchConfig('image_upload_enabled', next)} />
          </Row>
          {form.config.image_upload_enabled ? (
            <Row label={t('agentEditor.imageUpload.vlmModel')} required desc={t('agentEditor.imageUpload.vlmModelDesc')}>
              <Select data-field="vlm_model_id" className={`wk-ae-select ${FIELD_SELECT}`} value={form.config.vlm_model_id}
                onChange={(event) => patchConfig('vlm_model_id', event.target.value)}>
                <option value="">{t('agentEditor.imageUpload.vlmModelPlaceholder')}</option>
                {vlmModels.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
              </Select>
            </Row>
          ) : null}
          {form.config.image_upload_enabled ? (
            <Row label={t('agentEditor.imageUpload.imageUnderstandingLabel')} desc={t('agentEditor.imageUpload.imageUnderstandingDesc')}>
              <Switch field="attachment_image_understanding" label={t('agentEditor.imageUpload.imageUnderstandingLabel')} checked={form.config.attachment_image_understanding}
                onChange={(next) => patchConfig('attachment_image_understanding', next)} />
            </Row>
          ) : null}
          {form.config.image_upload_enabled && form.config.attachment_image_understanding ? (
            <Row label={t('agentEditor.imageUpload.ocrMaxPagesLabel')} desc={t('agentEditor.imageUpload.ocrMaxPagesDesc') + '（' + t('agentEditor.imageUpload.useGlobalDefault') + '）'}>
              <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={0} max={64} step={1}
                data-field="attachment_ocr_max_pages"
                value={form.config.attachment_ocr_max_pages}
                onChange={(event) => patchConfig('attachment_ocr_max_pages', Number(event.target.value) || 0)} />
            </Row>
          ) : null}
          {form.config.image_upload_enabled ? (
            <Row label={t('agentEditor.imageUpload.storageProvider')} desc={t('agentEditor.imageUpload.storageProviderDesc')}>
              <Select data-field="image_storage_provider" className={`wk-ae-select ${FIELD_SELECT}`} value={form.config.image_storage_provider}
                onChange={(event) => patchConfig('image_storage_provider', event.target.value)}>
                <option value="">{t('agentEditor.imageUpload.storageDefault')}</option>
                {storageOptions.map((option) => (
                  <option key={option.value} value={option.value} disabled={option.disabled}>
                    {option.label}{option.disabled ? ' · ' + t('agentEditor.imageUpload.notConfigured') : ''}
                  </option>
                ))}
              </Select>
              <button type="button" data-go-storage-settings
                className="mt-1 cursor-pointer border-0 bg-transparent p-0 text-[12px] text-[var(--td-brand-color,#0052d9)] hover:underline"
                onClick={() => navigate('/platform/settings?section=storage')}>
                {t('agentEditor.imageUpload.goStorageSettings')}
              </button>
            </Row>
          ) : null}
          <Row label={t('agentEditor.audioUpload.label')} desc={t('agentEditor.audioUpload.desc')}>
            <Switch field="audio_upload_enabled" label={t('agentEditor.audioUpload.label')} checked={form.config.audio_upload_enabled}
              onChange={(next) => patchConfig('audio_upload_enabled', next)} />
          </Row>
          {form.config.audio_upload_enabled ? (
            <Row label={t('agentEditor.audioUpload.asrModel')} desc={t('agentEditor.audioUpload.asrModelDesc')}>
              <Select data-field="asr_model_id" className={`wk-ae-select ${FIELD_SELECT}`} value={form.config.asr_model_id}
                onChange={(event) => patchConfig('asr_model_id', event.target.value)}>
                <option value="">{t('agentEditor.audioUpload.asrModelPlaceholder')}</option>
                {asrModels.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
              </Select>
            </Row>
          ) : null}
          <Row label={t('agentEditor.chatParser.waitTimeoutLabel')} desc={t('agentEditor.chatParser.waitTimeoutDesc')}>
            <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={0} max={600} step={10}
              data-field="attachment_parse_wait_timeout_sec"
              value={form.config.attachment_parse_wait_timeout_sec}
              onChange={(event) => patchConfig('attachment_parse_wait_timeout_sec', Number(event.target.value) || 0)} />
          </Row>
          {/* R486 — Vue 869-878 <KBParserSettings embedded>: per-file-type
              parser engine rows for chat attachments (CHAT_PARSER_EXTENSIONS
              families only), reusing the knowledge-settings rule math. */}
          <div className="rounded-lg border border-[var(--td-component-stroke,#e7e7e7)] p-3" data-parser-policy-block>
            <p className="m-0 text-[13px] font-medium">{t('agentEditor.chatParser.label')}</p>
            <p className="m-0 mt-1 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agentEditor.chatParser.desc')}</p>
            <div className="mt-2">
              <AgentParserRules
                engines={deps.parserEngines}
                rules={Array.isArray(form.config.chat_parser_engine_rules) ? form.config.chat_parser_engine_rules as ParserEngineRule[] : []}
                onChange={(rules) => patchConfig('chat_parser_engine_rules', rules)}
                t={t}
              />
            </div>
          </div>
        </div>
      </section>
    );
  }

  function renderMcp() {
    const serviceRows = mcpOptionRows(deps.mcpServices, form.config.mcp_services, t);
    const showServiceSelect = serviceRows.length > 0 || form.config.mcp_services.length > 0;
    return (
      <section className="wk-ae-section" data-editor-section="mcp">
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agentEditor.mcp.label')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agentEditor.mcp.desc')}</p>
        </header>
        <div className="flex flex-col gap-[18px]">
          <Row label={t('agentEditor.mcp.label')} desc={t('agentEditor.mcp.desc')}>
            <RadioGroup
              name="mcp-mode"
              value={mcpMode}
              options={[
                { value: 'all', label: t('agentEditor.selection.all') },
                { value: 'selected', label: t('agentEditor.selection.selected') },
                { value: 'none', label: t('agentEditor.selection.disabled') },
              ]}
              onChange={(value) => onScopeModeChange('mcp', value as ScopeSelectionMode)}
            />
          </Row>
          {mcpMode === 'selected' && showServiceSelect ? (
            <Row label={t('agentEditor.mcp.selectLabel')} desc={t('agentEditor.mcp.selectDesc')}>
              <div className="flex w-full max-w-[460px] flex-col gap-1" data-field="mcp_services">
                {serviceRows.map((row) => (
                  <label key={row.value} className={`flex items-center gap-2 rounded-md border border-[var(--td-component-stroke,#e7e7e7)] px-2 py-1.5 text-[13px] ${row.disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'}`}>
                    <Checkbox
                      data-mcp-service={row.value}
                      checked={form.config.mcp_services.includes(row.value)}
                      disabled={row.disabled}
                      onChange={(event) => patch((draft) => {
                        const set = new Set(draft.config.mcp_services);
                        if (event.target.checked) set.add(row.value); else set.delete(row.value);
                        draft.config.mcp_services = [...set];
                      })}
                    />
                    <span>{row.label}</span>
                  </label>
                ))}
              </div>
            </Row>
          ) : null}
          {mcpMode !== 'none' ? (
            <Row label={t('agentEditor.mcp.authWaitTimeout')} desc={t('agentEditor.mcp.authWaitTimeoutDesc')}>
              <Input type="number" className={`wk-ae-input ${FIELD_NUMBER}`} min={5} max={3600}
                data-field="mcp_auth_wait_timeout"
                value={form.config.mcp_auth_wait_timeout}
                placeholder={t('agentEditor.mcp.authWaitTimeoutPlaceholder')}
                onChange={(event) => patchConfig('mcp_auth_wait_timeout', Number(event.target.value) || 0)} />
            </Row>
          ) : null}
        </div>
      </section>
    );
  }

  function renderSkills() {
    const readyRows = skillRows.filter((row) => row.selectable);
    const pendingRows = skillRows.filter((row) => !row.selectable);
    return (
      <section className="wk-ae-section" data-editor-section="skills">
        <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
          <h2>{t('agent.editor.skillsConfig')}</h2>
          <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.skillsConfigDesc')}</p>
        </header>
        <div className="flex flex-col gap-[18px]">
          <Row label={t('agent.editor.sandboxBackend')} desc={t('agent.editor.sandboxBackendHint')}>
            <Select data-field="sandbox_config_id" className={`wk-ae-select ${FIELD_SELECT}`} value={form.config.sandbox_config_id}
              onChange={(event) => patchConfig('sandbox_config_id', event.target.value)}>
              <option value="">{t('agent.editor.sandboxBackendDefault')}</option>
              {sandboxOptions.map((cfg) => <option key={cfg.id} value={cfg.id}>{cfg.name}</option>)}
            </Select>
            {/* R484 D11 — Vue AgentEditorModal.vue:1323-1331 sandbox-select-links:
                管理沙箱 jumps to the settings sandbox section (uiStore.openSettings('sandbox')). */}
            <button type="button" data-go-sandbox-settings
              className="mt-1 cursor-pointer border-0 bg-transparent p-0 text-[12px] text-[var(--td-brand-color,#0052d9)] hover:underline"
              onClick={() => navigate('/platform/settings?section=sandbox')}>
              {t('agent.editor.goSandboxSettings')}
            </button>
            {sandboxOptions.length === 0 ? <p className="m-0 mt-1 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.sandboxNoConfigs')}</p> : null}
          </Row>
          <Row label={t('agent.editor.skillsSelection')} desc={t('agent.editor.skillsSelectionDesc')}>
            <RadioGroup
              name="skills-mode"
              value={skillsMode}
              options={[
                { value: 'all', label: t('agent.editor.skillsAll'), disabled: !hasSandbox && namedSandboxes.length !== 1 },
                { value: 'selected', label: t('agent.editor.skillsSelected'), disabled: !hasSandbox && namedSandboxes.length !== 1 },
                { value: 'none', label: t('agent.editor.skillsNone') },
              ]}
              onChange={(value) => onScopeModeChange('skills', value as ScopeSelectionMode)}
            />
            {!hasSandbox ? <p className="m-0 mt-1 text-[12px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">{t('agent.editor.skillsNeedSandbox')}</p> : null}
          </Row>
          {skillsMode !== 'none' && hasSandbox && skillRows.length > 0 ? (
            <div className="flex w-full flex-col gap-1">
              {readyRows.length > 0 ? (
                <div className="wk-ae-skill-group">
                  <p className="mb-[2px] mt-[6px] text-[12px] font-semibold text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.skillsGroupAvailable')}</p>
                  {readyRows.map(renderSkillRow)}
                </div>
              ) : null}
              {pendingRows.length > 0 ? (
                <div className="wk-ae-skill-group">
                  <p className="mb-[2px] mt-[6px] text-[12px] font-semibold text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agent.editor.skillsGroupUnavailable')}</p>
                  {pendingRows.map(renderSkillRow)}
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      </section>
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
    const checked = form.config.selected_skills.includes(row.name);
    const selectable = row.selectable;
    const busy = row.installStatus === 'installing' || row.installStatus === 'removing';
    const canInstall = !busy && (!row.installed || row.installStatus === 'failed')
      && hasSandbox && namedSandboxes.some((cfg) => cfg.id === form.config.sandbox_config_id);
    return (
      <div key={row.id} className={`flex items-center gap-2 rounded-md border border-[var(--td-component-stroke,#e7e7e7)] px-2 py-1.5 text-[13px]${selectable ? '' : ' opacity-70'}`} data-skill-row={row.id}>
        {skillsMode === 'selected' ? (
          <Checkbox
            data-skill-id={row.id}
            checked={checked}
            disabled={!selectable}
            onChange={(event) => {
              const next = event.target.checked;
              patch((draft) => {
                const set = new Set(draft.config.selected_skills);
                if (next) set.add(row.name); else set.delete(row.name);
                draft.config.selected_skills = [...set];
              });
            }}
          />
        ) : null}
        <span className="font-medium">{row.name}</span>
        {row.description ? <span className="text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{row.description}</span> : null}
        {!selectable ? (
          <span className="ml-auto text-[12px] text-[var(--td-warning-color,#e37318)]">{row.installed ? row.installStatus : t('agent.editor.skillNotInstalled')}</span>
        ) : null}
        {canInstall ? (
          <button type="button" className="cursor-pointer rounded-md border border-[var(--td-brand-color,#0052d9)] bg-[var(--td-bg-color-container,#fff)] px-2.5 py-[2px] text-[12px] text-[var(--td-brand-color,#0052d9)] hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)]" disabled={installingId !== ''}
            onClick={() => void installSkill(row.id)}>
            {installingId === row.id ? t('common.loading') : t('agent.editor.installShort')}
          </button>
        ) : null}
      </div>
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

  return (
    <div className="fixed inset-0 z-[1000] flex items-center justify-center bg-[rgba(0,0,0,0.5)] backdrop-blur-[4px]" data-editor-overlay onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <div className="relative flex h-[85vh] max-h-[780px] w-[90vw] max-w-[1100px] flex-col overflow-hidden rounded-xl bg-[var(--td-bg-color-container,#fff)] text-[var(--td-text-color-primary,rgba(0,0,0,0.9))] shadow-[0_8px_32px_rgba(0,0,0,0.12)]" role="dialog" aria-modal="true" aria-label={editorMode === 'create' ? t('agent.editor.createTitle') : t('agent.editor.editTitle')} data-testid="agent-editor-modal">
        <button type="button" className="absolute top-3 right-3 z-10 h-8 w-8 cursor-pointer rounded-md border-none bg-transparent text-[18px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))] hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)]" aria-label={t('common.close')} onClick={onClose}>×</button>
        {initializing ? <div className="absolute inset-0 z-20 flex items-center justify-center bg-[var(--td-bg-color-container,#fff)]" role="status">{t('common.loading')}</div> : null}
        <div className="flex h-full w-full overflow-hidden">
          <aside className="flex w-[208px] shrink-0 flex-col overflow-hidden border-r border-[var(--td-component-stroke,#e7e7e7)] bg-[var(--td-bg-color-settings-modal,#fafafa)]">
            <h2 className="m-0 border-b border-[var(--td-component-stroke,#e7e7e7)] px-3.5 pt-4 pb-3 text-[16px] font-semibold" data-editor-title>
              {editorMode === 'create' ? t('agent.editor.createTitle') : t('agent.editor.editTitle')}
            </h2>
            <nav className="flex-1 overflow-y-auto p-2" data-guide="agent-editor-sidebar">
              {navGroups.map((group) => (
                <div key={group.key}>
                  <p className="mx-1.5 mb-[2px] mt-[6px] text-[12px] font-semibold text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">{t(group.labelKey)}</p>
                  {group.items.map((item) => (
                    <button
                      key={item.key}
                      type="button"
                      className={`mb-[2px] flex w-full cursor-pointer items-center rounded-md border-none bg-transparent px-3 py-[7px] text-left text-[14px] hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)] ${section === item.key ? 'bg-[var(--td-bg-color-secondarycontainer,#f2f3f5)] font-medium text-[var(--td-brand-color,#0052d9)]' : 'text-[var(--td-text-color-primary,rgba(0,0,0,0.9))]'}`}
                      data-section-key={item.key}
                      onClick={() => setSection(item.key)}
                    >
                      <span>{t(item.labelKey)}</span>
                    </button>
                  ))}
                </div>
              ))}
            </nav>
          </aside>
          <div className="flex min-w-0 flex-1 flex-col">
            <div className="flex-1 overflow-y-auto px-6 py-5">
              {renderSection()}
              {editorMode === 'edit' && form.id && !readOnly ? (
                <div className="mt-8 border-t border-[var(--td-component-stroke,#e7e7e7)] pt-5">
                  <AgentVersionActions api={marketplaceApi} agentId={form.id} agentName={form.name} />
                </div>
              ) : null}
            </div>
            <footer className="flex flex-col gap-1.5 border-t border-[var(--td-component-stroke,#e7e7e7)] px-6 py-2.5">
              {/* R486 KB warn — Vue MessagePlugin.warning(kbIncompatibleWarn, 4000);
                  React has no global toast host here, so the warning rides the
                  modal footer as a polite status region (shell share-toast form). */}
              {kbWarn ? (
                <p className="m-0 text-[12px] text-[var(--td-warning-color,#e37318)]" role="status" data-agent-type-warn>{kbWarn}</p>
              ) : null}
              {postCreate ? (
                <p className="m-0 text-[12px] text-[var(--td-success-color,#2ba471)]" data-post-create>
                  <strong>{t('agent.editor.postCreateHint.title')}</strong>
                  {t('agent.editor.postCreateHint.footer')}
                </p>
              ) : null}
              {saveError ? <p className="m-0 mt-0.5 text-[12px] text-[var(--td-error-color,#d54941)]" role="alert">{saveError}</p> : null}
              <div className="flex justify-end gap-2">
                <button type="button" className={`${AE_BTN} border border-[var(--td-component-stroke,#dcdcdc)] bg-[var(--td-bg-color-container,#fff)] text-inherit hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)]`} data-editor-cancel onClick={onClose}>{t('common.cancel')}</button>
                {!readOnly ? <button
                  type="button"
                  className={`${AE_BTN} border bg-[var(--td-brand-color,#0052d9)] border-[var(--td-brand-color,#0052d9)] text-white hover:bg-[var(--td-brand-color-hover,#266fe8)] disabled:cursor-not-allowed disabled:opacity-60`}
                  data-editor-save
                  disabled={readOnly || saving || initializing}
                  onClick={() => void handleSave()}
                >{saving ? t('common.loading') : saveLabel}</button> : null}
              </div>
            </footer>
          </div>
        </div>
      </div>
    </div>
  );
}
