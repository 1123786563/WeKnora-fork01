import { useEffect, useMemo, useState } from 'react';
import type { ElementType, ReactNode } from 'react';
import type { KnowledgeBase } from '@weknora/contracts';
import type { ChunkingPreviewResult, WeKnoraClient } from '@weknora/api-client';
import { GraphSettings, type GraphExtractConfig } from './GraphSettings.tsx';
import { DataSourcesPage } from '../data-sources/DataSourcesPage.tsx';
import { KnowledgeBaseActivityPanel } from '../knowledge-bases/KnowledgeBaseActivityPanel.tsx';
import { KBShareSettingsSection } from './KBShareSettingsSection.tsx';
import {
  QUESTION_COUNT_RANGE,
  clampQuestionCount,
  filterKnowledgeSettingsModels,
  type KnowledgeSettingsModelOption,
} from './editorSections.ts';
// R490: the Vue-parity chunking form (fields + separator chips + debug drawer)
// now lives in chunkingSection.tsx so the KB editor drawer (App.tsx) mounts the
// same implementation instead of a second, divergent one.
import { ChunkingSettingsFields, EditorSettingRow } from './chunkingSection.tsx';
import { ParserSettingsSection, allParserFileTypes, buildCompleteParserRules, buildParserFileTypeGroups, type ParserEngineInfo, type ParserEngineRule } from './parserSettings.tsx';
import { createTranslator, useAppLocale } from '../i18n.ts';
import './KnowledgeSettingsPage.css';

type ProjectUi = typeof import('@weknora/ui');

export type { KnowledgeSettingsModelOption } from './editorSections.ts';

export interface KnowledgeEditorOptions {
  parserEngines: Array<{ Name: string; Description: string; FileTypes?: string[]; Available?: boolean }>;
  // R485: the storage rows echo the backend config (endpoint/bucket/path
  // hint) and mark the default instance — Vue KBStorageSettings reads both
  // off listStorageBackends().
  storageBackends: Array<{ id: string; name: string; provider: string; status: string; config?: Record<string, unknown> }>;
  defaultStorageBackendId: string;
  vectorStores: Array<{ id: string; name: string; engine_type: string; source: string; readonly: boolean }>;
  models: KnowledgeSettingsModelOption[];
  loading: boolean;
  error: string | null;
}

const idleEditorOptions: KnowledgeEditorOptions = { parserEngines: [], storageBackends: [], defaultStorageBackendId: '', vectorStores: [], models: [], loading: false, error: null };

// Loads the live parser/vector/storage/model catalogues through the
// authenticated settings/configuration APIs when the settings surface opens
// (Vue editorResources + chatResources contract). Each endpoint degrades
// independently so one failing catalogue cannot blank the others.
export async function loadKnowledgeSettingsOptions(client: WeKnoraClient): Promise<Omit<KnowledgeEditorOptions, 'loading'>> {
  const [parser, storage, vector, models] = await Promise.allSettled([
    Promise.resolve().then(() => client.knowledgeBases.settings.parserEngines()),
    Promise.resolve().then(() => client.knowledgeBases.settings.storageBackends()),
    Promise.resolve().then(() => client.knowledgeBases.settings.vectorStores()),
    Promise.resolve().then(() => client.configuration.models.list()),
  ]);
  const outcomes = [parser, storage, vector, models];
  const failures = outcomes.filter((outcome) => outcome.status === 'rejected') as Array<PromiseRejectedResult>;
  const text = (value: unknown): string => (typeof value === 'string' ? value : '');
  return {
    parserEngines: parser.status === 'fulfilled' ? parser.value.data.map((item) => ({ Name: item.Name, Description: item.Description, ...(item.FileTypes === undefined ? {} : { FileTypes: item.FileTypes }), ...(item.Available === undefined ? {} : { Available: item.Available }) })) : [],
    storageBackends: storage.status === 'fulfilled' ? storage.value.data.map((item) => ({ id: item.id, name: item.name, provider: item.provider, status: item.status, config: item.config })) : [],
    defaultStorageBackendId: storage.status === 'fulfilled' ? (storage.value.default_storage_backend_id ?? '') : '',
    vectorStores: vector.status === 'fulfilled' ? vector.value.data.map((item) => ({ id: item.id, name: item.name, engine_type: item.engine_type, source: item.source, readonly: item.readonly })) : [],
    models: models.status === 'fulfilled' ? models.value.map((item) => ({ id: item.id, name: item.name, displayName: text(item.display_name), type: text(item.type), source: text(item.source), ...(item.status === undefined ? {} : { status: text(item.status) }) })) : [],
    error: failures.length > 0 ? (failures[0]!.reason instanceof Error ? failures[0]!.reason.message : 'Unable to load settings options') : null,
  };
}

// Full Vue editor section keys (KnowledgeBaseEditorModal.vue navItems).
// Sections the React surface already implements stay interactive; the rest
// render the shared not-yet-ported placeholder so the information architecture
// matches Vue without fabricating editors.
export type KnowledgeSettingsSectionKey =
  | 'basic' | 'models' | 'vectorStore' | 'faq' | 'parser' | 'chunking'
  | 'multimodal' | 'asr' | 'graph' | 'advanced' | 'storage' | 'datasource'
  | 'share' | 'activity';

const PORTED_KNOWLEDGE_SETTINGS_SECTIONS = new Set<KnowledgeSettingsSectionKey>([
  'vectorStore', 'parser', 'storage', 'datasource', 'share', 'activity', 'graph',
  'models', 'chunking', 'advanced', 'multimodal', 'asr', 'faq', 'basic',
]);

export function isPortedKnowledgeSettingsSection(key: KnowledgeSettingsSectionKey): boolean {
  return PORTED_KNOWLEDGE_SETTINGS_SECTIONS.has(key);
}

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
  faq_config?: { index_mode?: string; question_index_mode?: string };
  // R441: document-KB base-update round-trip state (Vue loadKBData reads the
  // same rows off the GET /knowledge-bases/:id response).
  wiki_config?: { synthesis_model_id?: string; max_pages_per_ingest?: number; extraction_granularity?: string; content_instructions?: string; extraction_instructions?: string };
  indexing_strategy?: { vector_enabled?: boolean; keyword_enabled?: boolean; wiki_enabled?: boolean; graph_enabled?: boolean };
  auto_tag_config?: { enabled?: boolean; model_id?: string; max_tags?: number; skip_if_tagged?: boolean };
};

export interface KnowledgeSettingsSection {
  key: KnowledgeSettingsSectionKey;
  label: string;
  description: string;
  // R457: the heading/description copy resolves through the locale translator;
  // label/description keep the en-US fallback text (identical to the pre-R457
  // hardcoded strings) for the raw-contract consumers.
  labelKey: string;
  descriptionKey: string;
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
  // R455: the activity/datasource/share/graph overview tiles are a React-side
  // addition (Vue's nav groups carry no such tiles), so their copy is
  // localized through fresh kbSettings.summary.* keys present in all five
  // locales. label/detail keep the en-US fallback text; when the keys are
  // present the section renderer resolves them through the locale translator.
  labelKey?: string;
  labelParams?: Record<string, string | number>;
  detailKey?: string;
}

// R484 (R482 B1 差异4): labelKey/descriptionKey now point at the exact Vue
// section-header keys each settings component renders
// (KnowledgeBaseEditorModal.vue basic header + KBModelConfig/
// KBVectorStoreSettings/KBParserSettings/KBChunkingSettings/
// KBAdvancedSettings/KBStorageSettings/GraphSettings/DataSourceSettings/
// KBShareSettings/KnowledgeBaseActivitySettings h2/h3 + section-description),
// replacing the React-invented kbSettings.sections.* copy. label/description
// keep the en-US fallback text for raw-contract consumers.
const sections: KnowledgeSettingsSection[] = [
  { key: 'basic', label: 'Basics', description: 'Name, description and knowledge-base type', labelKey: 'knowledgeEditor.basic.title', descriptionKey: 'knowledgeEditor.basic.description' },
  { key: 'models', label: 'Models', description: 'Language and embedding models', labelKey: 'knowledgeEditor.models.title', descriptionKey: 'knowledgeEditor.models.description' },
  { key: 'faq', label: 'FAQ', description: 'FAQ indexing modes', labelKey: 'knowledgeEditor.faq.title', descriptionKey: 'knowledgeEditor.faq.description' },
  { key: 'multimodal', label: 'Multimodal', description: 'Image description processing', labelKey: 'knowledgeEditor.multimodal.title', descriptionKey: 'knowledgeEditor.multimodal.description' },
  { key: 'asr', label: 'Speech recognition', description: 'Audio transcription model', labelKey: 'knowledgeEditor.asr.title', descriptionKey: 'knowledgeEditor.asr.description' },
  { key: 'vectorStore', label: 'Vector store', description: 'Bound retrieval engine and health', labelKey: 'kbSettings.vectorStore.title', descriptionKey: 'kbSettings.vectorStore.description' },
  { key: 'parser', label: 'Parser', description: 'File-type parser rules', labelKey: 'kbSettings.parser.title', descriptionKey: 'kbSettings.parser.description' },
  { key: 'chunking', label: 'Chunking', description: 'Chunk size and splitting behavior', labelKey: 'knowledgeEditor.chunking.title', descriptionKey: 'knowledgeEditor.chunking.description' },
  { key: 'advanced', label: 'Advanced', description: 'Question generation and extra options', labelKey: 'knowledgeEditor.advanced.title', descriptionKey: 'knowledgeEditor.advanced.description' },
  { key: 'storage', label: 'Storage', description: 'Files and document instance', labelKey: 'kbSettings.storage.title', descriptionKey: 'kbSettings.storage.selectDescription' },
  { key: 'datasource', label: 'Data sources', description: 'External connectors and sync status', labelKey: 'dataSource.title', descriptionKey: 'dataSource.description' },
  { key: 'share', label: 'Share', description: 'Spaces with access to this knowledge base', labelKey: 'organization.share.title', descriptionKey: 'knowledgeEditor.share.description' },
  { key: 'activity', label: 'Activity', description: 'Recent configuration changes', labelKey: 'knowledgeEditor.activity.title', descriptionKey: 'knowledgeEditor.activity.description' },
  { key: 'graph', label: 'Knowledge graph', description: 'Entity and relationship extraction', labelKey: 'graphSettings.title', descriptionKey: 'graphSettings.description' },
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

function parserRules(knowledgeBase: KnowledgeSettingsInput | null | undefined): Array<Record<string, unknown>> {
  if (!knowledgeBase) return [];
  return Array.isArray(knowledgeBase.chunking_config?.parser_engine_rules)
    ? knowledgeBase.chunking_config.parser_engine_rules
    : [];
}

// The parser tab edits the rules through the ParserEngineRule shape (Vue
// KBParserSettings ParserEngineRule); the raw KB rows cast across.
function parserRulesAsEngineRules(knowledgeBase: KnowledgeSettingsInput | null | undefined): ParserEngineRule[] {
  return parserRules(knowledgeBase) as unknown as ParserEngineRule[];
}

export function getKnowledgeSettingsSections(
  knowledgeBase: KnowledgeSettingsInput,
  options: { canViewActivity?: boolean } = {},
): KnowledgeSettingsSection[] {
  const canViewActivity = options.canViewActivity ?? true;
  return sections.filter((section) => {
    // Vue gates chunking/advanced (and parser/storage/graph/multimodal/asr)
    // behind !isFAQ; models stays available for FAQ bases like the Vue basic
    // group, which instead exposes the faq section.
    if (section.key === 'faq') return isFaqKnowledgeBase(knowledgeBase);
    if (section.key === 'parser' || section.key === 'storage' || section.key === 'graph' || section.key === 'chunking' || section.key === 'advanced' || section.key === 'multimodal' || section.key === 'asr') return !isFaqKnowledgeBase(knowledgeBase);
    if (section.key === 'activity') return canViewActivity;
    return true;
  });
}

// ---- Grouped navigation (Vue KnowledgeBaseEditorModal.vue navGroups) ----

export interface KnowledgeSettingsNavItem {
  key: KnowledgeSettingsSectionKey;
  labelKey: string;
  badge?: number;
}

export interface KnowledgeSettingsNavGroup {
  key: string;
  labelKey: string;
  items: KnowledgeSettingsNavItem[];
}

// Vue navItems labels: every item uses knowledgeEditor.sidebar.<key> except
// parser, which reuses settings.parserEngine.
function knowledgeSettingsNavItemLabelKey(key: KnowledgeSettingsSectionKey): string {
  return key === 'parser' ? 'settings.parserEngine' : `knowledgeEditor.sidebar.${key}`;
}

const KNOWLEDGE_SETTINGS_NAV_GROUP_LABEL_KEYS: Record<string, string> = {
  basic: 'knowledgeEditor.navGroups.basic',
  processing: 'knowledgeEditor.navGroups.processing',
  data: 'knowledgeEditor.navGroups.data',
  integration: 'knowledgeEditor.navGroups.integration',
  management: 'knowledgeEditor.navGroups.management',
};

// Mirrors the Vue editor: navItems order then pickItems regrouping. FAQ bases
// collapse to the basic group (+ share/activity when permitted) because the
// document-only items never enter navItems; empty groups are dropped.
export function getKnowledgeSettingsNavGroups(
  knowledgeBase: Pick<KnowledgeSettingsInput, 'type' | 'id'> & { data_source_count?: number },
  options: { canViewActivity?: boolean } = {},
): KnowledgeSettingsNavGroup[] {
  const canViewActivity = options.canViewActivity ?? true;
  const isFaq = knowledgeBase.type?.toLowerCase() === 'faq';
  const hasKbId = Boolean(knowledgeBase.id);
  const itemMap = new Map<KnowledgeSettingsSectionKey, KnowledgeSettingsNavItem>();
  const push = (key: KnowledgeSettingsSectionKey, badge?: number) => {
    itemMap.set(key, { key, labelKey: knowledgeSettingsNavItemLabelKey(key), ...(badge === undefined ? {} : { badge }) });
  };
  push('basic');
  push('models');
  push('vectorStore');
  if (isFaq) {
    push('faq');
  } else {
    push('parser');
    push('multimodal');
    push('asr');
    push('storage');
    push('chunking');
    push('graph');
    push('advanced');
    if (hasKbId) {
      const dataSourceCount = typeof knowledgeBase.data_source_count === 'number' && knowledgeBase.data_source_count > 0
        ? knowledgeBase.data_source_count
        : undefined;
      push('datasource', dataSourceCount);
    }
  }
  if (hasKbId) push('share');
  if (canViewActivity) push('activity');
  const pick = (keys: KnowledgeSettingsSectionKey[]): KnowledgeSettingsNavItem[] =>
    keys.map((key) => itemMap.get(key)).filter((item): item is KnowledgeSettingsNavItem => Boolean(item));
  return [
    { key: 'basic', items: pick(['basic', 'models', 'vectorStore', 'faq']) },
    { key: 'processing', items: pick(['parser', 'chunking', 'multimodal', 'asr', 'graph', 'advanced']) },
    { key: 'data', items: pick(['storage', 'datasource']) },
    { key: 'integration', items: pick(['share']) },
    { key: 'management', items: pick(['activity']) },
  ]
    .map((group) => ({ ...group, labelKey: KNOWLEDGE_SETTINGS_NAV_GROUP_LABEL_KEYS[group.key]! }))
    .filter((group) => group.items.length > 0);
}

// Inline lucide-style stroke icons keyed by the Vue t-icon names
// (KnowledgeBaseEditorModal.vue navItems icon field).
function navIcon(paths: ReactNode): ReactNode {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {paths}
    </svg>
  );
}

