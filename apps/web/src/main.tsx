import { createRoot } from 'react-dom/client';
import { RouterProvider } from '@tanstack/react-router';
import { createRefreshCoordinator, createWeKnoraClient, type AuthSession, type Credential } from '@weknora/api-client';
import { isLocale, loadingLabel, type Locale } from '@weknora/i18n/runtime';
import { parseOIDCCallbackHash } from './auth/oidc.ts';
import { reloginAfterRefreshFailure } from './auth/relogin.ts';
import { computeAuthLanding } from './auth/session-persist.ts';
import { readPendingInviteToken, clearPendingInviteToken } from './auth/invite-flow.ts';
import { importLegacyPlatformState, persistSelectedTenant, readReactPlatformState, type ReactPlatformState } from './platform/legacy-session.ts';
import { createBrowserTransport } from './platform/http.ts';
import { readStoredLocale } from './i18n.ts';
import { createBrowserCredentialAdapter, persistBrowserCredential } from './platform/credentials.ts';
import { initTheme } from './theme.ts';
import { createWebScopeRuntime } from './platform/scope-runtime.ts';
import { installNavigationObserver } from './platform/navigation.ts';
import { resolveRoute } from './routes.tsx';
import { createWeKnoraRouter } from './router.tsx';
import './styles.css';

const oidcCallback = parseOIDCCallbackHash(window.location.hash);
let initialLoginError: string | undefined;
if (oidcCallback?.kind === 'success') {
  persistBrowserCredential(window.localStorage, { kind: 'bearer', accessToken: oidcCallback.session.token, refreshToken: oidcCallback.session.refreshToken });
  // Vue App.vue redeems a pending invite token after the OIDC round-trip.
  const pendingInvite = readPendingInviteToken(window.sessionStorage);
  const target = pendingInvite
    ? `/login?token=${encodeURIComponent(pendingInvite)}`
    : '/platform/knowledge-bases';
  window.history.replaceState({}, document.title, target);
} else if (oidcCallback?.kind === 'error') {
  initialLoginError = oidcCallback.message;
  window.history.replaceState({}, document.title, '/login');
}

const development = (import.meta as ImportMeta & { env?: { DEV?: boolean } }).env?.DEV ?? false;
const loadingLocale: Locale = isLocale(navigator.language) ? navigator.language : 'en-US';
const loadingText = loadingLabel(loadingLocale);
// The embed entry keeps an anonymous session and skips the bearer credential
// adapter; every other entry boots the legacy platform session import.
const currentRoute = resolveRoute(`${window.location.pathname}${window.location.search}`, { development });
const importedPlatformState = currentRoute.kind === 'embed' ? null : importLegacyPlatformState(window.localStorage);
let session: ReactPlatformState = currentRoute.kind === 'embed'
  ? { credential: { kind: 'anonymous' }, tenantId: null, preferences: {} }
  : importedPlatformState!;
const browserCredentialAdapter = currentRoute.kind === 'embed' ? undefined : createBrowserCredentialAdapter(window.localStorage);
const currentCredential = (): Credential => currentRoute.kind === 'embed'
  ? session.credential
  : readReactPlatformState(window.localStorage)?.credential ?? session.credential;
const injectedApiBaseUrl = (window as Window & { __WEKNORA_API_BASE__?: unknown }).__WEKNORA_API_BASE__;
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || (typeof injectedApiBaseUrl === 'string' ? injectedApiBaseUrl : '');
const liteMode = importedPlatformState?.preferences.weknora_lite_mode === 'true';
const scopeRuntime = createWebScopeRuntime(apiBaseUrl || window.location.origin, null, session.tenantId, {
  liteMode,
  persistTenant: (tenantId) => persistSelectedTenant(window.localStorage, tenantId),
});
const scopeController = scopeRuntime.controller;

let client: ReturnType<typeof createWeKnoraClient>;
const refreshCoordinator = browserCredentialAdapter ? createRefreshCoordinator({
  credentials: browserCredentialAdapter,
  refresh: (refreshToken) => client.auth.refresh(refreshToken),
}) : undefined;

client = createWeKnoraClient({
  baseURL: apiBaseUrl,
  transport: createBrowserTransport({
    credential: currentCredential,
    tenantId: () => scopeController.current().scope.tenantId,
    // Accept-Language resolves per request from the app locale convention
    // (Vue request.ts:86 getCurrentLanguage) — never navigator.language,
    // which pinned an English UI language inside a Chinese deployment.
    locale: () => readStoredLocale(),
    shouldRefresh: (request) => !request.url.endsWith('/api/v1/auth/refresh'),
    refresh: refreshCoordinator ? async () => {
      try {
        await refreshCoordinator.refresh();
      } catch (error) {
        // Vue authRefresh.ts:104-150 — a failed refresh leaves the session
        // cleared and the user on /login, never a silently poisoned session
        // (S00 negpath batch 3, T-3). The coordinator has already cleared the
        // bearer credential; reset the scope/session state and navigate.
        await reloginAfterRefreshFailure({
          clearSession: () => {
            scopeRuntime.logout();
            session = { ...session, credential: { kind: 'anonymous' }, tenantId: null };
          },
          pathname: window.location.pathname,
          assign: (url) => window.location.assign(url),
        });
        throw error;
      }
    } : undefined,
  }),
});

