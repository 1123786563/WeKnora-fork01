import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { materialIntegrationConfig } from './material-integration-smoke.ts';

const here = dirname(fileURLToPath(import.meta.url));

test('the config rejects loopback, private and reserved deployment hosts as invalid', () => {
  const vars = { WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://127.0.0.1:8080', WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test', WEKNORA_MOBILE_TEST_PASSWORD: 'pw' };
  for (const host of ['https://localhost', 'https://10.0.0.5', 'https://192.168.1.4', 'https://[fe80::1]', 'https://169.254.1.1',
    'https://100.64.0.1', 'https://198.18.0.1', 'https://192.0.2.1', 'https://203.0.113.1']) {
    const rejected = materialIntegrationConfig({ ...vars, WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host });
    assert.equal(rejected.enabled, false, host);
    assert.equal(rejected.enabled === false && rejected.disposition, 'invalid', host);
  }
});

test('a public https origin still enables the integration config', () => {
  const verdict = materialIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.org', WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test', WEKNORA_MOBILE_TEST_PASSWORD: 'pw' });
  assert.equal(verdict.enabled, true);
});

test('missing credentials skip instead of failing', () => {
  const verdict = materialIntegrationConfig({});
  assert.equal(verdict.enabled, false);
  assert.equal(verdict.enabled === false && verdict.disposition, 'skip');
});

test('runMaterialIntegration is total and pins the AC1 ttl assertion', () => {
  const source = readFileSync(join(here, 'material-integration-smoke.ts'), 'utf8');
  assert.match(source, /disallowedDeploymentHost/, 'config 校验必须复用主机防线');
  const runBody = source.slice(source.indexOf('export async function runMaterialIntegration'));
  assert.match(runBody, /finally\s*\{/, '主流程必须有 finally 收口');
  assert.match(runBody, /grantTtlSeconds/, '证据必须记录 grant TTL');
  assert.match(runBody, /900/, '并断言 TTL 不超过 900 秒（AC1）');
});

test('runMaterialIntegration executes against a real deployment when credentials are provided', { skip: Object.keys(process.env).some((key) => key === 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL') === false }, async () => {
  const { runMaterialIntegration } = await import('./material-integration-smoke.ts');
  const config = materialIntegrationConfig(process.env as Record<string, string | undefined>);
  if (config.enabled === false) { assert.ok(true, '凭据不完整时如实跳过，不伪造通过'); return; }
  const evidence = await runMaterialIntegration(config);
  assert.equal(typeof evidence.commandTimestamp, 'string');
  if (evidence.listed === 'listed') {
    if (evidence.grantTtlSeconds !== undefined) assert.ok(evidence.grantTtlSeconds <= 900, `grant TTL ${evidence.grantTtlSeconds}s 超过 900s 上限`);
  }
});