const KNOWLEDGE_SETTINGS_NAV_ICONS: Record<KnowledgeSettingsSectionKey, ReactNode> = {
  basic: navIcon(<><circle cx="12" cy="12" r="9" /><path d="M12 8h.01M12 11v5" /></>),
  models: navIcon(<><rect x="5" y="5" width="14" height="14" rx="2" /><rect x="9.5" y="9.5" width="5" height="5" /><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" /></>),
  vectorStore: navIcon(<><ellipse cx="12" cy="5" rx="8" ry="3" /><path d="M4 5v14c0 1.66 3.58 3 8 3s8-1.34 8-3V5" /><path d="M4 12c0 1.66 3.58 3 8 3s8-1.34 8-3" /></>),
  faq: navIcon(<><circle cx="12" cy="12" r="9" /><path d="M9.1 9a3 3 0 0 1 5.8 1c0 2-3 2.4-3 4M12 17.5h.01" /></>),
  parser: navIcon(<><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6" /><circle cx="11" cy="13" r="2.5" /><path d="M13 15l2.5 2.5" /></>),
  chunking: navIcon(<><rect x="8" y="8" width="13" height="13" rx="2" /><path d="M4 16V6a2 2 0 0 1 2-2h10" /></>),
  multimodal: navIcon(<><rect x="3" y="3" width="18" height="18" rx="2" /><circle cx="8.5" cy="8.5" r="1.5" /><path d="M21 15l-5-5L5 21" /></>),
  asr: navIcon(<><path d="M11 5L6 9H2v6h4l5 4z" /><path d="M15.5 8.5a5 5 0 0 1 0 7M18.5 5.5a9 9 0 0 1 0 13" /></>),
  graph: navIcon(<><circle cx="5" cy="6" r="2.5" /><circle cx="19" cy="6" r="2.5" /><circle cx="12" cy="18" r="2.5" /><path d="M7.5 6h9M6.5 8l4.5 8M17.5 8L13 16" /></>),
  advanced: navIcon(<><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.01a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h.01a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.01a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></>),
  storage: navIcon(<path d="M17.5 19a4.5 4.5 0 0 0 .38-8.98 7 7 0 0 0-13.76 1.86A4 4 0 0 0 6 19z" />),
  datasource: navIcon(<><path d="M17.5 19a4.5 4.5 0 0 0 .38-8.98 7 7 0 0 0-13.76 1.86A4 4 0 0 0 6 19z" /><path d="M12 12v7M9 16l3 3 3-3" /></>),
  share: navIcon(<><circle cx="18" cy="5" r="2.5" /><circle cx="6" cy="12" r="2.5" /><circle cx="18" cy="19" r="2.5" /><path d="M8.2 10.8l7.6-4.6M8.2 13.2l7.6 4.6" /></>),
  activity: navIcon(<><path d="M3 12a9 9 0 1 0 3-6.7L3 8" /><path d="M3 3v5h5" /><path d="M12 7v5l3 3" /></>),
};

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
    // R456: the data-driven literals (joined engine names, uppercased
    // extensions, store names, ids) stay key-free en-US fallbacks; the
    // hardcoded English fallback branches carry summary keys resolved by
    // the section renderer through the locale translator.
    parser: rules.length > 0
      ? {
        kind: 'configured' as const,
        label: engines.length > 0 ? engines.map(parserLabel).join(', ') : 'Custom rules',
        detail: extensions.length > 0 ? extensions.map((value) => value.toUpperCase()).join(', ') : 'File-type overrides',
        ...(engines.length > 0 ? {} : { labelKey: 'kbSettings.summary.parser.customRulesLabel' }),
        ...(extensions.length > 0 ? {} : { detailKey: 'kbSettings.summary.parser.overridesDetail' }),
      }
      : { kind: 'empty', label: 'Default parser', detail: 'No file-type overrides', labelKey: 'kbSettings.summary.parser.defaultLabel', detailKey: 'kbSettings.summary.parser.noOverridesDetail' },
    vectorStore: vectorUnavailable
      ? {
        kind: 'unavailable' as const,
        label: vectorName || 'Vector store unavailable',
        detail: 'Check the global vector-store settings',
        detailKey: 'kbSettings.summary.vectorStore.unavailableDetail',
        ...(vectorName ? {} : { labelKey: 'kbSettings.summary.vectorStore.unavailableLabel' }),
      }
      : vectorBound
        ? {
          kind: 'ready' as const,
          label: vectorName || 'Bound vector store',
          detail: [vectorEngine, vectorSource].filter(Boolean).join(' · ') || 'Explicit binding',
          ...(vectorName ? {} : { labelKey: 'kbSettings.summary.vectorStore.boundLabel' }),
          ...([vectorEngine, vectorSource].filter(Boolean).length > 0 ? {} : { detailKey: 'kbSettings.summary.vectorStore.explicitDetail' }),
        }
        : { kind: 'default', label: 'System default', detail: 'No explicit binding', labelKey: 'kbSettings.summary.vectorStore.defaultLabel', detailKey: 'kbSettings.summary.vectorStore.noBindingDetail' },
    storage: storageId || storageProvider
      ? {
        kind: 'configured' as const,
        label: storageProvider ? titleCase(storageProvider) : 'Storage instance',
        detail: storageId || 'Provider configured',
        ...(storageProvider ? {} : { labelKey: 'kbSettings.summary.storage.instanceLabel' }),
        ...(storageId ? {} : { detailKey: 'kbSettings.summary.storage.providerDetail' }),
      }
      : { kind: 'default', label: 'System default', detail: 'No explicit instance', labelKey: 'kbSettings.summary.storage.defaultLabel', detailKey: 'kbSettings.summary.storage.noInstanceDetail' },
    activity: (() => {
      if (activityCount <= 0) return { kind: 'empty', label: 'No activity yet', detail: 'Changes will appear here', labelKey: 'kbSettings.summary.activity.emptyLabel', detailKey: 'kbSettings.summary.activity.emptyDetail' };
      const activityDetail = [latestAction, latestOutcome].filter(Boolean).join(' · ');
      return {
        kind: 'available',
        label: `${activityCount} recent ${activityCount === 1 ? 'event' : 'events'}`,
        detail: activityDetail || 'Open to inspect changes',
        labelKey: activityCount === 1 ? 'kbSettings.summary.activity.countOne' : 'kbSettings.summary.activity.countOther',
        labelParams: { count: activityCount },
        // The detail is either live activity data (action · outcome, locale
        // neutral) or the translatable "open to inspect" fallback.
        ...(activityDetail ? {} : { detailKey: 'kbSettings.summary.activity.inspect' as const }),
      };
    })(),
    datasource: dataSourceCount > 0
      ? { kind: 'available', label: `${dataSourceCount} data source${dataSourceCount === 1 ? '' : 's'}`, detail: 'Open to inspect sync status', labelKey: dataSourceCount === 1 ? 'kbSettings.summary.datasource.countOne' : 'kbSettings.summary.datasource.countOther', labelParams: { count: dataSourceCount }, detailKey: 'kbSettings.summary.datasource.inspect' }
      : { kind: 'empty', label: 'No data sources', detail: 'Add an external connector', labelKey: 'kbSettings.summary.datasource.emptyLabel', detailKey: 'kbSettings.summary.datasource.emptyDetail' },
    share: shareCount > 0
      ? { kind: 'available', label: `${shareCount} shared space${shareCount === 1 ? '' : 's'}`, detail: 'Access is managed per share', labelKey: shareCount === 1 ? 'kbSettings.summary.share.countOne' : 'kbSettings.summary.share.countOther', labelParams: { count: shareCount }, detailKey: 'kbSettings.summary.share.managed' }
      : { kind: 'empty', label: 'Not shared', detail: 'No spaces have access', labelKey: 'kbSettings.summary.share.emptyLabel', detailKey: 'kbSettings.summary.share.emptyDetail' },
    graph: graphEnabled
      ? { kind: 'configured', label: 'Knowledge graph enabled', detail: 'Entity and relationship extraction', labelKey: 'kbSettings.summary.graph.enabledLabel', detailKey: 'kbSettings.summary.graph.enabledDetail' }
      : { kind: 'default', label: 'Knowledge graph disabled', detail: 'Configure extraction when graph storage is enabled', labelKey: 'kbSettings.summary.graph.disabledLabel', detailKey: 'kbSettings.summary.graph.disabledDetail' },
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

// Draft values edited through the R439 editor sections. Absent fields keep the
// round-trip values computed from the loaded KB config, so a section that was
// never opened still sends exactly the R437 payload.
export interface KnowledgeSettingsEditorOverrides {
  llmModelId?: string;
  embeddingModelId?: string;
  documentSplitting?: Partial<Pick<KnowledgeSettingsSavePayload['documentSplitting'], 'chunkSize' | 'chunkOverlap' | 'separators' | 'enableParentChild' | 'parentChunkSize' | 'childChunkSize' | 'strategy' | 'tokenLimit' | 'languages' | 'tableMetadataInstructions'>>;
  questionGeneration?: Partial<Pick<KnowledgeSettingsSavePayload['questionGeneration'], 'enabled' | 'questionCount' | 'customInstructions'>>;
  // R485 advanced tab (Vue KBAdvancedSettings autoTagConfig draft): persists
  // through the base update's auto_tag_config block.
  autoTag?: { enabled?: boolean; modelId?: string; maxTags?: number; skipIfTagged?: boolean };
  // R440 multimodal/asr sections (Vue multimodalConfig / asrConfig drafts).
  multimodal?: { enabled?: boolean; vllmModelId?: string; descriptionLanguage?: string; customInstructions?: string };
  asr?: { enabled?: boolean; modelId?: string };
  // R440 faq section (Vue faqConfig draft, saved through the base KB update).
  faqConfig?: { indexMode?: string; questionIndexMode?: string };
  // R441 basic section (Vue formData name/description, indexingStrategy and
  // wikiConfig drafts, all persisted through the base KB update).
  name?: string;
  description?: string;
  indexing?: { vectorEnabled?: boolean; keywordEnabled?: boolean; wikiEnabled?: boolean; graphEnabled?: boolean };
  wiki?: { extractionGranularity?: 'focused' | 'standard' | 'exhaustive'; contentInstructions?: string; extractionInstructions?: string };
  // R445 storage section (Vue KBStorageSettings handleChange emits the backend
  // id and its provider; both persist through the config PUT while the KB has
  // no files — the select locks with the same hasFiles signal otherwise).
  storageBackendId?: string;
  storageProvider?: string;
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
  overrides?: KnowledgeSettingsEditorOverrides,
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
  const payload: KnowledgeSettingsSavePayload = {
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
      // Vue loadKBData uses `||` fallbacks: a stored 0 (unset) reads as the
      // backend DefaultChunkOverlap, matching chunker.DefaultChunkOverlap.
      chunkOverlap: Number(chunking.chunk_overlap) || 80,
      // Vue `separators || defaults` keeps an empty array (truthy) as-is.
      separators: Array.isArray(chunking.separators) ? chunking.separators.map(String) : ['\n\n', '\n', '。', '！', '？', ';', '；'],
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
  // R439 editor sections: apply draft values on top of the round-trip payload.
  // Only explicitly defined override fields replace the computed value.
  if (overrides) {
    const defined = <T extends Record<string, unknown>>(patch: T | undefined): Partial<T> => {
      if (!patch) return {};
      return Object.fromEntries(Object.entries(patch).filter(([, value]) => value !== undefined)) as Partial<T>;
    };
    if (typeof overrides.llmModelId === 'string') payload.llmModelId = overrides.llmModelId;
    if (typeof overrides.embeddingModelId === 'string') payload.embeddingModelId = overrides.embeddingModelId;
    payload.documentSplitting = { ...payload.documentSplitting, ...defined(overrides.documentSplitting) };
    payload.questionGeneration = { ...payload.questionGeneration, ...defined(overrides.questionGeneration) };
    // Vue vlm_config semantics: model_id is cleared when the toggle is off
    // (handleMultimodalToggle clears vllmModelId, the payload guards with
    // `enabled ? … : ''`). Untouched fields keep the round-trip values.
    const multimodal = defined(overrides.multimodal);
    if (Object.keys(multimodal).length > 0) {
      const enabled = multimodal.enabled ?? payload.vlm_config.enabled;
      const vllmModelId = multimodal.vllmModelId ?? payload.vlm_config.model_id;
      payload.vlm_config = {
        enabled,
        model_id: enabled ? vllmModelId : '',
        description_language: multimodal.descriptionLanguage ?? payload.vlm_config.description_language,
        custom_instructions: multimodal.customInstructions ?? payload.vlm_config.custom_instructions,
      };
      payload.multimodal = { enabled };
    }
    // Vue asr_config semantics: same disabled-clears-model_id rule.
    const asr = defined(overrides.asr);
    if (Object.keys(asr).length > 0) {
      const enabled = asr.enabled ?? payload.asr_config.enabled;
      const modelId = asr.modelId ?? payload.asr_config.model_id;
      payload.asr_config = { enabled, model_id: enabled ? modelId : '', language: payload.asr_config.language };
    }
    // Vue KBStorageSettings handleChange emits the backend id and its provider
    // together; the select only unlocks while the KB has no files.
    if (typeof overrides.storageBackendId === 'string') payload.storageBackendId = overrides.storageBackendId;
    if (typeof overrides.storageProvider === 'string') payload.storageProvider = overrides.storageProvider;
  }
  return payload;
}

// Vue doSubmit step 1 on edit: updateKnowledgeBase carries name/description and
// the faq_config (FAQ bases) or wiki_config + auto_tag_config + indexing_strategy
// (document bases) through PUT /api/v1/knowledge-bases/:id, before the
// KBModelConfigRequest PUT below.
export function getKnowledgeBaseUpdatePath(knowledgeBaseId: string): string {
  return `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}`;
}

// Vue wikiConfig.extractionGranularity normalization (loadKBData +
// resolvedGranularity): unknown values fall back to 'standard', matching the
// backend WikiExtractionGranularity.Normalize() contract.
const WIKI_EXTRACTION_GRANULARITIES = ['focused', 'standard', 'exhaustive'] as const;
export type KnowledgeSettingsWikiGranularity = (typeof WIKI_EXTRACTION_GRANULARITIES)[number];

export function resolveKnowledgeSettingsGranularity(value: unknown): KnowledgeSettingsWikiGranularity {
  return WIKI_EXTRACTION_GRANULARITIES.includes(value as KnowledgeSettingsWikiGranularity)
    ? value as KnowledgeSettingsWikiGranularity
    : 'standard';
}

// Vue indexingStrategy resolution (loadKBData): absent backend rows fall back
// to vector+keyword on, wiki+graph off, with the live draft layered on top.
export function resolveKnowledgeSettingsIndexing(
  knowledgeBase: KnowledgeSettingsInput,
  overrides?: KnowledgeSettingsEditorOverrides,
): { vectorEnabled: boolean; keywordEnabled: boolean; wikiEnabled: boolean; graphEnabled: boolean } {
  const draft = overrides?.indexing ?? {};
  return {
    vectorEnabled: draft.vectorEnabled ?? knowledgeBase.indexing_strategy?.vector_enabled ?? true,
    keywordEnabled: draft.keywordEnabled ?? knowledgeBase.indexing_strategy?.keyword_enabled ?? true,
    wikiEnabled: draft.wikiEnabled ?? knowledgeBase.indexing_strategy?.wiki_enabled ?? false,
    graphEnabled: draft.graphEnabled ?? knowledgeBase.indexing_strategy?.graph_enabled ?? false,
  };
}

export interface KnowledgeSettingsBaseUpdate {
  name: string;
  description: string;
  config: {
    faq_config?: { index_mode: string; question_index_mode: string };
    wiki_config?: { synthesis_model_id: string; max_pages_per_ingest: number; extraction_granularity: KnowledgeSettingsWikiGranularity; content_instructions: string; extraction_instructions: string };
    auto_tag_config?: { enabled: boolean; model_id: string; max_tags: number; skip_if_tagged: boolean };
    indexing_strategy?: { vector_enabled: boolean; keyword_enabled: boolean; wiki_enabled: boolean; graph_enabled: boolean };
  };
}

export function buildKnowledgeSettingsBaseUpdate(
  knowledgeBase: KnowledgeSettingsInput,
  overrides?: KnowledgeSettingsEditorOverrides,
): KnowledgeSettingsBaseUpdate {
  const raw = (value: unknown): string => (typeof value === 'string' ? value : '');
  const config: KnowledgeSettingsBaseUpdate['config'] = {};
  if (knowledgeBase.type?.toLowerCase() === 'faq') {
    const kbFaq = knowledgeBase.faq_config ?? {};
    const faq = overrides?.faqConfig ?? {};
    config.faq_config = {
      index_mode: faq.indexMode ?? (typeof kbFaq.index_mode === 'string' && kbFaq.index_mode ? kbFaq.index_mode : 'question_only'),
      question_index_mode: faq.questionIndexMode ?? (typeof kbFaq.question_index_mode === 'string' && kbFaq.question_index_mode ? kbFaq.question_index_mode : 'separate'),
    };
  } else {
    // Vue doSubmit step 1 (document bases): wiki_config tunables, the
    // auto_tag_config block and the indexing strategy all persist through the
    // base update. Untouched fields round-trip the loaded KB row.
    const kbWiki = knowledgeBase.wiki_config ?? {};
    const wiki = overrides?.wiki ?? {};
    config.wiki_config = {
      synthesis_model_id: raw(kbWiki.synthesis_model_id),
      max_pages_per_ingest: typeof kbWiki.max_pages_per_ingest === 'number' ? kbWiki.max_pages_per_ingest : 0,
      extraction_granularity: wiki.extractionGranularity ?? resolveKnowledgeSettingsGranularity(kbWiki.extraction_granularity),
      content_instructions: wiki.contentInstructions ?? raw(kbWiki.content_instructions),
      extraction_instructions: wiki.extractionInstructions ?? raw(kbWiki.extraction_instructions),
    };
    const autoTag = knowledgeBase.auto_tag_config ?? {};
    // R485: the advanced-tab autoTag draft layers on top of the loaded row
    // (Vue KBAdvancedSettings update:autoTag), persisting through the base
    // update exactly like the wiki/indexing blocks.
    const autoTagDraft = overrides?.autoTag ?? {};
    config.auto_tag_config = {
      enabled: autoTagDraft.enabled ?? autoTag.enabled === true,
      model_id: autoTagDraft.modelId ?? raw(autoTag.model_id),
      max_tags: autoTagDraft.maxTags ?? (typeof autoTag.max_tags === 'number' && autoTag.max_tags > 0 ? autoTag.max_tags : 3),
      skip_if_tagged: autoTagDraft.skipIfTagged ?? (typeof autoTag.skip_if_tagged === 'boolean' ? autoTag.skip_if_tagged : true),
    };
    const indexing = resolveKnowledgeSettingsIndexing(knowledgeBase, overrides);
    config.indexing_strategy = {
      vector_enabled: indexing.vectorEnabled,
      keyword_enabled: indexing.keywordEnabled,
      wiki_enabled: indexing.wikiEnabled,
      graph_enabled: indexing.graphEnabled,
    };
  }
  return {
    name: overrides?.name ?? (typeof knowledgeBase.name === 'string' ? knowledgeBase.name : ''),
    description: overrides?.description ?? (typeof knowledgeBase.description === 'string' ? knowledgeBase.description : ''),
    config,
  };
}

// Sends the Vue doSubmit base update through the authenticated transport (the
// shared api-client update helper is not wired into this surface yet).
export async function saveKnowledgeSettingsBaseUpdate(client: WeKnoraClient, knowledgeBaseId: string, baseUpdate: KnowledgeSettingsBaseUpdate): Promise<void> {
  await client.request({ method: 'PUT', path: getKnowledgeBaseUpdatePath(knowledgeBaseId), body: baseUpdate });
}

// Sends the update through the authenticated client transport. The
// /initialization/config endpoint lives behind client.request because the
// shared api-client does not expose it yet (see report: api-client contract).
export async function saveKnowledgeSettings(client: WeKnoraClient, knowledgeBaseId: string, payload: KnowledgeSettingsSavePayload): Promise<void> {
  await client.request({ method: 'PUT', path: getKnowledgeBaseConfigPath(knowledgeBaseId), body: payload });
}

// R455: resolves the overview-tile copy through the locale translator when the
// summary carries a message key. R456 extends this to the parser/vectorStore/
// storage tiles' fallback branches; their data-driven literals (engine and
// store names, ids) stay key-free English fallbacks and en-US copy.
function localizedSummaryField(summary: SettingSummary, field: 'label' | 'detail', t: (key: string, values?: Record<string, string | number>) => string): string {
  if (field === 'label' && summary.labelKey) return t(summary.labelKey, summary.labelParams);
  if (field === 'detail' && summary.detailKey) return t(summary.detailKey);
  return summary[field];
}

interface KnowledgeSettingsPageProps {
  knowledgeBase?: KnowledgeSettingsInput;
  knowledgeBaseId?: string;
  client?: WeKnoraClient;
  role?: 'owner' | 'admin' | 'viewer';
  canViewActivity?: boolean;
  initialSection?: KnowledgeSettingsSectionKey;
  /** Vue handleClose (settings footer 取消): discard drafts and close the host. */
  onClose?: () => void;
}

export function knowledgeSettingsCanEdit(role: 'owner' | 'admin' | 'viewer' | undefined): boolean {
  return role === 'owner' || role === 'admin';
}

export function KnowledgeSettingsPage({ knowledgeBase: providedKnowledgeBase, knowledgeBaseId, client, role = 'viewer', canViewActivity = true, initialSection, onClose }: KnowledgeSettingsPageProps) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const [ui, setUi] = useState<ProjectUi | null>(null);
  const [loadedKnowledgeBase, setLoadedKnowledgeBase] = useState<KnowledgeSettingsInput | null>(providedKnowledgeBase ?? null);
  const [loadState, setLoadState] = useState<'idle' | 'loading' | 'error'>('idle');
  const [editorOptions, setEditorOptions] = useState<KnowledgeEditorOptions>(idleEditorOptions);
  // R484: the parser tab edits the full per-file-type rule set (Vue
  // KBParserSettings localEngineRules), not a single pending engine.
  const [parserEngineRules, setParserEngineRules] = useState<ParserEngineRule[]>(() => parserRulesAsEngineRules(providedKnowledgeBase ?? loadedKnowledgeBase));
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
  // Vue isIndexingLocked signal (loadKBData): when the settings surface opens
  // in edit mode for a document base, probe GET /knowledge-bases/:id/knowledge
  // once with page=1&page_size=1. total > 0 means the KB already has content
  // and the editor locks the indexing strategy (the backend requires a
  // non-empty KB to keep at least one index). A failed probe degrades to
  // unlocked so one rejected request cannot block the editor.
  const [indexingLocked, setIndexingLocked] = useState(false);
  const kbId = currentKnowledgeBase.id;
  const isFaqBase = currentKnowledgeBase.type?.toLowerCase() === 'faq';
  useEffect(() => {
    if (!client || !kbId || isFaqBase) return;
    let mounted = true;
    Promise.resolve()
      .then(() => client.knowledgeBases.documents.list(kbId, { page: 1, page_size: 1 }))
      .then((result) => {
        if (mounted) setIndexingLocked(((result as { total?: number }).total ?? 0) > 0);
      })
      .catch(() => {
        if (mounted) setIndexingLocked(false);
      });
    return () => { mounted = false; };
  }, [client, kbId, isFaqBase]);
  const availableSections = useMemo(() => getKnowledgeSettingsSections(currentKnowledgeBase, { canViewActivity }), [currentKnowledgeBase, canViewActivity]);
  // Vue editor contract: nav groups drive the sidebar; the content area renders
  // the active section, with unported Vue sections shown as placeholders.
  const navGroups = useMemo(() => getKnowledgeSettingsNavGroups(currentKnowledgeBase, { canViewActivity }), [currentKnowledgeBase, canViewActivity]);
  const navKeys = useMemo(() => navGroups.flatMap((group) => group.items.map((item) => item.key)), [navGroups]);
  const [activeSection, setActiveSection] = useState<KnowledgeSettingsSectionKey>(initialSection ?? navKeys[0] ?? 'basic');
  const [graphExtract, setGraphExtract] = useState<GraphExtractConfig>(() => ({
    enabled: currentKnowledgeBase.extract_config?.enabled === true,
    text: currentKnowledgeBase.extract_config?.text ?? '',
    tags: currentKnowledgeBase.extract_config?.tags ?? [],
    nodes: currentKnowledgeBase.extract_config?.nodes ?? [],
    relations: currentKnowledgeBase.extract_config?.relations ?? [],
    customInstructions: currentKnowledgeBase.extract_config?.customInstructions ?? currentKnowledgeBase.extract_config?.custom_instructions ?? '',
  }));
  // Vue KBParserSettings watches props.parserEngineRules (the loaded KB row)
  // and replaces its local rules; mirror that so a late KB load seeds the
  // per-group selects. Guard on the loaded row only — the pre-load fallback
  // placeholder is a fresh object each render, and keying the effect on it
  // loops setState (the graph page opens this surface with knowledgeBaseId
  // alone, so the pre-load window is real).
  useEffect(() => {
    if (!knowledgeBase) return;
    setParserEngineRules(parserRulesAsEngineRules(knowledgeBase));
  }, [knowledgeBase]);
  // Vue ensureCompleteRules: once the engine catalogue lands, materialise a
  // rule per file-type group (defaults filled in) so a save persists the
  // complete set like buildCompleteRules().
  useEffect(() => {
    if (editorOptions.loading || editorOptions.parserEngines.length === 0) return;
    const groups = buildParserFileTypeGroups((key) => key, allParserFileTypes(editorOptions.parserEngines));
    const complete = buildCompleteParserRules(groups, parserEngineRules, editorOptions.parserEngines);
    if (complete.length > 0 && complete.length > parserEngineRules.length) setParserEngineRules(complete);
  }, [editorOptions.loading, editorOptions.parserEngines, parserEngineRules]);
  const summary = summarizeKnowledgeSettings(currentKnowledgeBase);
  // Vue editor save semantics: one in-flight save at a time (button :loading),
  // success toast, failure keeps the form intact and surfaces the message.
  const [saveState, setSaveState] = useState<{ status: 'idle' | 'saving' | 'saved' | 'error'; message: string }>({ status: 'idle', message: '' });
  // R439 editor drafts (models/chunking/advanced): absent fields keep the
  // round-trip values from the loaded KB config.
  const [editorDraft, setEditorDraft] = useState<KnowledgeSettingsEditorOverrides>({});
  const canManage = knowledgeSettingsCanEdit(role);
  const canSave = canManage && Boolean(client) && Boolean(currentKnowledgeBase.id);
  // Single source of truth for the section controls: what a save would PUT
  // right now (committed config round-trip merged with the live draft).
  const editorPayload = useMemo(
    () => buildKnowledgeSettingsConfigPayload(currentKnowledgeBase, parserRules(currentKnowledgeBase), graphExtract, editorDraft),
    [currentKnowledgeBase, graphExtract, editorDraft],
  );
  const handleSave = () => {
    if (!client || !currentKnowledgeBase.id || saveState.status === 'saving' || !canSave) return;
    // Vue KBParserSettings buildCompleteRules: the per-group rule set (loaded
    // rules materialised with resolved defaults) is what a save persists.
    const rules = (parserEngineRules.length > 0
      ? parserEngineRules
      : parserRules(currentKnowledgeBase)) as Array<Record<string, unknown>>;
    const payload = buildKnowledgeSettingsConfigPayload(currentKnowledgeBase, rules, graphExtract, editorDraft);
    // Vue validateForm subset owned by these sections, in Vue order: the name
    // is required, a document base keeps at least one indexing strategy, an
    // enabled multimodal toggle requires a VLLM model, a FAQ base requires an
    // index mode. Each failure warns and jumps to the offending section before
    // any request.
    const candidateName = editorDraft.name ?? currentKnowledgeBase.name ?? '';
    if (!candidateName.trim()) {
      setSaveState({ status: 'error', message: t('knowledgeEditor.messages.nameRequired') });
      setActiveSection('basic');
      return;
    }
    const isFaq = currentKnowledgeBase.type?.toLowerCase() === 'faq';
    const indexing = resolveKnowledgeSettingsIndexing(currentKnowledgeBase, editorDraft);
    if (!isFaq) {
      if (!indexing.vectorEnabled && !indexing.keywordEnabled && !indexing.wikiEnabled && !indexing.graphEnabled) {
        setSaveState({ status: 'error', message: t('knowledgeEditor.indexing.atLeastOne') });
        setActiveSection('basic');
        return;
      }
    }
    // Vue validateForm model checks, in Vue order: the Embedding model is only
    // required while RAG search (vector|keyword indexing) is enabled, the
    // summary LLM is always required. Both jump to the models section.
    const needsEmbedding = indexing.vectorEnabled || indexing.keywordEnabled;
    if (needsEmbedding && !payload.embeddingModelId) {
      setSaveState({ status: 'error', message: t('knowledgeEditor.indexing.embeddingRequired') });
      setActiveSection('models');
      return;
    }
    if (!payload.llmModelId) {
      setSaveState({ status: 'error', message: t('knowledgeEditor.messages.summaryRequired') });
      setActiveSection('models');
      return;
    }
    if (payload.vlm_config.enabled && !payload.vlm_config.model_id) {
      setSaveState({ status: 'error', message: t('knowledgeEditor.messages.multimodalInvalid') });
      setActiveSection('multimodal');
      return;
    }
    const baseUpdate = buildKnowledgeSettingsBaseUpdate(currentKnowledgeBase, editorDraft);
    if (isFaq && !baseUpdate.config.faq_config?.index_mode) {
      setSaveState({ status: 'error', message: t('knowledgeEditor.messages.indexModeRequired') });
      setActiveSection('faq');
      return;
    }
    setSaveState({ status: 'saving', message: '' });
    // Vue doSubmit order on edit: the base update (name/description plus the
    // FAQ or wiki/auto-tag/indexing config blocks) always runs first, then the
    // full KBModelConfigRequest PUT.
    void saveKnowledgeSettingsBaseUpdate(client, currentKnowledgeBase.id, baseUpdate)
      .then(() => saveKnowledgeSettings(client, currentKnowledgeBase.id, payload))
      .then(() => {
        setSaveState({ status: 'saved', message: t('knowledgeEditor.messages.updateSuccess') });
      }).catch((error: unknown) => {
        setSaveState({ status: 'error', message: error instanceof Error && error.message ? error.message : t('common.error') });
      });
  };
  // Vue settings-footer 取消 (handleClose): every unsaved draft is dropped —
  // the loaded KB row becomes the editor state again — and the host surface
  // closes.
  const handleCancel = () => {
    if (saveState.status === 'saving') return;
    setEditorDraft({});
    setParserEngineRules(parserRulesAsEngineRules(currentKnowledgeBase));
    setGraphExtract({
      enabled: currentKnowledgeBase.extract_config?.enabled === true,
      text: currentKnowledgeBase.extract_config?.text ?? '',
      tags: currentKnowledgeBase.extract_config?.tags ?? [],
      nodes: currentKnowledgeBase.extract_config?.nodes ?? [],
      relations: currentKnowledgeBase.extract_config?.relations ?? [],
      customInstructions: currentKnowledgeBase.extract_config?.customInstructions ?? currentKnowledgeBase.extract_config?.custom_instructions ?? '',
    });
    setSaveState({ status: 'idle', message: '' });
    onClose?.();
  };
  const active = availableSections.find((section) => section.key === activeSection);

  useEffect(() => {
    void import('@weknora/ui').then(setUi);
  }, []);

  useEffect(() => {
    if (!navKeys.includes(activeSection)) setActiveSection(navKeys[0] ?? 'basic');
  }, [activeSection, navKeys]);

  const CardComponent = ui?.Card ?? 'section';
  const ButtonComponent = ui?.Button ?? 'button';
  const StatusComponent = ui?.Status ?? 'p';

  return (
    <CardComponent aria-label={`Knowledge settings for ${currentKnowledgeBase.name}`}>
      {/* Vue KnowledgeBaseEditorModal .settings-modal frame (1000x750) */}
      <div
        className="wkbs-modal"
        style={{ width: '90vw', maxWidth: '1000px', height: '85vh', maxHeight: '750px', borderRadius: '12px' }}
      >
        <div className="wkbs-container">
          {/* Vue .settings-sidebar (208px) */}
          <aside className="wkbs-sidebar" style={{ width: '208px' }}>
            <div className="wkbs-sidebar-header">
              <h2 className="wkbs-sidebar-title">{t('knowledgeEditor.titleEdit')}</h2>
            </div>
            <nav className="wkbs-nav" aria-label="Knowledge settings sections">
              {navGroups.map((group) => (
                <div key={group.key} className="wkbs-nav-group">
                  <div className="wkbs-nav-group-title">{t(group.labelKey)}</div>
                  {group.items.map((item) => (
                    <button
                      key={item.key}
                      type="button"
                      data-section={item.key}
                      className={`wkbs-nav-item${activeSection === item.key ? ' is-active' : ''}`}
                      aria-current={activeSection === item.key ? 'page' : undefined}
                      onClick={() => setActiveSection(item.key)}
                    >
                      <span className="wkbs-nav-icon">{KNOWLEDGE_SETTINGS_NAV_ICONS[item.key]}</span>
                      <span className="wkbs-nav-label">{t(item.labelKey)}</span>
                      {item.badge ? <span className="wkbs-nav-badge">{item.badge}</span> : null}
                    </button>
                  ))}
                </div>
              ))}
            </nav>
          </aside>

          {/* Vue .settings-content */}
          <section className="wkbs-content" aria-labelledby="knowledge-settings-section-title">
            <div className="wkbs-content-wrapper">
              {active ? (
                <>
                  {/* Vue section-header: one section-title + section-description
                      per tab (KnowledgeBaseEditorModal.vue basic header / each
                      settings component's own h2-h3 header). No eyebrow line. */}
                  <h3 id="knowledge-settings-section-title" style={{ margin: '0 0 0.2rem', fontSize: '1.4rem' }}>{t(active.labelKey)}</h3>
                  <p className="wk-muted" style={{ margin: '0 0 1.25rem' }}>{t(active.descriptionKey)}</p>
                </>
              ) : null}
              {loadState === 'loading' ? <StatusComponent>Loading knowledge-base settings…</StatusComponent> : loadState === 'error' ? <StatusComponent tone="error">Unable to load knowledge-base settings.</StatusComponent> : active ? <SettingsSection summary={summary[active.key as keyof KnowledgeSettingsSummary]} section={active.key} graphExtract={graphExtract} modelId={editorPayload.llmModelId} client={client} knowledgeBase={currentKnowledgeBase} knowledgeBaseId={currentKnowledgeBase.id} knowledgeBaseName={currentKnowledgeBase.name} canManage={knowledgeSettingsCanEdit(role)} editorOptions={editorOptions} parserEngineRules={parserEngineRules} indexingLocked={indexingLocked} onParserEngineRules={setParserEngineRules} t={t} StatusComponent={StatusComponent} onGraphChange={setGraphExtract} editorPayload={editorPayload} editorDraft={editorDraft} onDraftChange={setEditorDraft} /> : isPortedKnowledgeSettingsSection(activeSection) ? <StatusComponent>No settings available.</StatusComponent> : (
                // Vue renders this section fully; the React port has not migrated
                // it yet — surface the shared notice instead of a fabricated editor.
                <StatusComponent>{t('settings.notYetPorted')}</StatusComponent>
              )}
              {canSave ? (
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginTop: '1.25rem', borderTop: '1px solid #dce3ed', paddingTop: '1rem' }}>
                  {/* Vue settings-footer-actions: 取消 discards the drafts
                      (handleClose) and 保存并关闭 submits (saveButtonLabel in
                      edit mode). */}
                  <ButtonComponent
                    type="button"
                    disabled={saveState.status === 'saving'}
                    aria-label={t('common.cancel')}
                    onClick={handleCancel}
                  >
                    {t('common.cancel')}
                  </ButtonComponent>
                  <ButtonComponent
                    type="button"
                    disabled={saveState.status === 'saving'}
                    aria-busy={saveState.status === 'saving'}
                    aria-label={t('knowledgeEditor.buttons.saveAndClose')}
                    onClick={handleSave}
                  >
                    {t('knowledgeEditor.buttons.saveAndClose')}
                  </ButtonComponent>
                  {saveState.message ? (
                    <span role="status" aria-live="polite" style={{ color: saveState.status === 'error' ? '#b42318' : '#067647', fontWeight: 600 }}>{saveState.message}</span>
                  ) : null}
                </div>
              ) : null}
            </div>
          </section>
        </div>
      </div>
    </CardComponent>
  );
}

