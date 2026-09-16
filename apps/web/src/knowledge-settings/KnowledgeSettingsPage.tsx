import { useEffect, useMemo, useState } from 'react';
import type { ElementType } from 'react';
import type { KnowledgeBase } from '@weknora/contracts';
import type { WeKnoraClient } from '@weknora/api-client';
import { GraphSettings, type GraphExtractConfig } from './GraphSettings.tsx';
import { DataSourcesPage } from '../data-sources/DataSourcesPage.tsx';
import { KnowledgeBaseShareDialog } from '../knowledge-bases/KnowledgeBaseShareDialog.tsx';
import { KnowledgeBaseActivityPanel } from '../knowledge-bases/KnowledgeBaseActivityPanel.tsx';
import { createTranslator, useAppLocale } from '../i18n.ts';

type ProjectUi = typeof import('@weknora/ui');

export interface KnowledgeEditorOptions {
  parserEngines: Array<{ Name: string; Description: string; Available?: boolean }>;
  storageBackends: Array<{ id: string; name: string; provider: string; status: string }>;
  vectorStores: Array<{ id: string; name: string; engine_type: string; source: string; readonly: boolean }>;
  loading: boolean;
  error: string | null;
}

const idleEditorOptions: KnowledgeEditorOptions = { parserEngines: [], storageBackends: [], vectorStores: [], loading: false, error: null };

// Loads the live parser/vector/storage catalogues through the authenticated
// settings API when the settings surface opens (Vue editorResources contract).
// Each endpoint degrades independently so one failing catalogue cannot blank
// the other two.
export async function loadKnowledgeSettingsOptions(client: WeKnoraClient): Promise<Omit<KnowledgeEditorOptions, 'loading'>> {
  const [parser, storage, vector] = await Promise.allSettled([
    Promise.resolve().then(() => client.knowledgeBases.settings.parserEngines()),
    Promise.resolve().then(() => client.knowledgeBases.settings.storageBackends()),
    Promise.resolve().then(() => client.knowledgeBases.settings.vectorStores()),
  ]);
  const failures = [parser, storage, vector].filter((outcome) => outcome.status === 'rejected') as Array<PromiseRejectedResult>;
  return {
    parserEngines: parser.status === 'fulfilled' ? parser.value.data.map((item) => ({ Name: item.Name, Description: item.Description, ...(item.Available === undefined ? {} : { Available: item.Available }) })) : [],
    storageBackends: storage.status === 'fulfilled' ? storage.value.data.map((item) => ({ id: item.id, name: item.name, provider: item.provider, status: item.status })) : [],
    vectorStores: vector.status === 'fulfilled' ? vector.value.data.map((item) => ({ id: item.id, name: item.name, engine_type: item.engine_type, source: item.source, readonly: item.readonly })) : [],
    error: failures.length > 0 ? (failures[0]!.reason instanceof Error ? failures[0]!.reason.message : 'Unable to load settings options') : null,
  };
}

export type KnowledgeSettingsSectionKey = 'vectorStore' | 'parser' | 'storage' | 'datasource' | 'share' | 'activity' | 'graph';

export type KnowledgeSettingsInput = KnowledgeBase & {
  type?: string;
  chunking_config?: {
    parser_engine_rules?: Array<Record<string, unknown>>;
    chunk_size?: number;
    chunk_overlap?: number;
    separators?: string[];
    enable_parent_child?: boolean;
    parent_chunk_size?: number;
    child_chunk_size?: number;
    strategy?: string;
    token_limit?: number;
    languages?: string[];
    table_metadata_instructions?: string;
    [key: string]: unknown;
  };
  vector_store_id?: string | null;
  vector_store_source?: string;
  vector_store_name?: string;
  vector_store_engine_type?: string;
  vector_store_status?: string;
  storage_backend_id?: string | null;
  storage_provider_config?: { provider?: string };
  storage_config?: { provider?: string };
  activity?: Array<Record<string, unknown>>;
  activity_count?: number;
  data_source_count?: number;
  share_count?: number;
  summary_model_id?: string;
  extract_config?: Partial<GraphExtractConfig> & { custom_instructions?: string };
};

