// TanStack Router assembly for the React web app.
//
// The route tree is built in code (no file-based generation) by a factory so
// tests can mount it against a memory history with fake deps. Behavioural
// parity with the pre-router app is delegated, not reimplemented: guards run
// the existing pure `guardRoute` / `routeRedirect` functions from routes.tsx
// (still unit-tested there), and every protected page renders inside the same
// PlatformShell with the same lazy chunks.
//
// Redirect policy (open-redirect hardening): router redirect() calls only
// ever target the literal in-app SPA destinations, /login and
// /onboarding/workspace. Every other guard target keeps the pre-router
// behaviour — a hard window.location.replace — after an internal-path
// allowlist check (internalTarget). Navigation targets are normalized into
// local constants before any navigation call so the allowlisted value, not a
// raw user-influenced expression, is what reaches the navigator.
//
// Bridge contract with platform/navigation.ts: page-driven URL mutations
// arrive through the navigation sink (router.history.push/replace). Craft and
// the chat page keep their own URL state machines and are excluded from the
// bridge there.
import { lazy, Suspense, useEffect, useState, type ReactNode } from 'react';
import { createRootRoute, createRoute, createRouter, notFound, Outlet, redirect, type RouterHistory } from '@tanstack/react-router';
import { createWeKnoraClient, type AuthSession } from '@weknora/api-client';
import type { ScopeController } from '@weknora/domain/scope';
import { createWebScopeRuntime } from './platform/scope-runtime.ts';
import type { LegacyPlatformSession } from './platform/legacy-session.ts';
import { navigate } from './platform/navigation.ts';
import { guardRoute, organizationInviteCode, type RouteGuardContext, type RouteGuardDecision } from './routes.tsx';
import { shouldOpenWiki, wikiEntryPath } from './knowledge/wiki-route.ts';
import { createWikiSourceDocOpener } from './wiki/source-doc-open.ts';

// Craft mounts through the shared @weknora/views/craft assembly, which pulls
// in @weknora/ui (theme.css) — keep it lazy so the router module stays
// importable under the node test runner.
const CraftRoutes = lazy(() => import('./features/craft/routes.tsx').then((module) => ({ default: module.CraftRoutes })));

const ChatRoutePage = lazy(() => import('./chat/ChatRoutePage.tsx').then((module) => ({ default: module.ChatRoutePage })));
const KnowledgeDocumentsPage = lazy(() => import('./documents/KnowledgeDocumentsPage.tsx').then((module) => ({ default: module.KnowledgeDocumentsPage })));
const KnowledgeDocumentDetailPage = lazy(() => import('./documents/KnowledgeDocumentDetailPage.tsx').then((module) => ({ default: module.KnowledgeDocumentDetailPage })));
const WikiPage = lazy(() => import('./wiki/WikiPage.tsx').then((module) => ({ default: module.WikiPage })));
const FAQPage = lazy(() => import('./faq/FAQPage.tsx').then((module) => ({ default: module.FAQPage })));
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
const EmbedEntryPage = lazy(() => import('./embed/EmbedEntryPage.tsx').then((module) => ({ default: module.EmbedEntryPage })));

export interface WeKnoraRouterDeps {
  client: ReturnType<typeof createWeKnoraClient>;
  scopeController: ScopeController;
  scopeRuntime: ReturnType<typeof createWebScopeRuntime>;
  /** Live getter — logout/switch-tenant reassign the boot session object. */
  session: () => LegacyPlatformSession;
  liteMode: boolean;
  development: boolean;
  apiBaseUrl: string;
  loadingText: string;
  /** Live getter — the OIDC/refresh failure text is set after boot. */
  initialLoginError(): string | undefined;
  /** auth/me + scope hydration, once per boot; clears the session on failure. */
  ensureSessionHydrated(): Promise<{ ok: true; tenantId: string | null } | { ok: false }>;
  logout(): Promise<void>;
  switchTenantFromShell(tenantId: string): Promise<void>;
  completeAuthentication(next: AuthSession): void;
}

