import test from 'node:test';
import assert from 'node:assert/strict';
import { deliveryIntegrationConfig, deliveryRecoveryEvidenceOf, runDeliveryIntegration } from './delivery-integration-smoke.ts';
import { DeliveryRecoveryError } from '@weknora/mobile-core';

test('config is skipped without env and rejected on private hosts', () => {
  assert.deepEqual(deliveryIntegrationConfig({}), { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' });
  const rejected = deliveryIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://127.0.0.1:8080',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
  });
  assert.equal(rejected.enabled, false);
  assert.equal(rejected.disposition, 'invalid');
});

test('config accepts a public https origin and evidence helpers are exported', async () => {
  const smoke = await import('./delivery-integration-smoke.ts');
  const config = smoke.deliveryIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.com',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
  });
  assert.equal(config.enabled, true);
  assert.equal(typeof smoke.emitDeliveryIntegrationEvidence, 'function');
  assert.equal(typeof smoke.runDeliveryIntegration, 'function');
});

test('the recover flag is opt-in and defaults to off', () => {
  const base = {
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.com',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c',
    WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
  };
  const off = deliveryIntegrationConfig(base);
  assert.equal(off.enabled === true && off.recover === false, true);
  const on = deliveryIntegrationConfig({ ...base, WEKNORA_MOBILE_TEST_DELIVERY_RECOVER: '1' });
  assert.equal(on.enabled === true && on.recover === true, true);
  assert.equal(deliveryIntegrationConfig({}).enabled, false);
});


test('recovery results are recorded without hiding conflicts or failures', () => {
  assert.deepEqual(deliveryRecoveryEvidenceOf({ state: 'delivered' }), { recovery: 'recovered', recoveryState: 'delivered' });
  assert.deepEqual(deliveryRecoveryEvidenceOf(undefined, new DeliveryRecoveryError('DELIVERY_STATE_CONFLICT', 'state moved')), { recovery: 'not-needed' });
  assert.deepEqual(deliveryRecoveryEvidenceOf(undefined, new DeliveryRecoveryError('DELIVERY_INVALID_INPUT', 'missing record')), { recovery: 'not-needed' });
  assert.deepEqual(deliveryRecoveryEvidenceOf(undefined, new Error('deployment unavailable')), { recovery: 'failed', failure: 'recovery: deployment unavailable' });
});

const runId = 'run-exact-42';
const deliveryId = 'delivery-exact-99';
const deliveryWire = (state: string) => ({
  id: deliveryId, task_id: 'task-1', run_id: runId, state,
  repo: 'octocat/hello', baseline_sha: 'b0000000000000000000000000000000000000000',
  branch: 'weknora/task/task-1', commit_sha: 'c1', pr_number: 0, pr_url: '',
  remote_login: 'octocat', action_id: 'action-1', action_state: 'dispatched',
  digest: 'd1', approver: 'u1', failure: '', files: 1,
  created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:30Z',
});

