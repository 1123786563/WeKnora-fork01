import type { ClientRequest } from './client.ts';

/**
 * Plugin preview API — Issue #108 admin manifest preview
 * (POST /api/v1/plugins/installations/preview, internal/handler/plugin.go;
 * response DTO internal/handler/dto/plugin.go PluginPreviewResponse).
 *
 * T03 scope: the strict envelope parser plus a request-factory. Mounting the
 * domain on WeKnoraClient itself (client.plugins) is deferred to T08/T15,
 * which own client.ts/index.ts wiring; until then panels construct the API
 * from the client's exposed `request` transport.
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
  };
}
