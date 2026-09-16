import { SETTINGS_SECTIONS, type SettingsSection, type SettingsOperation, type SettingsRole, type SettingsScope } from '@weknora/views/settings/registry';

// Injected by vite define (apps/web/vite.config.ts) — frontend/vite.config.ts:71-74 parity.
declare const __FRONTEND_VERSION__: string | undefined;
declare const __FRONTEND_COMMIT__: string | undefined;
const uiVersion = typeof __FRONTEND_VERSION__ === "string" && __FRONTEND_VERSION__ ? __FRONTEND_VERSION__ : "unknown";
const uiCommit = typeof __FRONTEND_COMMIT__ === "string" && __FRONTEND_COMMIT__ ? __FRONTEND_COMMIT__ : "unknown";
import { validatePassword } from '@weknora/domain/auth/password-policy';
import { formatMessage, type Locale } from '@weknora/i18n';

export type SettingsCloseMode = 'history' | 'knowledge-bases';

const SYSTEM_ADMIN_CLOSE_SECTIONS = new Set([
  'system-global',
  'runtime-queues',
  'platform-api-keys',
  'system-audit-log',
]);

/** Mirrors Settings.vue: platform-admin pages close to knowledge bases; the
 * ordinary settings drawer returns to the route that opened it. */
export function settingsCloseMode(search: string): SettingsCloseMode {
  const section = new URLSearchParams(search).get('section');
  return section && SYSTEM_ADMIN_CLOSE_SECTIONS.has(section)
    ? 'knowledge-bases'
    : 'history';
}

// Vue section headers: every Vue settings component renders an
// "h2 + section-description" pair inside its section-header block
// (GeneralSettings.vue lines 3-6, EnvVarSettings.vue lines 3-15,
// ModelSettings.vue lines 3-8, TenantInfo.vue lines 3-6, UserProfile.vue
// lines 3-6, SystemInfo.vue lines 3-6). Keys live in packages/i18n.
const SECTION_HEADING_KEYS: Record<string, { title: string; description?: string }> = {
  general: { title: 'general.title', description: 'general.description' },
  userprofile: { title: 'userProfile.title', description: 'userProfile.description' },
  tenant: { title: 'tenant.title', description: 'tenant.sectionDescription' },
  members: { title: 'tenantMember.title' },
  mymemory: { title: 'memorySettings.title', description: 'memorySettings.description' },
  envvars: { title: 'envVarSettings.title', description: 'envVarSettings.description' },
  chathistory: { title: 'chatHistorySettings.title', description: 'chatHistorySettings.description' },
  memory: { title: 'memoryWorkspaceSettings.title', description: 'memoryWorkspaceSettings.description' },
  retrieval: { title: 'retrievalSettings.title', description: 'retrievalSettings.description' },
  models: { title: 'modelSettings.title', description: 'modelSettings.description' },
  ollama: { title: 'ollamaSettings.title', description: 'ollamaSettings.description' },
  weknoracloud: { title: 'settings.weknoraCloud.title', description: 'settings.weknoraCloud.description' },
  parser: { title: 'settings.parser.title', description: 'settings.parser.description' },
  storage: { title: 'settings.storage.title', description: 'settings.storage.description' },
  vectorstore: { title: 'vectorStoreSettings.title', description: 'vectorStoreSettings.description' },
  websearch: { title: 'webSearchSettings.title', description: 'webSearchSettings.description' },
  sandbox: { title: 'settings.sandbox.title', description: 'settings.sandbox.description' },
  skills: { title: 'settings.skills.title', description: 'settings.skills.description' },
  mcp: { title: 'settings.mcpService' },
  system: { title: 'system.title', description: 'system.sectionDescription' },
};

export function settingsSectionHeading(locale: Locale, key: string): { title: string; description: string } {
  const meta = settingsSectionMeta(key);
  const keys = SECTION_HEADING_KEYS[key];
  return {
    title: keys ? formatMessage(locale, keys.title) : meta?.title ?? key,
    // Keyed sections own their description: when Vue's section header has no
    // description line (e.g. TenantMembers.vue), render none instead of
    // leaking the registry apiDomain.
    description: keys
      ? keys.description
        ? formatMessage(locale, keys.description)
        : ''
      : meta?.description ?? '',
  };
}

