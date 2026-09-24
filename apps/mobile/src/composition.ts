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
import { createTaskMaterial } from '@weknora/mobile-core';
import type { TaskMaterial } from '@weknora/mobile-core';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createMobileLegacyTaskRemote } from '@weknora/api-client/mobile/legacy-tasks';
import { createMobileMaterialRemote } from '@weknora/api-client/mobile/materials';
import { createNativeOidcBrowser } from './adapters/oidc-browser.ts';
import { createNativeSecurePendingOidcStore } from './adapters/secure-store.ts';
import type { SecureStorePort } from './adapters/secure-store.ts';
import { createSecureVaultKeyStore, createSecureVaultStorage } from './adapters/vault-adapters.ts';
import { createNativeSecureCredentialStore } from './adapters/credential-store.ts';
import { createNativeSecureDeploymentStore } from './adapters/deployment-store.ts';
import { streamAuthorizedSse, type SseFetchLike } from './adapters/sse-stream.ts';
import { createNativeSecureDeploymentRegistry } from './adapters/deployment-registry.ts';
import { createDeviceRegistry, createNotificationInbox, type DeviceRegistry, type InboxItem, type NotificationInbox } from '@weknora/mobile-core';
import { createMobileDeviceRemote } from '@weknora/api-client/mobile/devices';
import { createMobileInboxRemote } from '@weknora/api-client/mobile/inbox';
import { createNativePushTokenIfAvailable } from './adapters/push-token.ts';
import { createNativeDeviceIdentity, nativeDevicePlatform } from './adapters/device-identity.ts';
import { createFetchBlobAdapter, createNativeSharePortIfAvailable } from './adapters/material-adapters.ts';
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

/** 四个模块级缓存统一以 origin::tenant 为键（B3-F26）：remount 只换组件不换缓存实例，
 * 同 origin 切租户/用户时不得复用含前 scope 残留状态（submissionStore、accumulated、
 * inbox view）的实例。条目超 8 个清最旧（防泄漏）。 */
const MAX_CACHE_ENTRIES = 8;
const cachePut = <T>(cache: Map<string, T>, key: string, make: () => T): T => {
  const existing = cache.get(key);
  if (existing !== undefined) return existing;
  if (cache.size >= MAX_CACHE_ENTRIES) cache.delete(cache.keys().next().value!);
  const created = make();
  cache.set(key, created);
  return created;
};

const taskOffices = new Map<string, TaskOffice>();

/** remount key 必须含 deployment origin（R1-F48/F49）：两部署租户 id 相同（自增小整数常见）时
 * 跨部署切换也必须 remount，否则 useEffect(load,[]) 不重跑、旧闭包命中已撤销 lease。 */
export function deploymentScopeKey(origin: string, activeTenantId: string): string {
  return `${origin}::${activeTenantId}`;
}

/** Task Office 按 deployment scope key 记忆化；lease 由 Runtime 提供，切租户即 fail closed。 */
function taskOfficeFor(activeRuntime: MobileRuntime, origin: string, tenantId: string): TaskOffice {
  return cachePut(taskOffices, deploymentScopeKey(origin, tenantId), () => {
    const remote = createTaskOfficeRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input), stream: (input, onChunk) => activeRuntime.authorizedEventStream(input, onChunk) });
    return createTaskOffice({
      backend: remote,
      detail: remote,
      interactions: remote,
      legacy: createMobileLegacyTaskRemote({
        origin,
        request: (input) => activeRuntime.authorizedRequest(input),
        stream: (input, onChunk) => activeRuntime.authorizedEventStream(input, onChunk),
      }),
      lease: () => activeRuntime.scopeLease(),
      // 显式装配（R1-F20 最小修复）：App 重启恢复需要持久 TaskProjectionStore（SQLite 后端，Round 2）；
      // 此处显式传 in-memory store 使「未注入持久化」成为组合根的显式决策而非静默回退。
      store: createInMemoryTaskProjectionStore(),
      intentLog: intentLogOf(),
      newRequestId: createNativeRequestId(),
    });
  });
}

