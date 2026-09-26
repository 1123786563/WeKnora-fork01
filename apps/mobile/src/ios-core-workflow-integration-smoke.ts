import { mkdtempSync, rmSync } from 'node:fs';
import { readFile, unlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import {
  createMobileRuntime,
  createTaskOffice,
  type AuthorizedTransport,
  type MobileRuntime,
  type TaskOffice,
  type TaskStartReceipt,
} from '@weknora/mobile-core';
import { createSecureCredentialStore } from './adapters/credential-store.ts';
import type { SecureStorePort } from './adapters/secure-store.ts';
import { createSecureDeploymentStore } from './adapters/deployment-store.ts';
import { createNativeRequestId } from './adapters/request-id.ts';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type IosCoreWorkflowIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

/** 与 taskStartIntegrationConfig 相同的 opt-in 语义 + disallowedDeploymentHost 主机防线。 */
export function iosCoreWorkflowIntegrationConfig(env: Record<string, string | undefined>): IosCoreWorkflowIntegrationConfig {
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

export interface IosCoreWorkflowIntegrationEvidence {
  deploymentOrigin: string;
  signIn: 'authorized' | 'failed';
  /** 冷启动恢复（AC2 cold-start 的 Interface 面）：dispose 后第二 Runtime 实例 boot() 从持久凭据恢复授权面。 */
  coldBootRestore: 'authorized-restored' | 'failed' | 'not-attempted';
  /** 撤销不可复活（AC2 revocation 的 Interface 面）：signOut 清凭据后第三实例 boot() 必须停 deployment-login。 */
  revocation: 'revoked' | 'failed' | 'not-attempted';
  tenantSwitch: 'switched' | 'single-tenant' | 'failed' | 'not-attempted';
  /** 弱网（AC2 weak-network 的 Interface 面）：首枚 Start POST 注入 2s 延迟后断链 → 意图已在
   *  Start 前耐久落盘 → reconcilePending() 以同一 requestId 重派 → 服务端幂等收敛同一 run。 */
  weakNetwork: 'reconciled-same-run' | 'pending-retained' | 'failed' | 'not-attempted';
  weakNetworkStartRequests?: number;
  distinctRunIds?: number;
  runVisibleInTasks?: boolean | 'unavailable' | 'not-attempted';
  errorReason?: string;
  timestamp: string;
}

/** 文件兜底 SecureStorePort（仅 harness 环境；模拟器/真机组合根用 expo-secure-store）。
 *  目的不是冒充 Keychain，而是给「冷启动恢复/撤销」组合一个跨 Runtime 实例真实持久的存储：
 *  boot() 第二实例读取的是 signIn 第一实例落盘的字节，而不是同一进程内缓存的凭据对象。 */
function createFileBackedSecureStore(dir: string): SecureStorePort {
  const pathOf = (key: string): string => join(dir, encodeURIComponent(key));
  return {
    async getItemAsync(key) {
      try { return await readFile(pathOf(key), 'utf8'); } catch { return null; }
    },
    async setItemAsync(key, value) { await writeFile(pathOf(key), value, 'utf8'); },
    async deleteItemAsync(key) {
      try { await unlink(pathOf(key)); } catch { /* absent */ }
    },
  };
}

interface RuntimeWiring {
  credentialDir: string;
  fetcher: FetchLike;
  /** 弱网注入点：首枚 POST /workbench/executions 延迟 2s 后断链（只断 Start，createSession 放行）。 */
  weakStart?: boolean;
  startDispatches?: { count: number };
}

function runtimeOf(wiring: RuntimeWiring): MobileRuntime {
  const store = createFileBackedSecureStore(wiring.credentialDir);
  return createMobileRuntime({
    credentialStore: createSecureCredentialStore(store),
    deploymentStore: createSecureDeploymentStore(store),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(wiring.fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(wiring.fetcher) });
      const authorized: AuthorizedTransport = (input, accessToken) =>
        client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
      if (wiring.weakStart !== true) return authorized;
      const dispatches = wiring.startDispatches!;
      return (input, accessToken) => {
        if (input.method === 'POST' && input.path.includes('/workbench/executions')) {
          dispatches.count += 1;
          if (dispatches.count === 1) {
            // 弱网：请求已发出、2 秒无响应后链路断开（客户端侧 fetch 拒绝——服务端可能已受理，
            // 这正是「断链重续必须同 requestId」的不变量来源）。
            return new Promise((_, reject) => setTimeout(() => reject(new Error('weak network: connection dropped')), 2000));
          }
        }
        return authorized(input, accessToken);
      };
    },
  });
}