export interface SettingsSectionMeta extends SettingsSection {
  readonly title: string;
  readonly description: string;
}

const descriptions: Record<string, { title: string; description: string }> = {
  general: { title: 'General and preferences', description: 'Local display preferences are read from the authenticated user profile.' },
  tenant: { title: 'Tenant information', description: 'The active tenant identity and server-owned metadata.' },
  userprofile: { title: 'User profile', description: 'Authenticated user profile and account metadata.' },
  ollama: { title: 'Ollama', description: 'Local-model availability and the current model inventory.' },
  parser: { title: 'Parser engines', description: 'Parser and document-reader connectivity reported by the server.' },
  retrieval: { title: 'Retrieval', description: 'Tenant retrieval configuration returned by the settings endpoint.' },
  memory: { title: 'Memory workspace', description: 'Shared memory workspace configuration for the active tenant.' },
  mymemory: { title: 'Personal memory', description: 'Personal memory settings and items owned by the current user.' },
  envvars: { title: 'Personal environment variables', description: 'Caller-owned skill and sandbox variable metadata.' },
  storage: { title: 'Storage', description: 'Configured storage backends and legacy engine status.' },
  vectorstore: { title: 'Vector stores', description: 'Configured vector-store resources returned by the server.' },
  websearch: { title: 'Web search', description: 'Configured web-search providers and availability metadata.' },
  chathistory: { title: 'Chat history', description: 'Chat-history configuration and aggregate statistics.' },
  system: { title: 'System information', description: 'Deployment information; this section never treats presence as health.' },
  weknoracloud: { title: 'WeKnora Cloud', description: 'Cloud connection status with credentials redacted at the API boundary.' },
  'system-global': { title: '系统全局设置', description: '管理平台级配置；修改会立即保存。' },
  'runtime-queues': { title: '运行时队列', description: '查看任务队列、工作池和模型限流状态。' },
  'platform-api-keys': { title: '平台 API Key', description: '管理平台级自动化凭据与能力范围。' },
  'system-audit-log': { title: '审计日志', description: '查看平台级管理操作和结果。' },
};

const meta = new Map<string, SettingsSectionMeta>(SETTINGS_SECTIONS.map((section) => {
  const copy = descriptions[section.key] ?? { title: section.viewId, description: section.apiDomain };
  return [section.key, { ...section, ...copy }];
}));

export function settingsSectionMeta(key: string): SettingsSectionMeta | undefined { return meta.get(key); }

export function settingsOperationLabel(operation: SettingsOperation): string {
  return ({ read: 'Read', save: 'Save', reset: 'Reset', test: 'Test', delete: 'Delete', unavailable: 'Unavailable' } as Record<SettingsOperation, string>)[operation];
}

export function settingsScopeLabel(scope: SettingsScope): string { return ({ local: 'Local', user: 'User', tenant: 'Tenant', platform: 'Platform' } as Record<SettingsScope, string>)[scope]; }

export function settingsRoleLabel(role: SettingsRole): string { return ({ viewer: 'Viewer', admin: 'Admin', owner: 'Owner', 'system-admin': 'System admin' } as Record<SettingsRole, string>)[role]; }

function isSecretKey(key: string): boolean {
  const normalized = key.toLowerCase();
  return normalized.includes('secret') || normalized.includes('password') || normalized.includes('token') || normalized.includes('api_key') || normalized.includes('access_key');
}

function printable(value: unknown): string {
  if (typeof value === 'string') return value;
  if (value === undefined) return '—';
  if (typeof value === 'number' || typeof value === 'boolean' || value === null) return String(value);
  try { return JSON.stringify(value); } catch { return '[unavailable]'; }
}

export function settingsValueEntries(value: unknown): Array<[string, string]> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return [['value', printable(value)]];
  return Object.entries(value as Record<string, unknown>)
    .filter(([key]) => !isSecretKey(key))
    .map(([key, item]) => [key, printable(item)]);
}

export interface TenantEditState { name: string; description: string }

export function tenantEditState(value: unknown): TenantEditState {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return { name: '', description: '' };
  const row = value as Record<string, unknown>;
  return { name: typeof row.name === 'string' ? row.name : '', description: typeof row.description === 'string' ? row.description : '' };
}

