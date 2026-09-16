import assert from 'node:assert/strict';
import test from 'node:test';

import { configurationPayload, configurationSections, configurationStatus, parseConfigurationObject } from './surface.ts';

test('keeps the four T15 configuration surfaces explicit and capability-labelled', () => {
  assert.deepEqual(configurationSections.map((section) => section.key), ['agents', 'models', 'mcp', 'skills']);
  assert.equal(configurationSections.find((section) => section.key === 'agents')?.writeSupport, 'supported');
  assert.equal(configurationSections.find((section) => section.key === 'models')?.writeSupport, 'supported');
  assert.equal(configurationSections.find((section) => section.key === 'mcp')?.writeSupport, 'supported');
  assert.equal(configurationSections.find((section) => section.key === 'skills')?.writeSupport, 'read-only');
  assert.equal(configurationStatus({ source: 'env', enabled: false }), 'environment-managed');
  assert.equal(configurationStatus({ enabled: true }), 'enabled');
});

test('builds configuration payloads without putting secrets into the main resource update', () => {
  assert.deepEqual(configurationPayload('models', {
    id: '', name: 'Model', description: 'remote', type: 'KnowledgeQA', source: 'custom',
    details: '{"base_url":"https://model.example"}', transportType: 'sse', url: '', enabled: true,
    apiKey: 'secret', appSecret: '',
  }), {
    name: 'Model', display_name: 'Model', description: 'remote', type: 'KnowledgeQA', source: 'custom',
    parameters: { base_url: 'https://model.example' },
  });
});

test('rejects secret-shaped keys in arbitrary main-resource details', () => {
  assert.throws(() => configurationPayload('models', {
    name: 'Model', details: '{"provider":{"apiKey":"secret"}}',
  }), /secret|credential/i);
  assert.throws(() => configurationPayload('mcp', {
    name: 'MCP', transportType: 'sse', details: '{"nested":{"clientSecret":"secret"}}',
  }), /secret|credential/i);
});

test('includes the selected MCP transport type while preserving safe auth structure', () => {
  assert.deepEqual(configurationPayload('mcp', {
    name: 'MCP', transportType: 'http-streamable', url: 'https://mcp.test', details: JSON.stringify({
      auth_type: 'api_key', api_key_header: 'X-API-Key', scopes: ['tools:read'],
    }),
  }), {
    name: 'MCP', url: 'https://mcp.test', enabled: true, transport_type: 'http-streamable',
    auth_config: { auth_type: 'api_key', api_key_header: 'X-API-Key', scopes: ['tools:read'] },
  });
  assert.throws(() => configurationPayload('mcp', {
    name: 'MCP', transportType: 'ftp', details: '{}',
  }), /transport/i);
});

test('rejects malformed configuration JSON instead of sending an empty object', () => {
  assert.deepEqual(parseConfigurationObject('{"enabled":true}', 'agent config'), { enabled: true });
  assert.throws(() => parseConfigurationObject('[]', 'agent config'), /must be a JSON object/);
  assert.throws(() => parseConfigurationObject('{', 'agent config'), /must be valid JSON/);
});

test('does not infer health from the mere presence of a configuration row', () => {
  assert.equal(configurationStatus({ id: 'model-1', name: 'Model' }), 'configured');
  assert.equal(configurationStatus({ id: 'mcp-1', name: 'MCP', enabled: false }), 'disabled');
});

test('requires a system prompt when saving a custom agent, matching the Vue editor', () => {
  assert.throws(() => configurationPayload('agents', {
    name: 'Custom agent', details: '{}', systemPrompt: '', isBuiltin: false,
  }), /system prompt/i);
  assert.doesNotThrow(() => configurationPayload('agents', {
    name: 'Built-in agent', details: '{}', systemPrompt: '', isBuiltin: true,
  }));
});
