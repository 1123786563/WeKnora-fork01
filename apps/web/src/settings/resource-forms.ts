// Structured-form domain for the three engine add/edit drawers — the React
// port of R482 B3-D1. Field rendering and payload assembly mirror the Vue
// sources exactly:
//   frontend/src/views/settings/VectorStoreSettings.vue   (onDrawerConfirm, fieldLabel)
//   frontend/src/views/settings/StorageBackendSettings.vue (blankConfig, needs*, save)
//   frontend/src/views/settings/WebSearchSettings.vue      (saveProvider, canTestConnection)
// The api-client returns unknown-typed rows, so type metadata is defensively
// parsed here before the drawers render it.

// ---------------------------------------------------------------------------
// Vector store engine metadata (GET /vector-stores/types)
// ---------------------------------------------------------------------------

export type VectorFieldType = 'string' | 'number' | 'boolean';

export interface VectorFieldSchema {
  name: string;
  type: VectorFieldType;
  required: boolean;
  sensitive?: boolean;
  description?: string;
  default?: string | number | boolean;
  min?: number;
  max?: number;
  enum?: string[];
}

export interface VectorStoreTypeInfo {
  type: string;
  display_name: string;
  connection_fields: VectorFieldSchema[];
  index_fields: VectorFieldSchema[];
}

function parseVectorField(value: unknown): VectorFieldSchema | null {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return null;
  const row = value as Record<string, unknown>;
  const name = typeof row.name === 'string' ? row.name : '';
  const type = row.type === 'string' || row.type === 'number' || row.type === 'boolean' ? row.type : null;
  if (!name || !type) return null;
  return {
    name,
    type,
    required: row.required === true,
    ...(row.sensitive === true ? { sensitive: true } : {}),
    ...(typeof row.description === 'string' ? { description: row.description } : {}),
    ...(row.default === undefined ? {} : { default: row.default as VectorFieldSchema['default'] }),
    ...(typeof row.min === 'number' ? { min: row.min } : {}),
    ...(typeof row.max === 'number' ? { max: row.max } : {}),
    ...(Array.isArray(row.enum)
      ? { enum: row.enum.filter((entry): entry is string => typeof entry === 'string') }
      : {}),
  };
}

function parseVectorFields(value: unknown): VectorFieldSchema[] {
  return Array.isArray(value)
    ? value.map(parseVectorField).filter((entry): entry is VectorFieldSchema => entry !== null)
    : [];
}

export function parseVectorStoreTypes(rows: readonly unknown[]): VectorStoreTypeInfo[] {
  return rows
    .filter((row): row is Record<string, unknown> => row !== null && typeof row === 'object' && !Array.isArray(row))
    .map((row) => {
      const type = typeof row.type === 'string' ? row.type : '';
      if (!type) return null;
      return {
        type,
        display_name: typeof row.display_name === 'string' ? row.display_name : type,
        connection_fields: parseVectorFields(row.connection_fields),
        index_fields: parseVectorFields(row.index_fields),
      } satisfies VectorStoreTypeInfo;
    })
    .filter((entry): entry is VectorStoreTypeInfo => entry !== null);
}

/** Vue VectorStoreSettings.fieldLabel — i18n key, falling back to the raw name. */
export function vectorStoreFieldLabel(t: (key: string) => string, name: string): string {
  const key = `vectorStoreSettings.fields.${name}`;
  const translated = t(key);
  return translated === key ? name : translated;
}

export function vectorStoreCreatePayload(input: {
  name: string;
  engineType: string;
  connectionConfig: Record<string, unknown>;
  indexConfig: Record<string, unknown>;
  includeIndex: boolean;
}): { name: string; engine_type: string; connection_config: Record<string, unknown>; index_config: Record<string, unknown> } {
  const name = input.name.trim();
  if (!name) throw new Error('Resource name is required');
  if (!input.engineType.trim()) throw new Error('Engine type is required');
  return {
    name,
    engine_type: input.engineType,
    connection_config: { ...input.connectionConfig },
    // Vue submits the typed index fields only while the advanced section is
    // expanded — collapsed means "keep engine defaults" ({}).
    index_config: input.includeIndex ? { ...input.indexConfig } : {},
  };
}

/** Edit mode only allows the name — engine/connection/index are immutable. */
export function vectorStoreUpdatePayload(name: string): { name: string } {
  const trimmed = name.trim();
  if (!trimmed) throw new Error('Resource name is required');
  return { name: trimmed };
}