export function tenantPatch(name: string, description: string): { name: string; description: string } {
  const nextName = name.trim();
  if (!nextName) throw new Error('Tenant name is required');
  return { name: nextName, description: description.trim() };
}

export function profilePasswordPatch(oldPassword: string, newPassword: string, confirmation: string, options: { complexPasswordEnabled?: boolean } = {}): { old_password: string; new_password: string } {
  if (!oldPassword.trim()) throw new Error('Current password is required');
  if (!newPassword.trim()) throw new Error('New password is required');
  if (newPassword === oldPassword) throw new Error('New password must be different from the current password');
  if (newPassword !== confirmation) throw new Error('New passwords must match');
  const policy = validatePassword(newPassword, options.complexPasswordEnabled === true);
  if (!policy.valid) throw new Error('New password does not satisfy the password policy: ' + policy.violations.join(', '));
  return { old_password: oldPassword, new_password: newPassword };
}

export function memoryItemPatch(content: string): { content: string } {
  const nextContent = content.trim();
  if (!nextContent) throw new Error('Memory content is required');
  return { content: nextContent };
}

export function memoryEnabledPatch(enabled: boolean): { enabled: boolean } { return { enabled }; }

export function memoryWorkspacePatch(enabled: boolean, writeMode: string, maxItems: number, vectorRecall: boolean, retrievalConditioning: boolean, extras: { extractModelId?: string; extractDelaySeconds?: number; extractMinIntervalSeconds?: number; extractInstructions?: string; interestThreshold?: number; embeddingModelId?: string } = {}): Record<string, unknown> {
  if (writeMode !== 'explicit_only' && writeMode !== 'auto') throw new Error('Unsupported memory write mode');
  if (!Number.isInteger(maxItems) || maxItems < 10 || maxItems > 2000) throw new Error('Memory max items must be between 10 and 2000');
  if (extras.extractDelaySeconds !== undefined && (!Number.isInteger(extras.extractDelaySeconds) || extras.extractDelaySeconds < 5 || extras.extractDelaySeconds > 3600)) throw new Error('Memory extract delay must be between 5 and 3600');
  if (extras.extractMinIntervalSeconds !== undefined && (!Number.isInteger(extras.extractMinIntervalSeconds) || extras.extractMinIntervalSeconds < 0 || extras.extractMinIntervalSeconds > 86400)) throw new Error('Memory extract interval must be between 0 and 86400');
  if (extras.interestThreshold !== undefined && (!Number.isInteger(extras.interestThreshold) || extras.interestThreshold < 1 || extras.interestThreshold > 20)) throw new Error('Memory interest threshold must be between 1 and 20');
  return { enabled, write_mode: writeMode, max_items: maxItems, vector_recall: vectorRecall, retrieval_conditioning: retrievalConditioning, ...(extras.extractModelId !== undefined ? { extract_model_id: extras.extractModelId } : {}), ...(extras.extractDelaySeconds !== undefined ? { extract_delay_seconds: extras.extractDelaySeconds } : {}), ...(extras.extractMinIntervalSeconds !== undefined ? { extract_min_interval_seconds: extras.extractMinIntervalSeconds } : {}), ...(extras.extractInstructions !== undefined ? { extract_instructions: extras.extractInstructions } : {}), ...(extras.interestThreshold !== undefined ? { interest_threshold: extras.interestThreshold } : {}), ...(extras.embeddingModelId !== undefined ? { embedding_model_id: extras.embeddingModelId } : {}) };
}

export function settingsResourceRows(value: unknown, section: 'storage' | 'vectorstore' | 'websearch'): Array<Record<string, unknown>> {
  const candidate = section === 'storage' && value !== null && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>).backends
    : value;
  return Array.isArray(candidate)
    ? candidate.filter((item): item is Record<string, unknown> => item !== null && typeof item === 'object' && !Array.isArray(item))
    : [];
}

export function settingsResourceInput(name: string, type: string, configText: string): { name: string; type: string; config: Record<string, unknown> } {
  const nextName = name.trim();
  const nextType = type.trim();
  if (!nextName) throw new Error('Resource name is required');
  if (!nextType) throw new Error('Resource type is required');
  let parsed: unknown;
  try { parsed = JSON.parse(configText.trim() || '{}'); } catch { throw new Error('Resource config must be valid JSON'); }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('Resource config must be a JSON object');
  return { name: nextName, type: nextType, config: parsed as Record<string, unknown> };
}

