import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { taskDetailIntegrationConfig } from './task-detail-integration-smoke.ts';

const here = dirname(fileURLToPath(import.meta.url));

test('the config rejects loopback, private and reserved deployment hosts as invalid', () => {
  const vars = { WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://127.0.0.1:8080', WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test', WEKNORA_MOBILE_TEST_PASSWORD: 'pw' };
  const verdict = taskDetailIntegrationConfig(vars);
  assert.equal(verdict.enabled, false);
  assert.equal(verdict.enabled === false && verdict.disposition, 'invalid');
  for (const host of ['https://localhost', 'https://10.0.0.5', 'https://192.168.1.4', 'https://[fe80::1]', 'https://169.254.1.1',
    'https://100.64.0.1', 'https://198.18.0.1', 'https://192.0.2.1', 'https://203.0.113.1']) {
    const rejected = taskDetailIntegrationConfig({ ...vars, WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host });
    assert.equal(rejected.enabled === false && rejected.disposition, 'invalid', host);
  }
});

test('a public https origin still enables the integration config', () => {
  const verdict = taskDetailIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.org', WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test', WEKNORA_MOBILE_TEST_PASSWORD: 'pw' });
  assert.equal(verdict.enabled, true);
});

test('runTaskDetailIntegration is total: every path disposes the runtime through one finally block', () => {
  const source = readFileSync(join(here, 'task-detail-integration-smoke.ts'), 'utf8');
  assert.match(source, /disallowedDeploymentHost/, 'config 校验必须复用主机防线（B2-F15）');
  const runBody = source.slice(source.indexOf('export async function runTaskDetailIntegration'));
  assert.match(runBody, /finally\s*\{/, '主流程必须有 finally 收口（B2-F14）');
  const finallyBody = runBody.slice(runBody.indexOf('finally'));
  assert.match(finallyBody, /close\(/, 'finally 内必须关闭句柄');
  assert.match(finallyBody, /dispose\(\)/, 'finally 内必须释放 runtime');
  assert.match(runBody, /failure/, '异常路径必须留证据（evidence.failure）');
});
