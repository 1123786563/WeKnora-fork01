import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { createNativeRequestId } from './adapters/request-id.ts';

export type TaskInterventionIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface TaskInterventionIntegrationEvidence {
  deploymentOrigin: string;
  start: 'admitted' | 'pending' | 'rejected' | 'failed';
  runId?: string;
  observedRunStatus?: string;
  steer: 'accepted' | 'conflict' | 'unknown' | 'skipped-terminal' | 'failed';
  stop: 'requested-then-confirmed' | 'requested-only' | 'unknown-then-reconciled' | 'conflict' | 'skipped-terminal' | 'failed';
  queueNext: 'admitted' | 'conflict' | 'unknown' | 'skipped' | 'failed';
  boundRunId?: string;
  nextRunId?: string;
  revisionCarried?: number;
  errorReason?: string;
  timestamp: string;
}

/** 与 task-start-integration-smoke.ts 相同的 opt-in 语义（自包含，不跨计划 import）。 */
export function taskInterventionIntegrationConfig(env: Record<string, string | undefined>): TaskInterventionIntegrationConfig {
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

const TERMINAL_STATUSES = new Set(['succeeded', 'failed', 'canceled']);
const sleep = (ms: number): Promise<void> => new Promise((resolve) => { setTimeout(resolve, ms); });

/**
 * 真实端到端（AC3 live）：生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter +
 * TaskHandle.act 编排。创建一个真 Task 后按观察到的 runStatus 如实干预——活动 Run：
 * steer → stop → 有界等待 canceled → queue-next 重启；已终态 Run：queue-next 直接重启。
 * 每一步如实记录（含冲突与未知），失败落 errorReason，绝不伪造通过。
 */
export async function runTaskInterventionIntegration(config: Extract<TaskInterventionIntegrationConfig, { enabled: true }>): Promise<TaskInterventionIntegrationEvidence> {
  const evidence: TaskInterventionIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    start: 'failed', steer: 'failed', stop: 'failed', queueNext: 'skipped',
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
  try {
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
    const remote = createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input), stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk) });
    const office: TaskOffice = createTaskOffice({
      backend: remote, detail: remote, commands: remote,
      lease: () => runtime.scopeLease(), newRequestId: createNativeRequestId(),
    });
    const requestId = createNativeRequestId()(); // 生成器调用一次产出本意图的幂等键字符串（start options 要求 string）
    const receipt = await office.start({ text: `T07 集成验证：${new Date().toISOString()}`, agentId, budgetUpper: 10 }, { requestId });
    evidence.start = receipt.phase === 'bound' ? 'admitted' : receipt.phase === 'rejected' ? 'rejected' : 'pending';
    if (receipt.phase !== 'bound' || receipt.runId === undefined) {
      evidence.errorReason = `start receipt phase ${receipt.phase}`;
      return evidence;
    }
    evidence.runId = receipt.runId;
    evidence.boundRunId = receipt.runId;
    // taskId（= sessionId，ADR-0004）：`TaskStartReceipt` 不携带会话 id，用任务列表反查（零新接口）。
    const page = await office.tasks({});
    const card = page.items.find((item) => item.runId === receipt.runId);
    if (card === undefined) {
      evidence.errorReason = 'created run not visible in the task list';
      return evidence;
    }
    const handle = office.open({ taskId: card.taskId, runId: receipt.runId });
    const view = await handle.hydrate();
    evidence.observedRunStatus = view.runStatus;
    evidence.revisionCarried = view.revision;
    if (!TERMINAL_STATUSES.has(view.runStatus)) {
      const steer = await handle.act({ kind: 'steer', text: '集成验证：继续' });
      evidence.steer = steer.outcome === 'accepted' ? 'accepted' : steer.outcome === 'conflict' ? 'conflict' : 'unknown';
      const stop = await handle.act({ kind: 'stop' });
      if (stop.outcome === 'conflict') {
        evidence.stop = 'conflict';
      } else {
        const stopPhaseAfterAck = handle.view()?.stop?.phase ?? 'requested';
        if (stopPhaseAfterAck === 'unknown') {
          // 投递结果未知：以真实 resync 核对（AC2 核对路径必须真实走通）。
          const reconciled = await handle.resync();
          // resolveUnknownStop（task-intent.ts）快照规则：非 canceled ⇒ 取消 CAS 不可能已
          // 落地（未落地）——如实落 requested-only，绝不捏造从未观察到的 CAS 拒绝（F9-2）。
          evidence.stop = reconciled.runStatus === 'canceled' ? 'unknown-then-reconciled' : 'requested-only';
        } else {
          let confirmed = false;
          for (let attempt = 0; attempt < 10 && !confirmed; attempt += 1) {
            await sleep(1000);
            const observed = await handle.resync();
            if (observed.runStatus === 'canceled') confirmed = true;
          }
          evidence.stop = confirmed ? 'requested-then-confirmed' : 'requested-only';
        }
      }
      const terminal = (await handle.resync());
      if (TERMINAL_STATUSES.has(terminal.runStatus)) {
        const restart = await handle.act({ kind: 'queue-next', text: '集成验证：重启' });
        evidence.queueNext = restart.outcome === 'accepted' ? 'admitted' : restart.outcome === 'conflict' ? 'conflict' : 'unknown';
        if (restart.nextRunId !== undefined) evidence.nextRunId = restart.nextRunId;
        // 投递结果 unknown 时（F9-1）：绑定的旧 Run 恒为终态，观察 resync 无法揭示新 Run
        // 的重准入命运（nextRunId 仅随 202 ack 携带，unknown 时无核对源）——如实保持上一行
        // 映射的 'unknown'，绝不洗白成 admitted。
      } else {
        evidence.queueNext = 'skipped';
      }
    } else {
      evidence.steer = 'skipped-terminal';
      evidence.stop = 'skipped-terminal';
      const restart = await handle.act({ kind: 'queue-next', text: '集成验证：重启' });
      evidence.queueNext = restart.outcome === 'accepted' ? 'admitted' : restart.outcome === 'conflict' ? 'conflict' : 'unknown';
      if (restart.nextRunId !== undefined) evidence.nextRunId = restart.nextRunId;
    }
    handle.close('integration-done');
    return evidence;
  } catch (error) {
    evidence.errorReason = error instanceof Error ? error.message : String(error); // 失败仍产出证据（不含凭据）
    return evidence;
  } finally {
    runtime.dispose();
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitTaskInterventionIntegrationEvidence(evidence: TaskInterventionIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