// Vue useTheme.initTheme parity: apply the stored theme on startup and
// re-apply on weknora:theme-changed / OS scheme changes (theme.ts).
// The embed entry forces light mode instead (frontend/embed.html sets
// theme-mode="light" before the app boots).
if (currentRoute.kind !== 'embed') initTheme();

const root = createRoot(document.getElementById('root')!);

function completeAuthentication(next: AuthSession): void {
  // Vue parity: no return-URL parameter — post-auth landing is the default
  // home, or onboarding when the session has no tenant yet.
  const landing = computeAuthLanding(next);
  session = { ...session, credential: { kind: 'bearer', accessToken: next.token, refreshToken: next.refreshToken }, tenantId: landing.activeTenantId };
  persistBrowserCredential(window.localStorage, session.credential);
  // Vue Login.vue:584-590 — apply the active-tenant override when the server
  // dropped us into a non-home tenant so X-Tenant-ID stays consistent.
  if (landing.activeTenantId) scopeRuntime.setTenant(landing.activeTenantId);
  window.location.assign(landing.target);
}

async function logout(): Promise<void> {
  try { await client.auth.logout(); } catch { /* local invalidation still wins */ }
  await browserCredentialAdapter?.clear();
  scopeRuntime.logout();
  session = { ...session, credential: { kind: 'anonymous' }, tenantId: null };
  window.location.assign('/login');
}

async function switchTenantFromShell(tenantId: string): Promise<void> {
  const credential = currentCredential();
  let switchedCredential: Credential | null = null;
  const next = await scopeRuntime.switchTenant(
    tenantId,
    (nextTenantId, refreshToken) => client.auth.switchTenant(nextTenantId, refreshToken),
    (authSession) => {
      switchedCredential = { kind: 'bearer', accessToken: authSession.token, refreshToken: authSession.refreshToken };
      persistBrowserCredential(window.localStorage, switchedCredential);
    },
    credential.kind === 'bearer' ? credential.refreshToken : undefined,
  );
  session = { ...session, credential: switchedCredential ?? credential, tenantId: next.scope.tenantId };
  window.location.assign('/platform/knowledge-bases');
}

// auth/me + scope hydration happens once per boot, before the first protected
// page renders; a failed restore clears the session like the pre-router
// bootstrap did (the login card then renders with the restore error).
let sessionHydration: { ok: true; tenantId: string | null } | { ok: false } | null = null;
let sessionRestoreError: string | undefined;
async function ensureSessionHydrated(): Promise<{ ok: true; tenantId: string | null } | { ok: false }> {
  if (sessionHydration !== null) return sessionHydration;
  if (session.credential.kind !== 'bearer') {
    sessionHydration = { ok: false };
    return sessionHydration;
  }
  try {
    const authMe = await client.auth.me();
    const hydrated = scopeRuntime.hydrate(authMe);
    session.tenantId = hydrated.scope.tenantId;
    sessionHydration = { ok: true, tenantId: hydrated.scope.tenantId };
  } catch (error) {
    scopeRuntime.logout();
    await browserCredentialAdapter?.clear();
    session = { ...session, credential: { kind: 'anonymous' }, tenantId: null };
    sessionRestoreError = error instanceof Error ? error.message : 'Your session could not be restored.';
    sessionHydration = { ok: false };
  }
  return sessionHydration;
}

// Route changes stay inside the mounted React tree: every URL mutation
// (anchor clicks, page search-param writes, guard SPA replaces) lands in the
// browser history and the navigation observer re-dispatches it as a popstate,
// so the TanStack router re-matches and re-renders — the same notify →
// re-render contract the pre-router app used. Craft and the chat page keep
// their own internal URL state machines (excluded in navigation.ts).
installNavigationObserver();
const router = createWeKnoraRouter({
  client,
  scopeController,
  scopeRuntime,
  session: () => session,
  liteMode,
  development,
  apiBaseUrl,
  loadingText,
  initialLoginError: () => initialLoginError ?? sessionRestoreError,
  ensureSessionHydrated,
  logout,
  switchTenantFromShell,
  completeAuthentication,
});

root.render(<RouterProvider router={router} />);
