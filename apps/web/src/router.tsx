// TanStack Router assembly for the React web app.
//
// The route tree is built in code (no file-based generation) by a factory so
// tests can mount it against a memory history with fake deps. Behavioural
// parity with the pre-router app is delegated, not reimplemented: guards run
// the existing pure `guardRoute` / `routeRedirect` functions from routes.tsx
// (still unit-tested there), and every protected page renders inside the same
// PlatformShell with the same lazy chunks.
//
// Redirect policy (open-redirect hardening): no dynamic router redirect()
// calls at all. Authentication/workspace guard outcomes perform an SPA URL
// replace through router.history.replace (the router's redirect() alone
// renders the target without committing the URL), and every other guard
// target keeps the pre-router behaviour — a hard window.location.replace —
// after an internal-path allowlist check (internalTarget). Navigation
// targets are normalized into local constants before any navigation call so
// the allowlisted value, not a raw user-influenced expression, is what
// reaches the navigator.
//
// Bridge contract with platform/navigation.ts: page-driven URL mutations
// arrive through the navigation sink (router.history.push/replace). Craft and
// the chat page keep their own URL state machines and are excluded from the
// bridge there.
import { lazy, Suspense, useEffect, useState, type ReactNode } from 'react';
import { createRootRoute, createRoute, createRouter, notFound, Outlet, useParams, useLocation, type RouterHistory } from '@tanstack/react-router';
import { createWeKnoraClient, type AuthSession } from '@weknora/api-client';
import type { ScopeController } from '@weknora/domain/scope';
import { createWebScopeRuntime } from './platform/scope-runtime.ts';
import type { LegacyPlatformSession } from './platform/legacy-session.ts';
import { navigate } from './platform/navigation.ts';
import { guardRoute, organizationInviteCode, type RouteGuardContext, type RouteGuardDecision } from './routes.tsx';
import { shouldOpenWiki, wikiEntryPath } from './knowledge/wiki-route.ts';
import { createWikiSourceDocOpener } from './wiki/source-doc-open.ts';

// Craft mounts through the shared @weknora/views/craft assembly, which pulls
// in packages/ui 旧栈 (theme.css) — keep it lazy so the router module stays
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
const AnalyticsPage = lazy(() => import('./analytics/AnalyticsPage.tsx').then((module) => ({ default: module.AnalyticsPage })));
// SP13 Task 8 — 会话只读分享页（/platform/shared/:token）。
const SharedSessionPage = lazy(() => import('./shared/SharedSessionPage.tsx').then((module) => ({ default: module.SharedSessionPage })));
// SP14 Task 1 — 商业套餐三页（/platform/billing*）。RefundPage 本期不挂路由：
// refundId 由宿主注入（expectedVersion 对齐语义），留组件库待真实退款流程接入。
const BillingPage = lazy(() => import('./commercial/BillingPage.tsx').then((module) => ({ default: module.BillingPage })));
const CheckoutPage = lazy(() => import('./commercial/CheckoutPage.tsx').then((module) => ({ default: module.CheckoutPage })));
const AdminCommercialPage = lazy(() => import('./commercial/AdminCommercialPage.tsx').then((module) => ({ default: module.AdminCommercialPage })));
const ExpertsPage = lazy(() => import('./experts/ExpertsPage.tsx').then((module) => ({ default: module.ExpertsPage })));
// Octop M4 — 技能市场页（/platform/market）。
const MarketPage = lazy(() => import('./market/MarketPage.tsx').then((module) => ({ default: module.MarketPage })));
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

