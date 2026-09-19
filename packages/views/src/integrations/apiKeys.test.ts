import assert from 'node:assert/strict';
import test from 'node:test';

import {
  apiKeyAccessMode,
  apiKeyValueDisplay,
  buildApiKeyCreatePayload,
  createDefaultApiKeySelections,
  isFreshKeyVisible,
  apiKeyKnowledgeScopeApplies,
  selectedApiKeyCapabilities,
  KB_SCOPED_API_KEY_CAPABILITIES,
  TENANT_API_KEY_CAPABILITIES,
  TENANT_API_KEY_CAPABILITY_GROUPS,
  type ApiKeyRow,
} from './apiKeys.ts';

// Vue baseline: frontend/src/components/ApiIntegrationSettings.vue —
// the table masks stored key values, labels the access mode from
// full_access/capabilities, and reveals a freshly created value once.

const row = (overrides: Partial<ApiKeyRow>): ApiKeyRow => ({
  id: 1, name: 'CI key', api_key: '', full_access: false,
  ...overrides,
});

test('masks stored key values like the Vue fingerprint column', () => {
  assert.equal(apiKeyValueDisplay(row({ api_key: 'wk-abcdef1234567890' }), false), 'wk-ab…7890');
  assert.equal(apiKeyValueDisplay(row({ api_key: 'short' }), false), 'short');
  assert.equal(apiKeyValueDisplay(row({ api_key: '' }), false), '(unavailable)');
});

test('freshly created keys reveal their value once', () => {
  assert.equal(apiKeyValueDisplay(row({ api_key: 'wk-abcdef1234567890' }), true), 'wk-abcdef1234567890');
  assert.equal(isFreshKeyVisible({ fresh: true, hasValue: true }), true);
  assert.equal(isFreshKeyVisible({ fresh: true, hasValue: false }), false);
  assert.equal(isFreshKeyVisible({ fresh: false, hasValue: true }), false);
});

test('access mode reflects full access or scoped capabilities', () => {
  assert.equal(apiKeyAccessMode(row({ full_access: true })), 'Full access');
  assert.equal(apiKeyAccessMode(row({ full_access: false, capabilities: ['retrieve', 'chat'] })), 'retrieve, chat');
  assert.equal(apiKeyAccessMode(row({ full_access: false })), 'Scoped');
});

// --- Capability matrix (Vue frontend/src/config/apiKeyCapabilities.ts) ---

test('tenant capability list mirrors the Vue 18-capability canonical order', () => {
  assert.deepEqual(TENANT_API_KEY_CAPABILITIES, [
    'retrieve', 'chat', 'read_agents', 'ingest', 'manage_kbs',
    'message_history', 'manage_agents', 'manage_mcp_services',
    'manage_datasources', 'manage_models', 'manage_vector_stores',
    'manage_storage_backends', 'manage_web_search', 'manage_channels',
    'run_evaluations', 'manage_members', 'manage_spaces',
    'manage_tenant_settings',
  ]);
});

test('capability groups mirror the Vue four-group matrix with label and hint keys', () => {
  assert.deepEqual(TENANT_API_KEY_CAPABILITY_GROUPS.map((group) => ({
    key: group.key,
    labelKey: group.labelKey,
    capabilities: group.capabilities.map((capability) => ({
      value: capability.value, labelKey: capability.labelKey, hintKey: capability.hintKey,
    })),
  })), [
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
  ]);
  // Groups cover every capability exactly once.
  const grouped = TENANT_API_KEY_CAPABILITY_GROUPS.flatMap((group) => group.capabilities.map((capability) => capability.value));
  assert.deepEqual([...grouped].sort(), [...TENANT_API_KEY_CAPABILITIES].sort());
});

test('default selections and KB-scoped set mirror the Vue defaults', () => {
  const selections = createDefaultApiKeySelections();
  const selected = selectedApiKeyCapabilities(selections);
  assert.deepEqual(selected, ['retrieve', 'chat', 'read_agents']);
  assert.equal(selected.includes('ingest'), false);
  assert.deepEqual([...KB_SCOPED_API_KEY_CAPABILITIES].sort(), [
    'chat', 'ingest', 'manage_agents', 'manage_datasources', 'manage_kbs', 'retrieve',
  ]);
});

// Vue ApiIntegrationSettings.vue createScopedAPIKey (L1459-1466): the create
// payload carries name/full_access/knowledge_base_ids/capabilities, KB scope
// only applies below full access and only for KB-scoped capabilities, and
// capabilities are dropped entirely for full-access keys.

test('buildApiKeyCreatePayload matches the Vue submit payload semantics', () => {
  assert.deepEqual(buildApiKeyCreatePayload({
    name: '  MCP 只读访问  ',
    fullAccess: false,
    capabilities: ['retrieve', 'chat', 'read_agents'],
    knowledgeBaseIds: [],
  }), {
    name: 'MCP 只读访问',
    full_access: false,
    knowledge_base_ids: [],
    capabilities: ['retrieve', 'chat', 'read_agents'],
  });
  // KB ids pass through only when a KB-scoped capability is selected.
  assert.deepEqual(buildApiKeyCreatePayload({
    name: 'k', fullAccess: false,
    capabilities: ['retrieve'], knowledgeBaseIds: ['kb-1', 'kb-2'],
  }).knowledge_base_ids, ['kb-1', 'kb-2']);
  // Without a KB-scoped capability the scope is cleared like Vue.
  assert.deepEqual(buildApiKeyCreatePayload({
    name: 'k', fullAccess: false,
    capabilities: ['manage_models'], knowledgeBaseIds: ['kb-1'],
  }), { name: 'k', full_access: false, knowledge_base_ids: [], capabilities: ['manage_models'] });
  // Full access: capabilities and KB scope are emptied.
  assert.deepEqual(buildApiKeyCreatePayload({
    name: 'k', fullAccess: true,
    capabilities: ['retrieve', 'manage_members'], knowledgeBaseIds: ['kb-1'],
  }), { name: 'k', full_access: true, knowledge_base_ids: [], capabilities: [] });
  // Selection order follows the Vue canonical list (read_agents before ingest).
  assert.deepEqual(buildApiKeyCreatePayload({
    name: 'k', fullAccess: false,
    capabilities: ['ingest', 'read_agents'], knowledgeBaseIds: [],
  }).capabilities, ['read_agents', 'ingest']);
});

test('apiKeyKnowledgeScopeApplies mirrors the Vue computed', () => {
  assert.equal(apiKeyKnowledgeScopeApplies(false, ['retrieve']), true);
  assert.equal(apiKeyKnowledgeScopeApplies(false, ['manage_models']), false);
  assert.equal(apiKeyKnowledgeScopeApplies(false, []), false);
  assert.equal(apiKeyKnowledgeScopeApplies(true, ['retrieve']), false);
});