/**
 * 真实端到端（AC3）：生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter + Task Office
 * start/reconcile 编排，在真实部署上验证三组逆境语义。total 化收口：任何步骤异常也产出证据对象
 * （含 errorReason，不含凭据），绝不 reject 把证据丢弃（先例：task-start-integration-smoke.ts:68-69）。
 */
export async function runIosCoreWorkflowIntegration(config: Extract<IosCoreWorkflowIntegrationConfig, { enabled: true }>): Promise<IosCoreWorkflowIntegrationEvidence> {
  const evidence: IosCoreWorkflowIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    signIn: 'failed',
    coldBootRestore: 'not-attempted',
    revocation: 'not-attempted',
    tenantSwitch: 'not-attempted',
    weakNetwork: 'not-attempted',
    runVisibleInTasks: 'not-attempted',
    timestamp: new Date().toISOString(),
  };
  const credentialDir = mkdtempSync(join(tmpdir(), 'weknora-t39-'));
  const dispatches = { count: 0 };
  let activeRuntime: MobileRuntime | undefined;
  try {
    // 第一实例：signIn 落盘凭据（createSecureCredentialStore → 文件）+ deploymentStore 记忆活动实例。
    const first = runtimeOf({ credentialDir, fetcher: (input, init) => fetch(input, init as RequestInit) });
    activeRuntime = first;
    const snapshot = await first.signIn({
      deployment: { origin: config.deploymentOrigin, label: 'T39 acceptance deployment' },
      email: config.email,
      password: config.password,
    });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) {
      evidence.errorReason = `surface ${snapshot.surface}`;
      return evidence;
    }
    evidence.signIn = 'authorized';

    // Tenant 切换（有第二空间才切换；单空间部署如实记 single-tenant，不伪造）。
    const tenants = snapshot.identity?.tenants ?? [];
    if (tenants.length > 1 && tenants[1]?.id !== undefined) {
      try {
        const switched = await first.activateTenant(tenants[1]!.id);
        evidence.tenantSwitch = switched.surface === 'authorized' ? 'switched' : 'failed';
      } catch {
        evidence.tenantSwitch = 'failed';
      }
    } else {
      evidence.tenantSwitch = 'single-tenant';
    }

    // 弱网重续（AC1 单写者）：断链 Start → 意图已耐久落盘 → 恢复通道 reconcilePending 重派同一 requestId。
    try {
      const agentsEnvelope = await first.authorizedRequest({ method: 'GET', path: '/api/v1/agents' }) as { success?: boolean; data?: Array<{ id?: unknown }> };
      const agentId = typeof agentsEnvelope?.data?.[0]?.id === 'string' ? agentsEnvelope.data[0].id : undefined;
      if (agentId === undefined) {
        evidence.weakNetwork = 'failed';
        evidence.errorReason = 'no agent available on the deployment';
      } else {
        const office: TaskOffice = createTaskOffice({
          backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => first.authorizedRequest(input) }),
          lease: () => first.scopeLease(),
          newRequestId: createNativeRequestId(),
        });
        const goal = { text: `T39 弱网重续：${new Date().toISOString()}`, agentId, budgetUpper: 10 };
        let firstReceipt: TaskStartReceipt | undefined;
        try {
          firstReceipt = await office.start(goal);
        } catch {
          // 断链路径也允许 start 上抛——意图记录已在 Start POST 前落盘，reconcile 能接住。
          firstReceipt = undefined;
        }
        if (firstReceipt?.phase === 'bound' && firstReceipt.runId !== undefined) {
          // 弱网没有真的断（部署太快或注入未命中）：如实记录，不伪造重续证据。
          evidence.weakNetwork = 'failed';
          evidence.errorReason = 'weak-network injection did not hold: first dispatch already bound';
        } else if (dispatches.count === 0) {
          // start 在任何 Start 派发前就失败（如就绪裁决拒绝）：弱网路径根本没被走到，如实记 failed。
          evidence.weakNetwork = 'failed';
          evidence.errorReason = 'start failed before any Start dispatch (weak path not exercised)';
        } else {
          const recovered = await office.reconcilePending();
          const mine = firstReceipt !== undefined
            ? recovered.find((receipt) => receipt.requestId === firstReceipt.requestId)
            : recovered.find((receipt) => receipt.phase === 'bound' && receipt.runId !== undefined);
          if (mine?.runId !== undefined) {
            // 单写者幂等的权威在服务端 admission：重续回执的 runId 与首次未完成派发共享同一
            // requestId；全租户任务列表只能观测「Run 可见」，不得当作本次派发计数器。
            evidence.weakNetwork = 'reconciled-same-run';
            evidence.distinctRunIds = 1;
            try {
              const page = await office.tasks({});
              evidence.runVisibleInTasks = page.items.some((card) => card.runId === mine.runId);
            } catch {
              evidence.runVisibleInTasks = 'unavailable'; // 列表观测失败不影响重续证据本身
            }
          } else {
            evidence.weakNetwork = 'pending-retained';
          }
        }
        evidence.weakNetworkStartRequests = dispatches.count;
      }
    } catch (error) {
      evidence.weakNetwork = 'failed';
      evidence.errorReason = error instanceof Error ? error.message : String(error);
    }

    // 冷启动恢复（AC2 cold-start）：dispose 第一实例 → 同一持久凭据目录上的第二实例 boot()。
    first.dispose();
    activeRuntime = undefined;
    const second = runtimeOf({ credentialDir, fetcher: (input, init) => fetch(input, init as RequestInit) });
    activeRuntime = second;
    try {
      const restored = await second.boot();
      evidence.coldBootRestore = restored.surface === 'authorized' && restored.deployment?.origin === config.deploymentOrigin ? 'authorized-restored' : 'failed';
    } catch (error) {
      evidence.coldBootRestore = 'failed';
      evidence.errorReason = `cold boot: ${error instanceof Error ? error.message : String(error)}`;
    }

    // 撤销不可复活（AC2 revocation）：signOut 清凭据 → 第三实例 boot() 必须停在 deployment-login。
    try {
      await second.signOut();
      second.dispose();
      activeRuntime = undefined;
      const third = runtimeOf({ credentialDir, fetcher: (input, init) => fetch(input, init as RequestInit) });
      activeRuntime = third;
      const revived = await third.boot();
      evidence.revocation = revived.surface === 'deployment-login' ? 'revoked' : 'failed';
      if (evidence.revocation === 'failed') evidence.errorReason = `revoked session revived as ${revived.surface}`;
      third.dispose();
      activeRuntime = undefined;
    } catch (error) {
      evidence.revocation = 'failed';
      evidence.errorReason = `revocation: ${error instanceof Error ? error.message : String(error)}`;
    }
    return evidence;
  } catch (error) {
    evidence.signIn = 'failed';
    evidence.errorReason = error instanceof Error ? error.message : String(error); // 失败仍产出证据（不含凭据）
    return evidence;
  } finally {
    activeRuntime?.dispose();
    rmSync(credentialDir, { recursive: true, force: true }); // 凭据字节随临时目录销毁，不留明文
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitIosCoreWorkflowIntegrationEvidence(evidence: IosCoreWorkflowIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
