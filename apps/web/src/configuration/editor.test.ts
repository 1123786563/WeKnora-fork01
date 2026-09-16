import assert from 'node:assert/strict';
import test from 'node:test';

import { configurationDraftFromRecord, credentialInput, credentialStatusAfterClear, newConfigurationDraft, savedConfigurationId } from './editor.ts';
import { configurationPayload } from './surface.ts';

test('creates an editable draft from redacted records without restoring secret placeholders', () => {
  const draft = configurationDraftFromRecord('models', {
    id: 'model-1', name: 'Model', description: 'Remote', type: 'KnowledgeQA', source: 'custom',
    parameters: { base_url: 'https://model.test', api_key: 'redacted-value' },
    credentials: { api_key: { configured: true } },
  });
  assert.equal(draft.id, 'model-1');
  assert.equal(draft.details, '{"base_url":"https://model.test"}');
  assert.equal(draft.apiKey, '');
});

test('preserves an existing agent avatar through draft hydration and payload construction', async () => {
  const draft = configurationDraftFromRecord('agents', {
    id: 'agent-1', name: 'Research', avatar: 'https://cdn.test/avatar.png', config: { system_prompt: 'Help.' },
  });
  assert.equal(draft.avatar, 'https://cdn.test/avatar.png');
  assert.equal(draft.isBuiltin, false);
  assert.deepEqual(configurationPayload('agents', draft), {
    name: 'Research', description: '', avatar: 'https://cdn.test/avatar.png', config: { system_prompt: 'Help.', memory_enabled: false },
  });
});

test('removes camelCase and nested secret keys from editable configuration details', () => {
  const draft = configurationDraftFromRecord('mcp', {
    id: 'mcp-1', name: 'MCP', auth_config: { apiKey: 'redacted', nested: { clientSecret: 'redacted', label: 'safe' } },
  });

  assert.equal(draft.details, '{"nested":{"label":"safe"}}');
});

test('defaults legacy MCP records without transport_type to SSE', () => {
  const draft = configurationDraftFromRecord('mcp', { id: 'mcp-1', name: 'Legacy MCP', auth_config: {} });
  assert.equal(draft.transportType, 'sse');
});

test('only sends newly entered non-empty credentials to the dedicated subresource', () => {
  assert.deepEqual(credentialInput('models', { apiKey: ' new-key ', appSecret: '' }), { apiKey: 'new-key' });
  assert.deepEqual(credentialInput('mcp', { token: 'token-1', apiKey: '   ' }), { token: 'token-1' });
  assert.deepEqual(credentialInput('agents', { apiKey: 'unexpected' }), {});
});

test('uses the server id before writing credentials for a newly created resource', () => {
  assert.equal(savedConfigurationId(undefined, 'created-model'), 'created-model');
  assert.equal(savedConfigurationId('existing-model', 'ignored'), 'existing-model');
  assert.throws(() => savedConfigurationId(undefined, ''), /server id/);
});

test('preserves MCP transport type and defaults new MCP drafts to SSE', () => {
  const draft = configurationDraftFromRecord('mcp', {
    id: 'mcp-1', name: 'MCP', transport_type: 'stdio', auth_config: { auth_type: 'none' },
  });
  assert.equal(draft.transportType, 'stdio');
  assert.equal(newConfigurationDraft('mcp').transportType, 'sse');
});

test('marks a cleared credential as unconfigured for the local editor state', () => {
  assert.deepEqual(credentialStatusAfterClear({ api_key: { configured: true } }, 'api_key'), {
    api_key: { configured: false },
  });
});

test('agent drafts expose typed system prompt and memory fields instead of raw JSON', () => {
  const draft = configurationDraftFromRecord('agents', {
    id: 'agent-1', name: 'Helper', config: { system_prompt: 'You help.', memory_enabled: true, agent_mode: 'quick-answer' },
  });
  assert.equal(draft.systemPrompt, 'You help.');
  assert.equal(draft.memoryEnabled, true);
  const parsed: Record<string, unknown> = JSON.parse(draft.details);
  assert.deepEqual(parsed, { agent_mode: 'quick-answer' }, 'typed fields are lifted out of the safe JSON');
  assert.equal(newConfigurationDraft('agents').systemPrompt, '');
  assert.equal(newConfigurationDraft('agents').memoryEnabled, false);
  assert.equal(newConfigurationDraft('models').systemPrompt, undefined, 'only agents use typed prompt fields');
});

test('agent payloads merge typed fields back into config', () => {
  const payload = configurationPayload('agents', {
    name: 'Helper',
    systemPrompt: 'Be terse.',
    memoryEnabled: true,
    details: '{"agent_mode":"smart-reasoning"}',
  } as never);
  assert.deepEqual(payload.config, { agent_mode: 'smart-reasoning', system_prompt: 'Be terse.', memory_enabled: true });
});

test('agent payloads default missing typed fields', () => {
  const payload = configurationPayload('agents', { name: 'Bare', details: '{}', isBuiltin: true } as never);
  assert.deepEqual(payload.config, { system_prompt: '', memory_enabled: false });
});
