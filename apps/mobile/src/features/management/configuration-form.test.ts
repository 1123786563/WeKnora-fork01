import assert from 'node:assert/strict';
import test from 'node:test';
import type { AgentConfiguration, McpConfiguration, ModelConfiguration } from '@weknora/api-client';
import { configurationPayload, credentialInput, nativeConfigurationDraftFrom, type NativeConfigurationDraft } from './configuration-form.ts';

const agentDraft: NativeConfigurationDraft = { section: 'agents', name: 'Helper', description: 'Useful', details: '{"temperature":0.2}', avatar: '', type: '', source: '', url: '', transportType: 'sse', enabled: true, apiKey: '', appSecret: '', token: '' };

test('native configuration form builds safe agent, model, and MCP payloads', () => {
  assert.deepEqual(configurationPayload(agentDraft), { name: 'Helper', description: 'Useful', avatar: '', config: { temperature: 0.2 } });
  assert.deepEqual(configurationPayload({ ...agentDraft, section: 'models', type: 'chat', source: 'custom', name: 'Model' }), { name: 'Model', display_name: 'Model', description: 'Useful', type: 'chat', source: 'custom', parameters: { temperature: 0.2 } });
  assert.deepEqual(configurationPayload({ ...agentDraft, section: 'mcp', name: 'Docs', url: 'https://mcp.example', transportType: 'http-streamable' }), { name: 'Docs', url: 'https://mcp.example', enabled: true, transport_type: 'http-streamable', auth_config: { temperature: 0.2 } });
});

test('native configuration form sends credentials only when explicitly entered', () => {
  assert.deepEqual(credentialInput({ ...agentDraft, section: 'models', apiKey: ' key ', appSecret: '' }), { apiKey: 'key' });
  assert.deepEqual(credentialInput({ ...agentDraft, section: 'mcp', token: ' token ' }), { token: 'token' });
  assert.deepEqual(credentialInput(agentDraft), {});
});

test('native configuration edit never prefills credential values and rejects secret-shaped JSON', () => {
  const draft = nativeConfigurationDraftFrom('models', { id: 'm-1', name: 'Model', type: 'chat', source: 'custom', parameters: { endpoint: 'https://model.example', api_key: 'server-secret' } } as ModelConfiguration);
  assert.equal(draft.apiKey, '');
  assert.match(draft.details, /endpoint/);
  assert.throws(() => configurationPayload({ ...agentDraft, details: '{"nested":{"token":"secret"}}' }), /secret field/);
  assert.throws(() => configurationPayload({ ...agentDraft, details: '[' }), /valid JSON/);
});