interface SettingsSectionProps {
  summary?: SettingSummary;
  section: KnowledgeSettingsSectionKey;
  graphExtract: GraphExtractConfig;
  modelId: string;
  client?: WeKnoraClient;
  knowledgeBase: KnowledgeSettingsInput;
  knowledgeBaseId: string;
  knowledgeBaseName: string;
  canManage: boolean;
  editorOptions: KnowledgeEditorOptions;
  parserEngineRules: ParserEngineRule[];
  indexingLocked: boolean;
  onParserEngineRules: (value: ParserEngineRule[]) => void;
  t: (key: string, values?: Record<string, string | number>) => string;
  StatusComponent: ElementType;
  onGraphChange: (value: GraphExtractConfig) => void;
  editorPayload: KnowledgeSettingsSavePayload;
  editorDraft: KnowledgeSettingsEditorOverrides;
  onDraftChange: (value: KnowledgeSettingsEditorOverrides) => void;
}

function SettingsSection({ summary, section, graphExtract, modelId, client, knowledgeBase, knowledgeBaseId, knowledgeBaseName, canManage, editorOptions, parserEngineRules, indexingLocked, onParserEngineRules, t, StatusComponent, onGraphChange, editorPayload, editorDraft, onDraftChange }: SettingsSectionProps) {
  // R484 (R482 B1 差异4): the R455 overview tiles are a React-side addition —
  // Vue's settings drawer carries no per-tab overview card — so they no
  // longer render inside the drawer. The summary still feeds the controls
  // that echo it (the disabled vector-store select, the missing-storage
  // option) with the localized label.
  const summaryLabel = summary ? localizedSummaryField(summary, 'label', t) : '';
  return (
    <div style={{ display: 'grid', gap: '0.9rem' }}>
      {section === 'basic' ? (
        <BasicSettingsSection knowledgeBase={knowledgeBase} editorDraft={editorDraft} indexingLocked={indexingLocked} t={t} onDraftChange={onDraftChange} />
      ) : null}
      {section === 'models' ? (
        <ModelsSettingsSection editorPayload={editorPayload} knowledgeBase={knowledgeBase} editorDraft={editorDraft} indexingLocked={indexingLocked} models={editorOptions.models} t={t} onDraftChange={onDraftChange} />
      ) : null}
      {section === 'chunking' ? (
        <ChunkingSettingsSection editorPayload={editorPayload} editorDraft={editorDraft} client={client} t={t} onDraftChange={onDraftChange} />
      ) : null}
      {section === 'advanced' ? (
        <AdvancedSettingsSection editorPayload={editorPayload} editorDraft={editorDraft} knowledgeBase={knowledgeBase} models={editorOptions.models} t={t} onDraftChange={onDraftChange} />
      ) : null}
      {section === 'multimodal' ? (
        <MultimodalSettingsSection editorPayload={editorPayload} editorDraft={editorDraft} models={editorOptions.models} t={t} onDraftChange={onDraftChange} />
      ) : null}
      {section === 'asr' ? (
        <AsrSettingsSection editorPayload={editorPayload} editorDraft={editorDraft} models={editorOptions.models} t={t} onDraftChange={onDraftChange} />
      ) : null}
      {section === 'faq' ? (
        <FaqSettingsSection knowledgeBase={knowledgeBase} editorDraft={editorDraft} t={t} onDraftChange={onDraftChange} />
      ) : null}
      {section === 'vectorStore' ? (
        // R485 (Vue KBVectorStoreSettings edit mode): a read-only bound-store
        // badge card — the binding is immutable after creation, so no select
        // renders at all (VectorStoreBadge + immutableEdit + unavailable hint).
        (() => {
          const boundSource = text(knowledgeBase.vector_store_source);
          const boundName = text(knowledgeBase.vector_store_name);
          const boundEngineType = text(knowledgeBase.vector_store_engine_type);
          const boundStatus = text(knowledgeBase.vector_store_status);
          const effectiveSource = boundSource || 'env';
          const unavailable = boundStatus === 'unavailable' || effectiveSource === 'unavailable';
          const displayName = effectiveSource === 'env'
            ? t('vectorStoreBadge.systemDefault')
            : effectiveSource === 'shared'
              ? t('vectorStoreBadge.sharedFromOrg')
              : boundName || t('vectorStoreBadge.unknownStore');
          return (
            <div style={{ display: 'grid', gap: '0.4rem' }}>
              <EditorSettingRow
                label={t('kbSettings.vectorStore.boundLabel')}
                description={t('kbSettings.vectorStore.immutableEdit')}
                control={(
                  <div style={{ display: 'grid', gap: '0.4rem', justifyItems: 'start' }}>
                    <span
                      data-vector-store-badge=""
                      data-vector-store-source={effectiveSource}
                      className={effectiveSource === 'user' ? 'wk-vs-badge wk-vs-badge-user' : effectiveSource === 'shared' ? 'wk-vs-badge wk-vs-badge-shared' : 'wk-vs-badge wk-vs-badge-env'}
                    >
                      {displayName}
                      {boundEngineType && (effectiveSource === 'user' || effectiveSource === 'env') ? <span> ({boundEngineType})</span> : null}
                      {unavailable ? <span data-vector-store-unavailable="" style={{ marginLeft: '0.4rem', color: '#b42318', fontWeight: 600 }}>{t('vectorStoreBadge.unavailable')}</span> : null}
                    </span>
                    {unavailable ? <p data-vector-store-unavailable-hint="" className="wk-muted" style={{ margin: 0, color: '#b54708' }}>{t('kbSettings.vectorStore.unavailableHint')}</p> : null}
                  </div>
                )}
              />
              {editorOptions.error ? <StatusComponent tone="error">{editorOptions.error}</StatusComponent> : null}
            </div>
          );
        })()
      ) : null}
      {section === 'parser' ? (
        <ParserSettingsSection
          engines={editorOptions.parserEngines as ParserEngineInfo[]}
          loading={editorOptions.loading}
          error={editorOptions.error}
          rules={parserEngineRules}
          onChange={onParserEngineRules}
          t={t}
        />
      ) : null}
      {section === 'storage' ? (
        // R485 (Vue KBStorageSettings): the instanceDesc row, the option tags
        // (uppercased provider + 默认 on the default backend), the selected
        // instance endpoint/bucket hint and the 管理存储实例 entry.
        (() => {
          const currentBackendId = editorDraft.storageBackendId ?? editorPayload.storageBackendId;
          const selectedBackend = editorOptions.storageBackends.find((candidate) => candidate.id === currentBackendId);
          const selectedHint = selectedBackend
            ? text((selectedBackend.config ?? {}).endpoint) || text((selectedBackend.config ?? {}).bucket_name) || text((selectedBackend.config ?? {}).path_prefix) || t('kbSettings.storage.localStorage')
            : '';
          return (
            <div style={{ display: 'grid', gap: '0.4rem' }}>
              <EditorSettingRow
                label={t('kbSettings.storage.instanceLabel')}
                description={t('kbSettings.storage.instanceDesc')}
                control={(
                  <div style={{ display: 'grid', gap: '0.4rem', justifyItems: 'stretch' }}>
                    {/* Vue KBStorageSettings: the select binds :disabled="!!hasFiles"
                        and handleChange emits the backend id + provider (both persist
                        through the config PUT). The committed value stays selectable
                        even when the loaded backends list no longer contains it. */}
                    <select
                      value={currentBackendId}
                      disabled={indexingLocked}
                      aria-label={t('kbSettings.storage.instanceLabel')}
                      onChange={(event) => {
                        const backendId = event.target.value;
                        const backend = editorOptions.storageBackends.find((candidate) => candidate.id === backendId);
                        onDraftChange({ ...editorDraft, storageBackendId: backendId, storageProvider: backend?.provider ?? editorDraft.storageProvider });
                      }}
                    >
                      {editorOptions.storageBackends.map((backend) => <option key={backend.id} value={backend.id}>{backend.name} · {backend.provider.toUpperCase()}{backend.id === editorOptions.defaultStorageBackendId ? ` (${t('kbSettings.storage.defaultTag')})` : ''}</option>)}
                      {editorOptions.storageBackends.every((backend) => backend.id !== editorPayload.storageBackendId) ? <option value={editorPayload.storageBackendId}>{summaryLabel || editorPayload.storageBackendId}</option> : null}
                    </select>
                    {/* Vue KBStorageSettings renders the migrate hint only while the
                        edit-mode KB has files (v-if="props.hasFiles"), otherwise the
                        selected instance endpoint/bucket hint. */}
                    {indexingLocked ? <p className="wk-muted" data-storage-migrate-hint="" style={{ margin: 0 }}>{t('kbSettings.storage.migrateHint')}</p> : null}
                    {!indexingLocked && selectedHint ? <p className="wk-muted" data-storage-instance-hint="" style={{ margin: 0 }}>{selectedHint}</p> : null}
                    <a data-storage-manage-instances="" href="/settings?section=storage" style={{ fontSize: '0.85rem' }}>{t('kbSettings.storage.manageInstances')}</a>
                  </div>
                )}
              />
              {editorOptions.error ? <StatusComponent tone="error">{editorOptions.error}</StatusComponent> : null}
            </div>
          );
        })()
      ) : null}
      {section === 'activity' ? (
        client && knowledgeBaseId
          ? <KnowledgeBaseActivityPanel client={client} knowledgeBaseId={knowledgeBaseId} embedded />
          : <p className="wk-muted" style={{ margin: 0 }}>{t('kbSettings.summary.activity.sectionEmpty')}</p>
      ) : null}
      {section === 'datasource' ? (
        client && knowledgeBaseId
          ? <div style={{ maxHeight: '34rem', overflow: 'auto' }}><DataSourcesPage client={client} knowledgeBaseId={knowledgeBaseId} canManage={canManage} embedded /></div>
          : <p className="wk-muted" style={{ margin: 0 }}>{t('kbSettings.summary.datasource.sectionEmpty')}</p>
      ) : null}
      {section === 'share' ? (
        // R485 (Vue KBShareSettings): the shared-to list form — count badge,
        // search, add-share popup, per-row permission select and unshare
        // confirm — replaces the React ShareDialog form.
        client && knowledgeBaseId
          ? <KBShareSettingsSection client={client} knowledgeBaseId={knowledgeBaseId} canShare={canManage} />
          : <p className="wk-muted" style={{ margin: 0 }}>{t('kbSettings.summary.share.sectionEmpty')}</p>
      ) : null}
      {section === 'graph' ? <GraphSettings graphExtract={graphExtract} modelId={modelId} client={client} embedded onChange={onGraphChange} /> : null}
    </div>
  );
}