const deviceRegistries = new Map<string, DeviceRegistry>();

/** Device Registry 按 deployment scope key 记忆化（同 taskOfficeFor 模式）：授权读通道与
 * scope lease 全部经 Runtime——设备注册的 token 不出 Runtime，注册身份按 Deployment 隔离（ADR-0007）。 */
function deviceRegistryFor(activeRuntime: Pick<MobileRuntime, 'authorizedRequest' | 'scopeLease'>, origin: string, tenantId: string): DeviceRegistry {
  return cachePut(deviceRegistries, deploymentScopeKey(origin, tenantId), () => createDeviceRegistry({
    remote: createMobileDeviceRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
    lease: () => activeRuntime.scopeLease(),
  }));
}

const notificationInboxes = new Map<string, NotificationInbox>();

/** 行动通知 Inbox 按 deployment scope key 记忆化（同 taskOfficeFor 模式）。 */
export function notificationInboxFor(activeRuntime: MobileRuntime, origin: string, tenantId: string): NotificationInbox {
  return cachePut(notificationInboxes, deploymentScopeKey(origin, tenantId), () => createNotificationInbox({
    remote: createMobileInboxRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
    lease: () => activeRuntime.scopeLease(),
  }));
}

/** 通知点击的唯一安全入口（#41 AC1/AC2）：安全深链解析 → 重新鉴权检查（fail closed）→
 * 导航 /tasks/detail（权威 Task 同步由 #35 的 TaskOffice.open + snapshot 水合承担）→
 * 本地 markRead（已读是投影，不是业务操作）。绝不执行通知描述的业务操作。 */
export async function openNotificationFromInbox(
  inbox: Pick<NotificationInbox, 'resolveTarget' | 'markRead'>,
  snapshot: Pick<RuntimeSnapshot, 'surface'>,
  item: Pick<InboxItem, 'notificationId' | 'deepLink'>,
  push: (path: string, params?: Record<string, string>) => void,
): Promise<'navigated' | 'blocked-unauthorized' | 'invalid-link'> {
  const target = inbox.resolveTarget(item); // 实现读 deepLink——参数类型与之对齐（B3-F29）
  if (target === undefined) return 'invalid-link';
  if (snapshot.surface !== 'authorized') return 'blocked-unauthorized';
  push('/tasks/detail', { taskId: target.taskId, runId: target.runId });
  await inbox.markRead(item.notificationId);
  return 'navigated';
}

/** 设备注册入口（spec §4「注册设备与 App 前后台生命周期」）：无原生 push token 或无安全
 * 设备身份时 fail closed 跳过；注册失败不阻塞授权主流程（best effort）。 */
export async function registerActiveDeviceIfPossible(
  activeRuntime: Pick<MobileRuntime, 'snapshot' | 'authorizedRequest' | 'scopeLease'>,
  tokenSource: { token(): Promise<string | undefined> } = createNativePushTokenIfAvailable(),
  identity: { deviceId(): Promise<string | undefined> } = createNativeDeviceIdentity(),
): Promise<'registered' | 'no-token' | 'no-device-id' | 'unauthorized' | 'failed'> {
  const snapshot = activeRuntime.snapshot();
  const origin = snapshot.deployment?.origin;
  if (snapshot.surface !== 'authorized' || origin === undefined) return 'unauthorized';
  const token = await tokenSource.token();
  if (token === undefined) return 'no-token';
  const deviceId = await identity.deviceId();
  if (deviceId === undefined) return 'no-device-id';
  try {
    await deviceRegistryFor(activeRuntime, origin, snapshot.identity?.activeTenantId ?? '').register({
      deviceId,
      token,
      platform: nativeDevicePlatform(),
    });
    return 'registered';
  } catch {
    return 'failed';
  }
}