export interface KnowledgeSettingsSection {
  key: KnowledgeSettingsSectionKey;
  label: string;
  description: string;
}

export interface KnowledgeSettingsSummary {
  parser: SettingSummary;
  vectorStore: SettingSummary;
  storage: SettingSummary;
  activity: SettingSummary;
  datasource: SettingSummary;
  share: SettingSummary;
  graph: SettingSummary;
}

interface SettingSummary {
  kind: 'configured' | 'ready' | 'unavailable' | 'default' | 'empty' | 'available';
  label: string;
  detail: string;
}

const sections: KnowledgeSettingsSection[] = [
  { key: 'vectorStore', label: 'Vector store', description: 'Bound retrieval engine and health' },
  { key: 'parser', label: 'Parser', description: 'File-type parser rules' },
  { key: 'storage', label: 'Storage', description: 'Files and document instance' },
  { key: 'datasource', label: 'Data sources', description: 'External connectors and sync status' },
  { key: 'share', label: 'Share', description: 'Spaces with access to this knowledge base' },
  { key: 'activity', label: 'Activity', description: 'Recent configuration changes' },
  { key: 'graph', label: 'Knowledge graph', description: 'Entity and relationship extraction' },
];

function isFaqKnowledgeBase(knowledgeBase: KnowledgeSettingsInput): boolean {
  return knowledgeBase.type?.toLowerCase() === 'faq';
}

function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

function titleCase(value: string): string {
  return value
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');
}

function parserLabel(value: string): string {
  const knownLabels: Record<string, string> = { mineru: 'MinerU', builtin: 'Built-in', opendataloader: 'OpenDataLoader' };
  return knownLabels[value.toLowerCase()] ?? titleCase(value);
}

function parserRules(knowledgeBase: KnowledgeSettingsInput): Array<Record<string, unknown>> {
  return Array.isArray(knowledgeBase.chunking_config?.parser_engine_rules)
    ? knowledgeBase.chunking_config.parser_engine_rules
    : [];
}

export function getKnowledgeSettingsSections(
  knowledgeBase: KnowledgeSettingsInput,
  options: { canViewActivity?: boolean } = {},
): KnowledgeSettingsSection[] {
  const canViewActivity = options.canViewActivity ?? true;
  return sections.filter((section) => {
    if (section.key === 'parser' || section.key === 'storage' || section.key === 'graph') return !isFaqKnowledgeBase(knowledgeBase);
    if (section.key === 'activity') return canViewActivity;
    return true;
  });
}

