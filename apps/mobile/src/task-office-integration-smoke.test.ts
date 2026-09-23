import test from 'node:test';
import assert from 'node:assert/strict';
import { taskOfficeIntegrationConfig, emitTaskOfficeIntegrationEvidence, type TaskOfficeIntegrationEvidence } from './task-office-integration-smoke.ts';

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
    commandTimestamp: '2026-09-24T00:00:00.000Z',
  };
  let record = '';
  emitTaskOfficeIntegrationEvidence(evidence, (value) => { record = value; });
  assert.deepEqual(JSON.parse(record), evidence);
  assert.doesNotMatch(record, /password|secret|access_token/i);
});