// Plain markup instead of the @weknora/ui Status pill: the router module must
// stay importable under the node test runner, which cannot parse the ui
// package's theme.css.
function RoutePending(props: { loadingText: string }): ReactNode {
  return (
    <main className="wk-page mx-auto box-border max-w-[960px] px-5 py-12">
      <p role="status">{props.loadingText}</p>
    </main>
  );
}

function searchOf(href: string): string {
  const index = href.indexOf('?');
  return index === -1 ? '' : href.slice(index + 1);
}

function pathWithQuery(href: string): string {
  const url = new URL(href);
  const search = searchOf(href);
  return url.pathname + (search ? `?${search}` : '');
}

// Redirect targets here are first-party (guardRoute/routeRedirect output), but
// the guard chain funnels user-influenced values in places (invite codes,
// legacy alias maps). internalTarget enforces the internal-path allowlist so a
// future regression cannot turn a guard redirect into an open redirect.
const INTERNAL_TARGET_PREFIXES = ['/platform', '/knowledgeBase', '/login', '/register', '/join', '/onboarding/', '/craft', '/embed/', '/creatChat'];

function internalTarget(path: string, fallback: string): string {
  if (path.startsWith('/') && !path.startsWith('//') && INTERNAL_TARGET_PREFIXES.some((prefix) => path === prefix || path.startsWith(prefix))) return path;
  return fallback;
}

function searchRecord(search: string): Record<string, string> {
  return Object.fromEntries(new URLSearchParams(search));
}

/** Never-settling promise so the superseded navigation does not render its
 * route while window.location.replace takes over. Rejects on abort so the
 * router can clean the load up. */
async function navigationTakesOver(abortSignal: AbortSignal | undefined): Promise<never> {
  await new Promise<never>((_resolve, reject) => {
    if (!abortSignal) return;
    abortSignal.addEventListener('abort', () => reject(abortSignal.reason ?? new Error('redirect aborted')), { once: true });
  });
  throw new Error('unreachable');
}

/**
 * guardRoute decisions keep their pre-router semantics: authentication and
 * workspace redirects were SPA URL replaces (now the two literal redirect()
 * destinations), capability/system-admin redirects were hard
 * window.location.replace calls.
 */
async function applyGuardDecision(decision: Extract<RouteGuardDecision, { kind: 'redirect' }>, abortSignal: AbortSignal | undefined): Promise<never> {
  if (decision.reason === 'authentication-required') {
    const nextSearch = searchRecord(decision.to.split('?')[1] ?? '');
    throw redirect({ to: '/login', search: nextSearch, reloadDocument: false });
  }
  if (decision.reason === 'workspace-required' && decision.to.startsWith('/onboarding')) {
    throw redirect({ to: '/onboarding/workspace', reloadDocument: false });
  }
  const hardParts = decision.to.split('?');
  const hardTarget = internalTarget(hardParts[0]!, '/platform/knowledge-bases');
  const hardQuery = hardParts[1] ? `?${hardParts[1]}` : '';
  const hardUrl = `${hardTarget}${hardQuery}`;
  window.location.replace(hardUrl);
  await navigationTakesOver(abortSignal);
}

