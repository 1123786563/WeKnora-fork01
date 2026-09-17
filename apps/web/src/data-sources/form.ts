import type { DataSource } from '@weknora/api-client';

export interface HeaderRow { key: string; value: string }

// Vue DataSourceEditorDialog GitLabProjectInput: each projects row is edited as
// { project_id, ref, pathsText } where pathsText joins the stored paths array
// with newlines for the textarea.
export interface GitLabProjectInput { project_id: string; ref: string; pathsText: string }

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
  // rss custom auth headers (Vue fieldType 'custom_headers'): edited as
  // key-value rows, serialized into config.credentials.auth_headers.
  authHeaders?: HeaderRow[];
  // gitlab projects (Vue gitlabProjects ref): edited as structured rows and
  // injected into config.settings.projects on save — the key=value settingsText
  // protocol cannot carry an array of objects.
  gitlabProjects?: GitLabProjectInput[];
}

// Vue syncGitLabProjectsToSettings: drop rows without a project_id, trim
// project_id/ref (an empty ref survives as ''), split paths on newlines or
// commas and drop blank entries.
export function serializeGitLabProjects(projects: GitLabProjectInput[]): Array<{ project_id: string; ref: string; paths: string[] }> {
  return projects
    .filter((project) => project.project_id.trim())
    .map((project) => ({
      project_id: project.project_id.trim(),
      ref: project.ref.trim(),
      paths: project.pathsText.split(/[\n,]/).map((path) => path.trim()).filter(Boolean),
    }));
}

// Vue openEditor edit branch: hydrate the row editor from the saved
// settings.projects, rejoining the paths array into textarea text. A malformed
// (non-array) value degrades to no rows.
export function gitlabProjectsFromSettings(settings: Record<string, unknown>): GitLabProjectInput[] {
  const saved = Array.isArray(settings.projects) ? settings.projects : [];
  return saved.map((project) => {
    const row = (project && typeof project === 'object' ? project : {}) as Record<string, unknown>;
    return {
      project_id: String(row.project_id ?? ''),
      ref: String(row.ref ?? ''),
      pathsText: Array.isArray(row.paths) ? row.paths.filter((path): path is string => typeof path === 'string').join('\n') : '',
    };
  });
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
  const credentials = parseCredentialLines(values.credentialsText);
  // Vue syncRssAuthHeadersToCredentials: only the rss connector maps its
  // key-value rows into credentials.auth_headers; empty rows leave the
  // credentials object untouched.
  if (type === 'rss') {
    const serialized = serializeAuthHeaders(values.authHeaders ?? []);
    if (serialized) credentials.auth_headers = serialized;
  }
  // Vue buildConfigPayload runs syncGitLabProjectsToSettings on every save:
  // for gitlab the structured rows own settings.projects (a stale hand-typed
  // `projects = ...` line in settingsText is overridden), while scalar
  // settingsText keys survive untouched. Other connectors never grow the key.
  const settings: Record<string, unknown> = parseCredentialLines(values.settingsText);
  if (type === 'gitlab') {
    delete settings.projects;
    settings.projects = serializeGitLabProjects(values.gitlabProjects ?? []);
  }
  return {
    name,
    type,
    sync_schedule: values.schedule.trim(),
    sync_mode: values.mode,
    conflict_strategy: values.conflict,
    sync_deletions: values.deletions,
    config: { credentials, settings, resource_ids: values.resourceIds },
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
    // Vue resets rssAuthHeaders to [] on dialog open: stored auth_headers stay
    // server-side untouched unless the user opts in to Replace and retypes rows.
    authHeaders: [],
    // Vue openEditor edit branch: settings.projects (an array of objects) never
    // round-trips through the scalar settingsText lines — it hydrates the
    // structured gitlab row editor instead.
    gitlabProjects: gitlabProjectsFromSettings(settings),
  };
}

// Vue serializeAuthHeaders: drop rows with blank keys, trim keys, join the
// remaining rows as "Key: Value" lines (values verbatim).
export function serializeAuthHeaders(rows: HeaderRow[]): string {
  return rows.filter((item) => item.key.trim()).map((item) => `${item.key.trim()}: ${item.value}`).join('\n');
}

export type CredentialField = { key: string; label: string; placeholder?: string; secret?: boolean; optional?: boolean; hint?: string };

