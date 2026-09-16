import { createRoot } from 'react-dom/client';
import { lazy, Suspense, useEffect, useState, type ReactNode } from 'react';
import { createRefreshCoordinator, createWeKnoraClient, type AuthSession, type Credential } from '@weknora/api-client';
import { Status } from '@weknora/ui';
import { isLocale, loadingLabel, type Locale } from '@weknora/i18n/runtime';
import { parseOIDCCallbackHash } from './auth/oidc.ts';
import { reloginAfterRefreshFailure } from './auth/relogin.ts';
import { computeAuthLanding } from './auth/session-persist.ts';
import { readPendingInviteToken, clearPendingInviteToken } from './auth/invite-flow.ts';
import { importLegacyPlatformState, persistSelectedTenant, readReactPlatformState, type ReactPlatformState } from './platform/legacy-session.ts';
import { createBrowserTransport } from './platform/http.ts';
import { createBrowserCredentialAdapter, persistBrowserCredential } from './platform/credentials.ts';
import { initTheme } from './theme.ts';
import { createWebScopeRuntime } from './platform/scope-runtime.ts';
import { createWebPlatformAdapters } from './platform/adapters.ts';
import { installNavigationObserver, navigate, subscribeNavigation } from './platform/navigation.ts';
import { guardRoute, organizationInviteCode, protectedPageForRoute, resolveRoute, routeRedirect } from './routes.tsx';
import { shouldOpenWiki, wikiEntryPath } from './knowledge/wiki-route.ts';
import { CraftRoutes } from './features/craft/routes.tsx';
const ChatRoutePage = lazy(() => import('./chat/ChatRoutePage.tsx').then((module) => ({ default: module.ChatRoutePage })));
const IntegrationsRoutePage = lazy(() => import('./integrations/IntegrationsRoutePage.tsx').then((module) => ({ default: module.IntegrationsRoutePage })));
const KnowledgeDocumentsPage = lazy(() => import('./documents/KnowledgeDocumentsPage.tsx').then((module) => ({ default: module.KnowledgeDocumentsPage })));
const KnowledgeDocumentDetailPage = lazy(() => import('./documents/KnowledgeDocumentDetailPage.tsx').then((module) => ({ default: module.KnowledgeDocumentDetailPage })));
const WikiPage = lazy(() => import('./wiki/WikiPage.tsx').then((module) => ({ default: module.WikiPage })));
const FAQPage = lazy(() => import('./faq/FAQPage.tsx').then((module) => ({ default: module.FAQPage })));
const DataSourcesPage = lazy(() => import('./data-sources/DataSourcesPage.tsx').then((module) => ({ default: module.DataSourcesPage })));
const KnowledgeSettingsPage = lazy(() => import('./knowledge-settings/KnowledgeSettingsPage.tsx').then((module) => ({ default: module.KnowledgeSettingsPage })));
const ConfigurationPage = lazy(() => import('./configuration/ConfigurationPage.tsx').then((module) => ({ default: module.ConfigurationPage })));
const AgentsPage = lazy(() => import('./agents/AgentsPage.tsx').then((module) => ({ default: module.AgentsPage })));
const AdministrationPage = lazy(() => import('./administration/AdministrationPage.tsx').then((module) => ({ default: module.AdministrationPage })));
const OrganizationsPage = lazy(() => import('./organizations/OrganizationsPage.tsx').then((module) => ({ default: module.OrganizationsPage })));
const SettingsPage = lazy(() => import('./settings/SettingsPage.tsx').then((module) => ({ default: module.SettingsPage })));
const KnowledgeGraphPage = lazy(() => import('./knowledge/KnowledgeGraphPage.tsx').then((module) => ({ default: module.KnowledgeGraphPage })));
const KnowledgeBasesPage = lazy(() => import('./App.tsx').then((module) => ({ default: module.KnowledgeBasesPage })));
const NotFoundPage = lazy(() => import('./NotFoundPage.tsx').then((module) => ({ default: module.NotFoundPage })));
const DevMarkdownPage = lazy(() => import('./DevMarkdownPage.tsx').then((module) => ({ default: module.DevMarkdownPage })));
const PlatformShell = lazy(() => import('./platform/PlatformShell.tsx').then((module) => ({ default: module.PlatformShell })));
const LoginPage = lazy(() => import('./auth/LoginPage.tsx').then((module) => ({ default: module.LoginPage })));
const JoinPage = lazy(() => import('./auth/JoinPage.tsx').then((module) => ({ default: module.JoinPage })));
const WorkspaceOnboardingPage = lazy(() => import('./auth/WorkspaceOnboardingPage.tsx').then((module) => ({ default: module.WorkspaceOnboardingPage })));
const AppsPage = lazy(() => import('./apps/AppsPages.tsx').then((module) => ({ default: module.AppsPage })));