export function createWeKnoraRouter(deps: WeKnoraRouterDeps, options: { history?: RouterHistory } = {}) {
  const { client, scopeController, scopeRuntime, liteMode, development, apiBaseUrl } = deps;

  const guardContext = (): RouteGuardContext => ({
    authenticated: deps.session().credential.kind === 'bearer',
    tenantId: scopeController.current().scope.tenantId,
    capabilities: scopeRuntime.capabilities(),
    isSystemAdmin: scopeRuntime.isSystemAdmin(),
    liteMode,
    development,
  });

  /** Hydrate the bearer session, then run the shared guard for this location. */
  const protectBeforeLoad = async ({ location, abortSignal }: { location: { href: string }; abortSignal?: AbortSignal }) => {
    if (deps.session().credential.kind === 'bearer') {
      const hydrated = await deps.ensureSessionHydrated();
      if (!hydrated.ok) {
        const nextPath = pathWithQuery(location.href);
        throw redirect({ to: '/login', search: { next: nextPath }, reloadDocument: false });
      }
    }
    const decision = guardRoute(pathWithQuery(location.href), guardContext());
    if (decision.kind === 'redirect') await applyGuardDecision(decision, abortSignal);
  };

  const guardBeforeLoad = async ({ location, abortSignal }: { location: { href: string }; abortSignal?: AbortSignal }) => {
    const decision = guardRoute(pathWithQuery(location.href), guardContext());
    if (decision.kind === 'redirect') await applyGuardDecision(decision, abortSignal);
  };

  const shellPage = (page: ReactNode): ReactNode => (
    <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
      <PlatformShell client={client} onLogout={deps.logout} onTenantSwitch={deps.switchTenantFromShell}>{page}</PlatformShell>
    </Suspense>
  );

  const rootRoute = createRootRoute({
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <Outlet />
      </Suspense>
    ),
    // Public unknown paths keep the pre-router behaviour: the 404 page inside
    // the platform shell (renderShell(NotFoundPage)). Protected unknown paths
    // are owned by the platform/knowledgeBase layouts, which guard first.
    notFoundComponent: () => shellPage(<NotFoundPage path={window.location.pathname} />),
  });

  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    beforeLoad: guardBeforeLoad,
  });

  const loginSearch = (search: Record<string, unknown>): Record<string, string> => ({
    ...(typeof search.token === 'string' && search.token !== '' ? { token: search.token } : {}),
    ...(typeof search.next === 'string' && search.next !== '' ? { next: search.next } : {}),
  });

  const loginBeforeLoad = async ({ location }: { location: { href: string; search?: unknown } }) => {
    // Ported from the pre-router bootstrap(): invite redemption, session
    // validation and lite-edition auto-setup all land on the login entry.
    const token = loginSearch((location.search ?? {}) as Record<string, unknown>).token;
    if (token) {
      if (deps.session().credential.kind === 'bearer') {
        // Vue Login.vue:798-801 — an existing session redeems the token
        // directly; an invalid token still enters the app.
        try {
          await client.auth.acceptInvitationByToken(token);
        } catch { /* Vue acceptAndEnter */ }
        throw redirect({ to: '/platform/knowledge-bases', reloadDocument: true });
      }
      // Vue Login.vue:803-808 — invite_only stays on the login card; open
      // deployments render the registration form.
      let registrationMode = 'self_serve';
      try { registrationMode = (await client.auth.registrationConfig()).registrationMode; } catch { /* fail open like loadAuthConfig */ }
      if (registrationMode === 'invite_only') return { inviteToken: token };
      const joinForward = { token };
      throw redirect({ to: '/join', search: joinForward, reloadDocument: false });
    }
    // Vue router.beforeEach redirects an already-authenticated visitor away
    // from /login; validate the imported session before choosing the
    // tenantless onboarding landing.
    if (deps.session().credential.kind === 'bearer') {
      const hydrated = await deps.ensureSessionHydrated();
      if (hydrated.ok) {
        if (hydrated.tenantId !== null) {
          throw redirect({ to: '/platform/knowledge-bases', reloadDocument: true });
        }
        throw redirect({ to: '/onboarding/workspace', reloadDocument: true });
      }
    }
    // Vue Login.vue:817-831 — lite-edition transparent auto-setup on /login.
    const AUTO_SETUP_FAILED_KEY = 'weknora_auto_setup_failed';
    if (window.localStorage.getItem(AUTO_SETUP_FAILED_KEY) !== 'true') {
      let autoSession: AuthSession | null = null;
      try {
        autoSession = await client.auth.autoSetup();
      } catch {
        window.localStorage.setItem(AUTO_SETUP_FAILED_KEY, 'true');
      }
      if (autoSession !== null) {
        window.localStorage.setItem('weknora_lite_mode', 'true');
        deps.completeAuthentication(autoSession);
        throw redirect({ to: '/platform/knowledge-bases', reloadDocument: true });
      }
    }
    return {};
  };

  function LoginCard(props: { initialMode: 'login' | 'register'; useLoaderData: () => unknown }): ReactNode {
    const data = props.useLoaderData() as { inviteToken?: string };
    return (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <LoginPage
          client={client}
          onAuthenticated={deps.completeAuthentication}
          apiBaseUrl={apiBaseUrl}
          initialError={deps.initialLoginError()}
          initialMode={props.initialMode}
          inviteToken={data.inviteToken ?? ''}
          onInviteAccepted={() => { window.location.assign('/platform/knowledge-bases'); }}
        />
      </Suspense>
    );
  }

  const loginRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/login',
    validateSearch: loginSearch,
    beforeLoad: loginBeforeLoad,
    component: ({ useLoaderData }): ReactNode => <LoginCard initialMode="login" useLoaderData={useLoaderData} />,
  });

  const registerRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/register',
    validateSearch: loginSearch,
    beforeLoad: loginBeforeLoad,
    component: ({ useLoaderData }): ReactNode => <LoginCard initialMode="register" useLoaderData={useLoaderData} />,
  });

  const joinRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/join',
    validateSearch: (search: Record<string, unknown>): Record<string, string> => ({
      ...(typeof search.token === 'string' && search.token !== '' ? { token: search.token } : {}),
      ...(typeof search.code === 'string' && search.code !== '' ? { code: search.code } : {}),
      ...(typeof search.next === 'string' && search.next !== '' ? { next: search.next } : {}),
    }),
    beforeLoad: ({ location, abortSignal }: { location: { href: string; search?: unknown }; abortSignal?: AbortSignal }) => {
      const search = (location.search ?? {}) as Record<string, string>;
      const joinToken = typeof search.token === 'string' ? search.token : '';
      // Vue share-links land on /login|/register?token — never dead-end /join.
      if (joinToken !== '') {
        const registerTarget = `/register?token=${encodeURIComponent(joinToken)}`;
        window.location.replace(registerTarget);
        return navigationTakesOver(abortSignal);
      }
      if (deps.session().credential.kind !== 'bearer') {
        const nextPath = pathWithQuery(location.href);
        const loginTarget = `/login?next=${encodeURIComponent(nextPath)}`;
        window.location.replace(loginTarget);
        return navigationTakesOver(abortSignal);
      }
      // Authenticated: forward to the invite landing (organizations?invite_code
      // when a code came along), preserving the pre-router hard replace.
      const decision = guardRoute(pathWithQuery(location.href), guardContext());
      const inviteCode = typeof search.code === 'string' ? search.code : '';
      const fallback = inviteCode !== '' ? `/platform/organizations?invite_code=${encodeURIComponent(inviteCode)}` : '/platform/organizations';
      const decisionTarget = decision.kind === 'redirect' ? decision.to : fallback;
      const joinTarget = internalTarget(decisionTarget.split('?')[0]!, fallback);
      window.location.replace(joinTarget);
      return navigationTakesOver(abortSignal);
    },
  });

  const onboardingRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/onboarding/workspace',
    beforeLoad: guardBeforeLoad,
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <WorkspaceOnboardingPage client={client} scopeRuntime={scopeRuntime} onLogout={deps.logout} />
      </Suspense>
    ),
  });

  // Vue renders /embed/:channelId from an isolated embed.html document; React
  // keeps one bundle, so the bare entry mounts here: no shell, no bearer
  // session, anonymous client created inside the page itself.
  const embedRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/embed/$',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <EmbedEntryPage apiBaseUrl={apiBaseUrl} />
      </Suspense>
    ),
  });

  // W05 craft: two paths (/craft and /craft/:sessionId) resolved from
  // window.location by CraftRoutes' own state machine. One layout route keeps
  // the component identity stable across browser back/forward, so the
  // controller + message log survive internal navigation — no second router.
  const craftRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/craft',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <CraftRoutes client={client} scopeController={scopeController} session={deps.session()} apiBaseUrl={apiBaseUrl} />
      </Suspense>
    ),
  });
  const craftSplatRoute = createRoute({ getParentRoute: () => craftRoute, path: '$' });

  // Wiki documents/wikis share the wiki entry gate: a library without the wiki
  // capability falls back to the documents page at the wiki URL (parity with
  // the pre-router WikiEntry).
  function WikiEntry(props: { knowledgeBaseId: string; initialSlug?: string; initialDocumentId?: string }): ReactNode {
    const [wikiEnabled, setWikiEnabled] = useState<boolean | null>(null);
    useEffect(() => {
      let active = true;
      void client.knowledgeBases.settings.get(props.knowledgeBaseId).then((kb) => {
        if (active) setWikiEnabled(shouldOpenWiki(kb));
      }).catch(() => {
        // Preserve the existing Wiki error surface when capability lookup is unavailable.
        if (active) setWikiEnabled(true);
      });
      return () => { active = false; };
    }, [client, props.knowledgeBaseId]);
    if (wikiEnabled === false) {
      window.history.replaceState({}, document.title, wikiEntryPath(props.knowledgeBaseId));
      return (
        <KnowledgeDocumentsPage
          client={client}
          knowledgeBaseId={props.knowledgeBaseId}
          initialDocumentId={props.initialDocumentId}
          onOpenDocument={(document) => navigate(`/knowledgeBase/${encodeURIComponent(props.knowledgeBaseId)}/documents/${encodeURIComponent(document.id)}`)}
        />
      );
    }
    if (wikiEnabled === null) return <p role="status">加载中…</p>;
    return (
      <WikiPage
        client={client}
        knowledgeBaseId={props.knowledgeBaseId}
        initialSlug={props.initialSlug}
        canContribute={scopeRuntime.role() !== 'viewer'}
        onOpenSourceDoc={createWikiSourceDocOpener({ knowledgeBaseId: props.knowledgeBaseId, navigate })}
      />
    );
  }

  /** ?tab= documents|wiki|graph dispatch shared by the two knowledge-base paths. */
  function KnowledgeBaseView(props: { knowledgeBaseId: string; href: string }): ReactNode {
    const query = new URLSearchParams(searchOf(props.href));
    const requestedTab = query.get('tab');
    const tab = requestedTab === 'wiki' || requestedTab === 'graph' || requestedTab === 'documents' ? requestedTab : undefined;
    const slug = query.get('slug')?.trim() || undefined;
    const initialDocumentId = query.get('knowledge_id')?.trim() || undefined;
    if (tab === 'wiki') return <WikiEntry knowledgeBaseId={props.knowledgeBaseId} initialSlug={slug} initialDocumentId={initialDocumentId} />;
    if (tab === 'graph') return <KnowledgeGraphPage client={client} knowledgeBaseId={props.knowledgeBaseId} slug={slug} />;
    return (
      <KnowledgeDocumentsPage
        client={client}
        knowledgeBaseId={props.knowledgeBaseId}
        initialDocumentId={initialDocumentId}
        onOpenDocument={(document) => navigate(`/knowledgeBase/${encodeURIComponent(props.knowledgeBaseId)}/documents/${encodeURIComponent(document.id)}`)}
      />
    );
  }

  function ChatPage(props: { knowledgeBaseId?: string }): ReactNode {
    return (
      <ChatRoutePage
        client={client}
        scopeController={scopeController}
        apiBaseUrl={apiBaseUrl}
        knowledgeBaseId={props.knowledgeBaseId}
        canViewChannelSessions={scopeRuntime.canViewChannelSessions()}
      />
    );
  }

  function ChatPageSuspensed(props: { knowledgeBaseId?: string }): ReactNode {
    return (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <ChatPage knowledgeBaseId={props.knowledgeBaseId} />
      </Suspense>
    );
  }

  // Historical flat chat entry point (Vue-compatible alias kept on its own
  // URL — the pre-router app rendered the chat page without rewriting it).
  const flatChatRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/creatChat',
    beforeLoad: protectBeforeLoad,
    component: (): ReactNode => <ChatPageSuspensed />,
  });

  const platformRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/platform',
    beforeLoad: protectBeforeLoad,
    component: (): ReactNode => shellPage(<Outlet />),
    // Unknown /platform/* children: 404 inside the shell after the guard ran.
    notFoundComponent: () => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <NotFoundPage path={window.location.pathname} />
      </Suspense>
    ),
  });

  const knowledgeBasesRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'knowledge-bases',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <KnowledgeBasesPage client={client} scopeController={scopeController} />
      </Suspense>
    ),
  });

  const platformKnowledgeBaseRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'knowledge-bases/$kbId',
    component: ({ useParams, useLocation }): ReactNode => {
      const { kbId } = useParams({ strict: false }) as { kbId?: string };
      const location = useLocation();
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <KnowledgeBaseView knowledgeBaseId={decodeURIComponent(kbId ?? '')} href={location.href} />
        </Suspense>
      );
    },
  });

  const platformKnowledgeChatRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'knowledge-bases/$kbId/creatChat',
    component: ({ useParams }): ReactNode => {
      const { kbId } = useParams({ strict: false }) as { kbId?: string };
      return <ChatPageSuspensed knowledgeBaseId={decodeURIComponent(kbId ?? '')} />;
    },
  });

  const creatChatRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'creatChat',
    component: (): ReactNode => <ChatPageSuspensed />,
  });

  const chatIndexRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'chat',
    component: (): ReactNode => <ChatPageSuspensed />,
  });

  const chatSplatRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'chat/$',
    component: (): ReactNode => <ChatPageSuspensed />,
  });

  const agentsRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'agents',
    // Real agents list (parity with Vue AgentList.vue); the consolidated
    // configuration surface stays reachable at /platform/configuration.
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <AgentsPage client={client} tenantId={scopeRuntime.current().scope.tenantId} />
      </Suspense>
    ),
  });

  const configurationRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'configuration',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <ConfigurationPage client={client} />
      </Suspense>
    ),
  });

  const organizationsRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'organizations',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <OrganizationsPage
          client={client}
          inviteCode={organizationInviteCode(pathWithQuery(window.location.href))}
          role={scopeRuntime.role()}
        />
      </Suspense>
    ),
  });

  const settingsRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'settings',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <SettingsPage
          capabilities={scopeRuntime.capabilities()}
          liteMode={liteMode}
          client={client}
          tenantId={Number(scopeRuntime.current().scope.tenantId)}
          role={scopeRuntime.role() === 'owner' ? 'owner' : scopeRuntime.role() === 'admin' ? 'admin' : 'viewer'}
        />
      </Suspense>
    ),
  });

  const devMarkdownRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'dev/markdown',
    beforeLoad: () => {
      // resolveRoute only dispatched this fixture under development.
      if (!development) throw notFound();
    },
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <DevMarkdownPage />
      </Suspense>
    ),
  });

  const appsCatalogRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'apps',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <AppsPage client={client} mode="catalog" role={scopeRuntime.role()} />
      </Suspense>
    ),
  });

  const appsConnectionsRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'apps/connections',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <AppsPage client={client} mode="connections" role={scopeRuntime.role()} />
      </Suspense>
    ),
  });

  const appsAuthorizationRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'apps/authorization/$id',
    component: ({ useParams }): ReactNode => {
      const { id } = useParams({ strict: false }) as { id?: string };
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <AppsPage client={client} mode="authorization" id={decodeURIComponent(id ?? '')} role={scopeRuntime.role()} />
        </Suspense>
      );
    },
  });

  const appsActionRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'apps/actions/$id',
    component: ({ useParams }): ReactNode => {
      const { id } = useParams({ strict: false }) as { id?: string };
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <AppsPage client={client} mode="action" id={decodeURIComponent(id ?? '')} role={scopeRuntime.role()} />
        </Suspense>
      );
    },
  });

  const knowledgeBaseLayout = createRoute({
    getParentRoute: () => rootRoute,
    path: '/knowledgeBase',
    beforeLoad: protectBeforeLoad,
    component: (): ReactNode => shellPage(<Outlet />),
    notFoundComponent: () => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <NotFoundPage path={window.location.pathname} />
      </Suspense>
    ),
  });

  const knowledgeBaseIndexRoute = createRoute({
    getParentRoute: () => knowledgeBaseLayout,
    path: '/',
    // protectedPageForRoute maps the id-less library link to the list page.
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <KnowledgeBasesPage client={client} scopeController={scopeController} />
      </Suspense>
    ),
  });

  const knowledgeBaseRoute = createRoute({
    getParentRoute: () => knowledgeBaseLayout,
    path: '$kbId',
    component: ({ useParams, useLocation }): ReactNode => {
      const { kbId } = useParams({ strict: false }) as { kbId?: string };
      const location = useLocation();
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <KnowledgeBaseView knowledgeBaseId={decodeURIComponent(kbId ?? '')} href={location.href} />
        </Suspense>
      );
    },
  });

  const knowledgeBaseDocumentRoute = createRoute({
    getParentRoute: () => knowledgeBaseLayout,
    path: '$kbId/documents/$docId',
    component: ({ useParams }): ReactNode => {
      const { kbId, docId } = useParams({ strict: false }) as { kbId?: string; docId?: string };
      const knowledgeBaseId = decodeURIComponent(kbId ?? '');
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <KnowledgeDocumentDetailPage
            client={client}
            documentId={decodeURIComponent(docId ?? '')}
            onBack={() => navigate(`/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}`)}
          />
        </Suspense>
      );
    },
  });

  const knowledgeBaseWikiRoute = createRoute({
    getParentRoute: () => knowledgeBaseLayout,
    path: '$kbId/wiki',
    component: ({ useParams, useLocation }): ReactNode => {
      const { kbId } = useParams({ strict: false }) as { kbId?: string };
      const initialDocumentId = new URLSearchParams(searchOf(useLocation().href)).get('knowledge_id')?.trim() || undefined;
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <WikiEntry knowledgeBaseId={decodeURIComponent(kbId ?? '')} initialDocumentId={initialDocumentId} />
        </Suspense>
      );
    },
  });

  const knowledgeBaseFaqRoute = createRoute({
    getParentRoute: () => knowledgeBaseLayout,
    path: '$kbId/faq',
    component: ({ useParams }): ReactNode => {
      const { kbId } = useParams({ strict: false }) as { kbId?: string };
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <FAQPage client={client} knowledgeBaseId={decodeURIComponent(kbId ?? '')} />
        </Suspense>
      );
    },
  });

  const knowledgeBaseSettingsRoute = createRoute({
    getParentRoute: () => knowledgeBaseLayout,
    path: '$kbId/settings',
    component: ({ useParams }): ReactNode => {
      const { kbId } = useParams({ strict: false }) as { kbId?: string };
      const role = scopeRuntime.role();
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <KnowledgeSettingsPage
            client={client}
            knowledgeBaseId={decodeURIComponent(kbId ?? '')}
            role={role === 'owner' ? 'owner' : role === 'admin' ? 'admin' : 'viewer'}
          />
        </Suspense>
      );
    },
  });

  const routeTree = rootRoute.addChildren([
    indexRoute,
    loginRoute,
    registerRoute,
    joinRoute,
    onboardingRoute,
    flatChatRoute,
    embedRoute,
    craftRoute.addChildren([craftSplatRoute]),
    platformRoute.addChildren([
      knowledgeBasesRoute,
      platformKnowledgeBaseRoute,
      platformKnowledgeChatRoute,
      creatChatRoute,
      chatIndexRoute,
      chatSplatRoute,
      agentsRoute,
      configurationRoute,
      organizationsRoute,
      settingsRoute,
      devMarkdownRoute,
      appsCatalogRoute,
      appsConnectionsRoute,
      appsAuthorizationRoute,
      appsActionRoute,
    ]),
    knowledgeBaseLayout.addChildren([
      knowledgeBaseIndexRoute,
      knowledgeBaseRoute,
      knowledgeBaseDocumentRoute,
      knowledgeBaseWikiRoute,
      knowledgeBaseFaqRoute,
      knowledgeBaseSettingsRoute,
    ]),
  ]);

  return createRouter({
    routeTree,
    defaultPreload: false,
    ...(options.history ? { history: options.history } : {}),
  });
}

export type WeKnoraRouter = ReturnType<typeof createWeKnoraRouter>;
