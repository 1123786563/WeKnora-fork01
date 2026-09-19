import { parseActionSuccessResponse, type ActionSuccessResponse } from '@weknora/contracts';
import type { ClientRequest } from './client.ts';
import type { NativeFileSource } from './ports.ts';

export interface ConfigurationRecord { id: string; name: string; [key: string]: unknown }
export type AgentConfiguration = ConfigurationRecord & { config?: Record<string, unknown>; is_builtin?: boolean };
export type ModelConfiguration = ConfigurationRecord & { type?: string; source?: string; parameters?: Record<string, unknown> };
export type McpTransportType = 'sse' | 'http-streamable' | 'stdio';
export type McpConfiguration = ConfigurationRecord & { enabled?: boolean; url?: string; transport_type?: McpTransportType; tools?: unknown[] };
export type SkillConfiguration = ConfigurationRecord & { description?: string; skills_available?: boolean };
export interface AgentConfigurationList { items: AgentConfiguration[]; disabledOwnAgentIds: string[] }
export interface SkillConfigurationList { items: SkillConfiguration[]; skillsAvailable: boolean }
export interface McpOAuthAuthorization { authorizationUrl: string; authorizationAttempt: string }
export interface McpOAuthStatus { authorized: boolean; state: 'authorized' | 'refreshable' | 'reauth_required' | 'pending'; refreshAvailable: boolean; expiresAt?: string }
export interface ModelCredentialStatus { apiKey: boolean; appSecret: boolean }
export interface ModelConnectionTestInput { source?: 'remote' | 'local'; modelName: string; baseUrl?: string; apiKey?: string; provider?: string; interfaceType?: string; dimension?: number; supportsDimensionOverride?: boolean; customHeaders?: Record<string, string>; extraConfig?: Record<string, string>; appSecret?: string; modelId?: string }
export interface ModelConnectionTestResult { available: boolean; message: string; dimension?: number }
export interface McpCredentialStatus { apiKey: boolean; token: boolean }
export interface McpTestResult { success: boolean; message?: string; description?: string; oauthRequired?: boolean; tools?: McpTool[]; resources?: McpResource[] }
export interface McpTool { name: string; description?: string; inputSchema?: unknown; requireApproval?: boolean }
export interface McpResource { uri: string; name: string; description?: string; mimeType?: string }
export interface McpMetadata { serviceId: string; tools: McpTool[]; instructions: string; serverName: string; serverVersion: string; serverDescription: string; syncedAt: string; stale: boolean }
export interface McpToolApproval { id: string; serviceId: string; toolName: string; requireApproval: boolean; enabled: boolean }
export interface AgentConfigurationListOptions {
  creator?: 'all' | 'mine' | 'others';
  signal?: AbortSignal;
}

/** GET /api/v1/agents/:id/suggested-questions (routes_agent.go, Viewer+). */
export interface AgentSuggestedQuestionsOptions {
  knowledgeBaseIds?: string[];
  limit?: number;
  signal?: AbortSignal;
}

export interface ModelProvider {
  value: string;
  label: string;
  description: string;
  defaultUrls: Record<string, string>;
  modelTypes: string[];
}
export interface ModelDebugOptions {
  systemPrompt?: string;
  temperature?: number;
  topP?: number;
  maxTokens?: number;
  thinking?: boolean;
}
export interface ModelDebugInput {
  input?: string;
  documents?: string[];
  options?: ModelDebugOptions;
  file?: Blob | NativeFileSource;
}
export interface ModelDebugResult {
  ok: boolean;
  elapsedMs: number;
  request: Record<string, unknown>;
  rawResponse: unknown;
  observations: Record<string, unknown>;
  error?: string;
}

export type SkillStatus = 'installing' | 'ready' | 'failed' | 'removing';
export type SkillInstallStatus = SkillStatus | 'removed';
export interface SkillCatalogInstallation {
  skillId: string;
  sandboxConfigId: string;
  sandboxConfigName?: string;
  sandboxType?: string;
  status: SkillInstallStatus;
  enabled: boolean;
  error?: string;
  bundleSha256?: string;
  updatedAt?: string;
}
export interface SkillCatalog {
  id: string;
  name: string;
  version?: string;
  description?: string;
  bundleSha256?: string;
  createdAt?: string;
  updatedAt?: string;
  installations?: SkillCatalogInstallation[];
}
export interface SkillCatalogInstallResult {
  installs: Record<string, string>;
  errors?: Record<string, string>;
}
export interface SkillFile {
  path: string;
  size: number;
}
export interface SkillFileContent extends SkillFile {
  encoding: 'utf-8' | 'base64' | 'binary';
  content?: string;
  mediaType?: string;
  truncated?: boolean;
  binary?: boolean;
}
export interface InstalledSkillEnv {
  name: string;
  description?: string;
  required?: boolean;
  isSet: boolean;
}
export interface InstalledSkill {
  id: string;
  name: string;
  version?: string;
  description?: string;
  enabled: boolean;
  status: SkillStatus;
  error?: string;
  bundleSha256?: string;
  installedSnapshotId?: string;
  installSessionId?: string;
  installMessageId?: string;
  createdAt?: string;
  updatedAt?: string;
  envs?: InstalledSkillEnv[];
}
export interface SandboxSkillUpdate {
  enabled?: boolean;
  envs?: Record<string, string>;
}
export interface SkillAcceptedResult { skillId: string }

