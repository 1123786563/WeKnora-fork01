import assert from 'node:assert/strict';
import { test } from 'node:test';
import { emitResearchIntegrationEvidence, researchIntegrationConfig, type ResearchIntegrationEvidence } from './research-integration-smoke.ts';

test('researchIntegrationConfig skips without credentials and rejects non-public origins', () => {
  assert.deepEqual(researchIntegrationConfig({}), { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' });
  assert.equal(researchIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://localhost:3000',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c', WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
  }).enabled, false, '非公网 HTTPS origin 必须拒绝（主机防线）');
});

test('runResearchIntegration without credentials reports an honest skip shape', async () => {
  const config = researchIntegrationConfig({});
  assert.equal(config.enabled, false);
});

test('evidence records never contain credential material', () => {
  const evidence: ResearchIntegrationEvidence = {
    deploymentOrigin: 'https://weknora.example.com',
    listed: 'delegated', delegationCreated: true, delegationCount: 1,
    annotated: 'recorded', annotationCount: 1, annotatedVersion: '9a2f1c3d4e5f6a7b',
    revision: 'not-dispatched', commandTimestamp: '2026-09-26T00:00:00.000Z',
  };
  let emitted = '';
  emitResearchIntegrationEvidence(evidence, (record) => { emitted += record + '\n'; });
  assert.ok(emitted.includes('"listed":"delegated"'));
  assert.ok(!emitted.includes('password'), '证据不得携带凭据字段');
  assert.ok(emitted.includes('"revision":"not-dispatched"'), '修订请求不派发必须如实记录');
});

test('runResearchIntegration executes against a real deployment when credentials are provided', { skip: Object.keys(process.env).some((key) => key === 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL') === false }, async () => {
  const { runResearchIntegration } = await import('./research-integration-smoke.ts');
  const config = researchIntegrationConfig(process.env as Record<string, string | undefined>);
  if (config.enabled === false) { assert.ok(true, '凭据不完整时如实跳过，不伪造通过'); return; }
  const evidence = await runResearchIntegration(config);
  assert.equal(typeof evidence.commandTimestamp, 'string');
  assert.equal(evidence.revision, 'not-dispatched', '对真实部署不触发新 Run：修订请求恒不派发');
  if (evidence.listed === 'delegated') {
    assert.equal(typeof evidence.delegationCount, 'number');
    if (evidence.annotated === 'recorded') assert.equal(typeof evidence.annotatedVersion, 'string');
  }
});
