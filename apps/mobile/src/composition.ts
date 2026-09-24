import { createElement, useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileResourceRemote } from '@weknora/api-client/mobile/resources';
import { createJsonTransport } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createMobileRuntime } from '@weknora/mobile-core';
import { createScopedVault, createWebCryptoCipher, createInMemoryTaskProjectionStore } from '@weknora/mobile-core';
import type { MobileRuntime, RuntimeSnapshot, ScopedVault, Deployment } from '@weknora/mobile-core';
import { createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createNativeOidcBrowser } from './adapters/oidc-browser.ts';
import { createNativeSecurePendingOidcStore } from './adapters/secure-store.ts';
import type { SecureStorePort } from './adapters/secure-store.ts';
import { createSecureVaultKeyStore, createSecureVaultStorage } from './adapters/vault-adapters.ts';
import { createNativeSecureCredentialStore } from './adapters/credential-store.ts';
import { createNativeSecureDeploymentStore } from './adapters/deployment-store.ts';
import { streamAuthorizedSse, type SseFetchLike } from './adapters/sse-stream.ts';
import { createNativeSecureDeploymentRegistry } from './adapters/deployment-registry.ts';
import { HomeScreen } from './screens/HomeScreen.tsx';
import { TasksScreen } from './screens/TasksScreen.tsx';
import { DeploymentLoginScreen, validatedDeploymentOrigin } from './screens/DeploymentLoginScreen.tsx';
import { UpgradeRequiredScreen } from './screens/UpgradeRequiredScreen.tsx';
import { ReadOnlyScreen } from './screens/ReadOnlyScreen.tsx';
import type { ScopedStore } from '@weknora/mobile-core';
import { createNativeRequestId } from './adapters/request-id.ts';
import { createNativeSecureIntentLog } from './adapters/intent-log.ts';

/** App 生命周期单例：Runtime 撤销 scope 时 revoke 的就是这把 vault（#32）。 */
const nativeScopedVault = createNativeScopedVaultIfAvailable();
let nativeIntentLog: ReturnType<typeof createNativeSecureIntentLog> | undefined;
/** 惰性解析 expo-secure-store（与 pendingOidcStore 的函数体内 require 同模式；app-smoke 环境有 stub）。 */
const intentLogOf = (): ReturnType<typeof createNativeSecureIntentLog> => (nativeIntentLog ??= createNativeSecureIntentLog());

/** 授权 scope 的加密 drafts（New 屏离线草稿）；无 vault 或无授权 scope 返回 undefined（fail soft）。 */
export async function openScopedDraftStore(): Promise<ScopedStore | undefined> {
  const lease = runtime().scopeLease();
  if (!nativeScopedVault || !lease) return undefined;
  try {
    return await nativeScopedVault.open(lease);
  } catch {
    return undefined;
  }
}

export const OIDC_REDIRECT_URI = 'weknora://oidc';
const cloudFromBuild = typeof process !== 'undefined' ? process.env.EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN : undefined;
export const officialCloudOrigin = cloudFromBuild ? validatedDeploymentOrigin(cloudFromBuild) : undefined;

function nativeFetch(input: string, init?: { method?: string; headers?: Record<string, string>; body?: string | FormData; signal?: AbortSignal }) {
  return fetch(input, init as RequestInit);
}

/** Wires the Scoped Vault only where Web Crypto exists. Native crypto seam completes in T10 (#40); absence must not break login. */
function createNativeScopedVaultIfAvailable(): ScopedVault | undefined {
  try {
    const secure = require('expo-secure-store') as SecureStorePort;
    return createScopedVault({
      keyStore: createSecureVaultKeyStore(secure),
      storage: createSecureVaultStorage(secure),
      cipher: createWebCryptoCipher(),
    });
  } catch {
    return undefined;
  }
}

