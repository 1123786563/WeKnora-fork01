import test from 'node:test';
import assert from 'node:assert/strict';

import { createPluginsApi, parsePluginInstallation, parsePluginInstallations, parsePluginPreview } from './plugins.ts';
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

// ---- T08: installation envelopes (dto.PluginInstallationResponse / .PluginInstallationSummary) ----

function installationEnvelope(): unknown {
  return {
    success: true,
    data: {
      installation_id: 'inst-1',
      plugin_id: 'com.example.jira-todo',
      name: 'Jira 本周待办',
      description: '个人待办视角',
      version: '1.2.0',
      state: 'active',
      drift_state: 'none',
      transport_type: 'http-streamable',
      endpoint_url: 'https://plugins.example.com/jira-todo/v1.2.0/mcp',
      service_id: 'svc-1',
      tools: [
        { name: 'search_my_week_issues', description: '搜索本周待办', read_only: true, requires_personal_auth: true, scopes: ['read:jira-work'], enabled: true },
        { name: 'create_todo', description: '创建待办', read_only: false, requires_personal_auth: true, scopes: null, enabled: false },
      ],
    },
  };
}

function installationListEnvelope(): unknown {
  return {
    success: true,
    data: [
      { installation_id: 'inst-1', plugin_id: 'com.example.jira-todo', name: 'Jira 本周待办', version: '1.2.0', state: 'active', drift_state: 'none', requires_personal_auth: true, tool_count: 2 },
      { installation_id: 'inst-2', plugin_id: 'com.example.weather', name: '天气查询', version: '0.3.1', state: 'disabled', drift_state: 'detected', requires_personal_auth: false, tool_count: 1 },
    ],
  };
}

test('parsePluginInstallation rejects non-success envelope', () => {
  assert.throws(() => parsePluginInstallation({ success: false }), /success/);
  assert.throws(() => parsePluginInstallation(null), /plugins\/installations/);
});

test('parsePluginInstallation rejects envelopes with missing or malformed fields', () => {
  const drop = (field: string): unknown => {
    const envelope = installationEnvelope() as { data: Record<string, unknown> };
    delete envelope.data[field];
    return envelope;
  };
  for (const field of ['installation_id', 'plugin_id', 'name', 'version', 'state', 'drift_state', 'transport_type', 'endpoint_url', 'service_id', 'tools']) {
    assert.throws(() => parsePluginInstallation(drop(field)), new RegExp(field), `missing ${field} must be rejected`);
  }
  const mutate = (field: string, value: unknown): unknown => {
    const envelope = installationEnvelope() as { data: Record<string, unknown> };
    envelope.data[field] = value;
    return envelope;
  };
  assert.throws(() => parsePluginInstallation(mutate('state', 'paused')), /state/);
  assert.throws(() => parsePluginInstallation(mutate('drift_state', 'weird')), /drift_state/);
  assert.throws(() => parsePluginInstallation(mutate('transport_type', 'stdio')), /transport_type/);
  assert.throws(() => parsePluginInstallation(mutate('tools', {})), /tools/);
});

test('parsePluginInstallation maps fields, collapses null scopes and keeps a missing enabled as null', () => {
  const envelope = installationEnvelope() as { data: { tools: Array<Record<string, unknown>> } };
  // dto.PluginInstallationTool.Enabled is *bool omitempty: a tool row without
  // an explicit policy row serializes WITHOUT the key — that means unknown,
  // which must stay null, never coerce to false.
  delete envelope.data.tools[1]!.enabled;
  const value = parsePluginInstallation(envelope);
  assert.equal(value.installationId, 'inst-1');
  assert.equal(value.pluginId, 'com.example.jira-todo');
  assert.equal(value.name, 'Jira 本周待办');
  assert.equal(value.description, '个人待办视角');
  assert.equal(value.version, '1.2.0');
  assert.equal(value.state, 'active');
  assert.equal(value.driftState, 'none');
  assert.equal(value.transportType, 'http-streamable');
  assert.equal(value.endpointUrl, 'https://plugins.example.com/jira-todo/v1.2.0/mcp');
  assert.equal(value.serviceId, 'svc-1');
  assert.equal(value.tools.length, 2);
  assert.deepEqual(value.tools[0], {
    name: 'search_my_week_issues',
    description: '搜索本周待办',
    readOnly: true,
    requiresPersonalAuth: true,
    scopes: ['read:jira-work'],
    enabled: true,
  });
  assert.deepEqual(value.tools[1], {
    name: 'create_todo',
    description: '创建待办',
    readOnly: false,
    requiresPersonalAuth: true,
    scopes: [],
    enabled: null,
  });
});

