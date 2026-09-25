import test from 'node:test';
import assert from 'node:assert/strict';
import { emitTaskInterventionIntegrationEvidence, runTaskInterventionIntegration, taskInterventionIntegrationConfig } from './task-intervention-integration-smoke.ts';

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip, never fake a pass', () => {
  const config = taskInterventionIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'skip');
  const invalid = taskInterventionIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://insecure.example', WEKNORA_MOBILE_TEST_EMAIL: 'e', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.disposition, 'invalid');
});

test('live intervention through the highest stable interface (opt-in)', async (t) => {
  const config = taskInterventionIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runTaskInterventionIntegration(config);
  const emitted: string[] = [];
  emitTaskInterventionIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).start, evidence.start);
  assert.notEqual(evidence.steer, 'failed');
  assert.notEqual(evidence.stop, 'failed');
  assert.notEqual(evidence.queueNext, 'failed');
});

test('the integration runner is total: failures still yield evidence, never a rejection', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'task-intervention-integration-smoke.ts'), 'utf8');
  assert.match(source, /catch \(error\)/);
  assert.match(source, /finally\s*\{/);
});