function installDeliveryFetch(options: {
  tasks?: Array<{ run_id: string; session_id: string; status: string; created_at: string; updated_at: string }>;
  deliveryReads?: Array<'read' | 'absent'>;
  deliveryStates?: string[];
  initialState?: string;
  actionStatus?: number;
  actionBody?: unknown;
}) {
  const calls: Array<{ method: string; url: string }> = [];
  const previousFetch = globalThis.fetch;
  let deliveryReadIndex = 0;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    const method = (init?.method ?? 'GET').toUpperCase();
    calls.push({ method, url });
    const response = (status: number, body: unknown) => ({
      status,
      headers: { get: (name: string) => name.toLowerCase() === 'content-type' ? 'application/json' : null },
      json: async () => body,
      text: async () => JSON.stringify(body),
    }) as Response;
    const path = new URL(url).pathname;
    if (path === '/api/v1/auth/login') return response(200, { success: true, data: { token: 'access', refresh_token: 'refresh', user: { id: 'user-1' }, tenant: { id: 7 } } });
    if (path === '/api/v1/auth/me') return response(200, { success: true, data: { user: { id: 'user-1' }, tenant: { id: 7 } } });
    if (path === '/api/v1/system/capabilities') return response(200, { code: 0, data: { protocol_minimum: 3, protocol_maximum: 3 } });
    if (path === '/api/v1/workbench/executions') return response(200, { success: true, data: { items: options.tasks ?? [{ run_id: runId, session_id: 'task-1', status: 'succeeded', created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:30Z' }] } });
    if (path === `/api/v1/workbench/executions/${runId}/delivery`) {
      const readIndex = deliveryReadIndex++;
      const read = options.deliveryReads?.[readIndex] ?? 'read';
      if (read === 'absent') return response(404, { code: 'code_delivery_not_found' });
      const state = options.deliveryStates?.[readIndex] ?? options.initialState ?? 'pushed';
      return response(200, { success: true, data: { delivery: deliveryWire(state) } });
    }
    if (path.includes('/delivery/') && (path.endsWith('/dispatch') || path.endsWith('/resolve'))) {
      if (options.actionStatus !== undefined && options.actionStatus >= 400) {
        return response(options.actionStatus, options.actionBody ?? { code: 'code_delivery_state_conflict', message: 'state changed' });
      }
      return response(200, { success: true, data: { delivery: deliveryWire('delivered') } });
    }
    throw new Error(`unexpected fetch ${method} ${url}`);
  }) as typeof fetch;
  return { calls, restore: () => { globalThis.fetch = previousFetch; } };
}

const integrationConfig = (recover: boolean) => ({ enabled: true as const, deploymentOrigin: 'https://weknora.example.com', email: 'a@b.c', password: 'pw', recover });

const actionRequests = (calls: Array<{ method: string; url: string }>) => calls
  .map(({ method, url }) => ({ method, path: new URL(url).pathname }))
  .filter(({ path }) => /\/delivery\/[^/]+\/(dispatch|resolve)$/.test(path));

const expectedAction = (action: 'dispatch' | 'resolve') => [{
  method: 'POST',
  path: `/api/v1/workbench/executions/${runId}/delivery/${deliveryId}/${action}`,
}];

test('runDeliveryIntegration keeps recovery opt-in and skips recovery when no delivery was read', async () => {
  for (const scenario of [
    { recover: false, deliveryReads: ['read' as const] },
    { recover: true, deliveryReads: ['absent' as const] },
    { recover: true, tasks: [] },
  ]) {
    const fake = installDeliveryFetch(scenario);
    try {
      const evidence = await runDeliveryIntegration(integrationConfig(scenario.recover));
      assert.equal(evidence.recovery, 'skipped');
      assert.deepEqual(actionRequests(fake.calls), []);
      assert.equal(fake.calls.filter(({ url }) => new URL(url).pathname.endsWith(`/executions/${runId}/delivery`)).length, scenario.tasks ? 0 : 1);
    } finally { fake.restore(); }
  }
});

test('runDeliveryIntegration uses the exact run and delivery IDs and dispatches pushed state', async () => {
  const fake = installDeliveryFetch({});
  try {
    const evidence = await runDeliveryIntegration(integrationConfig(true));
    assert.equal(evidence.deliveryRead, 'read', `evidence=${JSON.stringify(evidence)} calls=${JSON.stringify(fake.calls)}`);
    assert.equal(evidence.recovery, 'recovered');
    assert.equal(evidence.recoveryState, 'delivered');
    assert.deepEqual(actionRequests(fake.calls), expectedAction('dispatch'));
  } finally { fake.restore(); }
});

test('runDeliveryIntegration reports an already-delivered record as not-needed without recovery calls', async () => {
  const fake = installDeliveryFetch({ initialState: 'delivered' });
  try {
    const evidence = await runDeliveryIntegration(integrationConfig(true));
    assert.equal(evidence.deliveryRead, 'read');
    assert.equal(evidence.deliveryState, 'delivered');
    assert.equal(evidence.recovery, 'not-needed');
    assert.equal(evidence.recoveryState, undefined);
    assert.equal(evidence.failure, undefined);
    assert.deepEqual(actionRequests(fake.calls), []);
  } finally { fake.restore(); }
});

test('runDeliveryIntegration reports a recovery read that converges to delivered as not-needed', async () => {
  const fake = installDeliveryFetch({ deliveryStates: ['pushed', 'delivered'], deliveryReads: ['read', 'read'] });
  try {
    const evidence = await runDeliveryIntegration(integrationConfig(true));
    assert.equal(evidence.deliveryRead, 'read');
    assert.equal(evidence.deliveryState, 'pushed');
    assert.equal(evidence.recovery, 'not-needed');
    assert.equal(evidence.recoveryState, undefined);
    assert.equal(evidence.failure, undefined);
    assert.equal(fake.calls.filter(({ url }) => new URL(url).pathname.endsWith(`/executions/${runId}/delivery`)).length, 2);
    assert.deepEqual(actionRequests(fake.calls), []);
  } finally { fake.restore(); }
});

test('runDeliveryIntegration resolves unknown state and records state conflicts as not-needed', async () => {
  const unknown = installDeliveryFetch({ initialState: 'unknown' });
  try {
    const evidence = await runDeliveryIntegration(integrationConfig(true));
    assert.equal(evidence.recovery, 'recovered');
    assert.deepEqual(actionRequests(unknown.calls), expectedAction('resolve'));
  } finally { unknown.restore(); }

  const conflict = installDeliveryFetch({ actionStatus: 409, actionBody: { code: 'code_delivery_state_conflict', message: 'state changed' } });
  try {
    const evidence = await runDeliveryIntegration(integrationConfig(true));
    assert.equal(evidence.recovery, 'not-needed');
    assert.equal(evidence.failure, undefined);
    assert.deepEqual(actionRequests(conflict.calls), expectedAction('dispatch'));
  } finally { conflict.restore(); }
});

test('runDeliveryIntegration classifies recovery input errors and appends backend failure detail', async () => {
  const invalid = installDeliveryFetch({ deliveryReads: ['read', 'absent'] });
  try {
    const evidence = await runDeliveryIntegration(integrationConfig(true));
    assert.equal(evidence.recovery, 'not-needed');
    assert.equal(evidence.failure, undefined);
    assert.deepEqual(actionRequests(invalid.calls), []);
  } finally { invalid.restore(); }

  const failed = installDeliveryFetch({ actionStatus: 503, actionBody: { code: 'service_unavailable', message: 'delivery service unavailable' } });
  try {
    const evidence = await runDeliveryIntegration(integrationConfig(true));
    assert.equal(evidence.recovery, 'failed');
    assert.match(evidence.failure ?? '', /recovery: delivery service unavailable/);
    assert.deepEqual(actionRequests(failed.calls), expectedAction('dispatch'));
  } finally { failed.restore(); }
});
