import { useEffect, useMemo, useState } from 'react';
import type { ElementType } from 'react';
import type { KnowledgeBase } from '@weknora/contracts';
import type { WeKnoraClient } from '@weknora/api-client';
import { GraphSettings, type GraphExtractConfig } from './GraphSettings.tsx';

type ProjectUi = typeof import('@weknora/ui');

export type KnowledgeSettingsSectionKey = 'vectorStore' | 'parser' | 'storage' | 'datasource' | 'share' | 'activity' | 'graph';

export type KnowledgeSettingsInput = KnowledgeBase & {
  type?: string;
  chunking_config?: {
    parser_engine_rules?: Array<Record<string, unknown>>;
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

function summaryTone(summary: SettingSummary): 'neutral' | 'error' | 'success' {
  if (summary.kind === 'unavailable') return 'error';
  if (summary.kind === 'ready' || summary.kind === 'configured' || summary.kind === 'available') return 'success';
  return 'neutral';
}

function activityRows(value: unknown): Array<Record<string, unknown>> {
  if (Array.isArray(value)) return value.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === 'object');
  if (value && typeof value === 'object' && Array.isArray((value as { data?: unknown }).data)) {
    return activityRows((value as { data: unknown }).data);
  }
  return [];
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
  const [ui, setUi] = useState<ProjectUi | null>(null);
  const [loadedKnowledgeBase, setLoadedKnowledgeBase] = useState<KnowledgeSettingsInput | null>(providedKnowledgeBase ?? null);
  const [loadState, setLoadState] = useState<'idle' | 'loading' | 'error'>('idle');
  const knowledgeBase = providedKnowledgeBase ?? loadedKnowledgeBase;
  useEffect(() => {
    if (providedKnowledgeBase || !knowledgeBaseId || !client) return;
    setLoadState('loading');
    void client.knowledgeBases.settings.get(knowledgeBaseId).then((value) => {
      setLoadedKnowledgeBase(value as KnowledgeSettingsInput);
      setLoadState('idle');
    }).catch(() => setLoadState('error'));
  }, [client, knowledgeBaseId, providedKnowledgeBase]);
  const fallbackKnowledgeBase: KnowledgeSettingsInput = { id: knowledgeBaseId ?? '', name: '', type: 'document' };
  const currentKnowledgeBase = knowledgeBase ?? fallbackKnowledgeBase;
  const availableSections = useMemo(() => getKnowledgeSettingsSections(currentKnowledgeBase, { canViewActivity }), [currentKnowledgeBase, canViewActivity]);
  const [activeSection, setActiveSection] = useState<KnowledgeSettingsSectionKey>(initialSection ?? availableSections[0]?.key ?? 'vectorStore');
  const [activity, setActivity] = useState<{ status: 'idle' | 'loading' | 'ready' | 'error'; rows: Array<Record<string, unknown>>; message?: string }>({ status: 'idle', rows: [] });
  const [dataSources, setDataSources] = useState<{ status: 'idle' | 'loading' | 'ready' | 'error'; rows: Array<Record<string, unknown>>; message?: string }>({ status: 'idle', rows: [] });
  const [shares, setShares] = useState<{ status: 'idle' | 'loading' | 'ready' | 'error'; rows: Array<Record<string, unknown>>; message?: string }>({ status: 'idle', rows: [] });
  const [graphExtract, setGraphExtract] = useState<GraphExtractConfig>(() => ({
    enabled: currentKnowledgeBase.extract_config?.enabled === true,
    text: currentKnowledgeBase.extract_config?.text ?? '',
    tags: currentKnowledgeBase.extract_config?.tags ?? [],
    nodes: currentKnowledgeBase.extract_config?.nodes ?? [],
    relations: currentKnowledgeBase.extract_config?.relations ?? [],
    customInstructions: currentKnowledgeBase.extract_config?.customInstructions ?? currentKnowledgeBase.extract_config?.custom_instructions ?? '',
  }));
  const summary = summarizeKnowledgeSettings({ ...currentKnowledgeBase, activity: activity.rows.length > 0 ? activity.rows : currentKnowledgeBase.activity });
  const active = availableSections.find((section) => section.key === activeSection) ?? availableSections[0];

  useEffect(() => {
    void import('@weknora/ui').then(setUi);
  }, []);

  useEffect(() => {
    if (!availableSections.some((section) => section.key === activeSection)) setActiveSection(availableSections[0]?.key ?? 'vectorStore');
  }, [activeSection, availableSections]);

  useEffect(() => {
    if (activeSection !== 'activity' || !client || !canViewActivity) return;
    let mounted = true;
    setActivity({ status: 'loading', rows: [] });
    void client.request({
      method: 'GET',
      path: getKnowledgeBaseActivityPath(currentKnowledgeBase.id),
    }).then((value) => {
      if (mounted) setActivity({ status: 'ready', rows: activityRows(value) });
    }).catch((error: unknown) => {
      if (mounted) setActivity({ status: 'error', rows: [], message: error instanceof Error ? error.message : 'Unable to load activity' });
    });
    return () => { mounted = false; };
  }, [activeSection, canViewActivity, client, currentKnowledgeBase.id]);

  useEffect(() => {
    if (activeSection !== 'datasource' || !client) return;
    let mounted = true;
    setDataSources({ status: 'loading', rows: [] });
    void client.request({ method: 'GET', path: getKnowledgeBaseDataSourcesPath(currentKnowledgeBase.id) }).then((value) => {
      if (mounted) setDataSources({ status: 'ready', rows: rowsFromEnvelope(value, 'data') });
    }).catch((error: unknown) => {
      if (mounted) setDataSources({ status: 'error', rows: [], message: error instanceof Error ? error.message : 'Unable to load data sources' });
    });
    return () => { mounted = false; };
  }, [activeSection, client, currentKnowledgeBase.id]);

  useEffect(() => {
    if (activeSection !== 'share' || !client) return;
    let mounted = true;
    setShares({ status: 'loading', rows: [] });
    void client.request({ method: 'GET', path: getKnowledgeBaseSharesPath(currentKnowledgeBase.id) }).then((value) => {
      if (mounted) setShares({ status: 'ready', rows: rowsFromEnvelope(value, 'shares') });
    }).catch((error: unknown) => {
      if (mounted) setShares({ status: 'error', rows: [], message: error instanceof Error ? error.message : 'Unable to load shares' });
    });
    return () => { mounted = false; };
  }, [activeSection, client, currentKnowledgeBase.id]);

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
          {active?.key === 'activity' && activity.status === 'loading' ? <StatusComponent>Loading activity…</StatusComponent> : null}
          {active?.key === 'activity' && activity.status === 'error' ? <StatusComponent tone="error">{activity.message}</StatusComponent> : null}
          {loadState === 'loading' ? <StatusComponent>Loading knowledge-base settings…</StatusComponent> : loadState === 'error' ? <StatusComponent tone="error">Unable to load knowledge-base settings.</StatusComponent> : active ? <SettingsSection summary={summary[active.key]} section={active.key} rows={activity.rows} dataSources={dataSources} shares={shares} graphExtract={graphExtract} modelId={currentKnowledgeBase.summary_model_id ?? ''} client={client} StatusComponent={StatusComponent} onGraphChange={setGraphExtract} /> : <StatusComponent>No settings available.</StatusComponent>}
        </section>
      </div>
    </CardComponent>
  );
}

