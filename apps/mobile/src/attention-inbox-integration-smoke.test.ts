import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { attentionInboxIntegrationConfig, emitAttentionInboxIntegrationEvidence } from './attention-inbox-integration-smoke.ts';

const here = dirname(fileURLToPath(import.meta.url));

test('the attention inbox integration config stays opt-in and reuses the host defense line', () => {
  assert.equal(attentionInboxIntegrationConfig({}).enabled, false);
  const vars = { WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.org', WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test', WEKNORA_MOBILE_TEST_PASSWORD: 'pw' };
  assert.equal(attentionInboxIntegrationConfig(vars).enabled, true);
  assert.equal(attentionInboxIntegrationConfig(vars).enabled && (attentionInboxIntegrationConfig(vars) as { decideEnabled: boolean }).decideEnabled, false, '决定动作默认关闭');
  const gated = attentionInboxIntegrationConfig({ ...vars, WEKNORA_MOBILE_TEST_DECIDE_INTERACTION: '1' });
  assert.equal(gated.enabled && gated.decideEnabled, true);
  for (const host of ['https://127.0.0.1:8080', 'https://localhost', 'https://10.0.0.5', 'https://192.168.1.4', 'https://169.254.1.1']) {
    const rejected = attentionInboxIntegrationConfig({ ...vars, WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host });
    assert.equal(rejected.enabled === false && rejected.disposition, 'invalid', host);
  }
});

test('emit produces the redacted evidence contract without credential fields', () => {
  const lines: string[] = [];
  emitAttentionInboxIntegrationEvidence({
    deploymentOrigin: 'https://weknora.example.org', inbox: 'browsed', pendingCount: 1,
    decide: 'skipped', commandTimestamp: '2026-09-24T00:00:00Z',
  }, (record) => lines.push(record));
  const evidence = JSON.parse(lines[0]);
  assert.equal(evidence.inbox, 'browsed');
  assert.equal(evidence.decide, 'skipped');
  assert.equal('password' in evidence || 'email' in evidence, false);
});

test('the smoke reuses the shared host defense and is total over failures', () => {
  const source = readFileSync(join(here, 'attention-inbox-integration-smoke.ts'), 'utf8');
  assert.match(source, /disallowedDeploymentHost/, 'config 校验必须复用主机防线');
  const runBody = source.slice(source.indexOf('export async function runAttentionInboxIntegration'));
  assert.match(runBody, /finally\s*\{/, '主流程必须有 finally 收口');
  assert.match(runBody, /dispose\(\)/, 'finally 内必须释放 runtime');
  assert.match(runBody, /browse-failed/, '读失败必须如实记录');
});

test('runs the live attention inbox loop when the environment is present', { skip: attentionInboxIntegrationConfig(process.env).enabled === false ? 'missing WEKNORA_MOBILE_TEST_* credentials' : false }, async () => {
  const { runAttentionInboxIntegration } = await import('./attention-inbox-integration-smoke.ts');
  const config = attentionInboxIntegrationConfig(process.env);
  assert.equal(config.enabled, true);
  const evidence = await runAttentionInboxIntegration(config);
  assert.ok(['browsed', 'browse-failed'].includes(evidence.inbox));
  assert.ok(['skipped', 'no-pending', 'recorded', 'delivery-unknown', 'superseded', 'gone', 'failed'].includes(evidence.decide));
});
