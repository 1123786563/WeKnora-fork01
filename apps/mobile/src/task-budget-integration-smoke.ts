import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileTaskBudgetRemote } from '@weknora/api-client/mobile/task-budget';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type TaskBudgetIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; extendBudget: boolean; extendCredits: number }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface TaskBudgetIntegrationEvidence {
  deploymentOrigin: string;
  budgetFacts: 'read' | 'no-tasks' | 'failed';
  /** 四数分立（Story 58）与算术一致性（remaining === limit-used-held）在真实 wire 上的投影。 */
  fourNumbersDistinct?: boolean;
  remainingConsistent?: boolean;
  pausedListed?: boolean;
  extend?: 'skipped' | 'extended' | 'refused' | 'failed';
  limitRaisedBy?: number;
  replayNeverDoubled?: boolean;
  failure?: string;
  commandTimestamp: string;
}

/** opt-in 语义与 T04/T05/T16 相同：真实 Deployment（HTTPS 公网主机）+ 测试账号；
 * 扩额臂额外需 WEKNORA_MOBILE_TEST_EXTEND_BUDGET=1（真实改账动作必须显式开门）。 */
export function taskBudgetIntegrationConfig(env: Record<string, string | undefined>): TaskBudgetIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try { parsed = new URL(deploymentOrigin); } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  const rawCredits = env.WEKNORA_MOBILE_TEST_EXTEND_CREDITS?.trim() ?? '1';
  const extendCredits = Number(rawCredits);
  if (!Number.isSafeInteger(extendCredits) || extendCredits <= 0) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_EXTEND_CREDITS must be a positive integer' };
  }
  return {
    enabled: true, deploymentOrigin: parsed.origin, email, password,
    extendBudget: env.WEKNORA_MOBILE_TEST_EXTEND_BUDGET === '1',
    extendCredits,
  };
}

/**
 * 真实 JSON transport + 授权通道 + 具体 Remote Adapter + Task Office 编排：读取
 * 第一个任务的预算四数字（含委派/暂停投影）；opt-in 扩额臂执行一次真实追加 +
 * 同键幂等重放断言不加倍。账号无任务时如实记 'no-tasks'；任何步骤异常 →
 * budgetFacts:'failed' + failure 摘要（无凭据字段），从不 reject。
 */
export async function runTaskBudgetIntegration(config: Extract<TaskBudgetIntegrationConfig, { enabled: true }>): Promise<TaskBudgetIntegrationEvidence> {
  const evidence: TaskBudgetIntegrationEvidence = { deploymentOrigin: config.deploymentOrigin, budgetFacts: 'failed', commandTimestamp: new Date().toISOString() };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  try {
    const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;

    const office = createTaskOffice({
      backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
      lease: () => runtime.scopeLease(),
      budget: createMobileTaskBudgetRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
    });
    const page = await office.tasks({});
    if (page.items.length === 0) { evidence.budgetFacts = 'no-tasks'; return evidence; }
    const taskId = page.items[0]!.taskId;

    const facts = await office.budget(taskId);
    evidence.budgetFacts = 'read';
    evidence.fourNumbersDistinct =
      facts.limitCredits !== facts.usedCredits || facts.usedCredits !== facts.heldCredits || facts.heldCredits !== facts.remainingCredits;
    evidence.remainingConsistent = facts.remainingCredits === facts.limitCredits - facts.usedCredits - facts.heldCredits;
    evidence.pausedListed = Array.isArray(facts.pausedRunIds);

    if (!config.extendBudget) { evidence.extend = 'skipped'; return evidence; }
    if (!facts.canExtend) { evidence.extend = 'refused'; return evidence; }

    const before = facts.limitCredits;
    await office.extendBudget({ taskId, additionalCredits: config.extendCredits });
    // 同键幂等重放走 remote 层（office 的键在成功后清除，第二次 office 调用是新键、
    // 会真实加额——不能用作重放断言）：固定 key 两次 POST，断言只计一次。
    const remote = createMobileTaskBudgetRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) });
    const replayKey = `smoke-replay-${Date.now()}`;
    await remote.extend({ taskId, additionalCredits: config.extendCredits, idempotencyKey: replayKey });
    await remote.extend({ taskId, additionalCredits: config.extendCredits, idempotencyKey: replayKey });
    const after = await office.budget(taskId);
    evidence.extend = 'extended';
    evidence.limitRaisedBy = after.limitCredits - before;
    // office 一次（+N）+ remote 同键两次（只计一次 +N）= 恰好 +2N：重放不加倍。
    evidence.replayNeverDoubled = evidence.limitRaisedBy === config.extendCredits * 2;
    return evidence;
  } catch (error) {
    evidence.budgetFacts = 'failed';
    evidence.failure = error instanceof Error ? error.message : String(error);
    return evidence;
  } finally {
    runtime.dispose();
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitTaskBudgetIntegrationEvidence(evidence: TaskBudgetIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
