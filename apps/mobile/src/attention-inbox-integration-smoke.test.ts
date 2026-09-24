import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { attentionInboxIntegrationConfig, emitAttentionInboxIntegrationEvidence, runAttentionInboxIntegration } from './attention-inbox-integration-smoke.ts';

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

// 修复轮 1（review fix）：证据契约如实证伪——决定已越过门控、有 pending 行、决定调用抛错时，
// 证据必须记 decide:'failed'（已尝试但失败），不得误记 'skipped'（语义=未尝试）。
// 通过 mock 全局 fetch 驱动真实 signIn → inbox → decide 链路（与 runAttentionInboxIntegration
// 内部 fetcher 引用全局 fetch 的接缝对齐），不依赖 WEKNORA_MOBILE_TEST_* 环境。
test('a failed decide attempt is recorded as failed, not skipped (evidence contract)', async () => {
  const jsonResponse = (status: number, body: unknown) => ({
    status,
    headers: { get: (name: string) => (name.toLowerCase() === 'content-type' ? 'application/json' : null) },
    json: async () => body,
    text: async () => JSON.stringify(body),
  });
  const seen: string[] = [];
  const previousFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    seen.push(`${init?.method ?? 'GET'} ${url}`);
    if (url.endsWith('/api/v1/auth/login')) {
      return jsonResponse(200, { success: true, data: { token: 't-1', refresh_token: 'r-1', user: { id: 'user-1' }, tenant: { id: 7 } } }) as unknown as Response;
    }
    if (url.endsWith('/api/v1/auth/me')) {
      return jsonResponse(200, { success: true, data: { user: { id: 'user-1' }, tenant: { id: 7 } } }) as unknown as Response;
    }
    if (url.endsWith('/api/v1/system/capabilities')) {
      return jsonResponse(200, { code: 0, data: { protocol_minimum: 3, protocol_maximum: 3 } }) as unknown as Response;
    }
    if (url.includes('/api/v1/workbench/interactions?')) {
      return jsonResponse(200, { success: true, data: [{ id: 'i-1', decision_id: '', kind: 'tool_approval', action: '', args_hash: 'sha256:aa', expected_revision: 4, run_id: 'run-1', created_at: '2026-09-24T00:00:00Z' }] }) as unknown as Response;
    }
    if (url.includes('/api/v1/workbench/executions/interactions/i-1/decisions')) {
      // 决定端点 500：remote.decide 无契约码映射 → mobile-core 折叠 TASK_OFFICE_BACKEND 抛出。
      return jsonResponse(500, { success: false, message: 'boom' }) as unknown as Response;
    }
    throw new Error(`unexpected fetch ${init?.method ?? 'GET'} ${url}`);
  }) as typeof fetch;
  try {
    const evidence = await runAttentionInboxIntegration({
      enabled: true, deploymentOrigin: 'https://weknora.example.org', email: 'user@example.test', password: 'pw', decideEnabled: true,
    });
    // 前两个断言证明 mock 链路真实走通（signIn 授权 + 收件箱读到 1 行 pending + 决定调用已发出）：
    // 若链路未走通，inbox 将为 'browse-failed'，此处即失败并显示实际值。
    assert.equal(evidence.inbox, 'browsed');
    assert.equal(evidence.pendingCount, 1);
    assert.ok(seen.some((entry) => entry.includes('/decisions')), '决定调用必须已发出（到达 catch 的前提是尝试过决定）');
    assert.equal(evidence.decide, 'failed', '决定已尝试且抛错：证据必须记 failed（已尝试但失败），不得误记 skipped（未尝试）');
    assert.ok(evidence.failure !== undefined && evidence.failure.length > 0, '失败摘要必须携带 error message');
  } finally {
    globalThis.fetch = previousFetch;
  }
});