// ---- R439 editor sections (Vue KBModelConfig / KBChunkingSettings / KBAdvancedSettings) ----

interface EditorSectionProps {
  editorPayload: KnowledgeSettingsSavePayload;
  editorDraft: KnowledgeSettingsEditorOverrides;
  t: (key: string) => string;
  onDraftChange: (value: KnowledgeSettingsEditorOverrides) => void;
}

// Vue .setting-row layout: info column (label + desc) and control column —
// R490 extraction: shared with the KB editor drawer via chunkingSection.tsx.

// Vue basic section (KnowledgeBaseEditorModal.vue `currentSection ===
// 'basic'`): the committed KB id with copy, the immutable type radios, the
// document-only indexing-strategy checks with the conditional wiki tunables,
// and the required name plus description editors. All values persist through
// the base KB update (Vue doSubmit step 1), not the config PUT. While the KB
// already has files (Vue isIndexingLocked) the indexing checks disable and the
// lockedTip renders — a non-empty KB must keep at least one index.
function BasicSettingsSection({ knowledgeBase, editorDraft, indexingLocked, t, onDraftChange }: { knowledgeBase: KnowledgeSettingsInput; editorDraft: KnowledgeSettingsEditorOverrides; indexingLocked: boolean; t: (key: string) => string; onDraftChange: (value: KnowledgeSettingsEditorOverrides) => void }) {
  const name = editorDraft.name ?? (typeof knowledgeBase.name === 'string' ? knowledgeBase.name : '');
  const description = editorDraft.description ?? (typeof knowledgeBase.description === 'string' ? knowledgeBase.description : '');
  const indexing = resolveKnowledgeSettingsIndexing(knowledgeBase, editorDraft);
  const wiki = editorDraft.wiki ?? {};
  const granularity = wiki.extractionGranularity ?? resolveKnowledgeSettingsGranularity(knowledgeBase.wiki_config?.extraction_granularity);
  const contentInstructions = wiki.contentInstructions ?? (typeof knowledgeBase.wiki_config?.content_instructions === 'string' ? knowledgeBase.wiki_config.content_instructions : '');
  const extractionInstructions = wiki.extractionInstructions ?? (typeof knowledgeBase.wiki_config?.extraction_instructions === 'string' ? knowledgeBase.wiki_config.extraction_instructions : '');
  const setIndexing = (patch: NonNullable<KnowledgeSettingsEditorOverrides['indexing']>) => {
    onDraftChange({ ...editorDraft, indexing: { ...editorDraft.indexing, ...patch } });
  };
  const setWiki = (patch: NonNullable<KnowledgeSettingsEditorOverrides['wiki']>) => {
    onDraftChange({ ...editorDraft, wiki: { ...editorDraft.wiki, ...patch } });
  };
  const copyKbId = () => {
    void navigator.clipboard?.writeText(knowledgeBase.id).catch(() => undefined);
  };
  const granularityHintKey = granularity === 'focused'
    ? 'knowledgeEditor.wiki.granularityFocusedHint'
    : granularity === 'exhaustive'
      ? 'knowledgeEditor.wiki.granularityExhaustiveHint'
      : 'knowledgeEditor.wiki.granularityStandardHint';
  const radio = (groupKey: string, labelKey: string, checked: boolean, onChange: () => void, disabled?: boolean) => (
    <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem', marginRight: '1rem' }}>
      <input type="radio" name={groupKey} aria-label={t(labelKey)} checked={checked} disabled={disabled} onChange={onChange} />
      {t(labelKey)}
    </label>
  );
  return (
    <div>
      {knowledgeBase.id ? (
        <EditorSettingRow
          label={t('knowledgeEditor.basic.kbId')}
          description={t('knowledgeEditor.basic.kbIdDesc')}
          control={(
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem' }}>
              <code style={{ background: '#f3f5f9', borderRadius: 6, padding: '0.15rem 0.5rem' }}>{knowledgeBase.id}</code>
              <button
                type="button"
                aria-label={t('common.copy')}
                title={t('common.copy')}
                style={{ background: 'transparent', border: 'none', cursor: 'pointer', padding: '0.15rem', color: 'inherit' }}
                onClick={copyKbId}
              >
                {navIcon(<><rect x="9" y="9" width="12" height="12" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></>)}
              </button>
            </span>
          )}
        />
      ) : null}
      <EditorSettingRow
        label={t('knowledgeEditor.basic.typeLabel')}
        required
        description={t('knowledgeEditor.basic.typeDescription')}
        control={(
          <div role="radiogroup" aria-label={t('knowledgeEditor.basic.typeLabel')}>
            {radio('kbType', 'knowledgeEditor.basic.typeDocument', knowledgeBase.type?.toLowerCase() !== 'faq', () => undefined, true)}
            {radio('kbType', 'knowledgeEditor.basic.typeFAQ', knowledgeBase.type?.toLowerCase() === 'faq', () => undefined, true)}
          </div>
        )}
      />
      {knowledgeBase.type?.toLowerCase() !== 'faq' ? (
        <>
          <EditorSettingRow
            label={t('knowledgeEditor.indexing.title')}
            required
            description={t('knowledgeEditor.indexing.description')}
            control={(
              <div style={{ display: 'grid', gap: '0.5rem' }}>
                <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', ...(indexingLocked ? { opacity: 0.6 } : {}) }}>
                  <input
                    type="checkbox"
                    aria-label={t('knowledgeEditor.indexing.searchTitle')}
                    checked={indexing.vectorEnabled}
                    disabled={indexingLocked}
                    onChange={(event) => {
                      // Vue toggleVectorIndexing early-returns while locked.
                      if (indexingLocked) return;
                      setIndexing({ vectorEnabled: event.target.checked, keywordEnabled: event.target.checked });
                    }}
                  />
                  {t('knowledgeEditor.indexing.searchTitle')}
                </label>
                <p className="wk-muted" style={{ margin: 0, fontSize: '0.85rem' }}>{t('knowledgeEditor.indexing.searchDesc')}</p>
                <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', ...(indexingLocked ? { opacity: 0.6 } : {}) }}>
                  <input
                    type="checkbox"
                    aria-label={t('knowledgeEditor.indexing.wikiTitle')}
                    checked={indexing.wikiEnabled}
                    disabled={indexingLocked}
                    onChange={(event) => {
                      // Vue toggleWikiIndexing early-returns while locked.
                      if (indexingLocked) return;
                      setIndexing({ wikiEnabled: event.target.checked });
                    }}
                  />
                  {t('knowledgeEditor.indexing.wikiTitle')}
                </label>
                <p className="wk-muted" style={{ margin: 0, fontSize: '0.85rem' }}>{t('knowledgeEditor.indexing.wikiDesc')}</p>
                {indexingLocked ? (
                  <p className="wk-muted" data-indexing-locked-tip="" style={{ margin: 0, fontSize: '0.85rem' }}>{t('knowledgeEditor.indexing.lockedTip')}</p>
                ) : null}
              </div>
            )}
          />
          {indexing.wikiEnabled ? (
            <>
              <EditorSettingRow
                label={t('knowledgeEditor.wiki.extractionGranularityLabel')}
                description={t('knowledgeEditor.wiki.extractionGranularityTip')}
                control={(
                  <div role="radiogroup" aria-label={t('knowledgeEditor.wiki.extractionGranularityLabel')}>
                    {radio('kbWikiGranularity', 'knowledgeEditor.wiki.granularityFocused', granularity === 'focused', () => setWiki({ extractionGranularity: 'focused' }))}
                    {radio('kbWikiGranularity', 'knowledgeEditor.wiki.granularityStandard', granularity === 'standard', () => setWiki({ extractionGranularity: 'standard' }))}
                    {radio('kbWikiGranularity', 'knowledgeEditor.wiki.granularityExhaustive', granularity === 'exhaustive', () => setWiki({ extractionGranularity: 'exhaustive' }))}
                  </div>
                )}
              />
              <p className="wk-muted" style={{ margin: '0 0 0.6rem', fontSize: '0.85rem' }}>{t(granularityHintKey)}</p>
              <EditorSettingRow
                label={t('knowledgeEditor.wiki.contentInstructionsLabel')}
                description={t('knowledgeEditor.wiki.contentInstructionsTip')}
                control={(
                  <textarea
                    aria-label={t('knowledgeEditor.wiki.contentInstructionsLabel')}
                    maxLength={4000}
                    rows={3}
                    placeholder={t('knowledgeEditor.wiki.contentInstructionsPlaceholder')}
                    value={contentInstructions}
                    onChange={(event) => setWiki({ contentInstructions: event.target.value })}
                  />
                )}
              />
              <EditorSettingRow
                label={t('knowledgeEditor.wiki.extractionInstructionsLabel')}
                description={t('knowledgeEditor.wiki.extractionInstructionsTip')}
                control={(
                  <textarea
                    aria-label={t('knowledgeEditor.wiki.extractionInstructionsLabel')}
                    maxLength={4000}
                    rows={3}
                    placeholder={t('knowledgeEditor.wiki.extractionInstructionsPlaceholder')}
                    value={extractionInstructions}
                    onChange={(event) => setWiki({ extractionInstructions: event.target.value })}
                  />
                )}
              />
            </>
          ) : null}
        </>
      ) : null}
      <EditorSettingRow
        label={t('knowledgeEditor.basic.nameLabel')}
        required
        control={(
          <input
            aria-label={t('knowledgeEditor.basic.nameLabel')}
            maxLength={50}
            placeholder={t('knowledgeEditor.basic.namePlaceholder')}
            value={name}
            onChange={(event) => onDraftChange({ ...editorDraft, name: event.target.value })}
          />
        )}
      />
      <EditorSettingRow
        label={t('knowledgeEditor.basic.descriptionLabel')}
        control={(
          <textarea
            aria-label={t('knowledgeEditor.basic.descriptionLabel')}
            maxLength={200}
            rows={3}
            placeholder={t('knowledgeEditor.basic.descriptionPlaceholder')}
            value={description}
            onChange={(event) => onDraftChange({ ...editorDraft, description: event.target.value })}
          />
        )}
      />
    </div>
  );
}

