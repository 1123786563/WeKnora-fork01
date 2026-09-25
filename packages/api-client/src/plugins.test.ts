import test from 'node:test';
import assert from 'node:assert/strict';

import {
  createPluginsApi,
  parsePluginInstallation,
  parsePluginInstallations,
  parsePluginPreview,
  parsePluginUpgradePreview,
} from './plugins.ts';
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

// ---- T15: upgrade-preview envelope (dto.PluginUpgradePreviewResponse, Issue #114) ----

function upgradePreviewEnvelope(): unknown {
  return {
    success: true,
    data: {
      diff: {
        plugin_id: 'com.example.jira-todo',
        current_version: '1.2.0',
        candidate_version: '1.1.0',
        is_downgrade: true,
        endpoint_changed: true,
        current_endpoint: 'https://plugins.example.com/jira-todo/v1.2.0/mcp',
        candidate_endpoint: 'https://plugins.example.com/jira-todo/v1.1.0/mcp',
        added_tools: [
          { name: 'added_tool', description: '新增工具', input_schema_digest: 'd'.repeat(64), read_only: true, requires_personal_auth: false, scopes: null },
        ],
        removed_tools: [
          { name: 'removed_tool', description: '移除工具', input_schema_digest: 'e'.repeat(64), read_only: false, requires_personal_auth: true, scopes: ['write:jira-work'] },
        ],
        changed_tools: [
          {
            name: 'changed_tool',
            schema_changed: true,
            scope_changed: true,
            read_write_class_changed: true,
            personal_auth_changed: true,
            current: { name: 'changed_tool', description: '现版', input_schema_digest: 'a'.repeat(64), read_only: true, requires_personal_auth: false, scopes: ['read:jira-work'] },
            candidate: { name: 'changed_tool', description: '候选版', input_schema_digest: 'b'.repeat(64), read_only: false, requires_personal_auth: true, scopes: ['read:jira-work', 'write:jira-work'] },
          },
        ],
      },
      candidate_fingerprint: 'f'.repeat(64),
      candidate_tools_digest: '0'.repeat(64),
    },
  };
}

test('parsePluginUpgradePreview maps the five diff dimensions and tolerates null scopes', () => {
  const value = parsePluginUpgradePreview(upgradePreviewEnvelope());
  assert.equal(value.diff.pluginId, 'com.example.jira-todo');
  assert.equal(value.diff.currentVersion, '1.2.0');
  assert.equal(value.diff.candidateVersion, '1.1.0');
  assert.equal(value.diff.isDowngrade, true, 'the downgrade flag maps (降级标注的解析前提)');
  assert.equal(value.diff.endpointChanged, true);
  assert.equal(value.diff.currentEndpoint, 'https://plugins.example.com/jira-todo/v1.2.0/mcp');
  assert.equal(value.diff.candidateEndpoint, 'https://plugins.example.com/jira-todo/v1.1.0/mcp');
  assert.equal(value.diff.addedTools.length, 1);
  assert.deepEqual(value.diff.addedTools[0], {
    name: 'added_tool',
    description: '新增工具',
    inputSchemaDigest: 'd'.repeat(64),
    readOnly: true,
    requiresPersonalAuth: false,
    scopes: [], // Go nil slice serializes as null — collapsed like every other scope list
  });
  assert.equal(value.diff.removedTools.length, 1);
  assert.deepEqual(value.diff.removedTools[0], {
    name: 'removed_tool',
    description: '移除工具',
    inputSchemaDigest: 'e'.repeat(64),
    readOnly: false,
    requiresPersonalAuth: true,
    scopes: ['write:jira-work'],
  });
  assert.equal(value.diff.changedTools.length, 1);
  const change = value.diff.changedTools[0]!;
  assert.equal(change.name, 'changed_tool');
  assert.equal(change.schemaChanged, true, 'schema 变更徽标维度');
  assert.equal(change.scopeChanged, true, 'scope 变更徽标维度');
  assert.equal(change.readWriteClassChanged, true, '读写分类变更徽标维度');
  assert.equal(change.personalAuthChanged, true, '授权面变更徽标维度');
  assert.equal(change.current.readOnly, true);
  assert.equal(change.candidate.readOnly, false);
  assert.equal(change.candidate.scopes.length, 2);
  assert.equal(value.candidateFingerprint, 'f'.repeat(64));
  assert.equal(value.candidateToolsDigest, '0'.repeat(64));
});

