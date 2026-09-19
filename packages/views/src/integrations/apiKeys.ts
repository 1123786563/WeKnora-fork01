// API-key table helpers ported from the Vue baseline
// frontend/src/views/integrations/ApiIntegrationSettings.vue: stored values are
// masked, full_access/capabilities drive the access-mode label, and a
// freshly created key reveals its value exactly once.

export interface ApiKeyRow {
  id: number | string;
  name: string;
  api_key: string;
  full_access: boolean;
  capabilities?: string[];
  created_at?: string;
}

export function apiKeyValueDisplay(key: ApiKeyRow, reveal: boolean): string {
  if (reveal) return key.api_key;
  const value = key.api_key;
  if (!value) return '(unavailable)';
  if (value.length <= 10) return value;
  return value.slice(0, 5) + '…' + value.slice(-4);
}

export function isFreshKeyVisible(state: { fresh: boolean; hasValue: boolean }): boolean {
  return state.fresh && state.hasValue;
}

export function apiKeyAccessMode(key: ApiKeyRow): string {
  if (key.full_access) return 'Full access';
  const capabilities = (key.capabilities ?? []).filter((item) => item !== '');
  return capabilities.length > 0 ? capabilities.join(', ') : 'Scoped';
}

// --- Capability matrix (port of frontend/src/config/apiKeyCapabilities.ts) ---
// The canonical list order also fixes the payload order: Vue
// selectedCapabilities() filters TENANT_API_KEY_CAPABILITIES directly.

export type TenantApiKeyCapability =
  | 'retrieve' | 'chat' | 'read_agents' | 'ingest' | 'manage_kbs'
  | 'message_history' | 'manage_agents' | 'manage_mcp_services'
  | 'manage_datasources' | 'manage_models' | 'manage_vector_stores'
  | 'manage_storage_backends' | 'manage_web_search' | 'manage_channels'
  | 'run_evaluations' | 'manage_members' | 'manage_spaces'
  | 'manage_tenant_settings';

export interface ApiKeyCapabilityOption {
  value: TenantApiKeyCapability;
  labelKey: string;
  hintKey: string;
}

export interface ApiKeyCapabilityGroup {
  key: string;
  labelKey: string;
  capabilities: readonly ApiKeyCapabilityOption[];
}

export const TENANT_API_KEY_CAPABILITIES: readonly TenantApiKeyCapability[] = [
  'retrieve', 'chat', 'read_agents', 'ingest', 'manage_kbs',
  'message_history', 'manage_agents', 'manage_mcp_services',
  'manage_datasources', 'manage_models', 'manage_vector_stores',
  'manage_storage_backends', 'manage_web_search', 'manage_channels',
  'run_evaluations', 'manage_members', 'manage_spaces',
  'manage_tenant_settings',
];

/** Vue default: scoped keys start with retrieval + chat + agent reads. */
export const DEFAULT_TENANT_API_KEY_CAPABILITIES: ReadonlySet<TenantApiKeyCapability> = new Set([
  'retrieve', 'chat', 'read_agents',
]);

/** Capabilities whose reach is limited by the selected knowledge bases. */
export const KB_SCOPED_API_KEY_CAPABILITIES: ReadonlySet<TenantApiKeyCapability> = new Set([
  'retrieve', 'chat', 'ingest', 'manage_kbs', 'manage_agents', 'manage_datasources',
]);

