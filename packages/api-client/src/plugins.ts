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
  /**
   * POST the bodyless read-only upgrade-preview (Admin): re-fetches the
   * installation's manifest source, verifies the candidate and resolves to the
   * five-dimension diff. The accepted version never changes server-side.
   */
  previewUpgrade(installationId: string, signal?: AbortSignal): Promise<PluginUpgradePreview>;
  /**
   * GET the member's personal connection view of one installation (Viewer+):
   * the three-state OAuth verdict plus the legacy MCP OAuth endpoint paths
   * mapped onto the materialized service_id. Never carries token material.
   */
  getMyConnection(installationId: string, signal?: AbortSignal): Promise<PluginMyConnection>;
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
    async previewUpgrade(installationId: string, signal?: AbortSignal): Promise<PluginUpgradePreview> {
      const id = installationId.trim();
      if (id === '') throw new Error('installationId must not be empty');
      return parsePluginUpgradePreview(await request({
        method: 'POST',
        path: `${INSTALLATIONS_PATH}/${encodeURIComponent(id)}/upgrade-preview`,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async getMyConnection(installationId: string, signal?: AbortSignal): Promise<PluginMyConnection> {
      const id = installationId.trim();
      if (id === '') throw new Error('installationId must not be empty');
      return parsePluginMyConnection(await request({
        method: 'GET',
        path: `${INSTALLATIONS_PATH}/${encodeURIComponent(id)}/connections/me`,
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

// ---- T15: upgrade-preview envelope (dto.PluginUpgradePreviewResponse, Issue #114;
// handler internal/handler/plugin.go PreviewUpgrade; five-dimension diff DTOs above it) ----

/** One tool snapshot row inside an upgrade-preview diff (dto.PluginToolSnapshotDTO). */
export interface PluginUpgradeToolSnapshot {
  readonly name: string;
  readonly description: string;
  readonly inputSchemaDigest: string;
  readonly readOnly: boolean;
  readonly requiresPersonalAuth: boolean;
  readonly scopes: readonly string[];
}

/**
 * One changed tool with the four independent change-reason badges the admin
 * reviews separately: schema / scope / 读写分类 / 授权面
 * (dto.PluginToolChangeDTO), plus before/after snapshots for drill-down.
 */
export interface PluginUpgradeToolChange {
  readonly name: string;
  readonly schemaChanged: boolean;
  readonly scopeChanged: boolean;
  readonly readWriteClassChanged: boolean;
  readonly personalAuthChanged: boolean;
  readonly current: PluginUpgradeToolSnapshot;
  readonly candidate: PluginUpgradeToolSnapshot;
}

/** Five-dimension version diff (dto.PluginVersionDiffDTO) in camelCase. */
export interface PluginVersionDiff {
  readonly pluginId: string;
  readonly currentVersion: string;
  readonly candidateVersion: string;
  readonly isDowngrade: boolean;
  readonly endpointChanged: boolean;
  readonly currentEndpoint: string;
  readonly candidateEndpoint: string;
  readonly addedTools: readonly PluginUpgradeToolSnapshot[];
  readonly removedTools: readonly PluginUpgradeToolSnapshot[];
  readonly changedTools: readonly PluginUpgradeToolChange[];
}

/** Upgrade-preview payload (dto.PluginUpgradePreviewResponse) in camelCase. */
export interface PluginUpgradePreview {
  readonly diff: PluginVersionDiff;
  readonly candidateFingerprint: string;
  readonly candidateToolsDigest: string;
}

const UPGRADE_PREVIEW_PATH = `${INSTALLATIONS_PATH}/:id/upgrade-preview`;

function parseUpgradeToolSnapshot(value: unknown, path: string): PluginUpgradeToolSnapshot {
  const row = record(value, path);
  return {
    name: required(row.name, `${path}.name`),
    description: optionalText(row.description, `${path}.description`),
    inputSchemaDigest: optionalText(row.input_schema_digest, `${path}.input_schema_digest`),
    readOnly: flag(row.read_only, `${path}.read_only`),
    requiresPersonalAuth: flag(row.requires_personal_auth, `${path}.requires_personal_auth`),
    scopes: scopeList(row.scopes, `${path}.scopes`),
  };
}

/**
 * Strict parser for the upgrade-preview envelope: the five-dimension diff
 * (version pair with the downgrade flag, endpoint pair, added/removed/changed
 * tool rows) plus the candidate identity the tenant WOULD accept. Metadata
 * only — schema digests cross the wire, never schema text.
 */
export function parsePluginUpgradePreview(value: unknown): PluginUpgradePreview {
  const envelope = record(value, UPGRADE_PREVIEW_PATH);
  if (envelope.success !== true) throw new Error(`${UPGRADE_PREVIEW_PATH}.success must be true`);
  const data = record(envelope.data, `${UPGRADE_PREVIEW_PATH}.data`);
  const rawDiff = record(data.diff, `${UPGRADE_PREVIEW_PATH}.data.diff`);
  for (const key of ['added_tools', 'removed_tools', 'changed_tools'] as const) {
    if (!Array.isArray(rawDiff[key])) throw new Error(`${UPGRADE_PREVIEW_PATH}.data.diff.${key} must be an array`);
  }
  return {
    diff: {
      pluginId: required(rawDiff.plugin_id, `${UPGRADE_PREVIEW_PATH}.data.diff.plugin_id`),
      currentVersion: required(rawDiff.current_version, `${UPGRADE_PREVIEW_PATH}.data.diff.current_version`),
      candidateVersion: required(rawDiff.candidate_version, `${UPGRADE_PREVIEW_PATH}.data.diff.candidate_version`),
      isDowngrade: flag(rawDiff.is_downgrade, `${UPGRADE_PREVIEW_PATH}.data.diff.is_downgrade`),
      endpointChanged: flag(rawDiff.endpoint_changed, `${UPGRADE_PREVIEW_PATH}.data.diff.endpoint_changed`),
      currentEndpoint: required(rawDiff.current_endpoint, `${UPGRADE_PREVIEW_PATH}.data.diff.current_endpoint`),
      candidateEndpoint: required(rawDiff.candidate_endpoint, `${UPGRADE_PREVIEW_PATH}.data.diff.candidate_endpoint`),
      addedTools: (rawDiff.added_tools as unknown[]).map((item, index) =>
        parseUpgradeToolSnapshot(item, `${UPGRADE_PREVIEW_PATH}.data.diff.added_tools[${index}]`)),
      removedTools: (rawDiff.removed_tools as unknown[]).map((item, index) =>
        parseUpgradeToolSnapshot(item, `${UPGRADE_PREVIEW_PATH}.data.diff.removed_tools[${index}]`)),
      changedTools: (rawDiff.changed_tools as unknown[]).map((item, index) => {
        const row = record(item, `${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}]`);
        return {
          name: required(row.name, `${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}].name`),
          schemaChanged: flag(row.schema_changed, `${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}].schema_changed`),
          scopeChanged: flag(row.scope_changed, `${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}].scope_changed`),
          readWriteClassChanged: flag(row.read_write_class_changed, `${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}].read_write_class_changed`),
          personalAuthChanged: flag(row.personal_auth_changed, `${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}].personal_auth_changed`),
          current: parseUpgradeToolSnapshot(row.current, `${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}].current`),
          candidate: parseUpgradeToolSnapshot(row.candidate, `${UPGRADE_PREVIEW_PATH}.data.diff.changed_tools[${index}].candidate`),
        };
      }),
    },
    candidateFingerprint: required(data.candidate_fingerprint, `${UPGRADE_PREVIEW_PATH}.data.candidate_fingerprint`),
    candidateToolsDigest: required(data.candidate_tools_digest, `${UPGRADE_PREVIEW_PATH}.data.candidate_tools_digest`),
  };
}

// ---- T12: member personal connection envelope (dto.PluginMyConnection,
// GET /plugins/installations/:id/connections/me, handler internal/handler/plugin.go GetMyConnection) ----

/**
 * The member's personal OAuth connection view of one installation: the
 * materialized service binding, the three-state verdict
 * (authorized | expired | unauthorized) and the legacy MCP OAuth endpoint
 * paths mapped onto that service_id — the panel drives authorize/revoke
 * through those existing endpoints. No token material ever crosses the wire.
 */
export interface PluginMyConnection {
  readonly installationId: string;
  readonly pluginId: string;
  readonly name: string;
  readonly serviceId: string;
  readonly requiresPersonalAuth: boolean;
  readonly authorized: boolean;
  readonly state: 'authorized' | 'expired' | 'unauthorized';
  /** Legacy authorize-url endpoint path; empty when the plugin needs no personal auth. */
  readonly authorizeUrlPath: string;
  /** Legacy DELETE-token endpoint path; empty when the plugin needs no personal auth. */
  readonly revokePath: string;
  readonly requiresAuthTools: readonly string[];
}

const CONNECTION_PATH = '/api/v1/plugins/installations/connections/me';

const CONNECTION_STATES = ['authorized', 'expired', 'unauthorized'] as const;

function connectionState(value: unknown, path: string): 'authorized' | 'expired' | 'unauthorized' {
  // Deliberately narrower than the mcp oauth STATUS vocabulary
  // (refreshable/reauth_required/pending) — that is a different endpoint's
  // state machine and must never pass as a plugin connection state.
  if (typeof value !== 'string' || !CONNECTION_STATES.includes(value as 'authorized' | 'expired' | 'unauthorized')) {
    throw new Error(`${path} must be one of ${CONNECTION_STATES.join('|')}`);
  }
  return value as 'authorized' | 'expired' | 'unauthorized';
}

/**
 * Strict parser for the personal connection envelope: identity, service
 * binding, three-state verdict and mapped endpoint paths. authorize_url_path /
 * revoke_path may legitimately be EMPTY (no-personal-auth plugins carry no
 * OAuth endpoints — T11 ruling), so only their type is enforced.
 */
export function parsePluginMyConnection(value: unknown): PluginMyConnection {
  const envelope = record(value, CONNECTION_PATH);
  if (envelope.success !== true) throw new Error(`${CONNECTION_PATH}.success must be true`);
  const data = record(envelope.data, `${CONNECTION_PATH}.data`);
  return {
    installationId: required(data.installation_id, `${CONNECTION_PATH}.data.installation_id`),
    pluginId: required(data.plugin_id, `${CONNECTION_PATH}.data.plugin_id`),
    name: required(data.name, `${CONNECTION_PATH}.data.name`),
    serviceId: required(data.service_id, `${CONNECTION_PATH}.data.service_id`),
    requiresPersonalAuth: flag(data.requires_personal_auth, `${CONNECTION_PATH}.data.requires_personal_auth`),
    authorized: flag(data.authorized, `${CONNECTION_PATH}.data.authorized`),
    state: connectionState(data.state, `${CONNECTION_PATH}.data.state`),
    authorizeUrlPath: optionalText(data.authorize_url_path, `${CONNECTION_PATH}.data.authorize_url_path`),
    revokePath: optionalText(data.revoke_path, `${CONNECTION_PATH}.data.revoke_path`),
    requiresAuthTools: scopeList(data.requires_auth_tools, `${CONNECTION_PATH}.data.requires_auth_tools`),
  };
}