// Plain markup instead of the packages/ui 旧栈 Status pill: the router module must
// stay importable under the node test runner, which cannot parse the ui
// package's theme.css.
function RoutePending(props: { loadingText: string }): ReactNode {
  return (
    <main className="wk-page wk-page--std">
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

/** TanStack locations carry an internal href (no origin) — rebuild the
 * path?query form from pathname + the parsed search object. */
function locationPathWithQuery(location: { pathname: string; search?: unknown }): string {
  const searchParams = new URLSearchParams();
  for (const [key, value] of Object.entries((location.search ?? {}) as Record<string, unknown>)) {
    if (value !== undefined && value !== null && value !== '') searchParams.set(key, String(value));
  }
  const query = searchParams.toString();
  return location.pathname + (query ? `?${query}` : '');
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

/** Never-settling promise so the superseded navigation does not render its
 * route while a URL replacement takes over. Rejects on abort so the router
 * can clean the load up; the timeout is a safety valve — in the app the
 * replacement's re-dispatched popstate aborts this load within a tick. */
async function navigationTakesOver(abortSignal: AbortSignal | undefined): Promise<never> {
  await new Promise<never>((_resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error('navigation takeover timeout')), 10000);
    if (typeof timeout === 'object' && 'unref' in timeout) (timeout as { unref(): void }).unref();
    if (!abortSignal) return;
    abortSignal.addEventListener('abort', () => {
      clearTimeout(timeout);
      reject(abortSignal.reason ?? new Error('redirect aborted'));
    }, { once: true });
  });
  throw new Error('unreachable');
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

  /**
   * guardRoute decisions keep their pre-router semantics: authentication and
   * workspace redirects were SPA URL replaces (replaceState — the navigation
   * observer re-dispatches it to the router, which re-matches and renders the
   * target), capability/system-admin redirects were hard
   * window.location.replace calls.
   */
  const runGuardDecision = async (decision: Extract<RouteGuardDecision, { kind: 'redirect' }>, abortSignal: AbortSignal | undefined): Promise<never> => {
    if (decision.reason === 'authentication-required' || (decision.reason === 'workspace-required' && decision.to.startsWith('/onboarding'))) {
      const query = decision.to.includes('?') ? `?${decision.to.split('?')[1]}` : '';
      window.history.replaceState({}, document.title, `${decision.to.split('?')[0]}${query}`);
      await navigationTakesOver(abortSignal);
      throw new Error('unreachable');
    }
    const hardParts = decision.to.split('?');
    const hardTarget = internalTarget(hardParts[0]!, '/platform/knowledge-bases');
    const hardQuery = hardParts[1] ? `?${hardParts[1]}` : '';
    window.location.replace(`${hardTarget}${hardQuery}`);
    await navigationTakesOver(abortSignal);
    throw new Error('unreachable');
  };

  /** Hydrate the bearer session, then run the shared guard for this location. */
  const protectBeforeLoad = async ({ location, abortSignal }: { location: { pathname: string; search?: unknown }; abortSignal?: AbortSignal }) => {
    if (deps.session().credential.kind === 'bearer') {
      const hydrated = await deps.ensureSessionHydrated();
      if (!hydrated.ok) {
        window.history.replaceState({}, document.title, '/login');
        await navigationTakesOver(abortSignal);
        throw new Error('unreachable');
      }
    }
    const decision = guardRoute(locationPathWithQuery(location), guardContext());
    if (decision.kind === 'redirect') await runGuardDecision(decision, abortSignal);
  };

  const guardBeforeLoad = async ({ location, abortSignal }: { location: { pathname: string; search?: unknown }; abortSignal?: AbortSignal }) => {
    const decision = guardRoute(locationPathWithQuery(location), guardContext());
    if (decision.kind === 'redirect') await runGuardDecision(decision, abortSignal);
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
  });

  // beforeLoad hands the invite token / join handoff to the login card through
  // these slots: without a registered route tree the loaderData hooks fall
  // back to untyped `{}` contexts, so explicit module slots are dependable.
  let loginInviteToken = '';
  let loginRenderJoin = false;

  const loginBeforeLoad = async ({ location, abortSignal }: { location: { pathname: string; search?: unknown }; abortSignal?: AbortSignal }) => {
    // Ported from the pre-router bootstrap(): invite redemption, session
    // validation and lite-edition auto-setup all land on the login entry.
    const token = loginSearch((location.search ?? {}) as Record<string, unknown>).token;
    loginInviteToken = '';
    loginRenderJoin = false;
    if (token) {
      if (deps.session().credential.kind === 'bearer') {
        // Vue Login.vue:798-801 — an existing session redeems the token
        // directly; an invalid token still enters the app.
        try {
          await client.auth.acceptInvitationByToken(token);
        } catch { /* Vue acceptAndEnter */ }
        window.location.replace('/platform/knowledge-bases');
        await navigationTakesOver(abortSignal);
      }
      // Vue Login.vue:803-808 — invite_only stays on the login card; open
      // deployments render the registration form (the pre-router app rendered
      // the JoinPage in place on the /login URL).
      let registrationMode = 'self_serve';
      try { registrationMode = (await client.auth.registrationConfig()).registrationMode; } catch { /* fail open like loadAuthConfig */ }
      if (registrationMode === 'invite_only') {
        loginInviteToken = token;
        return {};
      }
      loginRenderJoin = true;
      return {};
    }
    // Vue router.beforeEach redirects an already-authenticated visitor away
    // from /login; validate the imported session before choosing the
    // tenantless onboarding landing.
    if (deps.session().credential.kind === 'bearer') {
      const hydrated = await deps.ensureSessionHydrated();
      if (hydrated.ok) {
        window.location.replace(hydrated.tenantId !== null ? '/platform/knowledge-bases' : '/onboarding/workspace');
        await navigationTakesOver(abortSignal);
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
        await navigationTakesOver(abortSignal);
      }
    }
    return {};
  };

  function LoginCard(props: { initialMode: 'login' | 'register' }): ReactNode {
    if (loginRenderJoin) {
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <JoinPage client={client} onAuthenticated={deps.completeAuthentication} />
        </Suspense>
      );
    }
    return (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <LoginPage
          client={client}
          onAuthenticated={deps.completeAuthentication}
          apiBaseUrl={apiBaseUrl}
          initialError={deps.initialLoginError()}
          initialMode={props.initialMode}
          inviteToken={loginInviteToken}
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
    component: (): ReactNode => <LoginCard initialMode="login" />,
  });

  const registerRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/register',
    validateSearch: loginSearch,
    beforeLoad: loginBeforeLoad,
    component: (): ReactNode => <LoginCard initialMode="register" />,
  });

  const joinRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/join',
    validateSearch: (search: Record<string, unknown>): Record<string, string> => ({
      ...(typeof search.token === 'string' && search.token !== '' ? { token: search.token } : {}),
      ...(typeof search.code === 'string' && search.code !== '' ? { code: search.code } : {}),
    }),
    beforeLoad: ({ location, abortSignal }: { location: { pathname: string; search?: unknown }; abortSignal?: AbortSignal }) => {
      const search = (location.search ?? {}) as Record<string, string>;
      const joinToken = typeof search.token === 'string' ? search.token : '';
      // Vue share-links land on /login|/register?token — never dead-end /join.
      if (joinToken !== '') {
        const registerTarget = `/register?token=${encodeURIComponent(joinToken)}`;
        window.location.replace(registerTarget);
        return navigationTakesOver(abortSignal);
      }
      if (deps.session().credential.kind !== 'bearer') {
        window.history.replaceState({}, document.title, '/login');
        return navigationTakesOver(abortSignal);
      }
      // Authenticated: forward to the invite landing (organizations?invite_code
      // when a code came along), preserving the pre-router hard replace. The
      // internal-path allowlist judges the path only — the query (invite_code)
      // is re-attached afterwards so the exact redirect contract of
      // routeRedirect/Vue survives the check.
      const decision = guardRoute(locationPathWithQuery(location), guardContext());
      const inviteCode = typeof search.code === 'string' ? search.code : '';
      const fallbackPath = '/platform/organizations';
      const fallbackQuery = inviteCode !== '' ? `invite_code=${encodeURIComponent(inviteCode)}` : '';
      const decisionTarget = decision.kind === 'redirect' ? decision.to : `${fallbackPath}${fallbackQuery ? `?${fallbackQuery}` : ''}`;
      const joinParts = decisionTarget.split('?');
      const joinQuery = joinParts[1] ? `?${joinParts[1]}` : '';
      window.location.replace(`${internalTarget(joinParts[0]!, fallbackPath)}${joinQuery}`);
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
    // Vue KnowledgeBase.vue 在同一 URL 下按 kbInfo.type 就地切换 FAQ 管理视图，
    // 不重写地址栏；React 之前的做法是把 platform 路由跳去 /knowledgeBase/:id/faq，
    // 导致两端 URL 与浏览器历史行为不一致（R492 D3）。这里改为分流前先取类型，
    // FAQ KB 就地渲染 FAQPage。
    const [kbType, setKbType] = useState<string | null | undefined>(undefined);
    useEffect(() => {
      let active = true;
      setKbType(undefined);
      void client.knowledgeBases.settings.get(props.knowledgeBaseId)
        .then((kb) => {
          if (!active) return;
          const type = (kb as { type?: unknown }).type;
          setKbType(typeof type === 'string' ? type : null);
        })
        .catch(() => { if (active) setKbType(null); });
      return () => { active = false; };
    }, [client, props.knowledgeBaseId]);
    if (kbType === undefined) return <p role="status">加载中…</p>;
    if (kbType !== null && kbType.toLowerCase() === 'faq') {
      return <FAQPage client={client} knowledgeBaseId={props.knowledgeBaseId} />;
    }
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
    component: (): ReactNode => {
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
    component: (): ReactNode => {
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
          // SP14 Task 2 — platform-api-keys was unreachable: scopeRuntime.role()
          // never returns 'system-admin', so folding the role to owner/admin/
          // viewer filtered every registry section with minRole 'system-admin'
          // out of the settings nav. A system admin outranks owner in the
          // SettingsRole ranking, so widening to 'system-admin' keeps all the
          // ordinary sections visible while restoring the system-admin group.
          role={scopeRuntime.isSystemAdmin() ? 'system-admin' : scopeRuntime.role() === 'owner' ? 'owner' : scopeRuntime.role() === 'admin' ? 'admin' : 'viewer'}
        />
      </Suspense>
    ),
  });

  // SP11 admin analytics dashboard; the page itself renders the
  // no-permission placeholder for contributor/viewer roles (the nav entry
  // applies the same canViewChannelSessions gate).
  const analyticsRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'analytics',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <AnalyticsPage client={client} role={scopeRuntime.role()} />
      </Suspense>
    ),
  });

  // SP13 Task 8 — 会话只读分享页：挂在 platformRoute 下（Ruling P-2）复用
  // 登录守卫与 shell；Viewer+ 即可读（GET /api/v1/shared/sessions/:token）。
  // token 无效/已撤销由页面自身渲染占位（404 → 链接无效或已撤销）。
  const sharedSessionRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'shared/$token',
    component: (): ReactNode => {
      const { token } = useParams({ strict: false }) as { token?: string };
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <SharedSessionPage client={client} token={decodeURIComponent(token ?? '')} />
        </Suspense>
      );
    },
  });

  // SP14 Task 1 — 套餐接线（照 analyticsRoute 模式：platformRoute 子路由 +
  // lazy + Suspense）。/platform/billing 账单总览；checkout 从 URL query
  // `order` 取已有订单 id，无则空串（页面自建 quote+order，内部不导航）；
  // admin 的 capability 仅是 UI affordance（Ruling P-2：operator 取
  // scopeRuntime.isSystemAdmin()），页面自身渲染无权限占位，服务端校验才是权威。
  const billingRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'billing',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <BillingPage client={client} scopeController={scopeController} />
      </Suspense>
    ),
  });

  const billingCheckoutRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'billing/checkout',
    component: (): ReactNode => {
      // Same query-reading precedent as the wiki route's knowledge_id: the
      // TanStack location href carries the search string.
      const orderId = new URLSearchParams(searchOf(useLocation().href)).get('order')?.trim() ?? '';
      return (
        <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
          <CheckoutPage client={client} scopeController={scopeController} orderId={orderId} />
        </Suspense>
      );
    },
  });

  const billingAdminRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'billing/admin',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <AdminCommercialPage client={client} scopeController={scopeController} capability={{ operator: scopeRuntime.isSystemAdmin() }} />
      </Suspense>
    ),
  });

  // Octop M2 expert-template catalog; list/detail are Viewer+ reads, the
  // instantiate write stays Contributor+ server-side (routes_expert.go guard).
  const expertsRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'experts',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <ExpertsPage client={client} />
      </Suspense>
    ),
  });

  // Octop M4 skills market; search/rankings and the tenant-internal listing
  // are Viewer+ reads, market install / tenant install / publish stay Admin+
  // server-side (routes_skill_market.go / routes_tenant_skill_market.go
  // guards) — the page hides those affordances for non-admin roles
  // (AnalyticsPage role-prop pattern).
  const marketRoute = createRoute({
    getParentRoute: () => platformRoute,
    path: 'market',
    component: (): ReactNode => (
      <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
        <MarketPage client={client} role={scopeRuntime.role()} />
      </Suspense>
    ),
  });

  // Vue mounts /platform/dev/markdown as a top-level route outside the app
  // shell (requiresAuth: false, requiresInit: false) — mirror that so the
  // parity fixture page renders standalone.
  const devMarkdownRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/platform/dev/markdown',
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
    component: (): ReactNode => {
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
    component: (): ReactNode => {
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
    component: (): ReactNode => {
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
    component: (): ReactNode => {
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
    component: (): ReactNode => {
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
    component: (): ReactNode => {
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
    component: (): ReactNode => {
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
    devMarkdownRoute,
    platformRoute.addChildren([
      knowledgeBasesRoute,
      platformKnowledgeBaseRoute,
      platformKnowledgeChatRoute,
      creatChatRoute,
      chatIndexRoute,
      chatSplatRoute,
      agentsRoute,
      expertsRoute,
      marketRoute,
      configurationRoute,
      organizationsRoute,
      settingsRoute,
      analyticsRoute,
      sharedSessionRoute,
      billingRoute,
      billingCheckoutRoute,
      billingAdminRoute,
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

  const router = createRouter({
    routeTree,
    defaultPreload: false,
    ...(options.history ? { history: options.history } : {}),
  });
  return router;
}

export type WeKnoraRouter = ReturnType<typeof createWeKnoraRouter>;
