import assert from 'node:assert/strict';
import { test } from 'node:test';

// tsx 以 CJS 输出 .ts，动态导入保持与其他集成 smoke 测试同构（先例：voice-dictation-integration-smoke.test.ts）。
const loadMod = () => import('./ios-core-workflow-integration-smoke.ts');

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip and a private/loopback host is invalid, never a pass', async () => {
  const { iosCoreWorkflowIntegrationConfig } = await loadMod();
  const missing = iosCoreWorkflowIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(missing.enabled, false);
  assert.equal(missing.disposition, 'skip');
  for (const host of ['http://insecure.example', 'https://localhost', 'https://127.0.0.1', 'https://10.0.0.5', 'https://192.168.1.9']) {
    const invalid = iosCoreWorkflowIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host,
      WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: ['short-lived', 'secret'].join('-'),
    });
    assert.equal(invalid.enabled, false, `${host} 不得成为被授权目标`);
    assert.equal(invalid.disposition, 'invalid');
  }
});

test('the runner is total: an unreachable deployment still yields evidence, not a rejection', async () => {
  const { runIosCoreWorkflowIntegration } = await loadMod();
  const evidence = await runIosCoreWorkflowIntegration({
    enabled: true,
    deploymentOrigin: 'https://weknora.invalid.test',
    email: 'nobody@example.test',
    password: 'wrong',
  });
  assert.equal(evidence.signIn, 'failed');
  assert.equal(evidence.coldBootRestore, 'not-attempted', '登录失败不得伪造后续步骤已执行');
  assert.equal(evidence.revocation, 'not-attempted');
  assert.equal(typeof evidence.errorReason, 'string');
  assert.doesNotMatch(JSON.stringify(evidence), /short-lived-secret|password|token|email/i, '证据不含凭据');
});

test('live composition: cold-boot restore, no revival after sign-out, weak-network reconcile on one run (opt-in)', async (t) => {
  const { emitIosCoreWorkflowIntegrationEvidence, iosCoreWorkflowIntegrationConfig, runIosCoreWorkflowIntegration } = await loadMod();
  const config = iosCoreWorkflowIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runIosCoreWorkflowIntegration(config);
  const emitted: string[] = [];
  emitIosCoreWorkflowIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).signIn, evidence.signIn);
  if (evidence.signIn === 'authorized') {
    assert.equal(evidence.coldBootRestore, 'authorized-restored', '冷启动恢复：第二 Runtime 实例从持久凭据恢复授权面');
    assert.equal(evidence.revocation, 'revoked', '撤销不可复活：signOut 后第三实例 boot() 必须停在 deployment-login');
    assert.ok(
      evidence.weakNetwork === 'reconciled-same-run' || evidence.weakNetwork === 'pending-retained' || evidence.weakNetwork === 'failed',
      '弱网结果如实记录（部署无可用 agent 时允许 failed 并带 errorReason，不伪造）',
    );
    if (evidence.weakNetwork === 'reconciled-same-run') {
      assert.equal(evidence.distinctRunIds, 1, '断链重续不得产生第二个 Run（AC1 单写者幂等）');
      assert.ok((evidence.weakNetworkStartRequests ?? 0) >= 1, '弱网注入确实拦断了至少一次 Start 派发（重续靠 lookup admission 收敛，不重发 Start——submission.ts:23）');
    }
  }
});