test('parsePluginInstallation rejects malformed tool rows', () => {
  const envelope = installationEnvelope() as { data: { tools: Array<Record<string, unknown>> } };
  envelope.data.tools[0]!.enabled = 'yes';
  assert.throws(() => parsePluginInstallation(envelope), /enabled/);
  const noAuth = installationEnvelope() as { data: { tools: Array<Record<string, unknown>> } };
  delete noAuth.data.tools[0]!.requires_personal_auth;
  assert.throws(() => parsePluginInstallation(noAuth), /requires_personal_auth/);
});

test('parsePluginInstallations maps summary rows', () => {
  const rows = parsePluginInstallations(installationListEnvelope());
  assert.equal(rows.length, 2);
  assert.deepEqual(rows[0], {
    installationId: 'inst-1',
    pluginId: 'com.example.jira-todo',
    name: 'Jira 本周待办',
    version: '1.2.0',
    state: 'active',
    driftState: 'none',
    requiresPersonalAuth: true,
    toolCount: 2,
  });
  assert.equal(rows[1]!.state, 'disabled');
  assert.equal(rows[1]!.driftState, 'detected');
});

test('parsePluginInstallations rejects non-success envelopes, non-array data and malformed rows', () => {
  assert.throws(() => parsePluginInstallations({ success: false }), /success/);
  assert.throws(() => parsePluginInstallations({ success: true, data: {} }), /data/);
  const badCount = installationListEnvelope() as { data: Array<Record<string, unknown>> };
  badCount.data[0]!.tool_count = 'two';
  assert.throws(() => parsePluginInstallations(badCount), /tool_count/);
  const negativeCount = installationListEnvelope() as { data: Array<Record<string, unknown>> };
  negativeCount.data[0]!.tool_count = -1;
  assert.throws(() => parsePluginInstallations(negativeCount), /tool_count/);
  const badState = installationListEnvelope() as { data: Array<Record<string, unknown>> };
  badState.data[0]!.state = 'paused';
  assert.throws(() => parsePluginInstallations(badState), /state/);
  const missingAuth = installationListEnvelope() as { data: Array<Record<string, unknown>> };
  delete missingAuth.data[0]!.requires_personal_auth;
  assert.throws(() => parsePluginInstallations(missingAuth), /requires_personal_auth/);
});

test('createPluginsApi.confirmInstallation posts the preview id and parses the installation envelope', async () => {
  const requests: ClientRequest[] = [];
  const api = createPluginsApi(async (input: ClientRequest) => {
    requests.push(input);
    return installationEnvelope();
  });
  const result = await api.confirmInstallation('p-1');
  assert.equal(requests.length, 1);
  assert.equal(requests[0]!.method, 'POST');
  assert.equal(requests[0]!.path, '/api/v1/plugins/installations');
  assert.deepEqual(requests[0]!.body, { preview_id: 'p-1' });
  assert.equal(result.installationId, 'inst-1');
  assert.equal(result.state, 'active');
});

test('createPluginsApi.confirmInstallation rejects an empty preview id before any request', async () => {
  let calls = 0;
  const api = createPluginsApi(async () => {
    calls += 1;
    return installationEnvelope();
  });
  await assert.rejects(() => api.confirmInstallation('  '), /preview/);
  assert.equal(calls, 0);
});

test('createPluginsApi.listInstallations GETs the list path and parses rows', async () => {
  const requests: ClientRequest[] = [];
  const api = createPluginsApi(async (input: ClientRequest) => {
    requests.push(input);
    return installationListEnvelope();
  });
  const rows = await api.listInstallations();
  assert.equal(requests.length, 1);
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/plugins/installations');
  assert.equal(rows.length, 2);
  assert.equal(rows[0]!.name, 'Jira 本周待办');
  assert.equal(rows[1]!.driftState, 'detected');
});

test('createPluginsApi.getInstallation GETs the encoded id and rejects empty ids', async () => {
  const requests: ClientRequest[] = [];
  const api = createPluginsApi(async (input: ClientRequest) => {
    requests.push(input);
    return installationEnvelope();
  });
  const result = await api.getInstallation('inst/1');
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/plugins/installations/inst%2F1');
  assert.equal(result.installationId, 'inst-1');
  await assert.rejects(() => api.getInstallation(''), /installation/);
});

test('createPluginsApi.setInstallationState posts disable/enable and parses the response', async () => {
  const requests: ClientRequest[] = [];
  const api = createPluginsApi(async (input: ClientRequest) => {
    requests.push(input);
    return installationEnvelope();
  });
  const result = await api.setInstallationState('inst-1', 'disabled');
  assert.equal(result.state, 'active'); // stub returns the same envelope; the path is the contract under test
  assert.equal(requests[0]!.method, 'POST');
  assert.equal(requests[0]!.path, '/api/v1/plugins/installations/inst-1/disable');
  await api.setInstallationState('inst-1', 'active');
  assert.equal(requests[1]!.path, '/api/v1/plugins/installations/inst-1/enable');
  await assert.rejects(() => api.setInstallationState('', 'disabled'), /installation/);
});