// Vue KBModelConfig: llm (KnowledgeQA) and embedding (Embedding) selectors fed
// by the live model catalogue, unavailable models excluded (modelDefaults).
// Vue KBModelConfig (KnowledgeBaseEditorModal.vue `currentSection ===
// 'models'`): LLM plus Embedding selectors. R444: the same probe signal as the
// basic indexing lock (edit-mode hasFiles) also binds the Embedding row —
// `:disabled="ragEnabled && hasFiles"` with the knowledgeEditor.models.
// embeddingLocked warning in the info column. With every index strategy off
// (ragEnabled false) Vue keeps the selector editable even though the basic
// checks stay locked, so ragEnabled resolves from the live draft strategy.
function ModelsSettingsSection({ editorPayload, knowledgeBase, editorDraft, indexingLocked, models, t, onDraftChange }: EditorSectionProps & { knowledgeBase: KnowledgeSettingsInput; indexingLocked: boolean; models: KnowledgeSettingsModelOption[] }) {
  const optionLabel = (model: KnowledgeSettingsModelOption) => model.displayName || model.name;
  const indexing = resolveKnowledgeSettingsIndexing(knowledgeBase, editorDraft);
  const ragEnabled = Boolean(indexing.vectorEnabled || indexing.keywordEnabled);
  const embeddingLocked = ragEnabled && indexingLocked;
  const renderSelector = (type: string, labelKey: string, placeholderKey: string, value: string, onChange: (next: string) => void, required?: boolean, options: { disabled?: boolean; alert?: ReactNode; description?: string } = {}) => (
    <EditorSettingRow
      label={t(labelKey)}
      alert={options.alert}
      description={options.description}
      required={required}
      control={(
        <select value={value} aria-label={t(labelKey)} disabled={options.disabled} onChange={(event) => onChange(event.target.value)}>
          <option value="">{t(placeholderKey)}</option>
          {filterKnowledgeSettingsModels(models, type).map((model) => (
            <option key={model.id} value={model.id}>{optionLabel(model)}</option>
          ))}
        </select>
      )}
    />
  );
  return (
    <div>
      {/* R490 #10 (Vue KBModelConfig.vue:10-14): the LLM row carries the
          knowledgeEditor.models.llmDesc help line under the label — the
          R489 residual was the whole desc line missing on this section. */}
      {renderSelector('KnowledgeQA', 'knowledgeEditor.models.llmLabel', 'knowledgeEditor.models.llmPlaceholder', editorPayload.llmModelId, (next) => onDraftChange({ ...editorDraft, llmModelId: next }), true, { description: t('knowledgeEditor.models.llmDesc') })}
      {/* Vue KBModelConfig v-if="ragEnabled !== false || wikiEnabled": the row
          disappears for a pure-LLM draft; wiki-only keeps it with the optional
          copy and no required mark (clearable, validateForm waives the model). */}
      {ragEnabled || indexing.wikiEnabled ? (
        renderSelector('Embedding', 'knowledgeEditor.models.embeddingLabel', 'knowledgeEditor.models.embeddingPlaceholder', editorPayload.embeddingModelId, (next) => onDraftChange({ ...editorDraft, embeddingModelId: next }), ragEnabled, {
          disabled: embeddingLocked,
          // Vue KBModelConfig.vue:36-40 — (wikiEnabled && ragEnabled === false)
          // ? embeddingWikiOptionalDesc : embeddingDesc.
          description: !ragEnabled && indexing.wikiEnabled ? t('knowledgeEditor.models.embeddingWikiOptionalDesc') : t('knowledgeEditor.models.embeddingDesc'),
          alert: embeddingLocked ? (
            <p className="wk-muted" data-embedding-locked-tip="" style={{ margin: '0.5rem 0 0', fontSize: '0.85rem' }}>{t('knowledgeEditor.models.embeddingLocked')}</p>
          ) : undefined,
        })
      ) : null}
    </div>
  );
}