export function summarizeKnowledgeSettings(knowledgeBase: KnowledgeSettingsInput): KnowledgeSettingsSummary {
  const rules = parserRules(knowledgeBase);
  const engines = [...new Set(rules.map((rule) => text(rule.engine ?? rule.parser)).filter(Boolean))];
  const extensions = [...new Set(rules.flatMap((rule) => {
    const values = rule.file_types ?? rule.fileTypes;
    return Array.isArray(values) ? values.map(text).filter(Boolean) : [];
  }))];
  const vectorName = text(knowledgeBase.vector_store_name);
  const vectorEngine = text(knowledgeBase.vector_store_engine_type);
  const vectorSource = text(knowledgeBase.vector_store_source);
  const vectorUnavailable = knowledgeBase.vector_store_status === 'unavailable';
  const vectorBound = Boolean(vectorName || knowledgeBase.vector_store_id || vectorSource);
  const storageProvider = text(knowledgeBase.storage_provider_config?.provider)
    || text(knowledgeBase.storage_config?.provider);
  const storageId = text(knowledgeBase.storage_backend_id);
  const activities = Array.isArray(knowledgeBase.activity) ? knowledgeBase.activity : [];
  const activityCount = typeof knowledgeBase.activity_count === 'number' ? knowledgeBase.activity_count : activities.length;
  const latestActivity = activities[0];
  const latestAction = text(latestActivity?.action);
  const latestOutcome = text(latestActivity?.outcome);
  const dataSourceCount = typeof knowledgeBase.data_source_count === 'number' ? knowledgeBase.data_source_count : 0;
  const shareCount = typeof knowledgeBase.share_count === 'number' ? knowledgeBase.share_count : 0;
  const graphEnabled = knowledgeBase.extract_config?.enabled === true;

  return {
    parser: rules.length > 0
      ? { kind: 'configured', label: engines.length > 0 ? engines.map(parserLabel).join(', ') : 'Custom rules', detail: extensions.length > 0 ? extensions.map((value) => value.toUpperCase()).join(', ') : 'File-type overrides' }
      : { kind: 'empty', label: 'Default parser', detail: 'No file-type overrides' },
    vectorStore: vectorUnavailable
      ? { kind: 'unavailable', label: vectorName || 'Vector store unavailable', detail: 'Check the global vector-store settings' }
      : vectorBound
        ? { kind: 'ready', label: vectorName || 'Bound vector store', detail: [vectorEngine, vectorSource].filter(Boolean).join(' · ') || 'Explicit binding' }
        : { kind: 'default', label: 'System default', detail: 'No explicit binding' },
    storage: storageId || storageProvider
      ? { kind: 'configured', label: storageProvider ? titleCase(storageProvider) : 'Storage instance', detail: storageId || 'Provider configured' }
      : { kind: 'default', label: 'System default', detail: 'No explicit instance' },
    activity: activityCount > 0
      ? { kind: 'available', label: `${activityCount} recent ${activityCount === 1 ? 'event' : 'events'}`, detail: [latestAction, latestOutcome].filter(Boolean).join(' · ') || 'Open to inspect changes' }
      : { kind: 'empty', label: 'No activity yet', detail: 'Changes will appear here' },
    datasource: dataSourceCount > 0
      ? { kind: 'available', label: `${dataSourceCount} data source${dataSourceCount === 1 ? '' : 's'}`, detail: 'Open to inspect sync status' }
      : { kind: 'empty', label: 'No data sources', detail: 'Add an external connector' },
    share: shareCount > 0
      ? { kind: 'available', label: `${shareCount} shared space${shareCount === 1 ? '' : 's'}`, detail: 'Access is managed per share' }
      : { kind: 'empty', label: 'Not shared', detail: 'No spaces have access' },
    graph: graphEnabled
      ? { kind: 'configured', label: 'Knowledge graph enabled', detail: 'Entity and relationship extraction' }
      : { kind: 'default', label: 'Knowledge graph disabled', detail: 'Configure extraction when graph storage is enabled' },
  };
}

export function getKnowledgeBaseActivityPath(knowledgeBaseId: string): string {
  return `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/activity?limit=30`;
}

export function getKnowledgeBaseDataSourcesPath(knowledgeBaseId: string): string {
  return `/api/v1/datasource?kb_id=${encodeURIComponent(knowledgeBaseId)}`;
}

export function getKnowledgeBaseSharesPath(knowledgeBaseId: string): string {
  return `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/shares`;
}

// Vue editor contract: config updates go through PUT /initialization/config/:kbId
// (frontend updateKBConfig), which persists parser rules, chunking, storage and
// the extraction/question-generation blocks in one authoritative call.
export function getKnowledgeBaseConfigPath(knowledgeBaseId: string): string {
  return `/api/v1/initialization/config/${encodeURIComponent(knowledgeBaseId)}`;
}

export interface KnowledgeSettingsSavePayload {
  llmModelId: string;
  embeddingModelId: string;
  vlm_config: { enabled: boolean; model_id: string; description_language: string; custom_instructions: string };
  asr_config: { enabled: boolean; model_id: string; language: string };
  documentSplitting: {
    chunkSize: number;
    chunkOverlap: number;
    separators: string[];
    parserEngineRules: Array<Record<string, unknown>>;
    enableParentChild: boolean;
    parentChunkSize: number;
    childChunkSize: number;
    strategy: string;
    tokenLimit: number;
    languages: string[];
    tableMetadataInstructions: string;
  };
  multimodal: { enabled: boolean };
  storageBackendId: string;
  storageProvider: string;
  nodeExtract: { enabled: boolean; text: string; tags: string[]; nodes: unknown[]; relations: unknown[]; customInstructions: string };
  questionGeneration: { enabled: boolean; questionCount: number; customInstructions: string };
}

