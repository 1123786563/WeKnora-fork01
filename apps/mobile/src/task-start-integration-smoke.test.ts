import test from 'node:test';
import assert from 'node:assert/strict';
import { emitTaskStartIntegrationEvidence, runTaskStartIntegration, taskStartIntegrationConfig } from './task-start-integration-smoke.ts';

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip, never fake a pass', () => {
  delete process.env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL;
  const config = taskStartIntegrationConfig(env());
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'skip');
  const invalid = taskStartIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://insecure.example', WEKNORA_MOBILE_TEST_EMAIL: 'e', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.disposition, 'invalid');
});

test('live end-to-end task creation through the highest stable interface (opt-in)', async (t) => {
  const config = taskStartIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runTaskStartIntegration(config);
  const emitted: string[] = [];
  emitTaskStartIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).start, evidence.start);
  assert.equal(evidence.start, 'admitted', 'a live deployment with one agent must admit the goal');
  assert.equal(evidence.repeatSubmitSameRequest, 'no-second-dispatch', 'AC1: the same intent never dispatches twice');
  assert.notEqual(evidence.runVisibleInTasks, false, 'when the list is reachable, the created run must be observable in it');
});
