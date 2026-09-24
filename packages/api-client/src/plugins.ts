import type { ClientRequest } from './client.ts';

/**
 * Plugin preview + installation API — Issue #108/#110 manifest preview,
 * confirm-install, discovery list and enable/disable governance
 * (internal/handler/plugin.go; response DTOs internal/handler/dto/plugin.go).
 *
 * T08: confirmInstallation/listInstallations/getInstallation/setInstallationState
 * join previewInstallation here as the SINGLE source of the wire-format parsers
 * (the settings panel's former mirror copy was deleted in favour of this
 * module — deep-path import precedent SandboxSettingsPanel.tsx). Mounting the
 * domain on WeKnoraClient itself (client.plugins via index.ts) stays deferred
 * to a later slice that owns client.ts/index.ts wiring; panels construct the
 * API from the client's exposed `request` transport.
 */

/** One verified tool row of the admin preview table (dto.PluginPreviewTool). */
export interface PluginPreviewTool {
  readonly name: string;
  readonly description: string;
  readonly readOnly: boolean;
  readonly requiresPersonalAuth: boolean;
  readonly scopes: readonly string[];
}

/** Verified preview payload (dto.PluginPreviewResponse) in camelCase. */
export interface PluginPreviewResult {
  readonly previewId: string;
  readonly pluginId: string;
  readonly version: string;
  readonly name: string;
  readonly description: string;
  readonly transportType: string;
  readonly endpointUrl: string;
  readonly tools: readonly PluginPreviewTool[];
  readonly identityFingerprint: string;
  readonly expiresAt: string;
}

const PREVIEW_PATH = '/api/v1/plugins/installations/preview';

type RecordValue = Record<string, unknown>;