/** The app composition root is the only place that joins concrete native adapters to Runtime. */
export function createNativeMobileRuntime(): MobileRuntime {
  let runtime!: MobileRuntime;
  const browser = createNativeOidcBrowser(OIDC_REDIRECT_URI);
  runtime = createMobileRuntime({
    credentialStore: createNativeSecureCredentialStore(),
    deploymentStore: createNativeSecureDeploymentStore(),
    deploymentRegistry: createNativeSecureDeploymentRegistry(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    scopedVault: nativeScopedVault,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(nativeFetch) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(nativeFetch) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
    authorizedStream(origin) {
      // 惰性解析 expo/fetch（Node 测试环境无此模块；解析失败即无流通道，fail closed）
      let streamFetch: SseFetchLike | undefined;
      try { streamFetch = (require('expo/fetch') as { fetch: SseFetchLike }).fetch; } catch { streamFetch = undefined; }
      return streamFetch === undefined ? undefined : (input, accessToken, onChunk) => streamAuthorizedSse(origin, input, accessToken, onChunk, streamFetch);
    },
    resourceShelf: {
      remoteFor(origin) {
        const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(nativeFetch) });
        return createMobileResourceRemote({ origin, request: client.request });
      },
    },
    pendingOidcStore: createNativeSecurePendingOidcStore(),
    oidcBrowser: {
      async open(authorizationUrl) {
        const callbackUrl = await browser.open(authorizationUrl);
        await runtime.completeOidc(callbackUrl);
        return callbackUrl;
      },
    },
  });
  return runtime;
}

export interface RuntimeSurfaceProps {
  snapshot: RuntimeSnapshot;
  onSignIn: (input: { origin: string; email: string; password: string }) => Promise<void>;
  onBeginOidc: (input: { origin: string }) => Promise<void>;
  onSignOut: () => Promise<void>;
  onActivateTenant: (tenantId: string) => Promise<void>;
  deployments?: Deployment[];
  onSwitchDeployment?: (origin: string) => Promise<void>;
}

const taskOffices = new Map<string, TaskOffice>();

/** remount key 必须含 deployment origin（R1-F48/F49）：两部署租户 id 相同（自增小整数常见）时
 * 跨部署切换也必须 remount，否则 useEffect(load,[]) 不重跑、旧闭包命中已撤销 lease。 */
export function deploymentScopeKey(origin: string, activeTenantId: string): string {
  return `${origin}::${activeTenantId}`;
}

/** Task Office 按 deployment origin 记忆化；lease 由 Runtime 提供，切租户即 fail closed。 */
function taskOfficeFor(activeRuntime: MobileRuntime, origin: string): TaskOffice {
  let office = taskOffices.get(origin);
  if (!office) {
    const remote = createTaskOfficeRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input), stream: (input, onChunk) => activeRuntime.authorizedEventStream(input, onChunk) });
    office = createTaskOffice({
      backend: remote,
      detail: remote,
      interactions: remote,
      lease: () => activeRuntime.scopeLease(),
      // 显式装配（R1-F20 最小修复）：App 重启恢复需要持久 TaskProjectionStore（SQLite 后端，Round 2）；
      // 此处显式传 in-memory store 使「未注入持久化」成为组合根的显式决策而非静默回退。
      store: createInMemoryTaskProjectionStore(),
      intentLog: intentLogOf(),
      newRequestId: createNativeRequestId(),
    });
    taskOffices.set(origin, office);
  }
  return office;
}

/** /tasks 应用根：授权面才渲染列表屏，其余面回到 Runtime 裁决的 Surface。 */
export function MobileTasks({ onOpenTask }: { onOpenTask?: (taskId: string, runId: string) => void } = {}) {
  const activeRuntime = runtime();
  const snapshot = useSyncExternalStore(activeRuntime.subscribe, activeRuntime.snapshot, activeRuntime.snapshot);
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return null;
  return createElement(TasksScreen, {
    key: deploymentScopeKey(snapshot.deployment.origin, snapshot.identity.activeTenantId ?? ''),
    taskOffice: taskOfficeFor(activeRuntime, snapshot.deployment.origin),
    ...(onOpenTask === undefined ? {} : { onOpenTask: (card: { taskId: string; runId: string }) => onOpenTask(card.taskId, card.runId) }),
  });
}

/** 详情路由经此取当前授权 scope 的 Task Office（无授权面返回 undefined）。 */
export function activeTaskOffice(): TaskOffice | undefined {
  const activeRuntime = runtime();
  const snapshot = activeRuntime.snapshot();
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return undefined;
  return taskOfficeFor(activeRuntime, snapshot.deployment.origin);
}

