import { createElement, useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileResourceRemote } from '@weknora/api-client/mobile/resources';
import { createJsonTransport } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createMobileRuntime } from '@weknora/mobile-core';
import { createScopedVault, createWebCryptoCipher } from '@weknora/mobile-core';
import type { MobileRuntime, RuntimeSnapshot, ScopedVault, Deployment } from '@weknora/mobile-core';
import { createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createNativeOidcBrowser } from './adapters/oidc-browser.ts';
import { createNativeSecurePendingOidcStore } from './adapters/secure-store.ts';
import type { SecureStorePort } from './adapters/secure-store.ts';
import { createSecureVaultKeyStore, createSecureVaultStorage } from './adapters/vault-adapters.ts';
import { createNativeSecureCredentialStore } from './adapters/credential-store.ts';
import { createNativeSecureDeploymentStore } from './adapters/deployment-store.ts';
import { createNativeSecureDeploymentRegistry } from './adapters/deployment-registry.ts';
import { HomeScreen } from './screens/HomeScreen.tsx';
import { TasksScreen } from './screens/TasksScreen.tsx';
import { DeploymentLoginScreen, validatedDeploymentOrigin } from './screens/DeploymentLoginScreen.tsx';
import { UpgradeRequiredScreen } from './screens/UpgradeRequiredScreen.tsx';

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
    scopedVault: createNativeScopedVaultIfAvailable(),
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(nativeFetch) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(nativeFetch) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
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

/** Task Office 按 deployment origin 记忆化；lease 由 Runtime 提供，切租户即 fail closed。 */
function taskOfficeFor(activeRuntime: MobileRuntime, origin: string): TaskOffice {
  let office = taskOffices.get(origin);
  if (!office) {
    office = createTaskOffice({
      backend: createTaskOfficeRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
      lease: () => activeRuntime.scopeLease(),
    });
    taskOffices.set(origin, office);
  }
  return office;
}

/** /tasks 应用根：授权面才渲染列表屏，其余面回到 Runtime 裁决的 Surface。 */
export function MobileTasks() {
  const activeRuntime = runtime();
  const snapshot = useSyncExternalStore(activeRuntime.subscribe, activeRuntime.snapshot, activeRuntime.snapshot);
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return null;
  return createElement(TasksScreen, { key: snapshot.identity.activeTenantId, taskOffice: taskOfficeFor(activeRuntime, snapshot.deployment.origin) });
}

/** Selects a visible surface only from the presentation-safe Runtime snapshot. */
export function RuntimeSurface({ snapshot, deployments, onSignIn, onBeginOidc, onSignOut, onActivateTenant, onSwitchDeployment }: RuntimeSurfaceProps) {
  if (snapshot.surface === 'authorized' && snapshot.deployment && snapshot.identity?.userId && snapshot.identity.activeTenantId) {
    return createElement(HomeScreen, {
      key: snapshot.identity.activeTenantId,
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

/** Native application root; screens receive snapshots and callbacks only. */
export function MobileApp() {
  const activeRuntime = runtime();
  const booted = useRef(false);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  useEffect(() => {
    bootRuntimeOnce(activeRuntime, booted);
  }, [activeRuntime]);
  const snapshot = useSyncExternalStore(activeRuntime.subscribe, activeRuntime.snapshot, activeRuntime.snapshot);
  useEffect(() => {
    void activeRuntime.listDeployments().then(setDeployments);
  }, [activeRuntime, snapshot]);
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
