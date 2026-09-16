import type { DataSource } from '@weknora/api-client';

export interface NativeDataSourceDraft {
  name: string;
  type: string;
  schedule: string;
  mode: 'incremental' | 'full';
  conflict: 'overwrite' | 'skip';
  deletions: boolean;
  credentialsText: string;
  settingsText: string;
}

export function parseNativeKeyValueLines(text: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [index, raw] of text.split(/\r?\n/).entries()) {
    const line = raw.trim();
    if (!line) continue;
    const separator = line.indexOf('=');
    if (separator <= 0) throw new Error(`Line ${index + 1} must use key=value`);
    const key = line.slice(0, separator).trim();
    const value = line.slice(separator + 1).trim();
    if (!key || !value) throw new Error(`Line ${index + 1} must include a key and value`);
    if (result[key] !== undefined) throw new Error(`Key ${key} is duplicated`);
    result[key] = value;
  }
  return result;
}

export function buildNativeDataSourceInput(draft: NativeDataSourceDraft, knowledgeBaseId: string): Partial<DataSource> {
  const name = draft.name.trim();
  const type = draft.type.trim();
  if (!name) throw new Error('Data source name is required');
  if (!type) throw new Error('Data source type is required');
  if (!knowledgeBaseId.trim()) throw new Error('Knowledge base is required');
  return {
    name,
    type,
    knowledge_base_id: knowledgeBaseId,
    sync_schedule: draft.schedule.trim(),
    sync_mode: draft.mode,
    conflict_strategy: draft.conflict,
    sync_deletions: draft.deletions,
    config: { credentials: parseNativeKeyValueLines(draft.credentialsText), settings: parseNativeKeyValueLines(draft.settingsText) },
  };
}

export function nativeDataSourceDraftFrom(source: DataSource): NativeDataSourceDraft {
  const config = source.config && typeof source.config === 'object' && !Array.isArray(source.config) ? source.config as Record<string, unknown> : {};
  const settings = config.settings && typeof config.settings === 'object' && !Array.isArray(config.settings) ? config.settings as Record<string, unknown> : {};
  return {
    name: source.name,
    type: source.type,
    schedule: source.sync_schedule ?? '',
    mode: source.sync_mode === 'full' ? 'full' : 'incremental',
    conflict: source.conflict_strategy === 'skip' ? 'skip' : 'overwrite',
    deletions: source.sync_deletions !== false,
    credentialsText: '',
    settingsText: Object.entries(settings).filter(([, value]) => typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean').map(([key, value]) => `${key} = ${String(value)}`).join('\n'),
  };
}
