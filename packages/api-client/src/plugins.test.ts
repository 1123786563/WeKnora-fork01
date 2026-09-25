import test from 'node:test';
import assert from 'node:assert/strict';

import {
  createPluginsApi,
  parsePluginInstallation,
  parsePluginInstallations,
  parsePluginMyConnection,
  parsePluginPreview,
  parsePluginToolPolicyRows,
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

// T15-OCR1-F1 回归：input_schema_digest 是 schema_changed 徽标的判定依据，
// 后端清单校验（manifest.go schemaDigestPattern）与快照核验两处都保证下发
// 非空 64 位 hex——协议上不存在空串合法场景，空串必须拒绝（required 而非
// optionalText，与「拒绝任何缺失/畸形字段」的模块契约一致）。
test('parsePluginUpgradePreview rejects an empty input_schema_digest', () => {
  const envelope = upgradePreviewEnvelope() as { data: { diff: { added_tools: Array<Record<string, unknown>> } } };
  envelope.data.diff.added_tools[0]!.input_schema_digest = '';
  assert.throws(() => parsePluginUpgradePreview(envelope), /input_schema_digest/);
});

// T15-OCR2-low（plugins.ts CONNECTION_PATH）：connection envelope 的诊断前缀
// 必须含 :id 占位段——与 UPGRADE_PREVIEW_PATH 同约定，缺段前缀会误导排障时的
// 路径定位（真实请求是 /installations/{id}/connections/me）。
test('parsePluginMyConnection 诊断前缀含 :id 段（与 UPGRADE_PREVIEW_PATH 同约定）', () => {
  assert.throws(() => parsePluginMyConnection(null), /installations\/:id\/connections\/me/);
});

// ---- T12: member personal connection (dto.PluginMyConnection,
// GET /plugins/installations/:id/connections/me, handler GetMyConnection) ----

function myConnectionEnvelope(): { success: boolean; data: Record<string, unknown> } {
  return {
    success: true,
    data: {
      installation_id: 'inst-1',
      plugin_id: 'com.example.jira-todo',
      name: 'Jira 本周待办',
      service_id: 'svc-1',
      requires_personal_auth: true,
      authorized: false,
      state: 'unauthorized',
      authorize_url_path: '/api/v1/mcp-services/svc-1/oauth/authorize-url',
      revoke_path: '/api/v1/mcp-services/svc-1/oauth/token',
      requires_auth_tools: ['search_my_week_issues', 'create_todo'],
    },
  };
}

test('parsePluginMyConnection maps the three-state verdict, service binding and legacy OAuth paths', () => {
  const value = parsePluginMyConnection(myConnectionEnvelope());
  assert.deepEqual(value, {
    installationId: 'inst-1',
    pluginId: 'com.example.jira-todo',
    name: 'Jira 本周待办',
    serviceId: 'svc-1',
    requiresPersonalAuth: true,
    authorized: false,
    state: 'unauthorized',
    authorizeUrlPath: '/api/v1/mcp-services/svc-1/oauth/authorize-url',
    revokePath: '/api/v1/mcp-services/svc-1/oauth/token',
    requiresAuthTools: ['search_my_week_issues', 'create_todo'],
  });
});

test('parsePluginMyConnection keeps empty endpoint paths for no-auth plugins and collapses null tool lists', () => {
  // T11 Ruling: requires_personal_auth=false leaves authorize_url_path/revoke_path
  // empty — there is no OAuth to point at; Go nil slices serialize as null.
  const envelope = myConnectionEnvelope();
  envelope.data.requires_personal_auth = false;
  envelope.data.authorized = true;
  envelope.data.state = 'authorized';
  envelope.data.authorize_url_path = '';
  envelope.data.revoke_path = '';
  envelope.data.requires_auth_tools = null;
  const value = parsePluginMyConnection(envelope);
  assert.equal(value.requiresPersonalAuth, false);
  assert.equal(value.state, 'authorized');
  assert.equal(value.authorized, true);
  assert.equal(value.authorizeUrlPath, '');
  assert.equal(value.revokePath, '');
  assert.deepEqual(value.requiresAuthTools, []);
});

test('parsePluginMyConnection rejects non-success envelopes, foreign states and malformed fields', () => {
  assert.throws(() => parsePluginMyConnection({ success: false }), /success/);
  assert.throws(() => parsePluginMyConnection(null), /connections\/me/);
  // The mcp oauth STATUS vocabulary (refreshable/reauth_required/pending) is a
  // different endpoint's state machine — it must never pass as a connection state.
  const foreignState = myConnectionEnvelope();
  foreignState.data.state = 'refreshable';
  assert.throws(() => parsePluginMyConnection(foreignState), /state/);
  for (const field of ['installation_id', 'plugin_id', 'name', 'service_id', 'requires_personal_auth', 'authorized', 'state', 'authorize_url_path', 'revoke_path']) {
    const drop = myConnectionEnvelope();
    delete drop.data[field];
    assert.throws(() => parsePluginMyConnection(drop), new RegExp(field), `missing ${field} must be rejected`);
  }
  const badTools = myConnectionEnvelope();
  badTools.data.requires_auth_tools = 'search_my_week_issues';
  assert.throws(() => parsePluginMyConnection(badTools), /requires_auth_tools/);
  const badAuthorized = myConnectionEnvelope();
  badAuthorized.data.authorized = 'yes';
  assert.throws(() => parsePluginMyConnection(badAuthorized), /authorized/);
});

test('parsePluginMyConnection tolerates an empty service_id from the confirm-interruption window', () => {
  // T12-OCR1-F3 regression: server healing (plugin_install_service.go
  // serviceIDByInstallation) resolves an orphan materialized service when one
  // exists, but a confirm interrupted BEFORE CreateMCPService leaves no
  // orphan to find — the endpoint still answers 200 with service_id "" and
  // empty endpoint paths. That is a legal envelope: the row must degrade to
  // a badge + disabled entries, never die as a parse error.
  const envelope = myConnectionEnvelope();
  envelope.data.service_id = '';
  envelope.data.authorize_url_path = '';
  envelope.data.revoke_path = '';
  const value = parsePluginMyConnection(envelope);
  assert.equal(value.serviceId, '');
  assert.equal(value.state, 'unauthorized');
  assert.equal(value.authorizeUrlPath, '');
  assert.equal(value.revokePath, '');
});

test('createPluginsApi.getMyConnection GETs connections/me with the encoded id and rejects empty ids', async () => {
  const requests: ClientRequest[] = [];
  const api = createPluginsApi(async (input: ClientRequest) => {
    requests.push(input);
    return myConnectionEnvelope();
  });
  const value = await api.getMyConnection('inst/1');
  assert.equal(requests.length, 1);
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/plugins/installations/inst%2F1/connections/me');
  assert.equal(value.state, 'unauthorized');
  await assert.rejects(() => api.getMyConnection('  '), /installation/);
});

// ---- T19: tool governance surface (GET .../tools, PUT .../tools/:tool_name/policy) ----
// 治理面（dto.PluginInstallationTool，internal/handler/dto/plugin.go）是
// require_approval 的权威确定值视图——enabled / require_approval /
// disabled_reason 全为确定值（详情面省略 require_approval 键是 T18-OCR1-F2
// 的既定契约，治理面必须带键）。

function toolPolicyEnvelope(): unknown {
  return {
    success: true,
    data: [
      {
        name: 'r',
        description: 'read tool',
        read_only: true,
        requires_personal_auth: false,
        scopes: [],
        enabled: true,
        require_approval: false,
        disabled_reason: '',
      },
      {
        name: 'w',
        description: 'write tool',
        read_only: false,
        requires_personal_auth: false,
        scopes: ['write:demo'],
        enabled: false,
        require_approval: true,
        disabled_reason: 'write tool disabled by default; enable explicit',
      },
    ],
  };
}

test('parsePluginToolPolicyRows maps governance rows with definite require_approval', () => {
  const rows = parsePluginToolPolicyRows(toolPolicyEnvelope());
  assert.equal(rows.length, 2);
  assert.deepEqual(
    { ...rows[0] },
    {
      name: 'r',
      description: 'read tool',
      readOnly: true,
      requiresPersonalAuth: false,
      scopes: [],
      enabled: true,
      requireApproval: false,
      disabledReason: '',
    },
  );
  assert.equal(rows[1]!.name, 'w');
  assert.equal(rows[1]!.enabled, false);
  assert.equal(rows[1]!.requireApproval, true, 'the governance row carries the CURRENT approval verdict as a definite value');
  assert.equal(rows[1]!.disabledReason, 'write tool disabled by default; enable explicit');
});

test('parsePluginToolPolicyRows rejects non-success envelopes, non-array data and malformed rows', () => {
  assert.throws(() => parsePluginToolPolicyRows({ success: false }), /success/);
  assert.throws(() => parsePluginToolPolicyRows({ success: true, data: {} }), /array/);
  const badRow = toolPolicyEnvelope() as { data: Array<Record<string, unknown>> };
  delete badRow.data[0]!.name;
  assert.throws(() => parsePluginToolPolicyRows(badRow), /\.name/);
});

test('parsePluginToolPolicyRows rejects a governance row that omits require_approval', () => {
  const envelope = toolPolicyEnvelope() as { data: Array<Record<string, unknown>> };
  delete envelope.data[1]!.require_approval;
  assert.throws(() => parsePluginToolPolicyRows(envelope), /require_approval/,
    'the governance surface asserts a DEFINITE approval verdict — an omitted key is a contract break, not a silent false');
});

test('createPluginsApi.listInstallationTools GETs the governance list and rejects empty ids', async () => {
  const requests: ClientRequest[] = [];
  const api = createPluginsApi(async (input: ClientRequest) => {
    requests.push(input);
    return toolPolicyEnvelope();
  });
  const rows = await api.listInstallationTools('inst/1');
  assert.equal(requests.length, 1);
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/plugins/installations/inst%2F1/tools');
  assert.equal(rows.length, 2);
  assert.equal(rows[1]!.requireApproval, true);
  await assert.rejects(() => api.listInstallationTools(' '), /installation/);
});

test('createPluginsApi.setInstallationToolPolicy PUTs the encoded patch and parses refreshed rows', async () => {
  const requests: ClientRequest[] = [];
  const api = createPluginsApi(async (input: ClientRequest) => {
    requests.push(input);
    return toolPolicyEnvelope();
  });
  const rows = await api.setInstallationToolPolicy('inst/1', 'write tool', { enabled: true, requireApproval: true });
  assert.equal(requests.length, 1);
  assert.equal(requests[0]!.method, 'PUT');
  assert.equal(requests[0]!.path, '/api/v1/plugins/installations/inst%2F1/tools/write%20tool/policy');
  assert.deepEqual(requests[0]!.body, { enabled: true, require_approval: true });
  assert.equal(rows.length, 2);
  await assert.rejects(() => api.setInstallationToolPolicy('', 'w', { enabled: true }), /installation/);
  await assert.rejects(() => api.setInstallationToolPolicy('inst-1', '', { enabled: true }), /tool/);
});

test('createPluginsApi.setInstallationToolPolicy rejects an empty patch before any request', async () => {
  let fired = 0;
  const api = createPluginsApi(async () => {
    fired += 1;
    return toolPolicyEnvelope();
  });
  await assert.rejects(() => api.setInstallationToolPolicy('inst-1', 'w', {}), /enabled or require_approval/,
    'the client-side guard mirrors the server 400 — at least one field per patch');
  assert.equal(fired, 0, 'no request leaves the client for an empty patch');
});

