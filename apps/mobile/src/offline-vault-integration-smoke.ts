import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import {
  createInMemoryCredentialStore, createInMemoryVaultKeyStore, createInMemoryVaultStorage, createMobileRuntime,
  createOfflineGate, createScopedVault, createTaskOffice, createVaultTaskProjectionStore, createWebCryptoCipher,
  OfflineGateError, guardInteractionBackend, guardTaskBackend, type OfflineActionKind, type TaskOffice,
} from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type OfflineVaultIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface OfflineVaultIntegrationEvidence {
  deploymentOrigin: string;
  onlineStart: 'admitted' | 'pending' | 'rejected' | 'failed';
  offlineBlockedActions: string[];
  offlineDraftRoundTrip: boolean;
  offlineDetailView: 'projected' | 'unavailable' | 'failed';
  ciphertextProjectionRows: boolean;
  onlineResync: 'recovered' | 'failed' | 'not-attempted';
  errorReason?: string;
  timestamp: string;
}

/** 与 runtime-integration-smoke.ts 相同的 opt-in 语义；主机防线复用 disallowedDeploymentHost（B2-F15）。 */
export function offlineVaultIntegrationConfig(env: Record<string, string | undefined>): OfflineVaultIntegrationConfig {
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
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

/** decide 经 Task Office 的错误包装（attention-inbox.ts:154-161）：OfflineGateError 落在 cause 链上。 */
const offlineBlockOf = (error: unknown): OfflineActionKind | undefined => {
  if (error instanceof OfflineGateError) return error.action;
  if (error instanceof Error && error.cause instanceof OfflineGateError) return error.cause.action;
  return undefined;
};

/**
 * 真实端到端（AC3）：生产 JSON transport + Runtime 授权通道 + 真实 Scoped Vault（AES-GCM，
 * in-memory keyStore/storage Adapter——vault 深模块本身真实执行）+ guarded Task Office +
 * vault 投影持久化。断网模拟的唯一注入点是 transport fetcher（离线时直接拒绝）。
 * total 化收口：任何步骤异常也产出证据对象（含 errorReason，不含凭据），绝不 reject。
 */
export async function runOfflineVaultIntegration(config: Extract<OfflineVaultIntegrationConfig, { enabled: true }>): Promise<OfflineVaultIntegrationEvidence> {
  const evidence: OfflineVaultIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    onlineStart: 'failed',
    offlineBlockedActions: [],
    offlineDraftRoundTrip: false,
    offlineDetailView: 'failed',
    ciphertextProjectionRows: false,
    onlineResync: 'not-attempted',
    timestamp: new Date().toISOString(),
  };
  let networkOnline = true;
  const fetcher: FetchLike = (input, init) => {
    if (!networkOnline) return Promise.reject(new Error('offline: network unreachable'));
    return fetch(input, init as RequestInit);
  };
  const keyStore = createInMemoryVaultKeyStore();
  const storage = createInMemoryVaultStorage();
  const vault = createScopedVault({ keyStore, storage, cipher: createWebCryptoCipher() });
  const gate = createOfflineGate({ online: async () => networkOnline });
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    scopedVault: vault,
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
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) {
      evidence.errorReason = `surface ${snapshot.surface}`;
      return evidence;
    }
    const remote = createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) });
    const office: TaskOffice = createTaskOffice({
      backend: guardTaskBackend(remote, gate),
      // AC3（final review critical）：新意图 start 在 createSession 之前经 office 级门——
      // guardTaskBackend 只拦 Start POST，createSession 离线时会以 transport 错误伪装
      // 成 TASK_OFFICE_BACKEND（cause 非 OfflineGateError），'run' 判决不可达。
      gate,
      detail: remote,
      interactions: guardInteractionBackend(remote, gate),
      lease: () => runtime.scopeLease(),
      store: createVaultTaskProjectionStore({ vault, lease: () => runtime.scopeLease() }),
    });
    const agentsEnvelope = await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/agents' }) as { success?: boolean; data?: Array<{ id?: unknown }> };
    const agentId = typeof agentsEnvelope?.data?.[0]?.id === 'string' ? agentsEnvelope.data[0].id : undefined;
    if (!agentId) {
      evidence.errorReason = 'no agent available on the deployment';
      return evidence;
    }
    // ① 联网 + 用户确认提交：真实 Start（新意图新 request_id）
    const goal = `T10 集成验证：${new Date().toISOString()}`;
    const receipt = await office.start({ text: goal, agentId, budgetUpper: 10 });
    evidence.onlineStart = receipt.phase === 'bound' ? 'admitted' : receipt.phase === 'rejected' ? 'rejected' : 'pending';
    if (receipt.phase !== 'bound' || receipt.runId === undefined) {
      evidence.errorReason = `receipt phase ${receipt.phase}`;
      return evidence;
    }
    // ② 在线 hydrate：投影（含离线快照）经 Scoped Vault 加密落盘
    const page = await office.tasks({});
    const card = page.items.find((item) => item.runId === receipt.runId);
    if (card === undefined) {
      evidence.errorReason = 'created run not visible in the task list';
      return evidence;
    }
    const handle = office.open({ taskId: card.taskId, runId: receipt.runId! });
    await handle.hydrate();
    // 加密证据：vault 存储层除明文索引行（'["run.<id>"]' 属预期）外全部是 base64 密文。
    // 不用 includes(taskId)——数字 taskId 作子串在 base64 中误匹配率高（scoped-vault.test.ts:40-50 同款 base64 断言）。
    const ciphertextRows = [...storage.entries().values()].filter((value) => !value.startsWith('['));
    evidence.ciphertextProjectionRows = ciphertextRows.length > 0 && ciphertextRows.every((value) => /^[A-Za-z0-9+/]+={0,2}$/.test(value));
    // ③ 断网：四类危险动作全部 fail closed（run=真实 Start 重放，office 级 gate 在
    //    createSession 之前拒绝；approval=guarded decide；budget/external-action=gate
    //    通道——两类的服务端入口属 #39/#48/#51）
    networkOnline = false;
    const blocked: string[] = [];
    try {
      await office.start({ text: `${goal} (offline replay)`, agentId, budgetUpper: 10 });
    } catch (error) { if (offlineBlockOf(error) === 'run') blocked.push('run'); }
    try {
      await office.decide({
        item: { interactionId: 'probe', runId: receipt.runId!, kind: 'tool_approval', argsHash: '', expectedRevision: 0, createdAt: new Date().toISOString() },
        action: 'approve',
      });
    } catch (error) { if (offlineBlockOf(error) === 'approval') blocked.push('approval'); }
    try { await gate.assertOnline('budget'); } catch (error) { if (offlineBlockOf(error) === 'budget') blocked.push('budget'); }
    try { await gate.assertOnline('external-action'); } catch (error) { if (offlineBlockOf(error) === 'external-action') blocked.push('external-action'); }
    evidence.offlineBlockedActions = blocked;
    // ④ 离线草稿往返（加密保存于当前 scope）
    const lease = runtime.scopeLease()!;
    const drafts = await vault.open(lease);
    await drafts.drafts.put({ id: 'new-task', body: JSON.stringify({ text: goal }) });
    evidence.offlineDraftRoundTrip = (await drafts.drafts.get('new-task'))?.body.includes(goal) === true;
    // ⑤ 离线查看获准 Task 内容：detail 通道断开 → 加密投影降级视图
    try {
      const offlineView = await handle.resync();
      evidence.offlineDetailView = offlineView.interruption?.reason === 'offline' && offlineView.taskId === card.taskId ? 'projected' : 'unavailable';
    } catch {
      evidence.offlineDetailView = 'unavailable';
    }
    // ⑥ 恢复联网：显式 resync 恢复权威同步（不自动重放）
    networkOnline = true;
    try {
      const recovered = await handle.resync();
      evidence.onlineResync = recovered.interruption?.reason === 'offline' ? 'failed' : 'recovered';
    } catch {
      evidence.onlineResync = 'failed';
    }
    handle.close('integration-complete');
    return evidence;
  } catch (error) {
    evidence.errorReason = error instanceof Error ? error.message : String(error); // 失败仍产出证据（不含凭据）
    return evidence;
  } finally {
    runtime.dispose(); // 释放 lease/凭据通道并撤销 vault scope（撤权 fail closed 的收尾）
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitOfflineVaultIntegrationEvidence(evidence: OfflineVaultIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