type SettingsConfigSection = 'retrieval' | 'chathistory' | 'parser';

function finiteNumber(value: unknown, key: string, min: number, max: number): number {
  const parsed = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(parsed) || parsed < min || parsed > max) throw new Error(`${key} must be between ${min} and ${max}`);
  return parsed;
}

function modelIdSelection(value: unknown, allowedModelIds: readonly string[] | undefined, field: string): string {
  const id = typeof value === 'string' ? value.trim() : '';
  if (!id || !allowedModelIds || allowedModelIds.length === 0) return id;
  if (!allowedModelIds.includes(id)) throw new Error(field + ' must be one of the tenant models');
  return id;
}

export function settingsConfigPatch(section: SettingsConfigSection, values: Record<string, unknown>, options: { allowedModelIds?: readonly string[] } = {}): Record<string, unknown> {
  if (section === 'retrieval') {
    return {
      embedding_top_k: Math.trunc(finiteNumber(values.embedding_top_k, 'embedding_top_k', 1, 100)),
      vector_threshold: finiteNumber(values.vector_threshold, 'vector_threshold', 0, 1),
      keyword_threshold: finiteNumber(values.keyword_threshold, 'keyword_threshold', 0, 1),
      rerank_top_k: Math.trunc(finiteNumber(values.rerank_top_k, 'rerank_top_k', 1, 100)),
      rerank_threshold: finiteNumber(values.rerank_threshold, 'rerank_threshold', -10, 10),
      rerank_model_id: modelIdSelection(values.rerank_model_id, options.allowedModelIds, 'rerank_model_id'),
    };
  }
  if (section === 'chathistory') {
    if (typeof values.enabled !== 'boolean') throw new Error('Chat history enabled must be a boolean');
    return { enabled: values.enabled, embedding_model_id: modelIdSelection(values.embedding_model_id, options.allowedModelIds, 'embedding_model_id') };
  }
  const endpoint = typeof values.mineru_endpoint === 'string' ? values.mineru_endpoint.trim() : '';
  if (endpoint) {
    let parsed: URL;
    try { parsed = new URL(endpoint); } catch { throw new Error('Parser endpoint must be an absolute URL'); }
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') throw new Error('Parser endpoint must use HTTP(S)');
  }
  // ParserEngineSettings.vue builds one complete config payload.  Keeping
  // these fields here (rather than dropping every control except endpoint and
  // key) makes the React panel safe to use for all of the Vue engine options.
  const stringField = (key: string, fallback = '') => typeof values[key] === 'string' ? values[key].trim() : fallback;
  const booleanField = (key: string, fallback: boolean) => typeof values[key] === 'boolean' ? values[key] : fallback;
  const parseMethod = stringField('mineru_parse_method', 'auto');
  const patch: Record<string, unknown> = {
    mineru_endpoint: endpoint,
    mineru_model: stringField('mineru_model', 'pipeline'),
    mineru_vlm_server_url: stringField('mineru_vlm_server_url'),
    mineru_enable_formula: booleanField('mineru_enable_formula', true),
    mineru_enable_table: booleanField('mineru_enable_table', true),
    mineru_parse_method: parseMethod,
    // This legacy flag is intentionally derived from the newer Vue control.
    mineru_enable_ocr: parseMethod !== 'txt',
    mineru_language: stringField('mineru_language', 'ch'),
    mineru_cloud_model: stringField('mineru_cloud_model', 'pipeline'),
    mineru_cloud_enable_formula: booleanField('mineru_cloud_enable_formula', true),
    mineru_cloud_enable_table: booleanField('mineru_cloud_enable_table', true),
    mineru_cloud_enable_ocr: booleanField('mineru_cloud_enable_ocr', true),
    mineru_cloud_language: stringField('mineru_cloud_language', 'ch'),
    paddleocr_vl_endpoint: stringField('paddleocr_vl_endpoint'),
    paddleocr_vl_use_seal_recognition: booleanField('paddleocr_vl_use_seal_recognition', true),
    paddleocr_vl_use_chart_recognition: booleanField('paddleocr_vl_use_chart_recognition', false),
    paddleocr_vl_cloud_token: stringField('paddleocr_vl_cloud_token'),
    paddleocr_vl_cloud_model: stringField('paddleocr_vl_cloud_model', 'PaddleOCR-VL-1.6'),
    paddleocr_vl_cloud_use_seal_recognition: booleanField('paddleocr_vl_cloud_use_seal_recognition', true),
    paddleocr_vl_cloud_use_chart_recognition: booleanField('paddleocr_vl_cloud_use_chart_recognition', false),
  };
  const apiKey = typeof values.mineru_api_key === 'string' ? values.mineru_api_key.trim() : '';
  if (apiKey) patch.mineru_api_key = apiKey;
  return patch;
}

