import { createRoot } from 'react-dom/client';
import { createRefreshCoordinator, createWeKnoraClient, type AuthSession, type Credential } from '@weknora/api-client';
import { Status } from '@weknora/ui';
import { LoginPage } from './auth/LoginPage.tsx';
import { JoinPage } from './auth/JoinPage.tsx';
import { WorkspaceOnboardingPage } from './auth/WorkspaceOnboardingPage.tsx';
import { parseOIDCCallbackHash } from './auth/oidc.ts';
import { importLegacyPlatformState, persistSelectedTenant, readReactPlatformState, type ReactPlatformState } from './platform/legacy-session.ts';
import { createBrowserTransport } from './platform/http.ts';
import { createBrowserCredentialAdapter, persistBrowserCredential } from './platform/credentials.ts';
import { createWebScopeRuntime } from './platform/scope-runtime.ts';
import { createWebPlatformAdapters } from './platform/adapters.ts';
import { guardRoute, protectedPageForRoute, resolveRoute, routeRedirect } from './routes.tsx';
import { ChatRoutePage } from './chat/ChatRoutePage.tsx';
import { IntegrationsRoutePage } from './integrations/IntegrationsRoutePage.tsx';
import { KnowledgeDocumentsPage } from './documents/KnowledgeDocumentsPage.tsx';
import { KnowledgeDocumentDetailPage } from './documents/KnowledgeDocumentDetailPage.tsx';
import { WikiPage } from './wiki/WikiPage.tsx';
import { FAQPage } from './faq/FAQPage.tsx';
import { DataSourcesPage } from './data-sources/DataSourcesPage.tsx';
import { KnowledgeSettingsPage } from './knowledge-settings/KnowledgeSettingsPage.tsx';
import { ConfigurationPage } from './configuration/ConfigurationPage.tsx';
import { AdministrationPage } from './administration/AdministrationPage.tsx';
import { OrganizationsPage } from './organizations/OrganizationsPage.tsx';
import { SettingsPage } from './settings/SettingsPage.tsx';
import { KnowledgeBasesPage } from './App.tsx';
import { NotFoundPage } from './NotFoundPage.tsx';
import { DevMarkdownPage } from './DevMarkdownPage.tsx';
import './styles.css';

const oidcCallback = parseOIDCCallbackHash(window.location.hash);
let initialLoginError: string | undefined;
if (oidcCallback?.kind === 'success') {
  persistBrowserCredential(window.localStorage, { kind: 'bearer', accessToken: oidcCallback.session.token, refreshToken: oidcCallback.session.refreshToken });
  window.history.replaceState({}, document.title, '/platform/knowledge-bases');
} else if (oidcCallback?.kind === 'error') {
  initialLoginError = oidcCallback.message;
  window.history.replaceState({}, document.title, '/login');
}

const route = resolveRoute(window.location.pathname);
const importedPlatformState = route.kind === 'embed' ? null : importLegacyPlatformState(window.localStorage);
let session: ReactPlatformState = route.kind === 'embed'
  ? { credential: { kind: 'anonymous' }, tenantId: null, preferences: {} }
  : importedPlatformState!;
const browserCredentialAdapter = route.kind === 'embed' ? undefined : createBrowserCredentialAdapter(window.localStorage);
const currentCredential = (): Credential => route.kind === 'embed'
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
    refresh: refreshCoordinator ? async () => { await refreshCoordinator.refresh(); } : undefined,
  }),
});

const root = createRoot(document.getElementById('root')!);
const platformAdapters = createWebPlatformAdapters();

// Most route transitions intentionally use full navigations so authentication,
// tenant scope, and capability guards are re-evaluated. Settings and legacy
// integrations may use history.pushState; reload those history entries instead
// of leaving the initial route's React tree mounted after Back/Forward.
window.addEventListener('popstate', () => window.location.reload());

function nextPathAfterAuth(): string {
  const next = new URLSearchParams(window.location.search).get('next');
  return next && next.startsWith('/') && !next.startsWith('//') ? next : '/platform/knowledge-bases';
}

function completeAuthentication(next: AuthSession): void {
  session = { ...session, credential: { kind: 'bearer', accessToken: next.token, refreshToken: next.refreshToken }, tenantId: null };
  persistBrowserCredential(window.localStorage, session.credential);
  window.location.assign(nextPathAfterAuth());
}

function renderLogin(error = initialLoginError) {
  root.render(<LoginPage client={client} onAuthenticated={completeAuthentication} apiBaseUrl={apiBaseUrl} initialError={error} initialMode={route.kind === 'login' ? route.mode : 'login'} />);
}

