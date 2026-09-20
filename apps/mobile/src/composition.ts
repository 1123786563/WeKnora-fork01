import { createElement, useEffect, useRef, useSyncExternalStore } from 'react';
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createJsonTransport } from '@weknora/api-client/transport';
import { createMobileRuntime } from '@weknora/mobile-core';
import type { MobileRuntime, RuntimeSnapshot } from '@weknora/mobile-core';
import { createNativeOidcBrowser } from './adapters/oidc-browser.ts';
import { createNativeSecurePendingOidcStore } from './adapters/secure-store.ts';
import { createNativeSecureCredentialStore } from './adapters/credential-store.ts';
import { createNativeSecureDeploymentStore } from './adapters/deployment-store.ts';
import { AuthorizedLandingScreen } from './screens/AuthorizedLandingScreen.tsx';
import { DeploymentLoginScreen, validatedDeploymentOrigin } from './screens/DeploymentLoginScreen.tsx';
import { UpgradeRequiredScreen } from './screens/UpgradeRequiredScreen.tsx';

const OIDC_REDIRECT_URI = 'weknora://oidc';
const cloudFromBuild = typeof process !== 'undefined' ? process.env.EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN : undefined;
export const officialCloudOrigin = cloudFromBuild ? validatedDeploymentOrigin(cloudFromBuild) : undefined;

function nativeFetch(input: string, init?: { method?: string; headers?: Record<string, string>; body?: string | FormData; signal?: AbortSignal }) {
  return fetch(input, init as RequestInit);
}

/** The app composition root is the only place that joins concrete native adapters to Runtime. */
export function createNativeMobileRuntime(): MobileRuntime {
  let runtime!: MobileRuntime;
  const browser = createNativeOidcBrowser(OIDC_REDIRECT_URI);
  runtime = createMobileRuntime({
    credentialStore: createNativeSecureCredentialStore(),
    deploymentStore: createNativeSecureDeploymentStore(),
    clientVersion: 3,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(nativeFetch) });
      return createMobileRuntimeRemote({ origin, request: client.request });
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
}

/** Selects a visible surface only from the presentation-safe Runtime snapshot. */
export function RuntimeSurface({ snapshot, onSignIn, onBeginOidc, onSignOut }: RuntimeSurfaceProps) {
  if (snapshot.surface === 'authorized' && snapshot.deployment && snapshot.identity?.userId && snapshot.identity.activeTenantId) {
    return createElement(AuthorizedLandingScreen, { deploymentLabel: snapshot.deployment.label, userId: snapshot.identity.userId, tenantId: snapshot.identity.activeTenantId, onSignOut });
  }
  if (snapshot.surface === 'deployment-login') {
    return createElement(DeploymentLoginScreen, { officialCloudOrigin, onSignIn, onBeginOidc });
  }
  return createElement(UpgradeRequiredScreen, { deploymentLabel: snapshot.deployment?.label, reason: snapshot.reason, onSignOut });
}

let nativeRuntime: MobileRuntime | undefined;

function runtime(): MobileRuntime {
  nativeRuntime ??= createNativeMobileRuntime();
  return nativeRuntime;
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
  useEffect(() => {
    bootRuntimeOnce(activeRuntime, booted);
  }, [activeRuntime]);
  const snapshot = useSyncExternalStore(activeRuntime.subscribe, activeRuntime.snapshot, activeRuntime.snapshot);
  return createElement(RuntimeSurface, {
    snapshot,
    onSignIn: async ({ origin, email, password }) => { await activeRuntime.signIn({ deployment: { origin }, email, password }); },
    onBeginOidc: async ({ origin }) => { await activeRuntime.beginOidc({ deployment: { origin }, redirectUri: OIDC_REDIRECT_URI }); },
    onSignOut: () => activeRuntime.signOut(),
  });
}

export function completeNativeOidcCallback(callbackUrl: string): Promise<RuntimeSnapshot> {
  return runtime().completeOidc(callbackUrl);
}