interface GraphExtractInput {
  enabled: boolean;
  text?: string;
  tags?: string[];
  nodes?: unknown[];
  relations?: unknown[];
  customInstructions?: string;
}

// Builds the exact KBModelConfigRequest body the Vue KnowledgeBaseEditorModal
// sends on update: the loaded KB config round-trips unchanged while the given
// parser-engine rules (the only editable control on this surface) replace
// chunking_config.parser_engine_rules. The vector-store binding is intentionally
// absent — Vue only ever sends vector_store_id on create, the binding is
// immutable afterwards.
export function buildKnowledgeSettingsConfigPayload(
  knowledgeBase: KnowledgeSettingsInput,
  parserEngineRules: Array<Record<string, unknown>>,
  nodeExtract?: GraphExtractInput,
): KnowledgeSettingsSavePayload {
  const text = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');
  const record = (value: unknown): Record<string, unknown> => (value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {});
  const chunking = record(knowledgeBase.chunking_config);
  const vlm = record(knowledgeBase.vlm_config);
  const asr = record(knowledgeBase.asr_config);
  const questionGeneration = record(knowledgeBase.question_generation_config);
  const extract = record(knowledgeBase.extract_config);
  const vlmEnabled = vlm.enabled === true;
  const extractSource = nodeExtract ?? {
    enabled: extract.enabled === true,
    text: text(extract.text),
    tags: Array.isArray(extract.tags) ? extract.tags : [],
    nodes: Array.isArray(extract.nodes) ? extract.nodes : [],
    relations: Array.isArray(extract.relations) ? extract.relations : [],
    customInstructions: text(extract.custom_instructions ?? extract.customInstructions),
  };
  const storageProvider = text(knowledgeBase.storage_provider_config && record(knowledgeBase.storage_provider_config).provider)
    || text(knowledgeBase.storage_config && record(knowledgeBase.storage_config).provider)
    || 'local';
  return {
    llmModelId: text(knowledgeBase.summary_model_id),
    embeddingModelId: text(knowledgeBase.embedding_model_id),
    vlm_config: {
      enabled: vlmEnabled,
      model_id: vlmEnabled ? text(vlm.model_id) : '',
      description_language: text(vlm.description_language),
      custom_instructions: text(vlm.custom_instructions),
    },
    asr_config: {
      enabled: asr.enabled === true,
      model_id: asr.enabled === true ? text(asr.model_id) : '',
      language: text(asr.language),
    },
    documentSplitting: {
      chunkSize: typeof chunking.chunk_size === 'number' && chunking.chunk_size > 0 ? chunking.chunk_size : 512,
      chunkOverlap: typeof chunking.chunk_overlap === 'number' ? chunking.chunk_overlap : 80,
      separators: Array.isArray(chunking.separators) && chunking.separators.length > 0 ? chunking.separators.map(String) : ['\n\n', '\n', '。', '！', '？', ';', '；'],
      parserEngineRules,
      enableParentChild: chunking.enable_parent_child === true,
      parentChunkSize: typeof chunking.parent_chunk_size === 'number' && chunking.parent_chunk_size > 0 ? chunking.parent_chunk_size : 4096,
      childChunkSize: typeof chunking.child_chunk_size === 'number' && chunking.child_chunk_size > 0 ? chunking.child_chunk_size : 384,
      strategy: text(chunking.strategy),
      tokenLimit: typeof chunking.token_limit === 'number' ? chunking.token_limit : 0,
      languages: Array.isArray(chunking.languages) ? chunking.languages.map(String) : [],
      tableMetadataInstructions: text(chunking.table_metadata_instructions),
    },
    multimodal: { enabled: vlmEnabled },
    storageBackendId: text(knowledgeBase.storage_backend_id),
    storageProvider,
    nodeExtract: {
      enabled: extractSource.enabled === true,
      text: text(extractSource.text),
      tags: Array.isArray(extractSource.tags) ? extractSource.tags : [],
      nodes: Array.isArray(extractSource.nodes) ? extractSource.nodes : [],
      relations: Array.isArray(extractSource.relations) ? extractSource.relations : [],
      customInstructions: text(extractSource.customInstructions),
    },
    questionGeneration: {
      enabled: questionGeneration.enabled === true,
      questionCount: typeof questionGeneration.question_count === 'number' && questionGeneration.question_count > 0 ? questionGeneration.question_count : 3,
      customInstructions: text(questionGeneration.custom_instructions),
    },
  };
}