async function logout(): Promise<void> {
  try { await client.auth.logout(); } catch { /* local invalidation still wins */ }
  await browserCredentialAdapter?.clear();
  scopeRuntime.logout();
  session = { ...session, credential: { kind: 'anonymous' }, tenantId: null };
  window.location.assign('/login');
}

function renderProtected() {
  const pathname = `${window.location.pathname}${window.location.search}`;
  const current = scopeRuntime.current().scope;
  const decision = guardRoute(pathname, {
    authenticated: session.credential.kind === 'bearer',
    tenantId: current.tenantId,
    capabilities: scopeRuntime.capabilities(),
    isSystemAdmin: scopeRuntime.isSystemAdmin(),
    liteMode,
  });
  if (decision.kind === 'redirect') {
    if (decision.to === '/onboarding/workspace') {
      if (window.location.pathname !== decision.to) platformAdapters.replace(decision.to);
      root.render(<WorkspaceOnboardingPage client={client} scopeRuntime={scopeRuntime} onLogout={logout} />);
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
  if (route.kind === 'onboarding') {
    root.render(<WorkspaceOnboardingPage client={client} scopeRuntime={scopeRuntime} onLogout={logout} />);
    return;
  }
  if (protectedPageForRoute(route) === 'knowledge-bases') {
    root.render(<KnowledgeBasesPage client={client} scopeController={scopeController} />);
  } else if (protectedPageForRoute(route) === 'markdown-test') {
    root.render(<DevMarkdownPage />);
  } else if (route.kind === 'knowledge-document') {
    root.render(<KnowledgeDocumentDetailPage client={client} documentId={route.documentId} onBack={() => window.location.assign(`/knowledgeBase/${encodeURIComponent(route.knowledgeBaseId)}`)} />);
  } else if (route.kind === 'knowledge-wiki') {
    root.render(<WikiPage client={client} knowledgeBaseId={route.knowledgeBaseId} />);
  } else if (route.kind === 'knowledge-faq') {
    root.render(<FAQPage client={client} knowledgeBaseId={route.knowledgeBaseId} />);
  } else if (route.kind === 'knowledge-settings') {
    root.render(<KnowledgeSettingsPage client={client} knowledgeBaseId={route.knowledgeBaseId} />);
  } else if (route.path === '/platform/configuration' || route.path === '/platform/agents') {
    root.render(<ConfigurationPage client={client} />);
  } else if (route.path === '/platform/administration') {
    root.render(<AdministrationPage client={client} tenantId={Number(scopeRuntime.current().scope.tenantId)} />);
  } else if (route.path === '/platform/organizations') {
    root.render(<OrganizationsPage client={client} />);
  } else if (route.path === '/platform/settings') {
    root.render(<SettingsPage client={client} tenantId={Number(scopeRuntime.current().scope.tenantId)} />);
  } else if (route.path === '/platform/system') {
    root.render(<AdministrationPage client={client} tenantId={Number(scopeRuntime.current().scope.tenantId)} systemAdmin />);
  } else if (route.kind === 'knowledge-base' && route.knowledgeBaseId) {
    const knowledgeBaseId = route.knowledgeBaseId;
    root.render(<KnowledgeDocumentsPage client={client} knowledgeBaseId={knowledgeBaseId} onOpenDocument={(document) => window.location.assign(`/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}/documents/${encodeURIComponent(document.id)}`)} />);
  } else if (route.kind === 'chat' || route.path === '/platform/creatChat' || route.path.startsWith('/platform/chat/')) {
    root.render(<ChatRoutePage client={client} scopeController={scopeController} apiBaseUrl={apiBaseUrl} />);
  } else if (route.path === '/platform/integrations') {
    root.render(<IntegrationsRoutePage client={client} />);
  } else if (route.kind === 'not-found') {
    root.render(<NotFoundPage path={route.path} />);
  } else {
    root.render(<NotFoundPage path={route.path} />);
  }
}

async function bootstrap() {
  if (route.kind === 'embed') {
    root.render(<main className="wk-page"><Status tone="error">Embed must use its isolated entrypoint.</Status></main>);
    return;
  }
  if (route.kind === 'login') {
    const inviteToken = new URLSearchParams(window.location.search).get('token')?.trim();
    if (route.mode === 'register' && inviteToken) {
      root.render(<JoinPage client={client} onAuthenticated={completeAuthentication} />);
    } else renderLogin();
    return;
  }
  if (route.kind === 'join') {
    const redirect = routeRedirect(`${window.location.pathname}${window.location.search}`);
    if (session.credential.kind !== 'bearer') {
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