function record(value: unknown, path: string): RecordValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path} must be an object`);
  return value as RecordValue;
}

function required(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}

function optionalText(value: unknown, path: string): string {
  // dto.PluginPreviewResponse.Description has no omitempty; the manifest
  // protocol allows an empty description, so "" is valid — only the type is
  // enforced here.
  if (typeof value !== 'string') throw new Error(`${path} must be a string`);
  return value;
}

function flag(value: unknown, path: string): boolean {
  if (typeof value !== 'boolean') throw new Error(`${path} must be a boolean`);
  return value;
}

function scopeList(value: unknown, path: string): string[] {
  // Go nil slices serialize as JSON null; a verified tool may declare none.
  if (value === null || value === undefined) return [];
  if (!Array.isArray(value) || value.some((item) => typeof item !== 'string')) throw new Error(`${path} must be a string array`);
  return value as string[];
}

function parsePreviewTool(value: unknown, path: string): PluginPreviewTool {
  const row = record(value, path);
  return {
    name: required(row.name, `${path}.name`),
    description: optionalText(row.description, `${path}.description`),
    readOnly: flag(row.read_only, `${path}.read_only`),
    requiresPersonalAuth: flag(row.requires_personal_auth, `${path}.requires_personal_auth`),
    scopes: scopeList(row.scopes, `${path}.scopes`),
  };
}

/**
 * Strict parser for the plugin preview envelope: rejects non-success
 * envelopes and any missing/malformed preview field before untrusted remote
 * data reaches caller code. The tool table intentionally carries metadata
 * only — the input schema itself never crosses the wire (the backend rejects
 * the whole preview on a digest mismatch), so no schema text can flow in here.
 */
export function parsePluginPreview(value: unknown): PluginPreviewResult {
  const envelope = record(value, PREVIEW_PATH);
  if (envelope.success !== true) throw new Error(`${PREVIEW_PATH}.success must be true`);
  const data = record(envelope.data, `${PREVIEW_PATH}.data`);
  const transportType = required(data.transport_type, `${PREVIEW_PATH}.data.transport_type`);
  if (transportType !== 'http-streamable' && transportType !== 'sse') {
    throw new Error(`${PREVIEW_PATH}.data.transport_type is invalid`);
  }
  const tools = data.tools;
  if (!Array.isArray(tools)) throw new Error(`${PREVIEW_PATH}.data.tools must be an array`);
  return {
    previewId: required(data.preview_id, `${PREVIEW_PATH}.data.preview_id`),
    pluginId: required(data.plugin_id, `${PREVIEW_PATH}.data.plugin_id`),
    version: required(data.version, `${PREVIEW_PATH}.data.version`),
    name: required(data.name, `${PREVIEW_PATH}.data.name`),
    description: optionalText(data.description, `${PREVIEW_PATH}.data.description`),
    transportType,
    endpointUrl: required(data.endpoint_url, `${PREVIEW_PATH}.data.endpoint_url`),
    tools: tools.map((item, index) => parsePreviewTool(item, `${PREVIEW_PATH}.data.tools[${index}]`)),
    identityFingerprint: required(data.identity_fingerprint, `${PREVIEW_PATH}.data.identity_fingerprint`),
    expiresAt: required(data.expires_at, `${PREVIEW_PATH}.data.expires_at`),
  };
}

export interface PluginsApi {
  /** POST the admin-pasted manifest URL; resolves to the verified preview. */
  previewInstallation(manifestUrl: string, signal?: AbortSignal): Promise<PluginPreviewResult>;
  /** POST the reviewed preview id; consumes the preview and installs the pinned version (Admin). */
  confirmInstallation(previewId: string, signal?: AbortSignal): Promise<PluginInstallation>;
  /** GET the tenant's installed plugins (Viewer+ discovery, tenant-scoped server-side). */
  listInstallations(signal?: AbortSignal): Promise<PluginInstallationSummary[]>;
  /** GET one installation's full view (Viewer+; foreign-tenant ids read as errors server-side). */
  getInstallation(installationId: string, signal?: AbortSignal): Promise<PluginInstallation>;
  /** POST disable/enable for one installation (Admin); state is 'active' | 'disabled'. */
  setInstallationState(installationId: string, state: 'active' | 'disabled', signal?: AbortSignal): Promise<PluginInstallation>;
}

/** Build the plugins domain API over the shared client request transport. */
export function createPluginsApi(request: (input: ClientRequest) => Promise<unknown>): PluginsApi {
  return {
    async previewInstallation(manifestUrl: string, signal?: AbortSignal): Promise<PluginPreviewResult> {
      const url = manifestUrl.trim();
      if (url === '') throw new Error('manifestUrl must not be empty');
      return parsePluginPreview(await request({
        method: 'POST',
        path: PREVIEW_PATH,
        body: { manifest_url: url },
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async confirmInstallation(previewId: string, signal?: AbortSignal): Promise<PluginInstallation> {
      const id = previewId.trim();
      if (id === '') throw new Error('previewId must not be empty');
      return parsePluginInstallation(await request({
        method: 'POST',
        path: INSTALLATIONS_PATH,
        body: { preview_id: id },
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async listInstallations(signal?: AbortSignal): Promise<PluginInstallationSummary[]> {
      return parsePluginInstallations(await request({
        method: 'GET',
        path: INSTALLATIONS_PATH,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async getInstallation(installationId: string, signal?: AbortSignal): Promise<PluginInstallation> {
      const id = installationId.trim();
      if (id === '') throw new Error('installationId must not be empty');
      return parsePluginInstallation(await request({
        method: 'GET',
        path: `${INSTALLATIONS_PATH}/${encodeURIComponent(id)}`,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async setInstallationState(installationId: string, state: 'active' | 'disabled', signal?: AbortSignal): Promise<PluginInstallation> {
      const id = installationId.trim();
      if (id === '') throw new Error('installationId must not be empty');
      return parsePluginInstallation(await request({
        method: 'POST',
        path: `${INSTALLATIONS_PATH}/${encodeURIComponent(id)}/${state === 'disabled' ? 'disable' : 'enable'}`,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
  };
}

// ---- T08: installation envelopes (dto.PluginInstallationResponse / .PluginInstallationSummary,
// internal/handler/dto/plugin.go; responses internal/handler/plugin.go confirm/state/get/list) ----

/** One tool row of an installation directory (dto.PluginInstallationTool). */
export interface PluginInstallationTool {
  readonly name: string;
  readonly description: string;
  readonly readOnly: boolean;
  readonly requiresPersonalAuth: boolean;
  readonly scopes: readonly string[];
  /** dto Enabled *bool omitempty: absent key = no explicit policy row = unknown, kept as null. */
  readonly enabled: boolean | null;
}

/** Full installation payload (confirm / state change / get-by-id) in camelCase. */
export interface PluginInstallation {
  readonly installationId: string;
  readonly pluginId: string;
  readonly name: string;
  readonly description: string;
  readonly version: string;
  readonly state: 'active' | 'disabled';
  readonly driftState: 'none' | 'detected';
  readonly transportType: 'http-streamable' | 'sse';
  readonly endpointUrl: string;
  readonly serviceId: string;
  readonly tools: readonly PluginInstallationTool[];
}

/** One row of the member-facing list (dto.PluginInstallationSummary) in camelCase. */
export interface PluginInstallationSummary {
  readonly installationId: string;
  readonly pluginId: string;
  readonly name: string;
  readonly version: string;
  readonly state: 'active' | 'disabled';
  readonly driftState: 'none' | 'detected';
  readonly requiresPersonalAuth: boolean;
  readonly toolCount: number;
}

const INSTALLATIONS_PATH = '/api/v1/plugins/installations';

const INSTALLATION_STATES = ['active', 'disabled'] as const;
const DRIFT_STATES = ['none', 'detected'] as const;

function installationState(value: unknown, path: string): 'active' | 'disabled' {
  if (typeof value !== 'string' || !INSTALLATION_STATES.includes(value as 'active' | 'disabled')) {
    throw new Error(`${path} must be one of ${INSTALLATION_STATES.join('|')}`);
  }
  return value as 'active' | 'disabled';
}

function driftState(value: unknown, path: string): 'none' | 'detected' {
  if (typeof value !== 'string' || !DRIFT_STATES.includes(value as 'none' | 'detected')) {
    throw new Error(`${path} must be one of ${DRIFT_STATES.join('|')}`);
  }
  return value as 'none' | 'detected';
}

function optionalFlag(value: unknown, path: string): boolean | null {
  // Go *bool with omitempty: the key is absent when the pointer is nil.
  if (value === null || value === undefined) return null;
  return flag(value, path);
}

function toolCount(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isInteger(value) || value < 0) {
    throw new Error(`${path} must be a non-negative integer`);
  }
  return value;
}

function parseInstallationTool(value: unknown, path: string): PluginInstallationTool {
  const row = record(value, path);
  return {
    name: required(row.name, `${path}.name`),
    description: optionalText(row.description, `${path}.description`),
    readOnly: flag(row.read_only, `${path}.read_only`),
    requiresPersonalAuth: flag(row.requires_personal_auth, `${path}.requires_personal_auth`),
    scopes: scopeList(row.scopes, `${path}.scopes`),
    enabled: optionalFlag(row.enabled, `${path}.enabled`),
  };
}

/**
 * Strict parser for the single-installation envelope (confirm, disable/enable,
 * get-by-id all return dto.PluginInstallationResponse). Metadata only — the
 * input schema itself never crosses the wire.
 */
export function parsePluginInstallation(value: unknown): PluginInstallation {
  const envelope = record(value, INSTALLATIONS_PATH);
  if (envelope.success !== true) throw new Error(`${INSTALLATIONS_PATH}.success must be true`);
  const data = record(envelope.data, `${INSTALLATIONS_PATH}.data`);
  const transportType = required(data.transport_type, `${INSTALLATIONS_PATH}.data.transport_type`);
  if (transportType !== 'http-streamable' && transportType !== 'sse') {
    throw new Error(`${INSTALLATIONS_PATH}.data.transport_type is invalid`);
  }
  const tools = data.tools;
  if (!Array.isArray(tools)) throw new Error(`${INSTALLATIONS_PATH}.data.tools must be an array`);
  return {
    installationId: required(data.installation_id, `${INSTALLATIONS_PATH}.data.installation_id`),
    pluginId: required(data.plugin_id, `${INSTALLATIONS_PATH}.data.plugin_id`),
    name: required(data.name, `${INSTALLATIONS_PATH}.data.name`),
    description: optionalText(data.description, `${INSTALLATIONS_PATH}.data.description`),
    version: required(data.version, `${INSTALLATIONS_PATH}.data.version`),
    state: installationState(data.state, `${INSTALLATIONS_PATH}.data.state`),
    driftState: driftState(data.drift_state, `${INSTALLATIONS_PATH}.data.drift_state`),
    transportType,
    endpointUrl: required(data.endpoint_url, `${INSTALLATIONS_PATH}.data.endpoint_url`),
    serviceId: required(data.service_id, `${INSTALLATIONS_PATH}.data.service_id`),
    tools: tools.map((item, index) => parseInstallationTool(item, `${INSTALLATIONS_PATH}.data.tools[${index}]`)),
  };
}

/**
 * Strict parser for the member-facing list envelope: data is an ARRAY of
 * dto.PluginInstallationSummary rows (handler ListInstallations).
 */
export function parsePluginInstallations(value: unknown): PluginInstallationSummary[] {
  const envelope = record(value, INSTALLATIONS_PATH);
  if (envelope.success !== true) throw new Error(`${INSTALLATIONS_PATH}.success must be true`);
  const data = envelope.data;
  if (!Array.isArray(data)) throw new Error(`${INSTALLATIONS_PATH}.data must be an array`);
  return data.map((item, index) => {
    const row = record(item, `${INSTALLATIONS_PATH}.data[${index}]`);
    return {
      installationId: required(row.installation_id, `${INSTALLATIONS_PATH}.data[${index}].installation_id`),
      pluginId: required(row.plugin_id, `${INSTALLATIONS_PATH}.data[${index}].plugin_id`),
      name: required(row.name, `${INSTALLATIONS_PATH}.data[${index}].name`),
      version: required(row.version, `${INSTALLATIONS_PATH}.data[${index}].version`),
      state: installationState(row.state, `${INSTALLATIONS_PATH}.data[${index}].state`),
      driftState: driftState(row.drift_state, `${INSTALLATIONS_PATH}.data[${index}].drift_state`),
      requiresPersonalAuth: flag(row.requires_personal_auth, `${INSTALLATIONS_PATH}.data[${index}].requires_personal_auth`),
      toolCount: toolCount(row.tool_count, `${INSTALLATIONS_PATH}.data[${index}].tool_count`),
    };
  });
}
