import test from 'node:test';
import assert from 'node:assert/strict';
import { knowledgeQAIntegrationConfig } from './knowledge-qa-integration-smoke.ts';

test('integration config stays opt-in and validates the credential-free HTTPS origin', () => {
  assert.deepEqual(knowledgeQAIntegrationConfig({}), {
    enabled: false,
    disposition: 'skip',
    reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD',
  });
  assert.equal(knowledgeQAIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://cloud.example.test', WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c', WEKNORA_MOBILE_TEST_PASSWORD: 'p' }).enabled, true);
  const invalid = knowledgeQAIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://cloud.example.test', WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.disposition, 'invalid');
  const embedded = knowledgeQAIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://user:pass@cloud.example.test', WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(embedded.enabled, false);
  assert.equal(embedded.disposition, 'invalid');
});

test('runKnowledgeQAIntegration is exported and only runs against an opted-in real deployment', async (t) => {
  const { knowledgeQAIntegrationConfig: configOf, runKnowledgeQAIntegration } = await import('./knowledge-qa-integration-smoke.ts');
  assert.equal(typeof runKnowledgeQAIntegration, 'function');
  const config = configOf(process.env);
  if (!config.enabled) {
    t.skip(config.reason); // 无凭据即 skip——不得伪造通过
    return;
  }
  const evidence = await runKnowledgeQAIntegration(config);
  assert.equal(evidence.asked, 'answered');
  assert.notEqual(evidence.evidenceState, 'absent', '真实部署必须返回 evidence 帧（否则服务端早于 T15）');
  assert.equal(evidence.retrievedAtParsed, true, '每条引用的检索时间必须可解析');
});
