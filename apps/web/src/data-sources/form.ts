import type { DataSource } from '@weknora/api-client';

export interface DataSourceFormValues {
  name: string;
  type: string;
  schedule: string;
  mode: 'incremental' | 'full';
  conflict: 'overwrite' | 'skip';
  deletions: boolean;
  credentialsText: string;
  settingsText: string;
  resourceIds: string[];
}

export function parseCredentialLines(text: string): Record<string, string> {
  const values: Record<string, string> = {};
  for (const [index, rawLine] of text.split(/\r?\n/).entries()) {
    const line = rawLine.trim();
    if (!line) continue;
    const separator = line.indexOf('=');
    if (separator <= 0) throw new Error(`Credential line ${index + 1} must use key=value`);
    const key = line.slice(0, separator).trim();
    const value = line.slice(separator + 1).trim();
    if (!key || !value) throw new Error(`Credential line ${index + 1} must include a key and value`);
    if (values[key] !== undefined) throw new Error(`Credential key ${key} is duplicated`);
    values[key] = value;
  }
  return values;
}

export function buildDataSourceInput(values: DataSourceFormValues): Partial<DataSource> {
  const name = values.name.trim();
  const type = values.type.trim();
  if (!name) throw new Error('Data source name is required');
  if (!type) throw new Error('Data source type is required');
  return {
    name,
    type,
    sync_schedule: values.schedule.trim(),
    sync_mode: values.mode,
    conflict_strategy: values.conflict,
    sync_deletions: values.deletions,
    config: { credentials: parseCredentialLines(values.credentialsText), settings: parseCredentialLines(values.settingsText), resource_ids: values.resourceIds },
  };
}

export function dataSourceFormFrom(source: DataSource): DataSourceFormValues {
  const config = (source.config && typeof source.config === 'object' && !Array.isArray(source.config)) ? source.config as Record<string, unknown> : {};
  const settings = config.settings && typeof config.settings === 'object' && !Array.isArray(config.settings) ? config.settings as Record<string, unknown> : {};
  const resourceIds = Array.isArray(config.resource_ids) ? config.resource_ids.filter((value): value is string => typeof value === 'string') : [];
  return {
    name: source.name,
    type: source.type,
    schedule: source.sync_schedule ?? '',
    mode: source.sync_mode === 'full' ? 'full' : 'incremental',
    conflict: source.conflict_strategy === 'skip' ? 'skip' : 'overwrite',
    deletions: source.sync_deletions !== false,
    credentialsText: '',
    settingsText: Object.entries(settings).filter(([, value]) => typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean').map(([key, value]) => `${key} = ${String(value)}`).join('\n'),
    resourceIds,
  };
}