// Vue KBChunkingSettings: strategy select with the debug-drawer trigger beside
// it, size/overlap sliders with the overlap warning, the separator chips field,
// parent-child sliders, and a collapsed token/language panel — the shared
// Vue-form implementation in chunkingSection.tsx (R490 extraction).
function ChunkingSettingsSection({ editorPayload, editorDraft, client, t, onDraftChange }: EditorSectionProps & { client?: WeKnoraClient }) {
  return (
    <ChunkingSettingsFields
      splitting={editorPayload.documentSplitting}
      onPatch={(patch) => onDraftChange({ ...editorDraft, documentSplitting: { ...editorDraft.documentSplitting, ...patch } })}
      client={client}
      t={t}
    />
  );
}


// Vue KBAdvancedSettings: the question-generation block only renders while
// RAG indexing (vector|keyword) is enabled (v-if="ragEnabled !== false"); the
// auto-tag block and the table-metadata instructions row always render and
// persist through the base update (auto_tag_config) / config PUT
// (documentSplitting.tableMetadataInstructions) respectively.
const AUTO_TAG_MAX_TAGS_RANGE = { min: 1, max: 10 } as const;
function clampAutoTagMaxTags(value: number): number {
  return Math.min(AUTO_TAG_MAX_TAGS_RANGE.max, Math.max(AUTO_TAG_MAX_TAGS_RANGE.min, Math.trunc(value) || 3));
}