// Sends the update through the authenticated client transport. The
// /initialization/config endpoint lives behind client.request because the
// shared api-client does not expose it yet (see report: api-client contract).
export async function saveKnowledgeSettings(client: WeKnoraClient, knowledgeBaseId: string, payload: KnowledgeSettingsSavePayload): Promise<void> {
  await client.request({ method: 'PUT', path: getKnowledgeBaseConfigPath(knowledgeBaseId), body: payload });
}

function summaryTone(summary: SettingSummary): 'neutral' | 'error' | 'success' {
  if (summary.kind === 'unavailable') return 'error';
  if (summary.kind === 'ready' || summary.kind === 'configured' || summary.kind === 'available') return 'success';
  return 'neutral';
}

interface KnowledgeSettingsPageProps {
  knowledgeBase?: KnowledgeSettingsInput;
  knowledgeBaseId?: string;
  client?: WeKnoraClient;
  role?: 'owner' | 'admin' | 'viewer';
  canViewActivity?: boolean;
  initialSection?: KnowledgeSettingsSectionKey;
}

export function knowledgeSettingsCanEdit(role: 'owner' | 'admin' | 'viewer' | undefined): boolean {
  return role === 'owner' || role === 'admin';
}

export function KnowledgeSettingsPage({ knowledgeBase: providedKnowledgeBase, knowledgeBaseId, client, role = 'viewer', canViewActivity = true, initialSection }: KnowledgeSettingsPageProps) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const [ui, setUi] = useState<ProjectUi | null>(null);
  const [loadedKnowledgeBase, setLoadedKnowledgeBase] = useState<KnowledgeSettingsInput | null>(providedKnowledgeBase ?? null);
  const [loadState, setLoadState] = useState<'idle' | 'loading' | 'error'>('idle');
  const [editorOptions, setEditorOptions] = useState<KnowledgeEditorOptions>(idleEditorOptions);
  const [pendingParserEngine, setPendingParserEngine] = useState('');
  const knowledgeBase = providedKnowledgeBase ?? loadedKnowledgeBase;
  useEffect(() => {
    if (providedKnowledgeBase || !knowledgeBaseId || !client) return;
    setLoadState('loading');
    void client.knowledgeBases.settings.get(knowledgeBaseId).then((value) => {
      setLoadedKnowledgeBase(value as KnowledgeSettingsInput);
      setLoadState('idle');
    }).catch(() => setLoadState('error'));
  }, [client, knowledgeBaseId, providedKnowledgeBase]);
  // Vue loads the parser/vector/storage catalogues through editorResources
  // when the settings surface opens; mirror that live loading here.
  useEffect(() => {
    if (!client) return;
    let mounted = true;
    setEditorOptions({ ...idleEditorOptions, loading: true });
    void loadKnowledgeSettingsOptions(client).then((options) => {
      if (mounted) setEditorOptions({ ...options, loading: false });
    });
    return () => { mounted = false; };
  }, [client]);
  const fallbackKnowledgeBase: KnowledgeSettingsInput = { id: knowledgeBaseId ?? '', name: '', type: 'document' };
  const currentKnowledgeBase = knowledgeBase ?? fallbackKnowledgeBase;
  const availableSections = useMemo(() => getKnowledgeSettingsSections(currentKnowledgeBase, { canViewActivity }), [currentKnowledgeBase, canViewActivity]);
  const [activeSection, setActiveSection] = useState<KnowledgeSettingsSectionKey>(initialSection ?? availableSections[0]?.key ?? 'vectorStore');
  const [graphExtract, setGraphExtract] = useState<GraphExtractConfig>(() => ({
    enabled: currentKnowledgeBase.extract_config?.enabled === true,
    text: currentKnowledgeBase.extract_config?.text ?? '',
    tags: currentKnowledgeBase.extract_config?.tags ?? [],
    nodes: currentKnowledgeBase.extract_config?.nodes ?? [],
    relations: currentKnowledgeBase.extract_config?.relations ?? [],
    customInstructions: currentKnowledgeBase.extract_config?.customInstructions ?? currentKnowledgeBase.extract_config?.custom_instructions ?? '',
  }));
  const parserRulesForSummary = pendingParserEngine
    ? [{ file_types: ['pdf'], engine: pendingParserEngine }]
    : currentKnowledgeBase.chunking_config?.parser_engine_rules;
  // Committed rules come back from the last successful save so the summary
  // reflects persisted state instead of the pre-save KB payload.
  const [savedParserRules, setSavedParserRules] = useState<Array<Record<string, unknown>> | null>(null);
  const effectiveParserRules = savedParserRules ?? parserRulesForSummary;
  const summary = summarizeKnowledgeSettings({ ...currentKnowledgeBase, chunking_config: { parser_engine_rules: effectiveParserRules } });
  // Vue editor save semantics: one in-flight save at a time (button :loading),
  // success toast, failure keeps the form intact and surfaces the message.
  const [saveState, setSaveState] = useState<{ status: 'idle' | 'saving' | 'saved' | 'error'; message: string }>({ status: 'idle', message: '' });
  const canManage = knowledgeSettingsCanEdit(role);
  const canSave = canManage && Boolean(client) && Boolean(currentKnowledgeBase.id);
  const handleSave = () => {
    if (!client || !currentKnowledgeBase.id || saveState.status === 'saving' || !canSave) return;
    const rules = (pendingParserEngine
      ? [{ file_types: ['pdf'], engine: pendingParserEngine }]
      : parserRules(currentKnowledgeBase)) as Array<Record<string, unknown>>;
    const payload = buildKnowledgeSettingsConfigPayload(currentKnowledgeBase, rules, graphExtract);
    setSaveState({ status: 'saving', message: '' });
    void saveKnowledgeSettings(client, currentKnowledgeBase.id, payload).then(() => {
      setSavedParserRules(rules);
      setPendingParserEngine('');
      setSaveState({ status: 'saved', message: t('knowledgeEditor.messages.updateSuccess') });
    }).catch((error: unknown) => {
      setSaveState({ status: 'error', message: error instanceof Error && error.message ? error.message : t('common.error') });
    });
  };
  const active = availableSections.find((section) => section.key === activeSection) ?? availableSections[0];

  useEffect(() => {
    void import('@weknora/ui').then(setUi);
  }, []);

  useEffect(() => {
    if (!availableSections.some((section) => section.key === activeSection)) setActiveSection(availableSections[0]?.key ?? 'vectorStore');
  }, [activeSection, availableSections]);

  const CardComponent = ui?.Card ?? 'section';
  const ButtonComponent = ui?.Button ?? 'button';
  const StatusComponent = ui?.Status ?? 'p';

  return (
    <CardComponent aria-label={`Knowledge settings for ${currentKnowledgeBase.name}`}>
      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(170px, 0.8fr) minmax(0, 2fr)', gap: '1.5rem', alignItems: 'start' }}>
        <aside aria-label="Knowledge settings navigation">
          <p className="wk-eyebrow">Knowledge settings</p>
          <h2 style={{ margin: '0.35rem 0 1.1rem', fontSize: '1.2rem' }}>{currentKnowledgeBase.name}</h2>
          <nav style={{ display: 'grid', gap: '0.4rem' }}>
            {availableSections.map((section) => {
              const item = summary[section.key];
              return (
                <ButtonComponent
                  key={section.key}
                  type="button"
                  aria-current={active?.key === section.key ? 'page' : undefined}
                  onClick={() => setActiveSection(section.key)}
                  style={{ textAlign: 'left', borderColor: active?.key === section.key ? '#2e6de6' : undefined, background: active?.key === section.key ? '#edf3ff' : '#fff' }}
                >
                  <span style={{ display: 'flex', justifyContent: 'space-between', gap: '0.5rem', fontWeight: 700 }}>
                    {section.label}
                    <span aria-hidden="true" style={{ color: item.kind === 'unavailable' ? '#b42318' : '#66758b' }}>•</span>
                  </span>
                  <span className="wk-muted" style={{ display: 'block', fontSize: '0.78rem' }}>{section.description}</span>
                </ButtonComponent>
              );
            })}
          </nav>
        </aside>

        <section aria-labelledby="knowledge-settings-section-title" style={{ minWidth: 0 }}>
          <p className="wk-eyebrow">{active?.label}</p>
          <h3 id="knowledge-settings-section-title" style={{ margin: '0.35rem 0 0.2rem', fontSize: '1.4rem' }}>{active?.label}</h3>
          <p className="wk-muted" style={{ margin: '0 0 1.25rem' }}>{active?.description}</p>
          {loadState === 'loading' ? <StatusComponent>Loading knowledge-base settings…</StatusComponent> : loadState === 'error' ? <StatusComponent tone="error">Unable to load knowledge-base settings.</StatusComponent> : active ? <SettingsSection summary={summary[active.key]} section={active.key} graphExtract={graphExtract} modelId={currentKnowledgeBase.summary_model_id ?? ''} client={client} knowledgeBaseId={currentKnowledgeBase.id} knowledgeBaseName={currentKnowledgeBase.name} canManage={knowledgeSettingsCanEdit(role)} editorOptions={editorOptions} pendingParserEngine={pendingParserEngine} configuredParserEngine={parserRules(currentKnowledgeBase)[0] ? text(parserRules(currentKnowledgeBase)[0]!.engine ?? parserRules(currentKnowledgeBase)[0]!.parser) : ''} onPendingParserEngine={setPendingParserEngine} t={t} StatusComponent={StatusComponent} onGraphChange={setGraphExtract} /> : <StatusComponent>No settings available.</StatusComponent>}
          {canSave ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginTop: '1.25rem', borderTop: '1px solid #dce3ed', paddingTop: '1rem' }}>
              <ButtonComponent
                type="button"
                disabled={saveState.status === 'saving'}
                aria-busy={saveState.status === 'saving'}
                aria-label={t('knowledgeEditor.buttons.save')}
                onClick={handleSave}
              >
                {t('knowledgeEditor.buttons.save')}
              </ButtonComponent>
              {saveState.message ? (
                <span role="status" aria-live="polite" style={{ color: saveState.status === 'error' ? '#b42318' : '#067647', fontWeight: 600 }}>{saveState.message}</span>
              ) : null}
            </div>
          ) : null}
        </section>
      </div>
    </CardComponent>
  );
}

