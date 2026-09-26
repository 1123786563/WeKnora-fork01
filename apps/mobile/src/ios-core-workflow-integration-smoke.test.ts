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
      assert.ok((evidence.weakNetworkStartRequests ?? 0) >= 2, '断链重试确实发生了第二次 Start 派发');
    }
  }
});