function AdvancedSettingsSection({ editorPayload, editorDraft, knowledgeBase, models, t, onDraftChange }: EditorSectionProps & { knowledgeBase: KnowledgeSettingsInput; models: KnowledgeSettingsModelOption[] }) {
  const questionGeneration = editorPayload.questionGeneration;
  const set = (patch: NonNullable<KnowledgeSettingsEditorOverrides['questionGeneration']>) => {
    onDraftChange({ ...editorDraft, questionGeneration: { ...editorDraft.questionGeneration, ...patch } });
  };
  // Vue loadKBData defaults for a KB row without auto_tag_config.
  const committedAutoTag = knowledgeBase.auto_tag_config ?? {};
  const autoTag = {
    enabled: editorDraft.autoTag?.enabled ?? committedAutoTag.enabled === true,
    modelId: editorDraft.autoTag?.modelId ?? (typeof committedAutoTag.model_id === 'string' ? committedAutoTag.model_id : ''),
    maxTags: editorDraft.autoTag?.maxTags ?? (typeof committedAutoTag.max_tags === 'number' && committedAutoTag.max_tags > 0 ? committedAutoTag.max_tags : 3),
    skipIfTagged: editorDraft.autoTag?.skipIfTagged ?? (typeof committedAutoTag.skip_if_tagged === 'boolean' ? committedAutoTag.skip_if_tagged : true),
  };
  const setAutoTag = (patch: NonNullable<KnowledgeSettingsEditorOverrides['autoTag']>) => {
    onDraftChange({ ...editorDraft, autoTag: { ...editorDraft.autoTag, ...patch } });
  };
  const tableMetadataInstructions = editorDraft.documentSplitting?.tableMetadataInstructions ?? editorPayload.documentSplitting.tableMetadataInstructions;
  const setTableMetadataInstructions = (value: string) => {
    onDraftChange({ ...editorDraft, documentSplitting: { ...editorDraft.documentSplitting, tableMetadataInstructions: value } });
  };
  // Vue ragEnabled prop: formData.indexingStrategy vector|keyword; a document
  // base without an explicit strategy defaults to vector+keyword (RAG on).
  const indexing = resolveKnowledgeSettingsIndexing(knowledgeBase, editorDraft);
  const ragEnabled = indexing.vectorEnabled || indexing.keywordEnabled;
  const classificationModels = filterKnowledgeSettingsModels(models, 'KnowledgeQA');
  return (
    <div>
      {ragEnabled ? (
        <>
          <EditorSettingRow
            label={t('knowledgeEditor.advanced.questionGeneration.label')}
            description={t('knowledgeEditor.advanced.questionGeneration.description')}
            control={(
              <input
                type="checkbox"
                aria-label={t('knowledgeEditor.advanced.questionGeneration.label')}
                checked={questionGeneration.enabled}
                onChange={(event) => set({ enabled: event.target.checked })}
              />
            )}
          />
          {questionGeneration.enabled ? (
            <>
              <EditorSettingRow
                label={t('knowledgeEditor.advanced.questionGeneration.countLabel')}
                control={(
                  <input
                    type="number"
                    aria-label={t('knowledgeEditor.advanced.questionGeneration.countLabel')}
                    min={QUESTION_COUNT_RANGE.min}
                    max={QUESTION_COUNT_RANGE.max}
                    step={QUESTION_COUNT_RANGE.step}
                    value={clampQuestionCount(questionGeneration.questionCount)}
                    onChange={(event) => set({ questionCount: clampQuestionCount(Number(event.target.value)) })}
                  />
                )}
              />
              <EditorSettingRow
                label={t('knowledgeEditor.advanced.questionGeneration.instructionsLabel')}
                control={(
                  <textarea
                    aria-label={t('knowledgeEditor.advanced.questionGeneration.instructionsLabel')}
                    maxLength={4000}
                    rows={3}
                    placeholder={t('knowledgeEditor.advanced.questionGeneration.instructionsPlaceholder')}
                    value={questionGeneration.customInstructions}
                    onChange={(event) => set({ customInstructions: event.target.value })}
                  />
                )}
              />
            </>
          ) : null}
        </>
      ) : null}
      <EditorSettingRow
        label={t('knowledgeEditor.advanced.autoTag.label')}
        description={t('knowledgeEditor.advanced.autoTag.description')}
        control={(
          <input
            type="checkbox"
            aria-label={t('knowledgeEditor.advanced.autoTag.label')}
            checked={autoTag.enabled}
            onChange={(event) => setAutoTag({ enabled: event.target.checked })}
          />
        )}
      />
      {autoTag.enabled ? (
        <div style={{ display: 'grid', gap: '0.4rem', padding: '0.75rem 1rem', marginTop: '0.5rem', background: '#f7f9fc', borderLeft: '3px solid var(--accent, #0052d9)', borderRadius: '6px' }}>
          <EditorSettingRow
            label={t('knowledgeEditor.advanced.autoTag.modelLabel')}
            description={t('knowledgeEditor.advanced.autoTag.modelDescription')}
            control={(
              <select
                data-autotag-model=""
                aria-label={t('knowledgeEditor.advanced.autoTag.modelLabel')}
                value={autoTag.modelId}
                onChange={(event) => setAutoTag({ modelId: event.target.value })}
              >
                {/* Vue ModelSelector clearable: the empty option maps to "use
                    the KB summary model" (modelDescription). */}
                <option value="">{t('knowledgeEditor.advanced.autoTag.modelPlaceholder')}</option>
                {classificationModels.map((model) => <option key={model.id} value={model.id}>{model.displayName || model.name}</option>)}
              </select>
            )}
          />
          <EditorSettingRow
            label={t('knowledgeEditor.advanced.autoTag.maxTagsLabel')}
            description={t('knowledgeEditor.advanced.autoTag.maxTagsDescription')}
            control={(
              <input
                type="number"
                data-autotag-max-tags=""
                aria-label={t('knowledgeEditor.advanced.autoTag.maxTagsLabel')}
                min={AUTO_TAG_MAX_TAGS_RANGE.min}
                max={AUTO_TAG_MAX_TAGS_RANGE.max}
                step={1}
                value={clampAutoTagMaxTags(autoTag.maxTags)}
                onChange={(event) => setAutoTag({ maxTags: clampAutoTagMaxTags(Number(event.target.value)) })}
              />
            )}
          />
          <EditorSettingRow
            label={t('knowledgeEditor.advanced.autoTag.skipIfTaggedLabel')}
            description={t('knowledgeEditor.advanced.autoTag.skipIfTaggedDescription')}
            control={(
              <input
                type="checkbox"
                aria-label={t('knowledgeEditor.advanced.autoTag.skipIfTaggedLabel')}
                checked={autoTag.skipIfTagged}
                onChange={(event) => setAutoTag({ skipIfTagged: event.target.checked })}
              />
            )}
          />
        </div>
      ) : null}
      <EditorSettingRow
        label={t('knowledgeEditor.advanced.tableMetadataInstructions.label')}
        description={t('knowledgeEditor.advanced.tableMetadataInstructions.description')}
        control={(
          <textarea
            aria-label={t('knowledgeEditor.advanced.tableMetadataInstructions.label')}
            maxLength={4000}
            rows={3}
            placeholder={t('knowledgeEditor.advanced.tableMetadataInstructions.placeholder')}
            value={tableMetadataInstructions}
            onChange={(event) => setTableMetadataInstructions(event.target.value)}
          />
        )}
      />
    </div>
  );
}