export function ollamaModelInput(value: string): string {
  const model = value.trim();
  if (!model) throw new Error('Ollama model name is required');
  return model;
}

export function cloudCredentialPatch(appId: string, appSecret: string): { app_id: string; app_secret: string } {
  const id = appId.trim();
  const secret = appSecret.trim();
  if (!id) throw new Error('WeKnora Cloud app ID is required');
  if (!secret) throw new Error('WeKnora Cloud app secret is required');
  return { app_id: id, app_secret: secret };
}
export type EnvVarScope = 'skill' | 'sandbox';

export interface EnvVarMutation {
  readonly scope: EnvVarScope;
  readonly name: string;
  readonly value: string;
  readonly body: Record<string, unknown>;
}

function envVarBody(scope: EnvVarScope, scopeId: string, name: string, value: string): Record<string, unknown> {
  return scope === 'skill' ? { skill_id: scopeId, name, value } : { sandbox_config_id: scopeId, name, value };
}

export function envVarSet(scope: EnvVarScope, scopeId: string, name: string, value: string): EnvVarMutation {
  const nextScopeId = scopeId.trim();
  const nextName = name.trim();
  if (!nextScopeId) throw new Error(scope === 'skill' ? 'Skill ID is required' : 'Sandbox config ID is required');
  if (!nextName) throw new Error('Variable name is required');
  return { scope, name: nextName, value, body: envVarBody(scope, nextScopeId, nextName, value) };
}

export function envVarRemove(scope: EnvVarScope, scopeId: string, name: string): Record<string, unknown> {
  const nextScopeId = scopeId.trim();
  const nextName = name.trim();
  if (!nextScopeId) throw new Error(scope === 'skill' ? 'Skill ID is required' : 'Sandbox config ID is required');
  if (!nextName) throw new Error('Variable name is required');
  return scope === 'skill' ? { skill_id: nextScopeId, name: nextName } : { sandbox_config_id: nextScopeId, name: nextName };
}

export function tenantModelIds(models: unknown): string[] {
  if (!Array.isArray(models)) return [];
  return models
    .filter((item): item is Record<string, unknown> => item !== null && typeof item === 'object' && !Array.isArray(item))
    .map((item) => (typeof item.id === 'string' ? item.id : ''))
    .filter((id) => id.length > 0);
}

export function chatHistoryEmbeddingLocked(stats: unknown): boolean {
  if (stats === null || typeof stats !== 'object' || Array.isArray(stats)) return false;
  return (stats as Record<string, unknown>).has_indexed_messages === true;
}

// ---------------------------------------------------------------------------
// System info display rows (item D).
// Ported from frontend/src/views/settings/SystemInfo.vue: the section renders
// a read-only "label + help text + formatted value" list (template lines
// 21-197) with a humanized uptime (formatUptime, lines 233-248) and the
// edition/migration tags. Values fall back to system.unknown like Vue.
export interface SystemInfoRow {
  readonly labelKey: string;
  readonly descriptionKey: string;
  readonly value: string;
  readonly tag?: string;
  readonly tagTone?: 'default' | 'warning' | 'danger';
  readonly commit?: string;
}

function text(value: unknown): string { return typeof value === "string" ? value.trim() : ""; }

