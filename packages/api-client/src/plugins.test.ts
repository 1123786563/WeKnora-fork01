import test from 'node:test';
import assert from 'node:assert/strict';

import { createPluginsApi, parsePluginPreview } from './plugins.ts';
import type { ClientRequest } from './client.ts';

function validEnvelope(): unknown {
  return {
    success: true,
    data: {
      preview_id: 'p1',
      plugin_id: 'com.example.jira-todo',
      version: '1.2.0',
      name: 'Jira 本周待办',
      description: '个人待办视角',
      transport_type: 'http-streamable',
      endpoint_url: 'https://plugins.example.com/jira-todo/v1.2.0/mcp',
      tools: [
        {
          name: 'search_my_week_issues',
          description: '搜索我本周的待办事项',
          read_only: true,
          requires_personal_auth: true,
          scopes: ['read:jira-work'],
        },
      ],
      identity_fingerprint: 'f'.repeat(64),
      expires_at: '2026-09-23T00:00:00Z',
    },
  };
}

test('parsePluginPreview rejects non-success envelope', () => {
  assert.throws(() => parsePluginPreview({ success: false }), /success/);
  assert.throws(() => parsePluginPreview({ data: {} }), /success/);
  assert.throws(() => parsePluginPreview(null), /plugins\/installations\/preview/);
});

test('parsePluginPreview rejects envelopes with missing or malformed fields', () => {
  const drop = (field: string): unknown => {
    const envelope = validEnvelope() as { data: Record<string, unknown> };
    delete envelope.data[field];
    return envelope;
  };
  for (const field of ['preview_id', 'plugin_id', 'version', 'name', 'transport_type', 'endpoint_url', 'tools', 'identity_fingerprint', 'expires_at']) {
    assert.throws(() => parsePluginPreview(drop(field)), new RegExp(field.replace(/_/g, '_')), `missing ${field} must be rejected`);
  }
  assert.throws(() => parsePluginPreview({ success: true }), /data/);

  const mutate = (field: string, value: unknown): unknown => {
    const envelope = validEnvelope() as { data: Record<string, unknown> };
    envelope.data[field] = value;
    return envelope;
  };
  assert.throws(() => parsePluginPreview(mutate('preview_id', '')), /preview_id/);
  assert.throws(() => parsePluginPreview(mutate('transport_type', 'stdio')), /transport_type/);
  assert.throws(() => parsePluginPreview(mutate('tools', {})), /tools/);
  assert.throws(() => parsePluginPreview(mutate('tools', [{ name: 't', description: '', read_only: 'yes', requires_personal_auth: false }])), /read_only/);
  assert.throws(() => parsePluginPreview(mutate('expires_at', 123)), /expires_at/);
});

test('parsePluginPreview maps preview fields and tolerates null scopes', () => {
  const envelope = validEnvelope() as { data: { tools: Array<Record<string, unknown>> } };
  // Go nil slices serialize as JSON null; a verified plugin may declare no scopes.
  envelope.data.tools[0]!.scopes = null;
  const value = parsePluginPreview(envelope);
  assert.equal(value.previewId, 'p1');
  assert.equal(value.pluginId, 'com.example.jira-todo');
  assert.equal(value.version, '1.2.0');
  assert.equal(value.name, 'Jira 本周待办');
  assert.equal(value.description, '个人待办视角');
  assert.equal(value.transportType, 'http-streamable');
  assert.equal(value.endpointUrl, 'https://plugins.example.com/jira-todo/v1.2.0/mcp');
  assert.equal(value.identityFingerprint, 'f'.repeat(64));
  assert.equal(value.expiresAt, '2026-09-23T00:00:00Z');
  assert.equal(value.tools.length, 1);
  assert.deepEqual(value.tools[0], {
    name: 'search_my_week_issues',
    description: '搜索我本周的待办事项',
    readOnly: true,
    requiresPersonalAuth: true,
    scopes: [],
  });
});

test('createPluginsApi.previewInstallation posts the manifest URL and parses the envelope', async () => {
  const requests: ClientRequest[] = [];
  const api = createPluginsApi(async (input: ClientRequest) => {
    requests.push(input);
    return validEnvelope();
  });
  const result = await api.previewInstallation('https://plugins.example.com/manifest.json');
  assert.equal(requests.length, 1);
  assert.equal(requests[0]!.method, 'POST');
  assert.equal(requests[0]!.path, '/api/v1/plugins/installations/preview');
  assert.deepEqual(requests[0]!.body, { manifest_url: 'https://plugins.example.com/manifest.json' });
  assert.equal(result.previewId, 'p1');
});

test('createPluginsApi.previewInstallation rejects an empty manifest URL before any request', async () => {
  let calls = 0;
  const api = createPluginsApi(async () => {
    calls += 1;
    return validEnvelope();
  });
  await assert.rejects(() => api.previewInstallation('   '), /manifest/);
  assert.equal(calls, 0, 'no request leaves the client for an invalid input');
});

test('createPluginsApi.previewInstallation surfaces parser errors for illegal envelopes', async () => {
  const api = createPluginsApi(async () => ({ success: false }));
  await assert.rejects(() => api.previewInstallation('https://plugins.example.com/manifest.json'), /success/);
});