/** Vue canTestConnection (create mode): every required connection field filled. */
export function vectorStoreCanTest(type: VectorStoreTypeInfo | null, connectionConfig: Record<string, unknown>): boolean {
  if (!type) return false;
  for (const field of type.connection_fields) {
    if (!field.required) continue;
    const value = connectionConfig[field.name];
    if (value == null || value === '' || (typeof value === 'string' && value.trim() === '')) return false;
  }
  return true;
}

/** Vue isReplicaField — replicas cap at 10, shards at 64 when max is absent. */
const replicaFieldNames = ['number_of_replicas', 'replication_factor', 'replica_number'];
export function isReplicaField(name: string): boolean {
  return replicaFieldNames.includes(name);
}

// ---------------------------------------------------------------------------
// Storage backend metadata (GET /storage-backends/types → provider ids)
// ---------------------------------------------------------------------------

export interface StorageBackendConfig {
  mode: string;
  endpoint: string;
  region: string;
  access_key_id: string;
  secret_access_key: string;
  bucket_name: string;
  path_prefix: string;
  app_id?: string;
  temp_bucket_name?: string;
  temp_region?: string;
  use_ssl: boolean;
  force_path_style?: boolean;
  use_temp_bucket?: boolean;
}

export function storageBlankConfig(): StorageBackendConfig {
  return { mode: 'remote', endpoint: '', region: '', access_key_id: '', secret_access_key: '', bucket_name: '', path_prefix: '', use_ssl: true };
}

export function parseStorageProviders(rows: readonly unknown[]): string[] {
  return rows.filter((row): row is string => typeof row === 'string');
}

export function storageNeedsEndpoint(provider: string, mode: string): boolean {
  return !['local', 'cos'].includes(provider) && !(provider === 'minio' && mode === 'docker');
}

export function storageNeedsRegion(provider: string): boolean {
  return !['local', 'minio'].includes(provider);
}

export function storageNeedsCredentials(provider: string, mode: string): boolean {
  return provider !== 'local' && !(provider === 'minio' && mode === 'docker');
}

export function storageBackendPayload(name: string, provider: string, config: StorageBackendConfig): { name: string; provider: string; config: Record<string, unknown> } {
  const trimmed = name.trim();
  if (!trimmed) throw new Error('Resource name is required');
  if (!provider.trim()) throw new Error('Provider type is required');
  return { name: trimmed, provider, config: { ...config } };
}

// ---------------------------------------------------------------------------
// Web search provider metadata (GET /web-search-providers/types)
// ---------------------------------------------------------------------------

export interface WebSearchConfigOption { label: string; label_key?: string; value: string }
export interface WebSearchConfigField {
  key: string;
  label: string;
  label_key?: string;
  type: 'select';
  required?: boolean;
  default?: string;
  description?: string;
  description_key?: string;
  options: WebSearchConfigOption[];
}

export interface WebSearchTypeInfo {
  id: string;
  name: string;
  requires_api_key: boolean;
  supports_optional_api_key?: boolean;
  requires_engine_id?: boolean;
  requires_base_url?: boolean;
  supports_proxy?: boolean;
  description?: string;
  docs_url?: string;
  config_fields: WebSearchConfigField[];
}