function WikiEntry({ client, knowledgeBaseId, initialSlug, initialDocumentId, canContribute }: { client: ReturnType<typeof createWeKnoraClient>; knowledgeBaseId: string; initialSlug?: string; initialDocumentId?: string; canContribute: boolean }) {
  const [wikiEnabled, setWikiEnabled] = useState<boolean | null>(null);
  useEffect(() => {
    let active = true;
    void client.knowledgeBases.settings.get(knowledgeBaseId).then((kb) => {
      if (active) setWikiEnabled(shouldOpenWiki(kb));
    }).catch(() => {
      // Preserve the existing Wiki error surface when capability lookup is unavailable.
      if (active) setWikiEnabled(true);
    });
    return () => { active = false; };
  }, [client, knowledgeBaseId]);
  if (wikiEnabled === false) {
    window.history.replaceState({}, document.title, wikiEntryPath(knowledgeBaseId));
    return <KnowledgeDocumentsPage client={client} knowledgeBaseId={knowledgeBaseId} initialDocumentId={initialDocumentId} onOpenDocument={(document) => navigate(`/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}/documents/${encodeURIComponent(document.id)}`)} />;
  }
  if (wikiEnabled === null) return <Status tone="neutral">加载中…</Status>;
  return <WikiPage client={client} knowledgeBaseId={knowledgeBaseId} initialSlug={initialSlug} canContribute={canContribute} />;
}
import './styles.css';

const oidcCallback = parseOIDCCallbackHash(window.location.hash);
let initialLoginError: string | undefined;
if (oidcCallback?.kind === 'success') {
  persistBrowserCredential(window.localStorage, { kind: 'bearer', accessToken: oidcCallback.session.token, refreshToken: oidcCallback.session.refreshToken });
  // Vue App.vue redeems a pending invite token after the OIDC round-trip and
  // honours ?next; keep the query so bootstrap can apply both.
  const pendingInvite = readPendingInviteToken(window.sessionStorage);
  const nextParam = new URLSearchParams(window.location.search).get('next');
  const target = pendingInvite
    ? `/login?token=${encodeURIComponent(pendingInvite)}`
    : nextParam && nextParam.startsWith('/') && !nextParam.startsWith('//') ? nextParam : '/platform/knowledge-bases';
  window.history.replaceState({}, document.title, target);
} else if (oidcCallback?.kind === 'error') {
  initialLoginError = oidcCallback.message;
  window.history.replaceState({}, document.title, '/login');
}

const development = (import.meta as ImportMeta & { env?: { DEV?: boolean } }).env?.DEV ?? false;
const loadingLocale: Locale = isLocale(navigator.language) ? navigator.language : 'en-US';
const loadingText = loadingLabel(loadingLocale);
const embedEntryError = ({ 'zh-CN': 'Embed 必须使用独立入口。', 'en-US': 'Embed must use its isolated entrypoint.', 'ja-JP': 'Embed は専用エントリーポイントを使用してください。', 'ko-KR': 'Embed는 전용 진입점을 사용해야 합니다.', 'ru-RU': 'Embed должен использовать изолированную точку входа.' } as Record<Locale, string>)[loadingLocale];
let currentRoute = resolveRoute(`${window.location.pathname}${window.location.search}`, { development });
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
    locale: navigator.language,
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

const root = createRoot(document.getElementById('root')!);
// Vue useTheme.initTheme parity: apply the stored theme on startup and
// re-apply on weknora:theme-changed / OS scheme changes (theme.ts).
initTheme();
const platformAdapters = createWebPlatformAdapters();