interface SettingsSectionProps {
  summary: SettingSummary;
  section: KnowledgeSettingsSectionKey;
  graphExtract: GraphExtractConfig;
  modelId: string;
  client?: WeKnoraClient;
  knowledgeBaseId: string;
  knowledgeBaseName: string;
  canManage: boolean;
  editorOptions: KnowledgeEditorOptions;
  pendingParserEngine: string;
  configuredParserEngine: string;
  onPendingParserEngine: (value: string) => void;
  t: (key: string) => string;
  StatusComponent: ElementType;
  onGraphChange: (value: GraphExtractConfig) => void;
}

function SettingsSection({ summary, section, graphExtract, modelId, client, knowledgeBaseId, knowledgeBaseName, canManage, editorOptions, pendingParserEngine, configuredParserEngine, onPendingParserEngine, t, StatusComponent, onGraphChange }: SettingsSectionProps) {
  return (
    <div style={{ display: 'grid', gap: '0.9rem' }}>
      <div style={{ border: '1px solid #dce3ed', borderRadius: 8, padding: '1rem' }}>
        <StatusComponent tone={summaryTone(summary)}>{summary.label}</StatusComponent>
        <p style={{ margin: '0.35rem 0 0', fontWeight: 600 }}>{summary.detail}</p>
      </div>
      {section === 'vectorStore' ? (
        <div style={{ display: 'grid', gap: '0.4rem' }}>
          <label style={{ display: 'grid', gap: '0.25rem' }}>
            {t('kbSettings.vectorStore.engineLabel')}
            <select value={summary.label} disabled aria-label={t('kbSettings.vectorStore.engineLabel')}>
              <option value={summary.label}>{summary.label}</option>
              {editorOptions.vectorStores.filter((store) => store.id && store.id !== '').map((store) => <option key={store.id} value={store.id}>{store.name} · {store.engine_type}</option>)}
            </select>
          </label>
          <p className="wk-muted" style={{ margin: 0 }}>{t('kbSettings.vectorStore.immutableHint')}</p>
          {editorOptions.error ? <StatusComponent tone="error">{editorOptions.error}</StatusComponent> : null}
        </div>
      ) : null}
      {section === 'parser' ? (
        <div style={{ display: 'grid', gap: '0.4rem' }}>
          <label style={{ display: 'grid', gap: '0.25rem' }}>
            {t('kbSettings.parser.title')}
            <select
              value={pendingParserEngine || configuredParserEngine}
              disabled={editorOptions.loading}
              aria-label={t('kbSettings.parser.title')}
              onChange={(event) => onPendingParserEngine(event.target.value)}
            >
              <option value="">{t('kbSettings.parser.default')}</option>
              {editorOptions.parserEngines.filter((engine) => engine.Available !== false).map((engine) => <option key={engine.Name} value={engine.Name}>{localizedEngineName(engine.Name, t)}</option>)}
            </select>
          </label>
          <p className="wk-muted" style={{ margin: 0 }}>{editorOptions.error ?? t('kbSettings.parser.goConfig')}</p>
        </div>
      ) : null}
      {section === 'storage' ? (
        <div style={{ display: 'grid', gap: '0.4rem' }}>
          <label style={{ display: 'grid', gap: '0.25rem' }}>
            {t('kbSettings.storage.instanceLabel')}
            <select value={summary.detail} disabled aria-label={t('kbSettings.storage.instanceLabel')}>
              <option value={summary.detail}>{summary.label}</option>
              {editorOptions.storageBackends.map((backend) => <option key={backend.id} value={backend.id}>{backend.name} · {backend.provider}</option>)}
            </select>
          </label>
          <p className="wk-muted" style={{ margin: 0 }}>{t('kbSettings.storage.migrateHint')}</p>
          {editorOptions.error ? <StatusComponent tone="error">{editorOptions.error}</StatusComponent> : null}
        </div>
      ) : null}
      {section === 'activity' ? (
        client && knowledgeBaseId
          ? <KnowledgeBaseActivityPanel client={client} knowledgeBaseId={knowledgeBaseId} />
          : <p className="wk-muted" style={{ margin: 0 }}>No recorded changes for this knowledge base.</p>
      ) : null}
      {section === 'datasource' ? (
        client && knowledgeBaseId
          ? <div style={{ maxHeight: '34rem', overflow: 'auto' }}><DataSourcesPage client={client} knowledgeBaseId={knowledgeBaseId} canManage={canManage} /></div>
          : <p className="wk-muted" style={{ margin: 0 }}>No data sources configured.</p>
      ) : null}
      {section === 'share' ? (
        client && knowledgeBaseId
          ? <KnowledgeBaseShareDialog client={client} knowledgeBaseId={knowledgeBaseId} knowledgeBaseName={knowledgeBaseName} open inline onClose={() => undefined} onChanged={() => undefined} />
          : <p className="wk-muted" style={{ margin: 0 }}>This knowledge base is not shared.</p>
      ) : null}
      {section === 'graph' ? <GraphSettings graphExtract={graphExtract} modelId={modelId} client={client} embedded onChange={onGraphChange} /> : null}
    </div>
  );
}

function localizedEngineName(name: string, t: (key: string) => string): string {
  const key = `kbSettings.parser.engines.${name}.name`;
  const translated = t(key);
  return translated === key ? name : translated;
}
