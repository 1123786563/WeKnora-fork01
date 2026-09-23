import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, TaskOfficeError, type RuntimeSnapshot, type TaskOffice } from '@weknora/mobile-core';

export type TaskOfficeIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface TaskOfficeIntegrationEvidence {
  deploymentOrigin: string;
  unauthenticatedRead: 'rejected' | 'failed-open';
  home: 'loaded' | 'failed';
  sections: { needsMe: number; running: number; recentlyCompleted: number; unreadNotifications: number } | 'unavailable';
  listSearch: 'matched' | 'no-match' | 'failed';
  archiveRoundtrip: 'archived-restored' | 'unavailable' | 'failed';
  archiveRestore: 'restored' | 'failed' | 'not-attempted' | 'cleanup-required';
  commandTimestamp: string;
}

/** A pre-login denial counts only when the Task Office rejects for its missing scope lease. */
export async function probeUnauthenticatedRead(office: TaskOffice): Promise<'rejected' | 'failed-open'> {
  try {
    await office.tasks({});
    return 'failed-open';
  } catch (error) {
    return error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED' ? 'rejected' : 'failed-open';
  }
}

/** Archive at most one live task, and always attempt to restore it after a successful archive. */
export async function runArchiveRoundtrip(
  office: TaskOffice,
  taskId: string,
  scopeStillMatches: () => boolean,
): Promise<Pick<TaskOfficeIntegrationEvidence, 'archiveRoundtrip' | 'archiveRestore'>> {
  try {
    await office.archive(taskId);
  } catch {
    // The remote write may have committed before Task Office noticed a revoked lease.
    // Compensate only while the original deployment/user/tenant is still active.
    let sameScope = false;
    try { sameScope = scopeStillMatches(); } catch { /* treat an unreadable scope as changed */ }
    if (!sameScope) return { archiveRoundtrip: 'failed', archiveRestore: 'cleanup-required' };
    try {
      await office.restore(taskId);
      return { archiveRoundtrip: 'failed', archiveRestore: 'restored' };
    } catch {
      return { archiveRoundtrip: 'failed', archiveRestore: 'failed' };
    }
  }
  let archivedVisible = false;
  let restoredVisible = false;
  let listFailed = false;
  let restoreFailed = false;
  try {
    const archivedPage = await office.tasks({ archived: true });
    archivedVisible = archivedPage.items.some((card) => card.taskId === taskId);
  } catch {
    listFailed = true;
  } finally {
    // Do not allow an intermediate read failure to leave a user's task archived.
    try {
      await office.restore(taskId);
    } catch {
      restoreFailed = true;
    }
  }
  if (restoreFailed) return { archiveRoundtrip: 'failed', archiveRestore: 'failed' };
  try {
    const restoredPage = await office.tasks({});
    restoredVisible = restoredPage.items.some((card) => card.taskId === taskId);
  } catch {
    listFailed = true;
  }
  return {
    archiveRoundtrip: !listFailed && archivedVisible && restoredVisible ? 'archived-restored' : 'failed',
    archiveRestore: 'restored',
  };
}

/** 与 T01 mobileRuntimeIntegrationConfig 相同的 opt-in 语义（自包含，不跨计划 import）。 */
export function taskOfficeIntegrationConfig(env: Record<string, string | undefined>): TaskOfficeIntegrationConfig {
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
 * 真实生产 JSON transport + 具体 Remote Adapter + Runtime 授权通道 + Task Office
 * 编排。归档回路只在账号确有任务时执行（否则如实记 'unavailable'，不伪造）。
 */
export async function runTaskOfficeIntegration(config: Extract<TaskOfficeIntegrationConfig, { enabled: true }>): Promise<TaskOfficeIntegrationEvidence> {
  const evidence: TaskOfficeIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    unauthenticatedRead: 'failed-open',
    home: 'failed',
    sections: 'unavailable',
    listSearch: 'failed',
    archiveRoundtrip: 'unavailable',
    archiveRestore: 'not-attempted',
    commandTimestamp: new Date().toISOString(),
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
  // Prove the exact production Task Office boundary fails closed before sign-in.
  // Reuse the same deployment origin and transport, but do not send credentials.
  const unauthenticatedOffice: TaskOffice = createTaskOffice({
    backend: createTaskOfficeRemote({
      origin: config.deploymentOrigin,
      request: (input) => runtime.authorizedRequest(input),
    }),
    lease: () => runtime.scopeLease(),
  });
  evidence.unauthenticatedRead = await probeUnauthenticatedRead(unauthenticatedOffice);
  if (evidence.unauthenticatedRead !== 'rejected') { runtime.dispose(); return evidence; }
  let snapshot: RuntimeSnapshot;
  try {
    snapshot = await runtime.signIn({
      deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
      email: config.email,
      password: config.password,
    });
  } catch {
    runtime.dispose();
    return evidence;
  }
  if (snapshot.surface !== 'authorized' || !snapshot.deployment) { runtime.dispose(); return evidence; }

  const office: TaskOffice = createTaskOffice({
    backend: createTaskOfficeRemote({
      origin: config.deploymentOrigin,
      request: (input) => runtime.authorizedRequest(input),
    }),
    lease: () => runtime.scopeLease(),
  });

  try {
    const homeView = await office.home();
    evidence.home = 'loaded';
    evidence.sections = {
      needsMe: homeView.needsMe.length,
      running: homeView.running.length,
      recentlyCompleted: homeView.recentlyCompleted.length,
      unreadNotifications: homeView.unreadNotifications,
    };

  // 搜索：用一个必然不存在的随机词，验证参数贯通服务端过滤（no-match 是合法结果）。
    const nonce = `zz-t04-${Date.now().toString(36)}`;
    const searched = await office.tasks({ search: nonce });
    evidence.listSearch = searched.items.length === 0 ? 'no-match' : 'matched';

  // 归档回路：只在账号确有任务时执行，否则如实 'unavailable'。
    const activePage = await office.tasks({});
    if (activePage.items.length === 0) return evidence;
    const target = activePage.items[0]!;
    const archiveScope = runtime.snapshot();
    const archiveOrigin = archiveScope.deployment?.origin;
    const archiveUser = archiveScope.identity?.userId;
    const archiveTenant = archiveScope.identity?.activeTenantId;
    try {
      Object.assign(evidence, await runArchiveRoundtrip(office, target.taskId, () => {
        const current = runtime.snapshot();
        return current.surface === 'authorized'
          && current.deployment?.origin === archiveOrigin
          && current.identity?.userId === archiveUser
          && current.identity?.activeTenantId === archiveTenant;
      }));
    } catch {
      evidence.archiveRoundtrip = 'failed';
    }
  } catch {
    evidence.home = evidence.home === 'loaded' ? evidence.home : 'failed';
  } finally {
    runtime.dispose();
  }
  return evidence;
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitTaskOfficeIntegrationEvidence(evidence: TaskOfficeIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