/** /tasks 应用根：授权面才渲染列表屏，其余面回到 Runtime 裁决的 Surface。 */
export function MobileTasks({ onOpenTask, onOpenLegacy }: { onOpenTask?: (taskId: string, runId: string) => void; onOpenLegacy?: () => void } = {}) {
  const activeRuntime = runtime();
  const snapshot = useSyncExternalStore(activeRuntime.subscribe, activeRuntime.snapshot, activeRuntime.snapshot);
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return null;
  return createElement(TasksScreen, {
    key: deploymentScopeKey(snapshot.deployment.origin, snapshot.identity.activeTenantId ?? ''),
    taskOffice: taskOfficeFor(activeRuntime, snapshot.deployment.origin, snapshot.identity.activeTenantId ?? ''),
    ...(onOpenTask === undefined ? {} : { onOpenTask: (card: { taskId: string; runId: string }) => onOpenTask(card.taskId, card.runId) }),
    ...(onOpenLegacy === undefined ? {} : { onOpenLegacy }),
  });
}

/** 详情路由经此取当前授权 scope 的 Task Office（无授权面返回 undefined）。 */
export function activeTaskOffice(): TaskOffice | undefined {
  const activeRuntime = runtime();
  const snapshot = activeRuntime.snapshot();
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return undefined;
  return taskOfficeFor(activeRuntime, snapshot.deployment.origin, snapshot.identity.activeTenantId ?? '');
}

const taskMaterials = new Map<string, TaskMaterial>();

/** Task Material 按 deployment scope key 记忆化；lease 由 Runtime 提供，切租户即 fail closed（module-seams §7）。 */
function taskMaterialFor(activeRuntime: MobileRuntime, origin: string, tenantId: string): TaskMaterial {
  return cachePut(taskMaterials, deploymentScopeKey(origin, tenantId), () => {
    const remote = createMobileMaterialRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) });
    return createTaskMaterial({ remote, blob: createFetchBlobAdapter(), share: createNativeSharePortIfAvailable() });
  });
}

/** 详情/材料路由经此取当前授权 scope 的 Task Material（无授权面返回 undefined）。 */
export function activeTaskMaterial(): TaskMaterial | undefined {
  const activeRuntime = runtime();
  const snapshot = activeRuntime.snapshot();
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return undefined;
  return taskMaterialFor(activeRuntime, snapshot.deployment.origin, snapshot.identity.activeTenantId ?? '');
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
      taskOffice: taskOfficeFor(runtime(), snapshot.deployment.origin, snapshot.identity.activeTenantId),
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
  const registeredFor = useRef(new Set<string>());
  const registrationAttempts = useRef(new Map<string, number>());
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  useEffect(() => {
    bootRuntimeOnce(activeRuntime, booted);
  }, [activeRuntime]);
  // T11（#41）：进入授权面后 best-effort 注册设备（每个 origin 一次；无 push token 时 fail closed 静默跳过）
  useEffect(() => {
    const unsubscribe = activeRuntime.subscribe((next) => {
      if (next.surface !== 'authorized' || !next.deployment) return;
      if (registeredFor.current.has(next.deployment.origin)) return;
      // 有界重试（B3-F28）：no-token/failed 不永久占用 origin——iOS 首次安装未授
      // 通知权限时，会话内还有重试路径；成功才标记 registeredFor。
      const attempts = registrationAttempts.current.get(next.deployment.origin) ?? 0;
      if (attempts >= 2) return;
      registrationAttempts.current.set(next.deployment.origin, attempts + 1);
      void registerActiveDeviceIfPossible(activeRuntime)
        .then((outcome) => {
          if (outcome === 'registered') registeredFor.current.add(next.deployment!.origin);
        })
        .catch(() => undefined); // 失败：surface 下次变化再试（直至会话上限）
    });
    return unsubscribe;
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