export const TENANT_API_KEY_CAPABILITY_GROUPS: readonly ApiKeyCapabilityGroup[] = [
  {
    key: 'knowledge',
    labelKey: 'integrations.api.apiKeyCapabilityGroupKnowledge',
    capabilities: [
      { value: 'retrieve', labelKey: 'integrations.api.capabilityRetrieve', hintKey: 'integrations.api.capabilityRetrieveHint' },
      { value: 'chat', labelKey: 'integrations.api.capabilityChat', hintKey: 'integrations.api.capabilityChatHint' },
      { value: 'ingest', labelKey: 'integrations.api.capabilityIngest', hintKey: 'integrations.api.capabilityIngestHint' },
      { value: 'manage_kbs', labelKey: 'integrations.api.capabilityManageKbs', hintKey: 'integrations.api.capabilityManageKbsHint' },
      { value: 'message_history', labelKey: 'integrations.api.capabilityMessageHistory', hintKey: 'integrations.api.capabilityMessageHistoryHint' },
    ],
  },
  {
    key: 'automation',
    labelKey: 'integrations.api.apiKeyCapabilityGroupAutomation',
    capabilities: [
      { value: 'read_agents', labelKey: 'integrations.api.capabilityReadAgents', hintKey: 'integrations.api.capabilityReadAgentsHint' },
      { value: 'manage_agents', labelKey: 'integrations.api.capabilityManageAgents', hintKey: 'integrations.api.capabilityManageAgentsHint' },
      { value: 'manage_mcp_services', labelKey: 'integrations.api.capabilityManageMcpServices', hintKey: 'integrations.api.capabilityManageMcpServicesHint' },
      { value: 'manage_datasources', labelKey: 'integrations.api.capabilityManageDatasources', hintKey: 'integrations.api.capabilityManageDatasourcesHint' },
    ],
  },
  {
    key: 'collaboration',
    labelKey: 'integrations.api.apiKeyCapabilityGroupCollaboration',
    capabilities: [
      { value: 'manage_members', labelKey: 'integrations.api.capabilityManageMembers', hintKey: 'integrations.api.capabilityManageMembersHint' },
      { value: 'manage_spaces', labelKey: 'integrations.api.capabilityManageSpaces', hintKey: 'integrations.api.capabilityManageSpacesHint' },
    ],
  },
  {
    key: 'tenant',
    labelKey: 'integrations.api.apiKeyCapabilityGroupTenant',
    capabilities: [
      { value: 'manage_models', labelKey: 'integrations.api.capabilityManageModels', hintKey: 'integrations.api.capabilityManageModelsHint' },
      { value: 'manage_vector_stores', labelKey: 'integrations.api.capabilityManageVectorStores', hintKey: 'integrations.api.capabilityManageVectorStoresHint' },
      { value: 'manage_storage_backends', labelKey: 'integrations.api.capabilityManageStorageBackends', hintKey: 'integrations.api.capabilityManageStorageBackendsHint' },
      { value: 'manage_web_search', labelKey: 'integrations.api.capabilityManageWebSearch', hintKey: 'integrations.api.capabilityManageWebSearchHint' },
      { value: 'manage_channels', labelKey: 'integrations.api.capabilityManageChannels', hintKey: 'integrations.api.capabilityManageChannelsHint' },
      { value: 'run_evaluations', labelKey: 'integrations.api.capabilityRunEvaluations', hintKey: 'integrations.api.capabilityRunEvaluationsHint' },
      { value: 'manage_tenant_settings', labelKey: 'integrations.api.capabilityManageTenantSettings', hintKey: 'integrations.api.capabilityManageTenantSettingsHint' },
    ],
  },
];

export type ApiKeyCapabilitySelections = Record<TenantApiKeyCapability, boolean>;

/** Vue openCreateAPIKeyDialog resets selections to DEFAULT_TENANT_API_KEY_CAPABILITIES. */
export function createDefaultApiKeySelections(): ApiKeyCapabilitySelections {
  return TENANT_API_KEY_CAPABILITIES.reduce((acc, capability) => {
    acc[capability] = DEFAULT_TENANT_API_KEY_CAPABILITIES.has(capability);
    return acc;
  }, {} as ApiKeyCapabilitySelections);
}

/** Vue selectedCapabilityValues: canonical-list order is preserved. */
export function selectedApiKeyCapabilities(selections: ApiKeyCapabilitySelections): TenantApiKeyCapability[] {
  return TENANT_API_KEY_CAPABILITIES.filter((capability) => selections[capability]);
}

/** Vue apiKeyKnowledgeScopeApplies computed. */
export function apiKeyKnowledgeScopeApplies(fullAccess: boolean, capabilities: readonly TenantApiKeyCapability[]): boolean {
  return !fullAccess && capabilities.some((capability) => KB_SCOPED_API_KEY_CAPABILITIES.has(capability));
}

/** Vue createScopedAPIKey payload (ApiIntegrationSettings.vue L1459-1466). */
export interface ApiKeyCreatePayload {
  name: string;
  full_access: boolean;
  knowledge_base_ids: string[];
  capabilities: TenantApiKeyCapability[];
}

export function buildApiKeyCreatePayload(input: {
  name: string;
  fullAccess: boolean;
  capabilities: readonly TenantApiKeyCapability[];
  knowledgeBaseIds: readonly string[];
}): ApiKeyCreatePayload {
  // Vue selectedCapabilities() filters the canonical list, so the payload
  // order is fixed by TENANT_API_KEY_CAPABILITIES regardless of selection order.
  const selected = new Set(input.capabilities);
  const capabilities = input.fullAccess
    ? []
    : TENANT_API_KEY_CAPABILITIES.filter((capability) => selected.has(capability));
  return {
    name: input.name.trim(),
    full_access: input.fullAccess,
    // KB scoping only applies to capabilities that touch knowledge bases.
    knowledge_base_ids: apiKeyKnowledgeScopeApplies(input.fullAccess, input.capabilities)
      ? [...input.knowledgeBaseIds]
      : [],
    // Capabilities only matter below full access; full access covers them all.
    capabilities,
  };
}
