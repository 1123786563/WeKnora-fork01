import test from 'node:test';
import assert from 'node:assert/strict';
import { taskOfficeIntegrationConfig, emitTaskOfficeIntegrationEvidence, probeUnauthenticatedRead, runArchiveRoundtrip, type TaskOfficeIntegrationEvidence } from './task-office-integration-smoke.ts';
import { createTaskOffice, TaskOfficeError, type TaskOffice } from '@weknora/mobile-core';

test('Task Office live integration remains opt-in and requires all credentials', () => {
  assert.deepEqual(taskOfficeIntegrationConfig({}), {
    enabled: false,
    disposition: 'skip',
    reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD',
  });
  const enabled = taskOfficeIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.org/',
    WEKNORA_MOBILE_TEST_EMAIL: ' user@example.test ',
    WEKNORA_MOBILE_TEST_PASSWORD: 'secret',
  });
  assert.deepEqual(enabled, {
    enabled: true,
    deploymentOrigin: 'https://weknora.example.org',
    email: 'user@example.test',
    password: 'secret',
  });
});

test('Task Office evidence reports unauthenticated rejection and never serializes credentials', () => {
  const evidence: TaskOfficeIntegrationEvidence = {
    deploymentOrigin: 'https://weknora.example.org',
    unauthenticatedRead: 'rejected',
    home: 'failed',
    sections: 'unavailable',
    listSearch: 'failed',
    archiveRoundtrip: 'unavailable',
    archiveRestore: 'not-attempted',
    commandTimestamp: '2026-09-24T00:00:00.000Z',
  };
  let record = '';
  emitTaskOfficeIntegrationEvidence(evidence, (value) => { record = value; });
  assert.deepEqual(JSON.parse(record), evidence);
  assert.doesNotMatch(record, /password|secret|access_token/i);
});

test('pre-login probe accepts only typed scope rejection and detects fail-open', async () => {
  let backendCalls = 0;
  const unauthenticated = createTaskOffice({
    lease: () => undefined,
    backend: {
      async overview() { backendCalls += 1; throw new Error('unexpected transport'); },
      async list() { backendCalls += 1; return { items: [] }; },
      async archive() { backendCalls += 1; }, async restore() { backendCalls += 1; },
    },
  });
  assert.equal(await probeUnauthenticatedRead(unauthenticated), 'rejected');
  const broken: TaskOffice = { ...unauthenticated, async tasks() { backendCalls += 1; throw new Error('network down'); } };
  assert.equal(await probeUnauthenticatedRead(broken), 'failed-open');
  const resolved: TaskOffice = { ...unauthenticated, async tasks() { backendCalls += 1; return { items: [], duplicateRunIds: [] }; } };
  assert.equal(await probeUnauthenticatedRead(resolved), 'failed-open');
  assert.equal(backendCalls, 2, 'the real no-lease Office made no backend call; only deliberately broken adapters did');
});

test('archive smoke restores in finally when archived listing fails, and exposes restore failure', async () => {
  const events: string[] = [];
  const office = {
    async archive() { events.push('archive'); },
    async tasks(query: { archived?: boolean }) {
      events.push(query.archived ? 'archived-list' : 'active-list');
      if (query.archived) throw new Error('list unavailable');
      return { items: [{ taskId: 'task-1' }], duplicateRunIds: [] };
    },
    async restore() { events.push('restore'); },
  } as unknown as TaskOffice;
  const outcome = await runArchiveRoundtrip(office, 'task-1', () => true);
  assert.deepEqual(events, ['archive', 'archived-list', 'restore', 'active-list']);
  assert.equal(outcome.archiveRoundtrip, 'failed');
  assert.equal(outcome.archiveRestore, 'restored');

  const failedRestore = { ...office, async restore() { throw new Error('restore denied'); } } as TaskOffice;
  assert.deepEqual(await runArchiveRoundtrip(failedRestore, 'task-1', () => true), {
    archiveRoundtrip: 'failed', archiveRestore: 'failed',
  });
});

test('ambiguous archive after scope change reports cleanup required without restoring into a new scope', async () => {
  let restores = 0;
  const office = {
    async archive() { throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED'); },
    async tasks() { return { items: [], duplicateRunIds: [] }; },
    async restore() { restores += 1; },
  } as unknown as TaskOffice;
  assert.deepEqual(await runArchiveRoundtrip(office, 'task-1', () => false), {
    archiveRoundtrip: 'failed', archiveRestore: 'cleanup-required',
  });
  assert.equal(restores, 0, 'a changed tenant never receives a compensating restore request');
});