// Route changes stay inside the mounted React tree. PlatformShell keeps its
// identity while the route-specific content is reconciled from the new URL.
installNavigationObserver();
subscribeNavigation(() => {
  currentRoute = resolveRoute(`${window.location.pathname}${window.location.search}`, { development });
  if (session.credential.kind === 'bearer') renderProtected();
});

function nextPathAfterAuth(): string {
  const next = new URLSearchParams(window.location.search).get('next');
  return next && next.startsWith('/') && !next.startsWith('//') ? next : '/platform/knowledge-bases';
}

function renderAuth(page: ReactNode): void {
  root.render(<Suspense fallback={<main className="wk-page mx-auto box-border max-w-[960px] px-5 py-12"><Status>{loadingText}</Status></main>}>{page}</Suspense>);
}

function completeAuthentication(next: AuthSession): void {
  const landing = computeAuthLanding(next, nextPathAfterAuth());
  session = { ...session, credential: { kind: 'bearer', accessToken: next.token, refreshToken: next.refreshToken }, tenantId: landing.activeTenantId };
  persistBrowserCredential(window.localStorage, session.credential);
  // Vue Login.vue:584-590 — apply the active-tenant override when the server
  // dropped us into a non-home tenant so X-Tenant-ID stays consistent.
  if (landing.activeTenantId) scopeRuntime.setTenant(landing.activeTenantId);
  window.location.assign(landing.target);
}