function SettingsSection({ summary, section, rows, dataSources, shares, graphExtract, modelId, client, StatusComponent, onGraphChange }: { summary: SettingSummary; section: KnowledgeSettingsSectionKey; rows: Array<Record<string, unknown>>; dataSources: { status: string; rows: Array<Record<string, unknown>>; message?: string }; shares: { status: string; rows: Array<Record<string, unknown>>; message?: string }; graphExtract: GraphExtractConfig; modelId: string; client?: WeKnoraClient; StatusComponent: ElementType; onGraphChange: (value: GraphExtractConfig) => void }) {
  return (
    <div style={{ display: 'grid', gap: '0.9rem' }}>
      <div style={{ border: '1px solid #dce3ed', borderRadius: 8, padding: '1rem' }}>
        <StatusComponent tone={summaryTone(summary)}>{summary.label}</StatusComponent>
        <p style={{ margin: '0.35rem 0 0', fontWeight: 600 }}>{summary.detail}</p>
      </div>
      {section === 'vectorStore' ? <p className="wk-muted" style={{ margin: 0 }}>Bindings are read-only after creation. An unavailable binding needs recovery in the global Vector Stores settings.</p> : null}
      {section === 'parser' ? <p className="wk-muted" style={{ margin: 0 }}>Parser overrides are grouped by file type; files without an override use the platform default.</p> : null}
      {section === 'storage' ? <p className="wk-muted" style={{ margin: 0 }}>The selected storage instance owns uploaded files. Existing files may require migration before changing it.</p> : null}
      {section === 'activity' ? (
        rows.length > 0 ? (
          <div style={{ display: 'grid', gap: '0.5rem' }}>
            {rows.slice(0, 5).map((row, index) => <div key={String(row.id ?? index)} style={{ borderBottom: '1px solid #edf0f5', padding: '0.65rem 0' }}><strong>{titleCase(text(row.action) || 'Change')}</strong><span className="wk-muted">{' · '}{titleCase(text(row.outcome) || 'recorded')}</span></div>)}
          </div>
        ) : <p className="wk-muted" style={{ margin: 0 }}>No recorded changes for this knowledge base.</p>
      ) : null}
      {section === 'datasource' ? <RemoteRows state={dataSources} empty="No data sources configured." label={(row) => `${String(row.name ?? 'Unnamed source')} · ${String(row.status ?? 'unknown')}`} StatusComponent={StatusComponent} /> : null}
      {section === 'share' ? <RemoteRows state={shares} empty="This knowledge base is not shared." label={(row) => `${String(row.organization_name ?? row.organization_id ?? 'Unknown space')} · ${String(row.permission ?? 'viewer')}`} StatusComponent={StatusComponent} /> : null}
      {section === 'graph' ? <GraphSettings graphExtract={graphExtract} modelId={modelId} client={client} embedded onChange={onGraphChange} /> : null}
    </div>
  );
}

function rowsFromEnvelope(value: unknown, preferredKey: 'data' | 'shares'): Array<Record<string, unknown>> {
  if (Array.isArray(value)) return activityRows(value);
  if (value && typeof value === 'object') {
    const row = value as Record<string, unknown>;
    return activityRows(row[preferredKey] ?? row.data ?? row.items);
  }
  return [];
}

function RemoteRows({ state, empty, label, StatusComponent }: { state: { status: string; rows: Array<Record<string, unknown>>; message?: string }; empty: string; label: (row: Record<string, unknown>) => string; StatusComponent: ElementType }) {
  if (state.status === 'loading') return <StatusComponent>Loading…</StatusComponent>;
  if (state.status === 'error') return <StatusComponent tone="error">{state.message}</StatusComponent>;
  if (state.rows.length === 0) return <p className="wk-muted" style={{ margin: 0 }}>{empty}</p>;
  return <div style={{ display: 'grid', gap: '0.5rem' }}>{state.rows.slice(0, 30).map((row, index) => <div key={String(row.id ?? index)} style={{ borderBottom: '1px solid #edf0f5', padding: '0.65rem 0' }}>{label(row)}</div>)}</div>;
}
