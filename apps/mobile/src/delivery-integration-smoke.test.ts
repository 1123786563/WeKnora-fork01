import test from 'node:test';
import assert from 'node:assert/strict';
import { DeliveryRecoveryError } from '@weknora/mobile-core';
import { deliveryIntegrationConfig, runDeliveryRecoveryEvidence } from './delivery-integration-smoke.ts';

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

test('recovery skips delivered records and recovers a later pushed or unknown record exactly once', async (t) => {
  for (const state of ['pushed', 'unknown'] as const) {
    await t.test(state, async () => {
      const calls: Array<{ runId: string; deliveryId: string }> = [];
      const result = await runDeliveryRecoveryEvidence({
        runIds: ['run-delivered', `run-${state}`],
        async readDelivery(runId) {
          return runId === 'run-delivered'
            ? { id: 'delivery-done', state: 'delivered' }
            : { id: `delivery-${state}`, state };
        },
        async recover(input) {
          calls.push(input);
          return { state: 'delivered' };
        },
      });

      assert.deepEqual(calls, [{ runId: `run-${state}`, deliveryId: `delivery-${state}` }]);
      assert.equal(result.recovery, 'recovered');
      assert.equal(result.recoveryRunId, `run-${state}`);
      assert.equal(result.recoveryDeliveryId, `delivery-${state}`);
      assert.equal(result.recoveryState, 'delivered');
      assert.deepEqual(result.firstDelivery, { runId: 'run-delivered', delivery: { id: 'delivery-done', state: 'delivered' } });
    });
  }
});

test('all ineligible records are not-needed and never call recovery', async () => {
  let recoveryCalls = 0;
  const result = await runDeliveryRecoveryEvidence({
    runIds: ['run-delivered', 'run-preparing'],
    async readDelivery(runId) {
      return runId === 'run-delivered'
        ? { id: 'delivery-done', state: 'delivered' }
        : { id: 'delivery-preparing', state: 'preparing' };
    },
    async recover() {
      recoveryCalls += 1;
      return { state: 'delivered' };
    },
  });

  assert.equal(recoveryCalls, 0);
  assert.equal(result.recovery, 'not-needed');
  assert.equal(result.recoveryState, 'preparing');
  assert.equal(result.recoveryRunId, undefined);
  assert.equal(result.recoveryDeliveryId, undefined);
});

test('conflict and invalid input are not-needed and scanning continues', async () => {
  const calls: string[] = [];
  const result = await runDeliveryRecoveryEvidence({
    runIds: ['run-conflict', 'run-invalid', 'run-ineligible'],
    async readDelivery(runId) {
      return { id: `delivery-${runId}`, state: runId === 'run-ineligible' ? 'delivered' : 'pushed' };
    },
    async recover({ runId }) {
      calls.push(runId);
      throw new DeliveryRecoveryError(
        runId === 'run-conflict' ? 'DELIVERY_STATE_CONFLICT' : 'DELIVERY_INVALID_INPUT',
        `${runId} no longer needs recovery`,
      );
    },
  });

  assert.deepEqual(calls, ['run-conflict', 'run-invalid']);
  assert.equal(result.recovery, 'not-needed');
  assert.equal(result.recoveryRunId, 'run-invalid');
  assert.equal(result.recoveryDeliveryId, 'delivery-run-invalid');
  assert.equal(result.recoveryState, 'pushed');
});

test('backend and scope failures stop scanning after one attempted write', async (t) => {
  for (const code of ['DELIVERY_BACKEND', 'DELIVERY_SCOPE_CHANGED'] as const) {
    await t.test(code, async () => {
      const reads: string[] = [];
      const calls: Array<{ runId: string; deliveryId: string }> = [];
      const result = await runDeliveryRecoveryEvidence({
        runIds: ['run-ambiguous', 'run-later'],
        async readDelivery(runId) {
          reads.push(runId);
          return { id: `delivery-${runId}`, state: 'unknown' };
        },
        async recover(input) {
          calls.push(input);
          throw new DeliveryRecoveryError(code, `${code} after attempt`);
        },
      });

      assert.deepEqual(reads, ['run-ambiguous']);
      assert.deepEqual(calls, [{ runId: 'run-ambiguous', deliveryId: 'delivery-run-ambiguous' }]);
      assert.equal(result.recovery, 'failed');
      assert.equal(result.recoveryRunId, 'run-ambiguous');
      assert.equal(result.recoveryDeliveryId, 'delivery-run-ambiguous');
      assert.equal(result.recoveryState, 'unknown');
      assert.equal(result.failure, `${code} after attempt`);
    });
  }
});
