import type { DataSource, DataSourceResource } from '@weknora/api-client';

export type DataSourceWorkspaceRole = 'owner' | 'admin' | 'contributor' | 'viewer' | string | undefined;

/** Connector types explicitly supported by the Vue editor definition. */
export const supportedDataSourceTypes = new Set(['feishu', 'lark', 'feishu_drive', 'lark_drive', 'notion', 'yuque', 'ima', 'rss', 'gitlab']);

export function filterSupportedDataSourceTypes<T extends { type: string }>(types: readonly T[]): T[] {
  return types.filter((type) => supportedDataSourceTypes.has(type.type));
}

/** Data-source mutations are restricted to the same workspace roles as the API. */
export function canManageDataSources(role: DataSourceWorkspaceRole): boolean {
  return role === 'owner' || role === 'admin';
}

/** Accepts a bare Drive folder token or a Feishu/Lark Drive folder URL. */
export function extractDriveFolderToken(input: string): string {
  const raw = input.trim();
  if (!raw) return '';
  if (!raw.includes('://') && !raw.includes('/')) return raw;
  const match = raw.match(/\/drive\/folder\/([^/?#]+)/);
  if (match?.[1]) return match[1];
  try {
    const pathname = new URL(raw).pathname.split('/').filter(Boolean);
    return pathname.at(-1) ?? raw;
  } catch { return raw; }
}

export type DataSourceCredentialField = { key: string; label: string; placeholder: string; secret?: boolean; optional?: boolean };

const credentialFields: Record<string, DataSourceCredentialField[]> = {
  feishu: [{ key: 'app_id', label: 'App ID', placeholder: 'cli_xxxx' }, { key: 'app_secret', label: 'App secret', placeholder: '', secret: true }, { key: 'base_url', label: 'Base URL', placeholder: 'https://open.feishu.cn', optional: true }],
  lark: [{ key: 'app_id', label: 'App ID', placeholder: 'cli_xxxx' }, { key: 'app_secret', label: 'App secret', placeholder: '', secret: true }, { key: 'base_url', label: 'Base URL', placeholder: 'https://open.larksuite.com', optional: true }],
  feishu_drive: [{ key: 'app_id', label: 'App ID', placeholder: 'cli_xxxx' }, { key: 'app_secret', label: 'App secret', placeholder: '', secret: true }, { key: 'base_url', label: 'Base URL', placeholder: 'https://open.feishu.cn', optional: true }],
  lark_drive: [{ key: 'app_id', label: 'App ID', placeholder: 'cli_xxxx' }, { key: 'app_secret', label: 'App secret', placeholder: '', secret: true }, { key: 'base_url', label: 'Base URL', placeholder: 'https://open.larksuite.com', optional: true }],
  notion: [{ key: 'api_key', label: 'Integration token', placeholder: 'ntn_xxxx', secret: true }],
  yuque: [{ key: 'api_token', label: 'API token', placeholder: '', secret: true }, { key: 'base_url', label: 'Base URL', placeholder: 'https://www.yuque.com', optional: true }],
  ima: [{ key: 'client_id', label: 'Client ID', placeholder: '', secret: true }, { key: 'api_key', label: 'API key', placeholder: '', secret: true }, { key: 'base_url', label: 'Base URL', placeholder: 'https://ima.qq.com', optional: true }],
  gitlab: [{ key: 'base_url', label: 'Base URL', placeholder: 'https://gitlab.example.com' }, { key: 'access_token', label: 'Access token', placeholder: '', secret: true }],
};

export function dataSourceCredentialFields(type: string): DataSourceCredentialField[] { return credentialFields[type] ?? []; }

export function validateDataSourceCredentials(type: string, values: Record<string, string>): string[] {
  return dataSourceCredentialFields(type).filter((field) => !field.optional && !values[field.key]?.trim()).map((field) => `${field.label} is required`);
}

export type ResourceCheckState = 'checked' | 'indeterminate' | 'unchecked';

function descendants(resources: DataSourceResource[], id: string): string[] {
  const children = resources.filter((resource) => resource.parent_id === id);
  return children.flatMap((child) => [child.external_id, ...descendants(resources, child.external_id)]);
}

function ancestors(resources: DataSourceResource[], id: string): string[] {
  const parent = resources.find((resource) => resource.external_id === id)?.parent_id;
  return parent ? [parent, ...ancestors(resources, parent)] : [];
}

export function resourceCheckState(resources: DataSourceResource[], selectedIds: string[], id: string): ResourceCheckState {
  const selected = new Set(selectedIds);
  if (selected.has(id) || ancestors(resources, id).some((ancestor) => selected.has(ancestor))) return 'checked';
  return descendants(resources, id).some((descendant) => selected.has(descendant)) ? 'indeterminate' : 'unchecked';
}

/** Keeps selected resource IDs as a minimal cover set, matching the Vue tree. */
export function toggleDataSourceResourceSelection(resources: DataSourceResource[], selectedIds: string[], id: string): string[] {
  const cover = new Set(selectedIds);
  const state = resourceCheckState(resources, selectedIds, id);
  const childIds = new Set(descendants(resources, id));
  if (state === 'unchecked') {
    for (const selected of cover) if (childIds.has(selected)) cover.delete(selected);
    cover.add(id);
    return [...cover];
  }
  const chain = [id, ...ancestors(resources, id)];
  const covering = chain.find((ancestor) => cover.has(ancestor));
  if (covering && covering !== id) {
    cover.delete(covering);
    const path = [id, ...ancestors(resources, id)];
    const index = path.indexOf(covering);
    for (const branch of resources.filter((resource) => resource.parent_id === covering)) {
      if (branch.external_id !== path[index - 1]) cover.add(branch.external_id);
    }
  } else cover.delete(id);
  for (const selected of cover) if (childIds.has(selected)) cover.delete(selected);
  return [...cover];
}

export function dataSourceStatusLabel(source: Pick<DataSource, 'status'>): string {
  return typeof source.status === 'string' && source.status.trim() ? source.status : 'unknown';
}

export function safeDataSourceType(source: Pick<DataSource, 'type'>): string {
  return typeof source.type === 'string' && source.type.trim() ? source.type : 'unknown';
}