// Vue SystemInfo.vue formatUptime (lines 233-248): days/hours/minutes are
// pushed when any larger unit is present; seconds close the string unless a
// day-level unit already anchors it.
export function formatUptimeText(totalSeconds: number, locale: Locale = "zh-CN"): string {
  const sec = Math.max(0, Math.floor(totalSeconds));
  const days = Math.floor(sec / 86400);
  const hours = Math.floor((sec % 86400) / 3600);
  const minutes = Math.floor((sec % 3600) / 60);
  const seconds = sec % 60;
  const parts: string[] = [];
  if (days > 0) parts.push(formatMessage(locale, "system.uptimeDays", { n: days }));
  if (hours > 0 || days > 0) parts.push(formatMessage(locale, "system.uptimeHours", { n: hours }));
  if (minutes > 0 || hours > 0 || days > 0) parts.push(formatMessage(locale, "system.uptimeMinutes", { n: minutes }));
  if (parts.length === 0) return formatMessage(locale, "system.uptimeSeconds", { n: seconds });
  if (seconds > 0 && days === 0) parts.push(formatMessage(locale, "system.uptimeSeconds", { n: seconds }));
  return parts.join(" ");
}

export function systemInfoRows(value: unknown, options: { now?: number; locale?: Locale } = {}): SystemInfoRow[] {
  const locale = options.locale ?? "zh-CN";
  const info = value !== null && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
  const unknown = formatMessage(locale, "system.unknown");
  const rows: SystemInfoRow[] = [];
  const row = (labelKey: string, descriptionKey: string, value: string, extra: Partial<SystemInfoRow> = {}): void => {
    rows.push({ labelKey, descriptionKey, value, ...extra });
  };

  // 应用版本 + edition tag + commit hash (SystemInfo.vue lines 21-45).
  const commit = text(info.commit_id);
  row("system.versionLabel", "system.versionDescription", text(info.version) || unknown, {
    tag: text(info.edition) === 'lite' ? 'Lite' : text(info.edition) ? 'Standard' : undefined,
    commit: commit || undefined,
  });

  // UI 版本 — the React build injects __FRONTEND_VERSION__/__FRONTEND_COMMIT__
  // via vite define (frontend/vite.config.ts:71-74 parity).
  row("system.frontendVersionLabel", "system.frontendVersionDescription", uiVersion, uiVersion === 'unknown' ? undefined : { commit: uiCommit });

  if (text(info.build_time)) row("system.buildTimeLabel", "system.buildTimeDescription", text(info.build_time));
  if (text(info.go_version)) row("system.goVersionLabel", "system.goVersionDescription", text(info.go_version));

  // 服务启动时间 + 运行时长 (lines 91-118): uptime prefers now - started_at,
// falling back to the reported uptime_seconds.
  const startedAt = text(info.started_at);
  let uptimeSeconds: number | null = null;
  if (startedAt) {
    const boot = new Date(startedAt).getTime();
    if (!Number.isNaN(boot)) uptimeSeconds = Math.max(0, Math.floor(((options.now ?? Date.now()) - boot) / 1000));
    row("system.startedAtLabel", "system.startedAtDescription", new Date(startedAt).toLocaleString(locale));
  }
  if (uptimeSeconds === null && info.uptime_seconds !== undefined && info.uptime_seconds !== null) {
    const reported = Number(info.uptime_seconds);
    if (Number.isFinite(reported)) uptimeSeconds = reported;
  }
  if (uptimeSeconds !== null) row("system.uptimeLabel", "system.uptimeDescription", formatUptimeText(uptimeSeconds, locale));

  if (text(info.db_version) || text(info.db_migration_error)) {
    row("system.dbVersionLabel", "system.dbVersionDescription", text(info.db_version) || unknown, {
      tag: text(info.db_migration_error) ? formatMessage(locale, 'system.dbMigrationFailedTag') : undefined,
      tagTone: text(info.db_migration_error) ? 'danger' : undefined,
    });
  }

  row("system.keywordIndexEngineLabel", "system.keywordIndexEngineDescription", text(info.keyword_index_engine) || unknown);
  row("system.vectorStoreEngineLabel", "system.vectorStoreEngineDescription", text(info.vector_store_engine) || unknown);
  row("system.graphDatabaseEngineLabel", "system.graphDatabaseEngineDescription", text(info.graph_database_engine) || unknown);
  return rows;
}