test('parsePluginUpgradePreview rejects non-success envelopes and malformed fields', () => {
  assert.throws(() => parsePluginUpgradePreview({ success: false }), /success/);
  assert.throws(() => parsePluginUpgradePreview(null), /upgrade-preview/);
  assert.throws(() => parsePluginUpgradePreview({ success: true }), /data/);

  const dropDiff = (field: string): unknown => {
    const envelope = upgradePreviewEnvelope() as { data: { diff: Record<string, unknown> } };
    delete envelope.data.diff[field];
    return envelope;
  };
  for (const field of [
    'plugin_id', 'current_version', 'candidate_version', 'is_downgrade',
    'endpoint_changed', 'current_endpoint', 'candidate_endpoint',
    'added_tools', 'removed_tools', 'changed_tools',
  ]) {
    assert.throws(() => parsePluginUpgradePreview(dropDiff(field)), new RegExp(field), `missing diff.${field} must be rejected`);
  }
  const dropTop = (field: string): unknown => {
    const envelope = upgradePreviewEnvelope() as { data: Record<string, unknown> };
    delete envelope.data[field];
    return envelope;
  };
  for (const field of ['diff', 'candidate_fingerprint', 'candidate_tools_digest']) {
    assert.throws(() => parsePluginUpgradePreview(dropTop(field)), new RegExp(field), `missing data.${field} must be rejected`);
  }

  const mutateDiff = (field: string, bad: unknown): unknown => {
    const envelope = upgradePreviewEnvelope() as { data: { diff: Record<string, unknown> } };
    envelope.data.diff[field] = bad;
    return envelope;
  };
  assert.throws(() => parsePluginUpgradePreview(mutateDiff('is_downgrade', 'yes')), /is_downgrade/);
  assert.throws(() => parsePluginUpgradePreview(mutateDiff('added_tools', {})), /added_tools/, 'a non-array added_tools is rejected');
  assert.throws(() => parsePluginUpgradePreview(mutateDiff('added_tools', [{ name: 't' }])), /added_tools/);
  assert.throws(
    () => parsePluginUpgradePreview(mutateDiff('added_tools', [{ name: 't', description: '' }])),
    /input_schema_digest/,
    'a snapshot row missing its digest is rejected',
  );

  const changedAsArray = upgradePreviewEnvelope() as { data: { diff: { changed_tools: Array<Record<string, unknown>> } } };
  delete changedAsArray.data.diff.changed_tools[0]!.current;
  assert.throws(() => parsePluginUpgradePreview(changedAsArray), /current/, 'a changed row without the current snapshot is rejected');

  const flagMutations: ReadonlyArray<[string, unknown]> = [
    ['schema_changed', 1], ['scope_changed', 'x'], ['read_write_class_changed', null], ['personal_auth_changed', 0],
  ];
  for (const [field, bad] of flagMutations) {
    const envelope = upgradePreviewEnvelope() as { data: { diff: { changed_tools: Array<Record<string, unknown>> } } };
    const row = envelope.data.diff.changed_tools[0]!;
    row[field] = bad;
    assert.throws(() => parsePluginUpgradePreview(envelope), new RegExp(field), `changed_tools[0].${field} must be a boolean`);
  }
});

test('createPluginsApi.previewUpgrade POSTs the encoded id and parses the envelope', async () => {
  const requests: ClientRequest[] = [];
  const api = createPluginsApi(async (input: ClientRequest) => {
    requests.push(input);
    return upgradePreviewEnvelope();
  });
  const result = await api.previewUpgrade('inst/1');
  assert.equal(requests.length, 1);
  assert.equal(requests[0]!.method, 'POST');
  assert.equal(requests[0]!.path, '/api/v1/plugins/installations/inst%2F1/upgrade-preview');
  assert.equal(requests[0]!.body, undefined, 'the preview is a bodyless POST — everything server-side derives from the installation row');
  assert.equal(result.diff.currentVersion, '1.2.0');
  assert.equal(result.diff.changedTools.length, 1);
});

test('createPluginsApi.previewUpgrade rejects an empty installation id before any request', async () => {
  let calls = 0;
  const api = createPluginsApi(async () => {
    calls += 1;
    return upgradePreviewEnvelope();
  });
  await assert.rejects(() => api.previewUpgrade('  '), /installation/);
  assert.equal(calls, 0, 'no request leaves the client for an invalid input');
});

