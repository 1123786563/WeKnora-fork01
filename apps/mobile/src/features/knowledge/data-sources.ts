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

/** Mirrors the Vue editor connector definitions (DataSourceEditorDialog.vue
 * lines 489-633): same keys, same technical placeholders, plus the i18n
 * labelKey/hintKey the Vue dialog resolves through t(). `label` stays as the
 * untranslated fallback for callers without a locale. */
export type DataSourceCredentialField = { key: string; labelKey: string; label: string; placeholder: string; secret?: boolean; optional?: boolean; hintKey?: string };

const credentialFields: Record<string, DataSourceCredentialField[]> = {
  feishu: [{ key: 'app_id', labelKey: 'dataSource.field.appId', label: 'App ID', placeholder: 'cli_xxxx' }, { key: 'app_secret', labelKey: 'dataSource.field.appSecret', label: 'App Secret', placeholder: '', secret: true }, { key: 'base_url', labelKey: 'dataSource.field.baseUrl', label: 'Base URL (optional)', placeholder: 'https://open.feishu.cn', optional: true, hintKey: 'dataSource.field.baseUrlHint' }],
  lark: [{ key: 'app_id', labelKey: 'dataSource.field.appId', label: 'App ID', placeholder: 'cli_xxxx' }, { key: 'app_secret', labelKey: 'dataSource.field.appSecret', label: 'App Secret', placeholder: '', secret: true }, { key: 'base_url', labelKey: 'dataSource.field.baseUrl', label: 'Base URL (optional)', placeholder: 'https://open.larksuite.com', optional: true, hintKey: 'dataSource.field.baseUrlHint' }],
  feishu_drive: [{ key: 'app_id', labelKey: 'dataSource.field.appId', label: 'App ID', placeholder: 'cli_xxxx' }, { key: 'app_secret', labelKey: 'dataSource.field.appSecret', label: 'App Secret', placeholder: '', secret: true }, { key: 'base_url', labelKey: 'dataSource.field.baseUrl', label: 'Base URL (optional)', placeholder: 'https://open.feishu.cn', optional: true, hintKey: 'dataSource.field.baseUrlHint' }],
  lark_drive: [{ key: 'app_id', labelKey: 'dataSource.field.appId', label: 'App ID', placeholder: 'cli_xxxx' }, { key: 'app_secret', labelKey: 'dataSource.field.appSecret', label: 'App Secret', placeholder: '', secret: true }, { key: 'base_url', labelKey: 'dataSource.field.baseUrl', label: 'Base URL (optional)', placeholder: 'https://open.larksuite.com', optional: true, hintKey: 'dataSource.field.baseUrlHint' }],
  notion: [{ key: 'api_key', labelKey: 'dataSource.field.integrationToken', label: 'Integration Token', placeholder: 'ntn_xxxx', secret: true }],
  yuque: [{ key: 'api_token', labelKey: 'dataSource.field.apiToken', label: 'API Token', placeholder: '', secret: true }, { key: 'base_url', labelKey: 'dataSource.field.baseUrl', label: 'Base URL (optional)', placeholder: 'https://www.yuque.com', optional: true, hintKey: 'dataSource.field.baseUrlHint' }],
  ima: [{ key: 'client_id', labelKey: 'dataSource.field.imaClientId', label: 'IMA ClientID', placeholder: '', secret: true }, { key: 'api_key', labelKey: 'dataSource.field.imaApiKey', label: 'IMA APIKey', placeholder: '', secret: true }, { key: 'base_url', labelKey: 'dataSource.field.baseUrl', label: 'Base URL (optional)', placeholder: 'https://ima.qq.com', optional: true, hintKey: 'dataSource.field.baseUrlHint' }],
  gitlab: [{ key: 'base_url', labelKey: 'dataSource.gitlab.baseUrl', label: 'GitLab URL', placeholder: 'https://gitlab.example.com' }, { key: 'access_token', labelKey: 'dataSource.gitlab.accessToken', label: 'Personal access token', placeholder: '', secret: true }],
};

export function dataSourceCredentialFields(type: string): DataSourceCredentialField[] { return credentialFields[type] ?? []; }

/** Required-but-empty credential fields; callers compose the localized
 * `${t(field.labelKey)} ${t('dataSource.isRequired')}` warning like the Vue
 * editor (DataSourceEditorDialog.vue:784). */
export function validateDataSourceCredentials(type: string, values: Record<string, string>): DataSourceCredentialField[] {
  return dataSourceCredentialFields(type).filter((field) => !field.optional && !values[field.key]?.trim());
}

export type ResourceCheckState = 'checked' | 'indeterminate' | 'unchecked';

export function resourceSelectionMarker(state: ResourceCheckState): string {
  return state === 'checked' ? '✓ ' : state === 'indeterminate' ? '− ' : '';
}

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

export function hasRunningSync(source: Pick<DataSource, 'latest_sync_log'>): boolean {
  return source.latest_sync_log?.status === 'running';
}

export function safeDataSourceType(source: Pick<DataSource, 'type'>): string {
  return typeof source.type === 'string' && source.type.trim() ? source.type : 'unknown';
}

/** formatMessage echoes a missing key back; fall back to the raw value so
 * unknown connector types/statuses never render as a dotted key. */
export function localizedOr(resolved: string, key: string, fallback: string): string {
  return resolved === key ? fallback : resolved;
}

/** i18n keys for the connector card labels (Vue connectorLabel / statusLabel /
 * syncModeLabel in DataSourceSettings.vue lines 126-137). */
export function dataSourceConnectorLabelKey(type: string): string {
  return `dataSource.connector.${safeDataSourceType({ type })}`;
}

export function dataSourceStatusLabelKey(status: string): string {
  return `dataSource.status.${dataSourceStatusLabel({ status })}`;
}

export function dataSourceSyncModeLabelKey(mode: string): string {
  return `dataSource.syncMode.${mode === 'full' ? 'full' : 'incremental'}`;
}