// Ported from the Vue DataSourceEditorDialog connectorDefs: per-connector
// credential fields with required/optional flags drive the field-level
// validation that runs before the connection test (validateStep1Fields).
// Labels, placeholders and hints mirror the Vue defs byte-for-byte.
export const VUE_CREDENTIAL_FIELDS: Record<string, CredentialField[]> = {
  feishu: [{ key: 'app_id', label: 'dataSource.field.appId', placeholder: 'cli_xxxx' }, { key: 'app_secret', label: 'dataSource.field.appSecret', secret: true }, { key: 'base_url', label: 'dataSource.field.baseUrl', placeholder: 'https://open.feishu.cn', optional: true, hint: 'dataSource.field.baseUrlHint' }],
  lark: [{ key: 'app_id', label: 'dataSource.field.appId', placeholder: 'cli_xxxx' }, { key: 'app_secret', label: 'dataSource.field.appSecret', secret: true }, { key: 'base_url', label: 'dataSource.field.baseUrl', placeholder: 'https://open.feishu.cn', optional: true, hint: 'dataSource.field.baseUrlHint' }],
  feishu_drive: [{ key: 'app_id', label: 'dataSource.field.appId', placeholder: 'cli_xxxx' }, { key: 'app_secret', label: 'dataSource.field.appSecret', secret: true }, { key: 'base_url', label: 'dataSource.field.baseUrl', placeholder: 'https://open.feishu.cn', optional: true, hint: 'dataSource.field.baseUrlHint' }],
  lark_drive: [{ key: 'app_id', label: 'dataSource.field.appId', placeholder: 'cli_xxxx' }, { key: 'app_secret', label: 'dataSource.field.appSecret', secret: true }, { key: 'base_url', label: 'dataSource.field.baseUrl', placeholder: 'https://open.feishu.cn', optional: true, hint: 'dataSource.field.baseUrlHint' }],
  notion: [{ key: 'api_key', label: 'dataSource.field.integrationToken', placeholder: 'ntn_xxxx', secret: true }],
  yuque: [{ key: 'api_token', label: 'dataSource.field.apiToken', secret: true }, { key: 'base_url', label: 'dataSource.field.baseUrl', placeholder: 'https://www.yuque.com', optional: true, hint: 'dataSource.field.baseUrlHint' }],
  ima: [{ key: 'client_id', label: 'dataSource.field.imaClientId', secret: true }, { key: 'api_key', label: 'dataSource.field.imaApiKey', secret: true }, { key: 'base_url', label: 'dataSource.field.baseUrl', placeholder: 'https://ima.qq.com', optional: true, hint: 'dataSource.field.baseUrlHint' }],
  gitlab: [{ key: 'base_url', label: 'dataSource.gitlab.baseUrl', placeholder: 'https://gitlab.example.com' }, { key: 'access_token', label: 'dataSource.gitlab.accessToken', secret: true }],
};

// Vue renders rss feed URLs as a dedicated settings field with its hint line
// (datasource.field.feedUrlsHint) instead of generic credential inputs.
export const VUE_SETTINGS_FIELDS: Record<string, CredentialField[]> = {
  rss: [{ key: 'feed_urls', label: 'dataSource.field.feedUrls', placeholder: 'https://example.com/feed.xml', hint: 'dataSource.field.feedUrlsHint' }],
};

export function credentialValue(text: string, key: string): string {
  const line = text.split(/\r?\n/).find((item) => item.trim().startsWith(`${key} =`) || item.trim().startsWith(`${key}=`));
  return line ? line.slice(line.indexOf('=') + 1).trim() : '';
}

// Vue validateStep1Fields: walk the connector's credential fields in order and
// report the label key of the first missing required one (optional fields are
// skipped). Connectors without a field map (e.g. rss) never fail here. The
// caller renders `${t(label)} ${t('dataSource.isRequired')}` as a blocking
// warning before the connection test, matching MessagePlugin.warning in Vue.
export function firstMissingRequiredCredential(type: string, credentialsText: string): string | null {
  const fields = VUE_CREDENTIAL_FIELDS[type];
  if (!fields) return null;
  for (const field of fields) {
    if (field.optional) continue;
    if (!credentialValue(credentialsText, field.key)) return field.label;
  }
  return null;
}

// Vue DataSourceEditorDialog credentialsRequired: in edit mode a connector
// with already-configured credentials keeps them unless the user typed a
// replacement, so the per-field required walk must be skipped entirely.
// `credentialsConfigured` mirrors the response flag the backend stores at
// credentials.credentials.configured (datasource_credentials.go).
export function credentialsRequiredForValidation(input: { isEdit: boolean; credentialsConfigured: boolean; replacementTyped: boolean }): boolean {
  return !(input.isEdit && input.credentialsConfigured && !input.replacementTyped);
}

// --- Vue edit-mode credential step (Replace / Remove) ---

export type CredentialStepKind = 'configured' | 'unconfigured' | 'inputs';

// Vue DataSourceEditorDialog template branches on the credentials section:
// an edit of a configured connector shows the "configured" faux row (with
// Replace / Remove actions) until the user opts in to Replace; an edit of a
// row without stored credentials shows the "unconfigured" faux row whose
// Configure action reveals the inputs; create mode always shows the inputs.
export function credentialStepKind(input: { isEdit: boolean; credentialsConfigured: boolean; replaceMode: boolean }): CredentialStepKind {
  if (!input.isEdit || input.replaceMode) return 'inputs';
  return input.credentialsConfigured ? 'configured' : 'unconfigured';
}

export interface CredentialStepState {
  credentialsConfigured: boolean;
  replaceMode: boolean;
  pendingRemove: boolean;
}

export type CredentialStepAction =
  | 'enter-replace'
  | 'cancel-replace'
  | 'request-remove'
  | 'cancel-remove'
  | 'remove-confirmed'
  | 'replace-committed';

export function initialCredentialStepState(credentialsConfigured: boolean): CredentialStepState {
  return { credentialsConfigured, replaceMode: false, pendingRemove: false };
}

// Pure mirror of the Vue handlers: enterReplaceCredentials (closes any pending
// remove prompt), cancelReplaceCredentials (discards the typed draft, handled
// by the caller clearing credentialsText), requestRemoveCredentials /
// cancelPendingRemoveCredentials (inline confirm, no modal), confirmRemoveCredentials
// success (falls back to the unconfigured row) and commitCredentialsIfNeeded
// success (the replacement becomes the configured set, inputs collapse).
export function credentialStepReducer(state: CredentialStepState, action: CredentialStepAction): CredentialStepState {
  switch (action) {
    case 'enter-replace': return { ...state, replaceMode: true, pendingRemove: false };
    case 'cancel-replace': return { ...state, replaceMode: false, pendingRemove: false };
    case 'request-remove': return { ...state, pendingRemove: true };
    case 'cancel-remove': return { ...state, pendingRemove: false };
    case 'remove-confirmed': return { credentialsConfigured: false, replaceMode: false, pendingRemove: false };
    case 'replace-committed': return { credentialsConfigured: true, replaceMode: false, pendingRemove: false };
  }
}