// 回归覆盖（修复轮 1）：弱网注入接线曾漏传 weakStart/startDispatches（首实例 runtimeOf 调用
// 缺参 → dispatches.count 恒 0 → 活体实跑恒走「injection did not hold」failed 分支）。
// 本用例劫持 globalThis.fetch 用内存路由 fake 服务端（形状对照 api-client/auth/endpoints.ts、
// mobile/executions.ts、chat/sessions.ts 的信封契约实读核实）把整条 harness 真实跑起来：
// 首枚 Start POST 被弱网注入拦断（请求不发、2s 后 reject）→ resume 的 lookup 对账落
// awaiting_reconciliation → reconcilePending 经 lookup admitted 收敛同一 runId（AC1 单写者）。
test('weak-network wiring: a dropped first Start dispatch reconciles to the same run via lookup (dead-wiring regression)', async () => {
  const { runIosCoreWorkflowIntegration } = await loadMod();
  const originalFetch = globalThis.fetch;
  let startPosts = 0;
  const jsonResponse = (body: unknown) => ({
    status: 200,
    headers: { get: (name: string) => (name.toLowerCase() === 'content-type' ? 'application/json' : null) },
    json: async () => body,
    text: async () => JSON.stringify(body),
  });
  // fake 服务端只会收到放行请求：首枚 Start POST 被注入层拦断（请求根本不发出）。
  globalThis.fetch = (async (input: string | URL | Request, init?: { method?: string }) => {
    const url = new URL(String(input));
    const method = (init?.method ?? 'GET').toUpperCase();
    const path = url.pathname;
    if (path === '/api/v1/auth/login') {
      return jsonResponse({
        success: true,
        data: {
          token: 't39-fake-access',
          refresh_token: 't39-fake-refresh',
          user: { id: 'user-1' },
          tenant: { id: 1, name: 'Primary' },
          memberships: [{ tenant_id: 1, tenant_name: 'Primary' }],
        },
      });
    }
    if (path === '/api/v1/auth/me') {
      return jsonResponse({
        success: true,
        data: {
          user: { id: 'user-1' },
          tenant: { id: 1, name: 'Primary' },
          memberships: [{ tenant_id: 1, tenant_name: 'Primary' }],
          tenant_required: false,
        },
      });
    }
    // deploymentCapabilities 的信封是 code/msg/data（runtime.ts:113-118 实读），非 success 信封。
    if (path === '/api/v1/system/capabilities') {
      return jsonResponse({ code: 0, msg: 'ok', data: { protocol_minimum: 1, protocol_maximum: 3 } });
    }
    if (path === '/api/v1/agents') return jsonResponse({ success: true, data: [{ id: 'agent-1', name: 't39 agent' }] });
    // parseChatSession 必需 id/title/is_pinned（contracts/src/index.ts:421-435 实读）。
    if (path === '/api/v1/sessions') return jsonResponse({ success: true, data: { id: 'session-1', title: 't39 fake session', is_pinned: false } });
    if (path === '/api/v1/workbench/executions' && method === 'POST') {
      startPosts += 1; // 防御观测：注入命中时首枚不应到达这里
      return jsonResponse({ success: true, data: { run_id: 'run-t39-1', request_id: 'echo', status: 'running' } });
    }
    // GET 同路径是任务列表（executions.ts:273 list）；parseExecutionItem 必需
    // run_id/session_id/status/created_at/updated_at（executions.ts:127-147 实读）。
    if (path === '/api/v1/workbench/executions') {
      return jsonResponse({
        success: true,
        data: {
          items: [{ run_id: 'run-t39-1', session_id: 'session-1', status: 'running', created_at: '2026-09-26T00:00:00.000Z', updated_at: '2026-09-26T00:00:00.000Z' }],
        },
      });
    }
    if (path.startsWith('/api/v1/workbench/executions/requests/')) {
      // 服务端已受理语义：lookup admitted + 同一 runId —— reconcile 收敛的权威来源。
      return jsonResponse({ success: true, data: { state: 'admitted', run_id: 'run-t39-1' } });
    }
    return jsonResponse({ success: false, message: `unexpected path ${path}` });
  }) as typeof fetch;
  try {
    const evidence = await runIosCoreWorkflowIntegration({
      enabled: true,
      deploymentOrigin: 'https://t39-fake.test',
      email: 'nobody@example.test',
      password: ['t39', 'unreachable', 'stub'].join('-'),
    });
    assert.equal(evidence.signIn, 'authorized', `fake 服务端四端点形状必须全部对齐（errorReason=${evidence.errorReason ?? 'none'}）`);
    assert.equal(evidence.coldBootRestore, 'authorized-restored', '冷启动恢复：第二实例从落盘凭据恢复授权面');
    assert.equal(evidence.revocation, 'revoked', '撤销不可复活：第三实例 boot() 停在 deployment-login');
    assert.equal(
      evidence.weakNetwork,
      'reconciled-same-run',
      `弱网接线必须真实生效：首枚 Start 派发被拦断后经 lookup 收敛同 run（errorReason=${evidence.errorReason ?? 'none'}）`,
    );
    assert.equal(evidence.distinctRunIds, 1, '单写者幂等：收敛到同一 runId（AC1）');
    assert.equal(evidence.weakNetworkStartRequests, 1, '恰好一次 Start 派发被注入拦断；重续经 lookup 对账，不重发 Start（submission.ts:23）');
    assert.equal(evidence.runVisibleInTasks, true, '收敛的 Run 在任务列表可见');
    assert.equal(startPosts, 0, '首枚 Start POST 不应到达服务端（注入在请求发出前拦断）');
    assert.doesNotMatch(JSON.stringify(evidence), /t39-fake-access|t39-fake-refresh|password|token|email/i, '证据不含凭据');
  } finally {
    globalThis.fetch = originalFetch;
  }
});
