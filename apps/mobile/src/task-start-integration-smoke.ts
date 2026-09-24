import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { createNativeRequestId } from './adapters/request-id.ts';

export type TaskStartIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface TaskStartIntegrationEvidence {
  deploymentOrigin: string;
  start: 'admitted' | 'pending' | 'rejected' | 'failed';
  requestId?: string;
  runId?: string;
  repeatSubmitSameRequest: 'no-second-dispatch' | 'second-dispatch' | 'failed';
  runVisibleInTasks: boolean | 'unavailable';
  errorReason?: string;
  timestamp: string;
}

/** 与 task-office-integration-smoke.ts 相同的 opt-in 语义（自包含，不跨计划 import）。 */
export function taskStartIntegrationConfig(env: Record<string, string | undefined>): TaskStartIntegrationConfig {
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
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

/**
 * 真实端到端（AC3）：生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter +
 * Task Office start(goal) 编排，在真实部署上创建一个真 Task，并验证同一 request_id
 * 重入不产生第二次派发。agent 目录经授权通道读 GET /api/v1/agents。
 */
export async function runTaskStartIntegration(config: Extract<TaskStartIntegrationConfig, { enabled: true }>): Promise<TaskStartIntegrationEvidence> {
  const evidence: TaskStartIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    start: 'failed',
    repeatSubmitSameRequest: 'failed',
    runVisibleInTasks: 'unavailable',
    timestamp: new Date().toISOString(),
  };
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
  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  if (snapshot.surface !== 'authorized' || !snapshot.deployment) {
    evidence.errorReason = `surface ${snapshot.surface}`;
    return evidence;
  }
  const agentsEnvelope = await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/agents' }) as { success?: boolean; data?: Array<{ id?: unknown }> };
  const agentId = typeof agentsEnvelope?.data?.[0]?.id === 'string' ? agentsEnvelope.data[0].id : undefined;
  if (!agentId) {
    evidence.errorReason = 'no agent available on the deployment';
    return evidence;
  }
  const newRequestId = createNativeRequestId();
  // intentLog 用 office 缺省的进程内实现（同一 office 实例内的重入语义即可支撑本证据；
  // 跨进程耐久由 Task 4 的 secure Adapter 在组合根承载，真机重启证据属 #40）
  const office: TaskOffice = createTaskOffice({
    backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
    lease: () => runtime.scopeLease(),
    newRequestId,
  });
  const goal = { text: `T06 集成验证：${new Date().toISOString()}`, agentId, budgetUpper: 10 };
  const first = await office.start(goal);
  evidence.requestId = first.requestId;
  evidence.runId = first.runId;
  evidence.start = first.phase === 'bound' ? 'admitted' : first.phase === 'rejected' ? 'rejected' : 'pending';
  if (first.phase !== 'bound' || first.runId === undefined) {
    evidence.errorReason = `first receipt phase ${first.phase}`;
    return evidence;
  }
  // 同一意图重入（真服务端）：不得产生第二次派发
  const second = await office.start(goal, { requestId: first.requestId });
  evidence.repeatSubmitSameRequest = second.dispatched === false && second.runId === first.runId ? 'no-second-dispatch' : 'second-dispatch';
  // 创建的 run 可在任务列表观测（端到端闭环）
  try {
    const page = await office.tasks({});
    evidence.runVisibleInTasks = page.items.some((card) => card.runId === first.runId);
  } catch {
    evidence.runVisibleInTasks = 'unavailable';
  }
  return evidence;
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitTaskStartIntegrationEvidence(evidence: TaskStartIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