function renderLogin(error = initialLoginError, inviteToken = '') {
  renderAuth(<LoginPage client={client} onAuthenticated={completeAuthentication} apiBaseUrl={apiBaseUrl} initialError={error} initialMode={currentRoute.kind === 'login' ? currentRoute.mode : 'login'} inviteToken={inviteToken} onInviteAccepted={() => window.location.assign('/platform/knowledge-bases')} />);
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

// All protected /platform/* pages render inside the platform shell
// (sidebar matching the Vue menu.vue). Auth/onboarding/embed pages stay bare.
function renderShell(page: ReactNode): void {
  root.render(<Suspense fallback={<main className="wk-page mx-auto box-border max-w-[960px] px-5 py-12"><Status>{loadingText}</Status></main>}><PlatformShell client={client} onLogout={logout} onTenantSwitch={switchTenantFromShell}>{page}</PlatformShell></Suspense>);
}

function renderProtected() {
  const route = currentRoute;
  const pathname = `${window.location.pathname}${window.location.search}`;
  const current = scopeRuntime.current().scope;
  const decision = guardRoute(pathname, {
    authenticated: session.credential.kind === 'bearer',
    tenantId: current.tenantId,
    capabilities: scopeRuntime.capabilities(),
    isSystemAdmin: scopeRuntime.isSystemAdmin(),
    liteMode,
    development,
  });
  if (decision.kind === 'redirect') {
    if (decision.to === '/onboarding/workspace') {
      if (window.location.pathname !== decision.to) platformAdapters.replace(decision.to);
      renderAuth(<WorkspaceOnboardingPage client={client} scopeRuntime={scopeRuntime} onLogout={logout} />);
      return;
    }
    if (decision.reason === 'authentication-required') {
      platformAdapters.replace(decision.to);
      renderLogin();
      return;
    }
    window.location.replace(decision.to);
    return;
  }
  if (route.kind === 'craft') {
    // W05 craft mounts at /craft and /craft/:sessionId through the same
    // assembly (legacy session + shared scope + shared client) — no second
    // router. Craft stays reachable without an interactive session, matching
    // the pre-merge guard behaviour.
    root.render(<CraftRoutes client={client} scopeController={scopeController} session={session} apiBaseUrl={apiBaseUrl} />);
    return;
  }
  if (route.kind === 'onboarding') {
    renderAuth(<WorkspaceOnboardingPage client={client} scopeRuntime={scopeRuntime} onLogout={logout} />);
    return;
  }
  if (protectedPageForRoute(route) === 'knowledge-bases') {
    renderShell(<KnowledgeBasesPage client={client} scopeController={scopeController} />);
  } else if (protectedPageForRoute(route) === 'markdown-test') {
    renderShell(<DevMarkdownPage />);
  } else if (route.kind === 'knowledge-document') {
    renderShell(<KnowledgeDocumentDetailPage client={client} documentId={route.documentId} onBack={() => navigate(`/knowledgeBase/${encodeURIComponent(route.knowledgeBaseId)}`)} />);
  } else if (route.kind === 'knowledge-wiki') {
    renderShell(<WikiEntry client={client} knowledgeBaseId={route.knowledgeBaseId} initialDocumentId={new URLSearchParams(window.location.search).get('knowledge_id')?.trim() || undefined} canContribute={scopeRuntime.role() !== 'viewer'} />);
  } else if (route.kind === 'knowledge-faq') {
    renderShell(<FAQPage client={client} knowledgeBaseId={route.knowledgeBaseId} />);
  } else if (route.kind === 'knowledge-settings') {
    renderShell(<KnowledgeSettingsPage client={client} knowledgeBaseId={route.knowledgeBaseId} role={scopeRuntime.role() === 'owner' ? 'owner' : scopeRuntime.role() === 'admin' ? 'admin' : 'viewer'} />);
  } else if (route.path === '/platform/agents') {
    // Real agents list (parity with Vue AgentList.vue); the consolidated
    // configuration surface stays reachable at /platform/configuration.
    renderShell(<AgentsPage client={client} tenantId={scopeRuntime.current().scope.tenantId} />);
  } else if (route.path === '/platform/configuration') {
    renderShell(<ConfigurationPage client={client} />);
  } else if (route.path === '/platform/administration') {
    renderShell(<AdministrationPage client={client} tenantId={Number(scopeRuntime.current().scope.tenantId)} />);
  } else if (route.path === '/platform/organizations') {
    renderShell(<OrganizationsPage client={client} inviteCode={organizationInviteCode(pathname)} role={scopeRuntime.role()} />);
  } else if (route.path === '/platform/settings') {
    renderShell(<SettingsPage capabilities={scopeRuntime.capabilities()} liteMode={liteMode} client={client} tenantId={Number(scopeRuntime.current().scope.tenantId)} role={scopeRuntime.role() === 'owner' ? 'owner' : scopeRuntime.role() === 'admin' ? 'admin' : 'viewer'} />);
  } else if (route.path === '/platform/system') {
    renderShell(<AdministrationPage client={client} tenantId={Number(scopeRuntime.current().scope.tenantId)} systemAdmin />);
  } else if (route.kind === 'apps') {
    renderShell(<AppsPage client={client} mode={route.mode} id={route.id} role={scopeRuntime.role()} />);
  } else if (route.kind === 'knowledge-base' && route.knowledgeBaseId) {
    const knowledgeBaseId = route.knowledgeBaseId;
    const initialDocumentId = route.initialDocumentId;
    if (route.tab === 'wiki') renderShell(<WikiEntry client={client} knowledgeBaseId={knowledgeBaseId} initialSlug={route.slug} initialDocumentId={initialDocumentId} canContribute={scopeRuntime.role() !== 'viewer'} />);
    else if (route.tab === 'graph') renderShell(<KnowledgeGraphPage client={client} knowledgeBaseId={knowledgeBaseId} slug={route.slug} />);
    else renderShell(<KnowledgeDocumentsPage client={client} knowledgeBaseId={knowledgeBaseId} initialDocumentId={initialDocumentId} onOpenDocument={(document) => navigate(`/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}/documents/${encodeURIComponent(document.id)}`)} />);
  } else if (route.kind === 'chat' || route.path === '/platform/creatChat' || route.path.startsWith('/platform/chat/')) {
    renderShell(<ChatRoutePage client={client} scopeController={scopeController} apiBaseUrl={apiBaseUrl} knowledgeBaseId={route.kind === 'chat' ? route.knowledgeBaseId : undefined} canViewChannelSessions={scopeRuntime.canViewChannelSessions()} />);
  } else if (route.path === '/platform/integrations') {
    const integrationQuery = new URLSearchParams(window.location.search);
    renderShell(<IntegrationsRoutePage client={client} tenantId={scopeRuntime.current().scope.tenantId} activeAgentId={(integrationQuery.get('agentId') ?? integrationQuery.get('agent_id'))?.trim() || null} apiBaseUrl={apiBaseUrl} />);
  } else if (route.kind === 'not-found') {
    renderShell(<NotFoundPage path={route.path} />);
  } else {
    renderShell(<NotFoundPage path={route.path} />);
  }
}

async function bootstrap() {
  if (currentRoute.kind === 'embed') {
    root.render(<main className="wk-page mx-auto box-border max-w-[960px] px-[1.25rem] py-12"><Status tone="error">{embedEntryError}</Status></main>);
    return;
  }
  if (currentRoute.kind === 'login') {
    const inviteToken = new URLSearchParams(window.location.search).get('token')?.trim() ?? '';
    if (inviteToken) {
      // Vue Login.vue:798-801 — an existing session redeems the token directly.
      if (session.credential.kind === 'bearer') {
        try {
          await client.auth.acceptInvitationByToken(inviteToken);
          clearPendingInviteToken(window.sessionStorage);
        } catch { /* Vue acceptAndEnter: an invalid token still enters the app */ }
        window.location.assign('/platform/knowledge-bases');
        return;
      }
      // Vue Login.vue:803-808 — invite_only stays on the login card; open
      // deployments render the registration form.
      let registrationMode = 'self_serve';
      try { registrationMode = (await client.auth.registrationConfig()).registrationMode; } catch { /* fail open like loadAuthConfig */ }
      if (registrationMode === 'invite_only') {
        renderLogin(undefined, inviteToken);
      } else {
        renderAuth(<JoinPage client={client} onAuthenticated={completeAuthentication} />);
      }
      return;
    }
    // Vue router.beforeEach redirects an already-authenticated visitor away
    // from /login. Keep the same public-entry behavior in React: validate the
    // imported session before choosing the tenantless onboarding landing.
    if (session.credential.kind === 'bearer') {
      try {
        const authMe = await client.auth.me();
        const hydrated = scopeRuntime.hydrate(authMe);
        session.tenantId = hydrated.scope.tenantId;
        window.location.assign(hydrated.scope.tenantId ? '/platform/knowledge-bases' : '/onboarding/workspace');
        return;
      } catch {
        scopeRuntime.logout();
        await browserCredentialAdapter?.clear();
        session = { ...session, credential: { kind: 'anonymous' }, tenantId: null };
      }
    }
    // Vue Login.vue:817-831 — lite-edition transparent auto-setup on /login.
    const AUTO_SETUP_FAILED_KEY = 'weknora_auto_setup_failed';
    if (window.localStorage.getItem(AUTO_SETUP_FAILED_KEY) !== 'true') {
      try {
        const autoSession = await client.auth.autoSetup();
        window.localStorage.setItem('weknora_lite_mode', 'true');
        completeAuthentication(autoSession);
        return;
      } catch {
        window.localStorage.setItem(AUTO_SETUP_FAILED_KEY, 'true');
      }
    }
    renderLogin();
    return;
  }
  if (currentRoute.kind === 'join') {
    const redirect = routeRedirect(`${window.location.pathname}${window.location.search}`);
    const joinToken = new URLSearchParams(window.location.search).get('token')?.trim();
    if (joinToken) {
      // Vue share-links land on /login|/register?token — never dead-end /join.
      window.location.replace(`/register?token=${encodeURIComponent(joinToken)}`);
    } else if (session.credential.kind !== 'bearer') {
      window.location.replace(`/login?next=${encodeURIComponent(`${window.location.pathname}${window.location.search}`)}`);
    } else if (redirect) window.location.replace(redirect);
    return;
  }
  if (session.credential.kind !== 'bearer') {
    renderProtected();
    return;
  }
  try {
    const authMe = await client.auth.me();
    const hydrated = scopeRuntime.hydrate(authMe);
    session.tenantId = hydrated.scope.tenantId;
    renderProtected();
  } catch (error) {
    scopeRuntime.logout();
    await browserCredentialAdapter?.clear();
    renderLogin(error instanceof Error ? error.message : 'Your session could not be restored.');
  }
}

void bootstrap();