// Vue multimodal section (KnowledgeBaseEditorModal.vue `currentSection ===
// 'multimodal'`): toggle + conditional VLLM selector, description language and
// custom instructions rows, all fed by the live model catalogue.
const MULTIMODAL_DESCRIPTION_LANGUAGE_OPTIONS = [
  { value: 'Chinese', labelKey: 'language.zhCN' },
  { value: 'English', labelKey: 'language.enUS' },
  { value: 'Korean', labelKey: 'language.koKR' },
  { value: 'Russian', labelKey: 'language.ruRU' },
] as const;

function MultimodalSettingsSection({ editorPayload, editorDraft, models, t, onDraftChange }: EditorSectionProps & { models: KnowledgeSettingsModelOption[] }) {
  const multimodal = editorPayload.vlm_config;
  const draft = editorDraft.multimodal ?? {};
  const set = (patch: NonNullable<KnowledgeSettingsEditorOverrides['multimodal']>) => {
    onDraftChange({ ...editorDraft, multimodal: { ...editorDraft.multimodal, ...patch } });
  };
  const enabled = draft.enabled ?? multimodal.enabled;
  const vllmModelId = draft.vllmModelId ?? multimodal.model_id;
  const descriptionLanguage = draft.descriptionLanguage ?? multimodal.description_language;
  const customInstructions = draft.customInstructions ?? multimodal.custom_instructions;
  return (
    <div>
      <EditorSettingRow
        label={t('knowledgeEditor.advanced.multimodal.label')}
        description={t('knowledgeEditor.advanced.multimodal.description')}
        control={(
          <input
            type="checkbox"
            aria-label={t('knowledgeEditor.advanced.multimodal.label')}
            checked={enabled}
            onChange={(event) => set({ enabled: event.target.checked })}
          />
        )}
      />
      {enabled ? (
        <>
          <EditorSettingRow
            label={t('knowledgeEditor.advanced.multimodal.vllmLabel')}
            description={t('knowledgeEditor.advanced.multimodal.vllmDescription')}
            required
            control={(
              <select
                aria-label={t('knowledgeEditor.advanced.multimodal.vllmLabel')}
                value={vllmModelId}
                onChange={(event) => set({ vllmModelId: event.target.value })}
              >
                <option value="">{t('knowledgeEditor.advanced.multimodal.vllmPlaceholder')}</option>
                {filterKnowledgeSettingsModels(models, 'VLLM').map((model) => (
                  <option key={model.id} value={model.id}>{model.displayName || model.name}</option>
                ))}
              </select>
            )}
          />
          <EditorSettingRow
            label={t('knowledgeEditor.advanced.multimodal.descriptionLanguageLabel')}
            description={t('knowledgeEditor.advanced.multimodal.descriptionLanguageDescription')}
            control={(
              <select
                aria-label={t('knowledgeEditor.advanced.multimodal.descriptionLanguageLabel')}
                value={descriptionLanguage}
                onChange={(event) => set({ descriptionLanguage: event.target.value })}
              >
                <option value="">{t('knowledgeEditor.advanced.multimodal.descriptionLanguageAuto')}</option>
                {MULTIMODAL_DESCRIPTION_LANGUAGE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>{t(option.labelKey)}</option>
                ))}
              </select>
            )}
          />
          <EditorSettingRow
            label={t('knowledgeEditor.advanced.multimodal.customInstructionsLabel')}
            description={t('knowledgeEditor.advanced.multimodal.customInstructionsDescription')}
            control={(
              <textarea
                aria-label={t('knowledgeEditor.advanced.multimodal.customInstructionsLabel')}
                maxLength={4000}
                rows={3}
                placeholder={t('knowledgeEditor.advanced.multimodal.customInstructionsPlaceholder')}
                value={customInstructions}
                onChange={(event) => set({ customInstructions: event.target.value })}
              />
            )}
          />
        </>
      ) : null}
    </div>
  );
}

// Vue asr section: toggle + conditional ASR model selector. asr_config.language
// has no UI control in the Vue modal — it round-trips through the payload.
function AsrSettingsSection({ editorPayload, editorDraft, models, t, onDraftChange }: EditorSectionProps & { models: KnowledgeSettingsModelOption[] }) {
  const asr = editorPayload.asr_config;
  const draft = editorDraft.asr ?? {};
  const set = (patch: NonNullable<KnowledgeSettingsEditorOverrides['asr']>) => {
    onDraftChange({ ...editorDraft, asr: { ...editorDraft.asr, ...patch } });
  };
  const enabled = draft.enabled ?? asr.enabled;
  return (
    <div>
      <EditorSettingRow
        label={t('knowledgeEditor.asr.label')}
        description={t('knowledgeEditor.asr.desc')}
        control={(
          <input
            type="checkbox"
            aria-label={t('knowledgeEditor.asr.label')}
            checked={enabled}
            onChange={(event) => set({ enabled: event.target.checked })}
          />
        )}
      />
      {enabled ? (
        <EditorSettingRow
          label={t('knowledgeEditor.asr.modelLabel')}
          description={t('knowledgeEditor.asr.modelDescription')}
          required
          control={(
            <select
              aria-label={t('knowledgeEditor.asr.modelLabel')}
              value={draft.modelId ?? asr.model_id}
              onChange={(event) => set({ modelId: event.target.value })}
            >
              <option value="">{t('knowledgeEditor.asr.modelPlaceholder')}</option>
              {filterKnowledgeSettingsModels(models, 'ASR').map((model) => (
                <option key={model.id} value={model.id}>{model.displayName || model.name}</option>
              ))}
            </select>
          )}
        />
      ) : null}
    </div>
  );
}

// Vue faq section (FAQ bases only): the two index-mode radio groups plus the
// entry guide copy.
function FaqSettingsSection({ knowledgeBase, editorDraft, t, onDraftChange }: { knowledgeBase: KnowledgeSettingsInput; editorDraft: KnowledgeSettingsEditorOverrides; t: (key: string) => string; onDraftChange: (value: KnowledgeSettingsEditorOverrides) => void }) {
  const kbFaq = knowledgeBase.faq_config ?? {};
  const draft = editorDraft.faqConfig ?? {};
  const indexMode = draft.indexMode ?? (typeof kbFaq.index_mode === 'string' && kbFaq.index_mode ? kbFaq.index_mode : 'question_only');
  const questionIndexMode = draft.questionIndexMode ?? (typeof kbFaq.question_index_mode === 'string' && kbFaq.question_index_mode ? kbFaq.question_index_mode : 'separate');
  const set = (patch: NonNullable<KnowledgeSettingsEditorOverrides['faqConfig']>) => {
    onDraftChange({ ...editorDraft, faqConfig: { ...editorDraft.faqConfig, ...patch } });
  };
  const radio = (groupLabelKey: string, labelKey: string, checked: boolean, onChange: () => void) => (
    <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem', marginRight: '1rem' }}>
      <input type="radio" name={groupLabelKey} aria-label={t(labelKey)} checked={checked} onChange={onChange} />
      {t(labelKey)}
    </label>
  );
  return (
    <div>
      <EditorSettingRow
        label={t('knowledgeEditor.faq.indexModeLabel')}
        required
        control={(
          <div role="radiogroup" aria-label={t('knowledgeEditor.faq.indexModeLabel')}>
            {radio('faqIndexMode', 'knowledgeEditor.faq.modes.questionOnly', indexMode === 'question_only', () => set({ indexMode: 'question_only' }))}
            {radio('faqIndexMode', 'knowledgeEditor.faq.modes.questionAnswer', indexMode === 'question_answer', () => set({ indexMode: 'question_answer' }))}
          </div>
        )}
      />
      <p className="wk-muted" style={{ margin: '0 0 0.6rem', fontSize: '0.85rem' }}>{t('knowledgeEditor.faq.indexModeDescription')}</p>
      <EditorSettingRow
        label={t('knowledgeEditor.faq.questionIndexModeLabel')}
        required
        control={(
          <div role="radiogroup" aria-label={t('knowledgeEditor.faq.questionIndexModeLabel')}>
            {radio('faqQuestionIndexMode', 'knowledgeEditor.faq.modes.combined', questionIndexMode === 'combined', () => set({ questionIndexMode: 'combined' }))}
            {radio('faqQuestionIndexMode', 'knowledgeEditor.faq.modes.separate', questionIndexMode === 'separate', () => set({ questionIndexMode: 'separate' }))}
          </div>
        )}
      />
      <p className="wk-muted" style={{ margin: '0 0 0.6rem', fontSize: '0.85rem' }}>{t('knowledgeEditor.faq.questionIndexModeDescription')}</p>
      <p className="wk-muted" style={{ margin: 0 }}>{t('knowledgeEditor.faq.entryGuide')}</p>
    </div>
  );
}