/** Selects a visible surface only from the presentation-safe Runtime snapshot. */
export function RuntimeSurface({ snapshot, deployments, onSignIn, onBeginOidc, onSignOut, onActivateTenant, onSwitchDeployment }: RuntimeSurfaceProps) {
  if (snapshot.surface === 'authorized' && snapshot.deployment && snapshot.identity?.userId && snapshot.identity.activeTenantId) {
    return createElement(HomeScreen, {
      key: deploymentScopeKey(snapshot.deployment.origin, snapshot.identity.activeTenantId),
      deploymentLabel: snapshot.deployment.label,
      tenants: snapshot.identity.tenants ?? [{ id: snapshot.identity.activeTenantId }],
      activeTenantId: snapshot.identity.activeTenantId,
      onActivateTenant: (tenantId: string) => { void onActivateTenant(tenantId); },
      onSignOut,
      taskOffice: taskOfficeFor(runtime(), snapshot.deployment.origin),
      otherDeployments: (deployments ?? []).filter((deployment) => deployment.origin !== snapshot.deployment?.origin),
      onSwitchDeployment,
    });
  }
  if (snapshot.surface === 'read-only') {
    return createElement(ReadOnlyScreen, {
      deploymentLabel: snapshot.deployment?.label,
      handle: runtime().resourceShelf(),
      onSignOut,
    });
  }
  if (snapshot.surface === 'deployment-login') {
    return createElement(DeploymentLoginScreen, { officialCloudOrigin, deployments, onSignIn, onBeginOidc, onSwitchDeployment });
  }
  return createElement(UpgradeRequiredScreen, { deploymentLabel: snapshot.deployment?.label, reason: snapshot.reason, onSignOut });
}

let nativeRuntime: MobileRuntime | undefined;

function runtime(): MobileRuntime {
  nativeRuntime ??= createNativeMobileRuntime();
  return nativeRuntime;
}

/** Route files reach the app-lifetime runtime through this accessor only. */
export function activeMobileRuntime(): MobileRuntime {
  return runtime();
}

/** Starts verified Runtime restoration once for an application lifetime. */
export function bootRuntimeOnce(activeRuntime: { boot(): unknown }, booted: { current: boolean }): void {
  if (booted.current) return;
  booted.current = true;
  void activeRuntime.boot();
}

/** 部署列表同步：latest-wins + 失败包含（B2-F33）。导出以供 app-smoke 行为级直调。 */
export function createDeploymentListSync(
  activeRuntime: Pick<MobileRuntime, 'listDeployments'>,
  setDeployments: (deployments: Deployment[]) => void,
): () => void {
  let sequence = 0;
  return () => {
    const ticket = ++sequence;
    void activeRuntime.listDeployments().then(
      (deployments) => { if (ticket === sequence) setDeployments(deployments); },
      () => undefined, // 失败包含：维持现状，不形成 unhandled rejection
    );
  };
}

/** Native application root; screens receive snapshots and callbacks only. */
export function MobileApp() {
  const activeRuntime = runtime();
  const booted = useRef(false);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  useEffect(() => {
    bootRuntimeOnce(activeRuntime, booted);
  }, [activeRuntime]);
  const snapshot = useSyncExternalStore(activeRuntime.subscribe, activeRuntime.snapshot, activeRuntime.snapshot);
  // registry 内容只在 surface/origin 变化的发布中变化：依赖收窄，tenant 切换等发布不再重复读安全存储。
  const syncDeployments = useRef(createDeploymentListSync(activeRuntime, setDeployments));
  useEffect(() => { syncDeployments.current(); }, [activeRuntime, snapshot.surface, snapshot.deployment?.origin]);
  return createElement(RuntimeSurface, {
    snapshot,
    deployments,
    onSignIn: async ({ origin, email, password }) => { await activeRuntime.signIn({ deployment: { origin }, email, password }); },
    onBeginOidc: async ({ origin }) => { await activeRuntime.beginOidc({ deployment: { origin }, redirectUri: OIDC_REDIRECT_URI }); },
    onSignOut: () => activeRuntime.signOut(),
    onActivateTenant: async (tenantId) => { await activeRuntime.activateTenant(tenantId); },
    onSwitchDeployment: async (origin) => { await activeRuntime.switchDeployment(origin); },
  });
}

export function completeNativeOidcCallback(callbackUrl: string): Promise<RuntimeSnapshot> {
  return runtime().completeOidc(callbackUrl);
}