export function parseWebSearchTypes(rows: readonly unknown[]): WebSearchTypeInfo[] {
  return rows
    .filter((row): row is Record<string, unknown> => row !== null && typeof row === 'object' && !Array.isArray(row))
    .map((row): WebSearchTypeInfo | null => {
      const id = typeof row.id === 'string' ? row.id : '';
      if (!id) return null;
      return {
        id,
        name: typeof row.name === 'string' ? row.name : id,
        requires_api_key: row.requires_api_key === true,
        ...(row.supports_optional_api_key === true ? { supports_optional_api_key: true } : {}),
        ...(row.requires_engine_id === true ? { requires_engine_id: true } : {}),
        ...(row.requires_base_url === true ? { requires_base_url: true } : {}),
        ...(row.supports_proxy === true ? { supports_proxy: true } : {}),
        ...(typeof row.description === 'string' ? { description: row.description } : {}),
        ...(typeof row.docs_url === 'string' ? { docs_url: row.docs_url } : {}),
        config_fields: Array.isArray(row.config_fields)
          ? row.config_fields
              .filter((field): field is Record<string, unknown> => field !== null && typeof field === 'object' && !Array.isArray(field))
              .map((field) => ({
                key: typeof field.key === 'string' ? field.key : '',
                label: typeof field.label === 'string' ? field.label : '',
                ...(typeof field.label_key === 'string' ? { label_key: field.label_key } : {}),
                type: 'select' as const,
                ...(field.required === true ? { required: true } : {}),
                ...(typeof field.default === 'string' ? { default: field.default } : {}),
                ...(typeof field.description === 'string' ? { description: field.description } : {}),
                ...(typeof field.description_key === 'string' ? { description_key: field.description_key } : {}),
                options: Array.isArray(field.options)
                  ? field.options
                      .filter((option): option is Record<string, unknown> => option !== null && typeof option === 'object' && !Array.isArray(option))
                      .filter((option) => typeof option.value === 'string')
                      .map((option) => ({
                        label: typeof option.label === 'string' ? option.label : String(option.value),
                        ...(typeof option.label_key === 'string' ? { label_key: option.label_key } : {}),
                        value: option.value as string,
                      }))
                  : [],
              }))
              .filter((field) => field.key)
          : [],
      };
    })
    .filter((entry): entry is WebSearchTypeInfo => entry !== null);
}

/** Vue WebSearchSettings.paramsOut — fixed key order, empty extra_config dropped. */
export function webSearchParamsOut(input: {
  apiKey: string;
  engineId: string;
  baseUrl: string;
  proxyUrl: string;
  extraConfig: Record<string, string>;
}, options: { includeApiKey: boolean }): Record<string, unknown> {
  const out: Record<string, unknown> = {
    engine_id: input.engineId,
    base_url: input.baseUrl,
    proxy_url: input.proxyUrl,
  };
  const extraConfig = Object.fromEntries(
    Object.entries(input.extraConfig || {}).filter(([, value]) => value !== ''),
  );
  if (Object.keys(extraConfig).length > 0) out.extra_config = extraConfig;
  if (options.includeApiKey && input.apiKey) out.api_key = input.apiKey;
  return out;
}

export function webSearchCreatePayload(input: {
  name: string;
  provider: string;
  providerDisplayName: string;
  description: string;
  apiKey: string;
  engineId: string;
  baseUrl: string;
  proxyUrl: string;
  extraConfig: Record<string, string>;
  isDefault: boolean;
}): { name: string; provider: string; description: string; parameters: Record<string, unknown>; is_default: boolean } {
  if (!input.provider.trim()) throw new Error('Provider type is required');
  // Vue falls back to the provider display name when the name is blank.
  const name = input.name.trim() || input.providerDisplayName || input.provider;
  return {
    name,
    provider: input.provider,
    description: input.description,
    parameters: webSearchParamsOut(input, { includeApiKey: true }),
    is_default: input.isDefault,
  };
}

/** Update never carries api_key — fresh credentials go through /credentials. */
export function webSearchUpdatePayload(input: {
  name: string;
  provider: string;
  providerDisplayName: string;
  description: string;
  apiKey: string;
  engineId: string;
  baseUrl: string;
  proxyUrl: string;
  extraConfig: Record<string, string>;
  isDefault: boolean;
}): { name: string; provider: string; description: string; parameters: Record<string, unknown>; is_default: boolean } {
  const payload = webSearchCreatePayload(input);
  return { ...payload, parameters: webSearchParamsOut(input, { includeApiKey: false }) };
}

/** Vue canTestConnection (create mode): required capability fields filled. */
export function webSearchCanTest(type: WebSearchTypeInfo | null, input: {
  apiKey: string;
  engineId: string;
  baseUrl: string;
  extraConfig: Record<string, string>;
}): boolean {
  if (!type) return false;
  if (type.requires_api_key && !input.apiKey) return false;
  if (type.requires_engine_id && !input.engineId) return false;
  if (type.requires_base_url && !input.baseUrl) return false;
  if (type.config_fields.some((field) => field.required && !input.extraConfig?.[field.key])) return false;
  return true;
}

/** Vue configFieldText — i18n label_key with the raw label as fallback. */
export function webSearchConfigText(t: (key: string) => string, key: string | undefined, fallback: string): string {
  if (!key) return fallback;
  const translated = t(key);
  return translated === key ? fallback : translated;
}