type RecordValue = Record<string, unknown>;
function record(value: unknown, path: string): RecordValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path} must be an object`);
  return value as RecordValue;
}
function required(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}
const secretFields = new Set(['apikey', 'appsecret', 'accesstoken', 'refreshtoken', 'token', 'clientsecret', 'password', 'secret']);
function normalizedKey(key: string): string { return key.replace(/[^a-zA-Z0-9]/g, '').toLowerCase(); }
function isSecretKey(key: string): boolean { return secretFields.has(normalizedKey(key)); }
function credentialStatus(value: unknown): RecordValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value as RecordValue).flatMap(([field, metadata]) => {
    if (metadata === null || typeof metadata !== 'object' || Array.isArray(metadata)) return [];
    const configured = (metadata as RecordValue).configured;
    return typeof configured === 'boolean' ? [[field, { configured }]] : [];
  }));
}
function withoutSecrets(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(withoutSecrets);
  if (value === null || typeof value !== 'object') return value;
  return Object.fromEntries(
    Object.entries(value as RecordValue)
      .filter(([key]) => !isSecretKey(key))
      .map(([key, item]) => [key, key === 'credentials' ? credentialStatus(item) : withoutSecrets(item)]),
  );
}
function stripSecrets(value: RecordValue): RecordValue {
  return withoutSecrets(value) as RecordValue;
}
function parseRecord(value: unknown, path: string): ConfigurationRecord {
  const row = stripSecrets(record(value, path));
  return { ...row, id: required(row.id, `${path}.id`), name: required(row.name, `${path}.name`) };
}
function parseList(value: unknown, path: string): ConfigurationRecord[] {
  const envelope = record(value, path);
  if (envelope.success !== true || !Array.isArray(envelope.data)) throw new Error(`${path} must be a successful list envelope`);
  return envelope.data.map((item, index) => parseRecord(item, `${path}.data[${index}]`));
}
function parseOne(value: unknown, path: string): ConfigurationRecord {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
  return parseRecord(envelope.data, `${path}.data`);
}
function parseSkills(value: unknown): SkillConfiguration[] {
  return parseSkillList(value).items;
}
function parseSkillList(value: unknown): SkillConfigurationList {
  const envelope = record(value, '/skills');
  if (envelope.success !== true || !Array.isArray(envelope.data)) throw new Error('/skills must be a successful list envelope');
  if (typeof envelope.skills_available !== 'boolean') throw new Error('/skills.skills_available must be a boolean');
  const items = envelope.data.map((item, index) => {
    const row = stripSecrets(record(item, `/skills.data[${index}]`));
    const name = required(row.name, `/skills.data[${index}].name`);
    return { ...row, id: name, name } as SkillConfiguration;
  });
  return { items, skillsAvailable: envelope.skills_available };
}
function parseAgentList(value: unknown, path: string): AgentConfigurationList {
  const envelope = record(value, path);
  const items = parseList(value, path) as AgentConfiguration[];
  const rawDisabled = envelope.disabled_own_agent_ids;
  if (rawDisabled !== undefined && (!Array.isArray(rawDisabled) || rawDisabled.some((item) => typeof item !== 'string'))) {
    throw new Error(`${path}.disabled_own_agent_ids must be a string array`);
  }
  return { items, disabledOwnAgentIds: (rawDisabled as string[] | undefined) ?? [] };
}
function id(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}

function parseOAuthAuthorization(value: unknown): McpOAuthAuthorization {
  const envelope = record(value, '/mcp-services/oauth/authorize-url');
  if (envelope.success !== true) throw new Error('/mcp-services/oauth/authorize-url.success must be true');
  const data = record(envelope.data, '/mcp-services/oauth/authorize-url.data');
  return {
    authorizationUrl: required(data.authorization_url, '/mcp-services/oauth/authorize-url.data.authorization_url'),
    authorizationAttempt: required(data.authorization_attempt, '/mcp-services/oauth/authorize-url.data.authorization_attempt'),
  };
}

function parseOAuthStatus(value: unknown): McpOAuthStatus {
  const envelope = record(value, '/mcp-services/oauth/status');
  if (envelope.success !== true) throw new Error('/mcp-services/oauth/status.success must be true');
  const data = record(envelope.data, '/mcp-services/oauth/status.data');
  if (typeof data.authorized !== 'boolean') throw new Error('/mcp-services/oauth/status.data.authorized must be a boolean');
  if (data.state !== 'authorized' && data.state !== 'refreshable' && data.state !== 'reauth_required' && data.state !== 'pending') {
    throw new Error('/mcp-services/oauth/status.data.state is invalid');
  }
  if (typeof data.refresh_available !== 'boolean') throw new Error('/mcp-services/oauth/status.data.refresh_available must be a boolean');
  if (data.expires_at !== undefined && typeof data.expires_at !== 'string') throw new Error('/mcp-services/oauth/status.data.expires_at must be a string');
  return { authorized: data.authorized, state: data.state, refreshAvailable: data.refresh_available, ...(data.expires_at === undefined ? {} : { expiresAt: data.expires_at }) };
}

function parseMcpTool(value: unknown, path: string): McpTool {
  const row = record(value, path);
  const tool: McpTool = { name: required(row.name, `${path}.name`) };
  if (row.description !== undefined) {
    if (typeof row.description !== 'string') throw new Error(`${path}.description must be a string`);
    tool.description = row.description;
  }
  if (row.inputSchema !== undefined) tool.inputSchema = row.inputSchema;
  if (row.require_approval !== undefined) {
    if (typeof row.require_approval !== 'boolean') throw new Error(`${path}.require_approval must be a boolean`);
    tool.requireApproval = row.require_approval;
  }
  return tool;
}

function parseMcpResource(value: unknown, path: string): McpResource {
  const row = record(value, path);
  const resource: McpResource = {
    uri: required(row.uri, `${path}.uri`),
    name: required(row.name, `${path}.name`),
  };
  if (row.description !== undefined) {
    if (typeof row.description !== 'string') throw new Error(`${path}.description must be a string`);
    resource.description = row.description;
  }
  if (row.mimeType !== undefined) {
    if (typeof row.mimeType !== 'string') throw new Error(`${path}.mimeType must be a string`);
    resource.mimeType = row.mimeType;
  }
  return resource;
}

function parseCredentialStatus(value: unknown, fields: readonly string[], path: string): Record<string, boolean> {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
  const data = record(envelope.data, `${path}.data`);
  const metadata = record(data.fields, `${path}.data.fields`);
  return Object.fromEntries(fields.map((field) => {
    const item = metadata[field];
    if (item === undefined) throw new Error(`${path}.data.fields.${field} is required`);
    const configured = record(item, `${path}.data.fields.${field}`).configured;
    if (typeof configured !== 'boolean') throw new Error(`${path}.data.fields.${field}.configured must be a boolean`);
    return [field, configured];
  }));
}

function parseNoContent(value: unknown, path: string): void {
  if (value !== undefined) throw new Error(`${path} must be an empty 204 response`);
}

function successfulData(value: unknown, path: string): unknown {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) throw new Error(`${path}.data is required`);
  return envelope.data;
}

function acceptedData(value: unknown, path: string): { success: boolean; data: unknown } {
  const envelope = record(value, path);
  if (typeof envelope.success !== 'boolean') throw new Error(`${path}.success must be a boolean`);
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) throw new Error(`${path}.data is required`);
  return { success: envelope.success, data: envelope.data };
}

function numberValue(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) throw new Error(`${path} must be a finite number`);
  return value;
}

function optionalString(row: RecordValue, field: string, path: string): string | undefined {
  if (row[field] === undefined) return undefined;
  if (typeof row[field] !== 'string') throw new Error(`${path}.${field} must be a string`);
  return row[field] as string;
}

function stringMap(value: unknown, path: string, allowNull = false): Record<string, string> {
  if (value === null && allowNull) return {};
  const row = record(value, path);
  return Object.fromEntries(Object.entries(row).map(([key, item]) => {
    if (typeof item !== 'string') throw new Error(`${path}.${key} must be a string`);
    return [key, item];
  }));
}

function parseModelProvider(value: unknown, path: string): ModelProvider {
  const row = record(value, path);
  const modelTypes = row.modelTypes;
  if (!Array.isArray(modelTypes) || modelTypes.some((item) => typeof item !== 'string')) throw new Error(`${path}.modelTypes must be a string array`);
  return {
    value: required(row.value, `${path}.value`),
    label: required(row.label, `${path}.label`),
    description: typeof row.description === 'string' ? row.description : (() => { throw new Error(`${path}.description must be a string`); })(),
    defaultUrls: stringMap(row.defaultUrls, `${path}.defaultUrls`),
    modelTypes: modelTypes as string[],
  };
}

function parseModelConnectionTest(value: unknown, path: string): ModelConnectionTestResult {
  const data = record(successfulData(value, path), `${path}.data`);
  if (typeof data.available !== 'boolean') throw new Error(`${path}.data.available must be a boolean`);
  if (typeof data.message !== 'string') throw new Error(`${path}.data.message must be a string`);
  const result: ModelConnectionTestResult = { available: data.available, message: data.message };
  if (data.dimension !== undefined) { if (typeof data.dimension !== 'number' || data.dimension < 0) throw new Error(`${path}.data.dimension must be a non-negative number`); result.dimension = data.dimension; }
  return result;
}

function modelConnectionBody(input: ModelConnectionTestInput): Record<string, unknown> {
  if (!input.modelName.trim()) throw new Error('modelName must not be empty');
  return { ...input, modelName: input.modelName.trim(), ...(input.baseUrl === undefined ? {} : { baseUrl: input.baseUrl.trim() }) };
}

function modelDebugFields(options: ModelDebugOptions): Record<string, unknown> {
  return {
    ...(options.systemPrompt === undefined ? {} : { system_prompt: options.systemPrompt }),
    ...(options.temperature === undefined ? {} : { temperature: options.temperature }),
    ...(options.topP === undefined ? {} : { top_p: options.topP }),
    ...(options.maxTokens === undefined ? {} : { max_tokens: options.maxTokens }),
    ...(options.thinking === undefined ? {} : { thinking: options.thinking }),
  };
}

function multipartForm(fields: Record<string, string>, file?: Blob): FormData {
  if (typeof FormData === 'undefined') throw new Error('multipart form data is unavailable');
  const form = new FormData();
  for (const [key, value] of Object.entries(fields)) form.append(key, value);
  if (file !== undefined) form.append('file', file);
  return form;
}

function isNativeFileSource(value: Blob | NativeFileSource): value is NativeFileSource {
  return typeof Blob === 'undefined' || !(value instanceof Blob);
}

/**
 * Bridge a browser Blob (DOM File) into the platform-neutral NativeFileSource
 * shape the multipart transports consume. The web transport fetches the
 * object URL and revokes it after reading; native runtimes pass uri through.
 */
function toNativeFileSource(value: Blob | NativeFileSource): NativeFileSource {
  if (isNativeFileSource(value)) return value;
  const uri = URL.createObjectURL(value);
  const file = value as File;
  return {
    uri,
    name: file.name || 'file',
    type: value.type || 'application/octet-stream',
    size: value.size,
  };
}

function parseModelDebug(value: unknown): ModelDebugResult {
  const data = record(successfulData(value, '/models/debug'), '/models/debug.data');
  if (typeof data.ok !== 'boolean') throw new Error('/models/debug.data.ok must be a boolean');
  const elapsedMs = numberValue(data.elapsed_ms, '/models/debug.data.elapsed_ms');
  if (elapsedMs < 0) throw new Error('/models/debug.data.elapsed_ms must not be negative');
  const request = record(data.request, '/models/debug.data.request');
  const observations = record(data.observations, '/models/debug.data.observations');
  const error = optionalString(data, 'error', '/models/debug.data');
  return { ok: data.ok, elapsedMs, request, rawResponse: data.raw_response, observations, ...(error === undefined ? {} : { error }) };
}

function parseSkillStatus(value: unknown, path: string): SkillStatus {
  if (value !== 'installing' && value !== 'ready' && value !== 'failed' && value !== 'removing') throw new Error(`${path} has an invalid skill status`);
  return value;
}

function parseInstallStatus(value: unknown, path: string): SkillInstallStatus {
  if (value === 'removed') return value;
  return parseSkillStatus(value, path);
}

function parseCatalogInstallation(value: unknown, path: string): SkillCatalogInstallation {
  const row = record(value, path);
  const result: SkillCatalogInstallation = {
    skillId: required(row.skill_id, `${path}.skill_id`),
    sandboxConfigId: required(row.sandbox_config_id, `${path}.sandbox_config_id`),
    status: parseInstallStatus(row.status, `${path}.status`),
    enabled: row.enabled === true,
  };
  if (typeof row.enabled !== 'boolean') throw new Error(`${path}.enabled must be a boolean`);
  for (const [source, target] of [['sandbox_config_name', 'sandboxConfigName'], ['sandbox_type', 'sandboxType'], ['error', 'error'], ['bundle_sha256', 'bundleSha256'], ['updated_at', 'updatedAt']] as const) {
    const item = optionalString(row, source, path);
    if (item !== undefined) (result as unknown as Record<string, unknown>)[target] = item;
  }
  return result;
}

function parseCatalog(value: unknown, path: string, installationsRequired = true): SkillCatalog {
  const row = record(value, path);
  const result: SkillCatalog = {
    id: required(row.id, `${path}.id`),
    name: required(row.name, `${path}.name`),
  };
  for (const [source, target] of [['version', 'version'], ['description', 'description'], ['bundle_sha256', 'bundleSha256'], ['created_at', 'createdAt'], ['updated_at', 'updatedAt']] as const) {
    const item = optionalString(row, source, path);
    if (item !== undefined) (result as unknown as Record<string, unknown>)[target] = item;
  }
  if (row.installations !== undefined || installationsRequired) {
    if (!Array.isArray(row.installations)) throw new Error(`${path}.installations must be an array`);
    result.installations = row.installations.map((item, index) => parseCatalogInstallation(item, `${path}.installations[${index}]`));
  }
  return result;
}

function parseSkillFile(value: unknown, path: string): SkillFile {
  const row = record(value, path);
  const filePath = required(row.path, `${path}.path`);
  const size = numberValue(row.size, `${path}.size`);
  if (size < 0) throw new Error(`${path}.size must not be negative`);
  return { path: filePath, size };
}

function parseSkillFileContent(value: unknown, path: string): SkillFileContent {
  const row = record(value, path);
  const file = parseSkillFile(value, path);
  if (row.encoding !== 'utf-8' && row.encoding !== 'base64' && row.encoding !== 'binary') throw new Error(`${path}.encoding is invalid`);
  const result: SkillFileContent = { ...file, encoding: row.encoding };
  const content = optionalString(row, 'content', path);
  const mediaType = optionalString(row, 'media_type', path);
  if (content !== undefined) result.content = content;
  if (mediaType !== undefined) result.mediaType = mediaType;
  for (const field of ['truncated', 'binary'] as const) if (row[field] !== undefined) {
    if (typeof row[field] !== 'boolean') throw new Error(`${path}.${field} must be a boolean`);
    result[field] = row[field] as boolean;
  }
  return result;
}

function parseInstalledSkill(value: unknown, path: string): InstalledSkill {
  const row = record(value, path);
  const result: InstalledSkill = {
    id: required(row.id, `${path}.id`),
    name: required(row.name, `${path}.name`),
    enabled: row.enabled === true,
    status: parseSkillStatus(row.status, `${path}.status`),
  };
  if (typeof row.enabled !== 'boolean') throw new Error(`${path}.enabled must be a boolean`);
  for (const [source, target] of [['version', 'version'], ['description', 'description'], ['error', 'error'], ['bundle_sha256', 'bundleSha256'], ['installed_snapshot_id', 'installedSnapshotId'], ['install_session_id', 'installSessionId'], ['install_message_id', 'installMessageId'], ['created_at', 'createdAt'], ['updated_at', 'updatedAt']] as const) {
    const item = optionalString(row, source, path);
    if (item !== undefined) (result as unknown as Record<string, unknown>)[target] = item;
  }
  if (row.envs !== undefined) {
    if (!Array.isArray(row.envs)) throw new Error(`${path}.envs must be an array`);
    result.envs = row.envs.map((item, index) => {
      const env = record(item, `${path}.envs[${index}]`);
      const parsed: InstalledSkillEnv = { name: required(env.name, `${path}.envs[${index}].name`), isSet: env.is_set === true };
      if (typeof env.is_set !== 'boolean') throw new Error(`${path}.envs[${index}].is_set must be a boolean`);
      const description = optionalString(env, 'description', `${path}.envs[${index}]`);
      if (description !== undefined) parsed.description = description;
      if (env.required !== undefined) {
        if (typeof env.required !== 'boolean') throw new Error(`${path}.envs[${index}].required must be a boolean`);
        parsed.required = env.required;
      }
      return parsed;
    });
  }
  return result;
}

function parseSkillIdResult(value: unknown, path: string): SkillAcceptedResult {
  const data = record(successfulData(value, path), `${path}.data`);
  return { skillId: required(data.skill_id, `${path}.data.skill_id`) };
}

function parseCatalogList(value: unknown, path: string): SkillCatalog[] {
  const data = successfulData(value, path);
  if (!Array.isArray(data)) throw new Error(`${path}.data must be an array`);
  return data.map((item, index) => parseCatalog(item, `${path}.data[${index}]`, false));
}

function parseInstalledList(value: unknown, path: string): InstalledSkill[] {
  const data = successfulData(value, path);
  if (!Array.isArray(data)) throw new Error(`${path}.data must be an array`);
  return data.map((item, index) => parseInstalledSkill(item, `${path}.data[${index}]`));
}

function parseFileList(value: unknown, path: string): SkillFile[] {
  const data = successfulData(value, path);
  if (!Array.isArray(data)) throw new Error(`${path}.data must be an array`);
  return data.map((item, index) => parseSkillFile(item, `${path}.data[${index}]`));
}

function parseFileContent(value: unknown, path: string): SkillFileContent {
  return parseSkillFileContent(successfulData(value, path), `${path}.data`);
}

function parseActionEnvelope(value: unknown, path: string): void {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
}

function credentialBody(input: Record<string, unknown>, fields: Record<string, string>, path: string): Record<string, string> {
  const body: Record<string, string> = {};
  for (const [source, target] of Object.entries(fields)) {
    const value = input[source];
    if (value !== undefined) {
      if (typeof value !== 'string') throw new Error(`${path}.${source} must be a string`);
      body[target] = value;
    }
  }
  if (Object.keys(body).length === 0) throw new Error(`${path} must include at least one credential field`);
  return body;
}

function parseMcpTest(value: unknown): McpTestResult {
  const envelope = record(value, '/mcp-services/test');
  if (envelope.success !== true) throw new Error('/mcp-services/test.success must be true');
  const data = record(envelope.data, '/mcp-services/test.data');
  if (typeof data.success !== 'boolean') throw new Error('/mcp-services/test.data.success must be a boolean');
  const result: McpTestResult = { success: data.success };
  if (data.message !== undefined) result.message = required(data.message, '/mcp-services/test.data.message');
  if (data.description !== undefined) result.description = required(data.description, '/mcp-services/test.data.description');
  if (data.oauth_required !== undefined) {
    if (typeof data.oauth_required !== 'boolean') throw new Error('/mcp-services/test.data.oauth_required must be a boolean');
    result.oauthRequired = data.oauth_required;
  }
  if (data.tools !== undefined) {
    if (!Array.isArray(data.tools)) throw new Error('/mcp-services/test.data.tools must be an array');
    result.tools = data.tools.map((item, index) => parseMcpTool(item, `/mcp-services/test.data.tools[${index}]`));
  }
  if (data.resources !== undefined) {
    if (!Array.isArray(data.resources)) throw new Error('/mcp-services/test.data.resources must be an array');
    result.resources = data.resources.map((item, index) => parseMcpResource(item, `/mcp-services/test.data.resources[${index}]`));
  }
  return result;
}

function parseMcpTools(value: unknown): McpTool[] {
  const envelope = record(value, '/mcp-services/tools');
  if (envelope.success !== true || !Array.isArray(envelope.data)) throw new Error('/mcp-services/tools must be a successful array envelope');
  return envelope.data.map((item, index) => parseMcpTool(item, `/mcp-services/tools.data[${index}]`));
}
function parseMcpMetadata(value: unknown): McpMetadata | null {
  const data = successfulData(value, '/mcp-services/metadata');
  if (data === null) return null;
  const row = record(data, '/mcp-services/metadata.data');
  if (typeof row.service_id !== 'string' || typeof row.instructions !== 'string' || typeof row.server_name !== 'string' || typeof row.server_version !== 'string' || typeof row.server_description !== 'string' || typeof row.synced_at !== 'string' || typeof row.stale !== 'boolean' || !Array.isArray(row.tools)) throw new Error('/mcp-services/metadata.data is malformed');
  return { serviceId: row.service_id, instructions: row.instructions, serverName: row.server_name, serverVersion: row.server_version, serverDescription: row.server_description, syncedAt: row.synced_at, stale: row.stale, tools: row.tools.map((item, index) => parseMcpTool(item, `/mcp-services/metadata.data.tools[${index}]`)) };
}
function parseMcpApprovals(value: unknown): McpToolApproval[] {
  const data = successfulData(value, '/mcp-services/tool-approvals');
  if (!Array.isArray(data)) throw new Error('/mcp-services/tool-approvals.data must be an array');
  return data.map((item, index) => { const row = record(item, `/mcp-services/tool-approvals.data[${index}]`); if (typeof row.id !== 'string' || typeof row.service_id !== 'string' || typeof row.tool_name !== 'string' || typeof row.require_approval !== 'boolean' || typeof row.enabled !== 'boolean') throw new Error(`/mcp-services/tool-approvals.data[${index}] is malformed`); return { id: row.id, serviceId: row.service_id, toolName: row.tool_name, requireApproval: row.require_approval, enabled: row.enabled }; });
}

export function createConfigurationApi(request: (input: ClientRequest) => Promise<unknown>) {
  const collection = <T extends ConfigurationRecord>(path: string, parse: (value: unknown, path: string) => T) => ({
    async list(signal?: AbortSignal): Promise<T[]> {
      return parseList(await request({ method: 'GET', path, ...(signal === undefined ? {} : { signal }) }), path) as T[];
    },
    async get(itemId: string, signal?: AbortSignal): Promise<T> {
      return parseOne(await request({ method: 'GET', path: `${path}/${id(itemId, 'id')}`, ...(signal === undefined ? {} : { signal }) }), path) as T;
    },
    async create(input: Record<string, unknown>, signal?: AbortSignal): Promise<T> {
      return parse(await request({ method: 'POST', path, body: input, ...(signal === undefined ? {} : { signal }) }), path);
    },
    async update(itemId: string, input: Record<string, unknown>, signal?: AbortSignal): Promise<T> {
      return parse(await request({ method: 'PUT', path: `${path}/${id(itemId, 'id')}`, body: input, ...(signal === undefined ? {} : { signal }) }), path);
    },
    async remove(itemId: string, signal?: AbortSignal): Promise<ActionSuccessResponse> {
      return parseActionSuccessResponse(await request({ method: 'DELETE', path: `${path}/${id(itemId, 'id')}`, ...(signal === undefined ? {} : { signal }) }));
    },
  });

  const parseAgent = (value: unknown, path: string) => parseOne(value, path) as AgentConfiguration;
  const parseModel = (value: unknown, path: string) => parseOne(value, path) as ModelConfiguration;
  const parseMcp = (value: unknown, path: string) => parseOne(value, path) as McpConfiguration;
  const agentCollection = collection<AgentConfiguration>('/api/v1/agents', parseAgent);
  const models = collection<ModelConfiguration>('/api/v1/models', parseModel);
  const mcp = collection<McpConfiguration>('/api/v1/mcp-services', parseMcp);
  const mcpOAuth = {
    async authorizeUrl(serviceId: string, input: { redirectURI: string; frontendRedirect?: string }, signal?: AbortSignal): Promise<McpOAuthAuthorization> {
      if (typeof input.redirectURI !== 'string' || input.redirectURI.trim() === '') throw new Error('redirectURI must not be empty');
      return parseOAuthAuthorization(await request({
        method: 'POST', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/oauth/authorize-url`,
        body: { redirect_uri: input.redirectURI, ...(input.frontendRedirect === undefined ? {} : { frontend_redirect: input.frontendRedirect }) },
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async status(serviceId: string, authorizationAttempt?: string, signal?: AbortSignal): Promise<McpOAuthStatus> {
      const query = authorizationAttempt === undefined ? '' : `?authorization_attempt=${encodeURIComponent(authorizationAttempt)}`;
      return parseOAuthStatus(await request({ method: 'GET', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/oauth/status${query}`, ...(signal === undefined ? {} : { signal }) }));
    },
    async revoke(serviceId: string, signal?: AbortSignal): Promise<void> {
      parseNoContent(await request({ method: 'DELETE', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/oauth/token`, ...(signal === undefined ? {} : { signal }) }), '/mcp-services/oauth/token DELETE');
    },
  };
  const modelCredentials = {
    async put(modelId: string, input: Record<string, unknown>, signal?: AbortSignal): Promise<ModelCredentialStatus> {
      const body = credentialBody(input, { apiKey: 'api_key', appSecret: 'app_secret' }, 'model credentials');
      const fields = parseCredentialStatus(await request({
        method: 'PUT', path: `/api/v1/models/${id(modelId, 'modelId')}/credentials`,
        body,
        ...(signal === undefined ? {} : { signal }),
      }), ['api_key', 'app_secret'], '/models/credentials');
      return { apiKey: body.api_key === undefined ? false : fields.api_key === true, appSecret: body.app_secret === undefined ? false : fields.app_secret === true };
    },
    async remove(modelId: string, field: 'api_key' | 'app_secret', signal?: AbortSignal): Promise<void> {
      parseNoContent(await request({ method: 'DELETE', path: `/api/v1/models/${id(modelId, 'modelId')}/credentials/${encodeURIComponent(field)}`, ...(signal === undefined ? {} : { signal }) }), '/models/credentials DELETE');
    },
  };
  const mcpCredentials = {
    async put(serviceId: string, input: Record<string, unknown>, signal?: AbortSignal): Promise<McpCredentialStatus> {
      const body = credentialBody(input, { apiKey: 'api_key', token: 'token' }, 'MCP credentials');
      const fields = parseCredentialStatus(await request({
        method: 'PUT', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/credentials`,
        body,
        ...(signal === undefined ? {} : { signal }),
      }), ['api_key', 'token'], '/mcp-services/credentials');
      return { apiKey: body.api_key === undefined ? false : fields.api_key === true, token: body.token === undefined ? false : fields.token === true };
    },
    async remove(serviceId: string, field: 'api_key' | 'token', signal?: AbortSignal): Promise<void> {
      parseNoContent(await request({ method: 'DELETE', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/credentials/${encodeURIComponent(field)}`, ...(signal === undefined ? {} : { signal }) }), '/mcp-services/credentials DELETE');
    },
  };
  return {
    agents: {
      ...agentCollection,
      async listWithState(options: AgentConfigurationListOptions = {}): Promise<AgentConfigurationList> {
        if (options.creator !== undefined && !['all', 'mine', 'others'].includes(options.creator)) {
          throw new Error('creator must be all, mine, or others');
        }
        const query = options.creator && options.creator !== 'all' ? `?creator=${options.creator}` : '';
        const path = `/api/v1/agents${query}`;
        return parseAgentList(await request({
          method: 'GET', path, ...(options.signal === undefined ? {} : { signal: options.signal }),
        }), '/api/v1/agents');
      },
      /** POST /api/v1/agents/:id/copy (routes_agent.go, Contributor+). */
      async copy(agentId: string, signal?: AbortSignal): Promise<AgentConfiguration> {
        return parseAgent(await request({
          method: 'POST', path: `/api/v1/agents/${id(agentId, 'agentId')}/copy`, ...(signal === undefined ? {} : { signal }),
        }), '/api/v1/agents');
      },
      async suggestedQuestions(agentId: string, options: AgentSuggestedQuestionsOptions = {}): Promise<string[]> {
        if (agentId.trim() === '') throw new Error('agentId must not be empty');
        const query = new URLSearchParams();
        if (options.knowledgeBaseIds?.length) query.set('knowledge_base_ids', options.knowledgeBaseIds.join(','));
        if (options.limit !== undefined && options.limit > 0) query.set('limit', String(options.limit));
        const suffix = query.toString();
        const data = successfulData(await request({
          method: 'GET',
          path: `/api/v1/agents/${id(agentId, 'agentId')}/suggested-questions${suffix ? `?${suffix}` : ''}`,
          ...(options.signal === undefined ? {} : { signal: options.signal }),
        }), '/agents/suggested-questions');
        const questions = (data as { questions?: unknown }).questions;
        if (!Array.isArray(questions)) {
          throw new Error('/agents/suggested-questions.data.questions must be an array');
        }
        // Backend items are {question, source, knowledge_base_id} objects
        // (frontend/src/api/agent SuggestedQuestion); plain strings stay
        // accepted for older payloads.
        const parsed = questions.map((item) => {
          if (typeof item === 'string') return item;
          if (item !== null && typeof item === 'object' && typeof (item as { question?: unknown }).question === 'string') {
            return (item as { question: string }).question;
          }
          return null;
        });
        if (parsed.some((item) => item === null)) {
          throw new Error('/agents/suggested-questions.data.questions items must be strings or {question} objects');
        }
        return parsed as string[];
      },
    },
    models: {
      ...models,
      credentials: modelCredentials,
      providers: {
        async list(modelType?: string, signal?: AbortSignal): Promise<ModelProvider[]> {
          const query = modelType === undefined ? '' : `?model_type=${encodeURIComponent(modelType)}`;
          const data = successfulData(await request({
            method: 'GET', path: `/api/v1/models/providers${query}`, ...(signal === undefined ? {} : { signal }),
          }), '/models/providers');
          if (!Array.isArray(data)) throw new Error('/models/providers.data must be an array');
          return data.map((item, index) => parseModelProvider(item, `/models/providers.data[${index}]`));
        },
      },
      connection: {
        async remote(input: ModelConnectionTestInput, signal?: AbortSignal): Promise<ModelConnectionTestResult> {
          return parseModelConnectionTest(await request({ method: 'POST', path: '/api/v1/initialization/remote/check', body: modelConnectionBody(input), ...(signal === undefined ? {} : { signal }) }), '/initialization/remote/check');
        },
        async embedding(input: ModelConnectionTestInput, signal?: AbortSignal): Promise<ModelConnectionTestResult> {
          return parseModelConnectionTest(await request({ method: 'POST', path: '/api/v1/initialization/embedding/test', body: modelConnectionBody(input), ...(signal === undefined ? {} : { signal }) }), '/initialization/embedding/test');
        },
        async rerank(input: ModelConnectionTestInput, signal?: AbortSignal): Promise<ModelConnectionTestResult> {
          return parseModelConnectionTest(await request({ method: 'POST', path: '/api/v1/initialization/rerank/check', body: modelConnectionBody(input), ...(signal === undefined ? {} : { signal }) }), '/initialization/rerank/check');
        },
        async asr(input: ModelConnectionTestInput, signal?: AbortSignal): Promise<ModelConnectionTestResult> {
          return parseModelConnectionTest(await request({ method: 'POST', path: '/api/v1/initialization/asr/check', body: modelConnectionBody(input), ...(signal === undefined ? {} : { signal }) }), '/initialization/asr/check');
        },
      },
      async debug(modelId: string, input: ModelDebugInput, signal?: AbortSignal): Promise<ModelDebugResult> {
        const fields: Record<string, string> = {};
        if (input.input !== undefined) fields.input = input.input;
        if (input.documents !== undefined) fields.documents = JSON.stringify(input.documents);
        if (input.options !== undefined) fields.options = JSON.stringify(modelDebugFields(input.options));
        const browserFile = input.file !== undefined && !isNativeFileSource(input.file) ? input.file : undefined;
        const nativeFile = input.file !== undefined && isNativeFileSource(input.file) ? input.file : undefined;
        return parseModelDebug(await request({
          method: 'POST', path: `/api/v1/models/${id(modelId, 'modelId')}/debug`,
          ...(browserFile === undefined ? {} : { body: multipartForm(fields, browserFile) }),
          ...(nativeFile === undefined ? {} : { nativeFile }),
          ...(browserFile !== undefined || Object.keys(fields).length === 0 ? {} : { multipartFields: fields }),
          ...(signal === undefined ? {} : { signal }),
        }));
      },
    },
    mcp: {
      ...mcp,
      credentials: mcpCredentials,
      oauth: mcpOAuth,
      async test(serviceId: string, signal?: AbortSignal): Promise<McpTestResult> {
        return parseMcpTest(await request({ method: 'POST', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/test`, ...(signal === undefined ? {} : { signal }) }));
      },
      async tools(serviceId: string, signal?: AbortSignal): Promise<McpTool[]> {
        return parseMcpTools(await request({ method: 'GET', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/tools`, ...(signal === undefined ? {} : { signal }) }));
      },
      metadata: {
        async get(serviceId: string, signal?: AbortSignal): Promise<McpMetadata | null> { return parseMcpMetadata(await request({ method: 'GET', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/metadata`, ...(signal === undefined ? {} : { signal }) })); },
        async refresh(serviceId: string, signal?: AbortSignal): Promise<McpMetadata> { const value = parseMcpMetadata(await request({ method: 'POST', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/metadata/refresh`, body: {}, ...(signal === undefined ? {} : { signal }) })); if (!value) throw new Error('/mcp-services/metadata/refresh must return metadata'); return value; },
      },
      usageInstructions: {
        async generate(serviceId: string, language: string, signal?: AbortSignal): Promise<string> { const data = record(successfulData(await request({ method: 'POST', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/usage-instructions/generate`, body: { language }, ...(signal === undefined ? {} : { signal }) }), '/mcp-services/usage-instructions/generate'), '/mcp-services/usage-instructions/generate.data'); return required(data.usage_instructions, '/mcp-services/usage-instructions/generate.data.usage_instructions'); },
      },
      toolApprovals: {
        async list(serviceId: string, signal?: AbortSignal): Promise<McpToolApproval[]> { return parseMcpApprovals(await request({ method: 'GET', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/tool-approvals`, ...(signal === undefined ? {} : { signal }) })); },
        async update(serviceId: string, toolName: string, input: { enabled?: boolean; requireApproval?: boolean }, signal?: AbortSignal): Promise<void> { if (input.enabled === undefined && input.requireApproval === undefined) throw new Error('enabled or requireApproval is required'); parseActionEnvelope(await request({ method: 'PUT', path: `/api/v1/mcp-services/${id(serviceId, 'serviceId')}/tool-approvals/${encodeURIComponent(toolName)}`, body: { ...(input.enabled === undefined ? {} : { enabled: input.enabled }), ...(input.requireApproval === undefined ? {} : { require_approval: input.requireApproval }) }, ...(signal === undefined ? {} : { signal }) }), '/mcp-services/tool-approvals PUT'); },
      },
    },
    skills: {
      async list(sandboxConfigId?: string, signal?: AbortSignal): Promise<SkillConfiguration[]> {
        const query = sandboxConfigId ? `?sandbox_config_id=${encodeURIComponent(sandboxConfigId)}` : '';
        return parseSkills(await request({ method: 'GET', path: `/api/v1/skills${query}`, ...(signal === undefined ? {} : { signal }) }));
      },
      async listWithAvailability(sandboxConfigId?: string, signal?: AbortSignal): Promise<SkillConfigurationList> {
        const query = sandboxConfigId ? `?sandbox_config_id=${encodeURIComponent(sandboxConfigId)}` : '';
        return parseSkillList(await request({
          method: 'GET', path: `/api/v1/skills${query}`, ...(signal === undefined ? {} : { signal }),
        }));
      },
      catalog: {
        async list(signal?: AbortSignal): Promise<SkillCatalog[]> {
          return parseCatalogList(await request({ method: 'GET', path: '/api/v1/skills/catalog', ...(signal === undefined ? {} : { signal }) }), '/skills/catalog');
        },
        async register(input: { source: string } | { file: NativeFileSource | Blob }, signal?: AbortSignal): Promise<SkillCatalog> {
          if ('source' in input) {
            const source = input.source.trim();
            if (!source) throw new Error('skill catalog source must not be empty');
            return parseCatalog(successfulData(await request({ method: 'POST', path: '/api/v1/skills/catalog', body: { source }, ...(signal === undefined ? {} : { signal }) }), '/skills/catalog'), '/skills/catalog.data', false);
          }
          return parseCatalog(successfulData(await request({ method: 'POST', path: '/api/v1/skills/catalog', nativeFile: toNativeFileSource(input.file), ...(signal === undefined ? {} : { signal }) }), '/skills/catalog'), '/skills/catalog.data', false);
        },
        async install(catalogId: string, sandboxConfigIds: string[], signal?: AbortSignal): Promise<SkillCatalogInstallResult> {
          if (!Array.isArray(sandboxConfigIds) || sandboxConfigIds.some((value) => typeof value !== 'string' || value.trim() === '')) throw new Error('sandboxConfigIds must be a string array');
          const envelope = acceptedData(await request({ method: 'POST', path: `/api/v1/skills/catalog/${id(catalogId, 'catalogId')}/install`, body: { sandbox_config_ids: sandboxConfigIds }, ...(signal === undefined ? {} : { signal }) }), '/skills/catalog/install');
          const data = record(envelope.data, '/skills/catalog/install.data');
          const installs = stringMap(data.installs, '/skills/catalog/install.data.installs', true);
          const errors = data.errors === undefined ? undefined : stringMap(data.errors, '/skills/catalog/install.data.errors', true);
          if (!envelope.success && (!errors || Object.keys(errors).length === 0)) throw new Error('/skills/catalog/install.data.errors is required for a partial result');
          return { installs, ...(errors === undefined ? {} : { errors }) };
        },
        async files(catalogId: string, signal?: AbortSignal): Promise<SkillFile[]> {
          return parseFileList(await request({ method: 'GET', path: `/api/v1/skills/catalog/${id(catalogId, 'catalogId')}/files`, ...(signal === undefined ? {} : { signal }) }), '/skills/catalog/files');
        },
        async file(catalogId: string, filePath: string, signal?: AbortSignal): Promise<SkillFileContent> {
          if (!filePath.trim()) throw new Error('filePath must not be empty');
          return parseFileContent(await request({ method: 'GET', path: `/api/v1/skills/catalog/${id(catalogId, 'catalogId')}/files/content?path=${encodeURIComponent(filePath)}`, ...(signal === undefined ? {} : { signal }) }), '/skills/catalog/files/content');
        },
        async remove(catalogId: string, signal?: AbortSignal): Promise<void> {
          parseActionEnvelope(await request({ method: 'DELETE', path: `/api/v1/skills/catalog/${id(catalogId, 'catalogId')}`, ...(signal === undefined ? {} : { signal }) }), '/skills/catalog DELETE');
        },
      },
      installed: {
        async list(configId: string, signal?: AbortSignal): Promise<InstalledSkill[]> {
          return parseInstalledList(await request({ method: 'GET', path: `/api/v1/sandbox-configs/${id(configId, 'configId')}/skills`, ...(signal === undefined ? {} : { signal }) }), '/sandbox-configs/skills');
        },
        async get(configId: string, skillId: string, signal?: AbortSignal): Promise<InstalledSkill> {
          return parseInstalledSkill(successfulData(await request({ method: 'GET', path: `/api/v1/sandbox-configs/${id(configId, 'configId')}/skills/${id(skillId, 'skillId')}`, ...(signal === undefined ? {} : { signal }) }), '/sandbox-configs/skill'), '/sandbox-configs/skill.data');
        },
        async files(configId: string, skillId: string, signal?: AbortSignal): Promise<SkillFile[]> {
          return parseFileList(await request({ method: 'GET', path: `/api/v1/sandbox-configs/${id(configId, 'configId')}/skills/${id(skillId, 'skillId')}/files`, ...(signal === undefined ? {} : { signal }) }), '/sandbox-configs/skill/files');
        },
        async file(configId: string, skillId: string, filePath: string, signal?: AbortSignal): Promise<SkillFileContent> {
          if (!filePath.trim()) throw new Error('filePath must not be empty');
          return parseFileContent(await request({ method: 'GET', path: `/api/v1/sandbox-configs/${id(configId, 'configId')}/skills/${id(skillId, 'skillId')}/files/content?path=${encodeURIComponent(filePath)}`, ...(signal === undefined ? {} : { signal }) }), '/sandbox-configs/skill/files/content');
        },
        async reinstall(configId: string, skillId: string, instructions?: string, signal?: AbortSignal): Promise<SkillAcceptedResult> {
          return parseSkillIdResult(await request({ method: 'POST', path: `/api/v1/sandbox-configs/${id(configId, 'configId')}/skills/${id(skillId, 'skillId')}/reinstall`, body: instructions === undefined ? {} : { instructions }, ...(signal === undefined ? {} : { signal }) }), '/sandbox-configs/skill/reinstall');
        },
        async stop(configId: string, skillId: string, signal?: AbortSignal): Promise<InstalledSkill> {
          return parseInstalledSkill(successfulData(await request({ method: 'POST', path: `/api/v1/sandbox-configs/${id(configId, 'configId')}/skills/${id(skillId, 'skillId')}/stop`, ...(signal === undefined ? {} : { signal }) }), '/sandbox-configs/skill/stop'), '/sandbox-configs/skill/stop.data');
        },
        async update(configId: string, skillId: string, input: SandboxSkillUpdate, signal?: AbortSignal): Promise<InstalledSkill> {
          if (input.enabled === undefined && input.envs === undefined) throw new Error('enabled or envs is required');
          return parseInstalledSkill(successfulData(await request({ method: 'PATCH', path: `/api/v1/sandbox-configs/${id(configId, 'configId')}/skills/${id(skillId, 'skillId')}`, body: { ...(input.enabled === undefined ? {} : { enabled: input.enabled }), ...(input.envs === undefined ? {} : { envs: input.envs }) }, ...(signal === undefined ? {} : { signal }) }), '/sandbox-configs/skill/update'), '/sandbox-configs/skill/update.data');
        },
        async remove(configId: string, skillId: string, signal?: AbortSignal): Promise<SkillAcceptedResult> {
          return parseSkillIdResult(await request({ method: 'DELETE', path: `/api/v1/sandbox-configs/${id(configId, 'configId')}/skills/${id(skillId, 'skillId')}`, ...(signal === undefined ? {} : { signal }) }), '/sandbox-configs/skill/delete');
        },
      },
    },
  };
}

export type ConfigurationApi = ReturnType<typeof createConfigurationApi>;
