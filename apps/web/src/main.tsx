import { createRoot } from 'react-dom/client';
import { createRefreshCoordinator, createWeKnoraClient, type Credential } from '@weknora/api-client';
import { Status } from '@weknora/ui';
import { KnowledgeBasesPage } from './App.tsx';
import { LoginPage } from './auth/LoginPage.tsx';
import { JoinPage } from './auth/JoinPage.tsx';
import { WorkspaceOnboardingPage } from './auth/WorkspaceOnboardingPage.tsx';
import { parseOIDCCallbackHash } from './auth/oidc.ts';
import { persistSelectedTenant, readLegacyPlatformSession } from './platform/legacy-session.ts';
import { createBrowserTransport } from './platform/http.ts';
import { createBrowserCredentialAdapter, persistBrowserCredential } from './platform/credentials.ts';
import { createWebScopeRuntime } from './platform/scope-runtime.ts';
import { createWebPlatformAdapters } from './platform/adapters.ts';
import { guardRoute, resolveRoute, routeRedirect } from './routes.tsx';
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
let session = route.kind === 'embed' ? { credential: { kind: 'anonymous' } as const, tenantId: null } : readLegacyPlatformSession();
const browserCredentialAdapter = route.kind === 'embed' ? undefined : createBrowserCredentialAdapter(window.localStorage);
const currentCredential = (): Credential => route.kind === 'embed' ? session.credential : readLegacyPlatformSession().credential;
const injectedApiBaseUrl = (window as Window & { __WEKNORA_API_BASE__?: unknown }).__WEKNORA_API_BASE__;
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || (typeof injectedApiBaseUrl === 'string' ? injectedApiBaseUrl : '');
const liteMode = window.localStorage.getItem('weknora_lite_mode') === 'true';
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

function nextPathAfterAuth(): string {
  const next = new URLSearchParams(window.location.search).get('next');
  return next && next.startsWith('/') && !next.startsWith('//') ? next : '/platform/knowledge-bases';
}

function renderLogin(error = initialLoginError) {
  root.render(<LoginPage client={client} onAuthenticated={(next) => {
    session = { credential: { kind: 'bearer', accessToken: next.token, refreshToken: next.refreshToken }, tenantId: null };
    persistBrowserCredential(window.localStorage, session.credential);
    window.location.assign(nextPathAfterAuth());
  }} apiBaseUrl={apiBaseUrl} initialError={error} initialMode={route.kind === 'login' ? route.mode : 'login'} />);
}

async function logout(): Promise<void> {
  try { await client.auth.logout(); } catch { /* local invalidation still wins */ }
  await browserCredentialAdapter?.clear();
  scopeRuntime.logout();
  session = { credential: { kind: 'anonymous' }, tenantId: null };
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
  if (route.kind === 'knowledge-document') {
    root.render(<KnowledgeDocumentDetailPage client={client} documentId={route.documentId} onBack={() => window.location.assign(`/knowledgeBase/${encodeURIComponent(route.knowledgeBaseId)}`)} />);
  } else if (route.kind === 'knowledge-wiki') {
    root.render(<WikiPage client={client} knowledgeBaseId={route.knowledgeBaseId} />);
  } else if (route.kind === 'knowledge-faq') {
    root.render(<FAQPage client={client} knowledgeBaseId={route.knowledgeBaseId} />);
  } else if (route.kind === 'knowledge-settings') {
    root.render(<KnowledgeSettingsPage client={client} knowledgeBaseId={route.knowledgeBaseId} />);
  } else if (route.path === '/platform/configuration') {
    root.render(<ConfigurationPage client={client} />);
  } else if (route.path === '/platform/administration') {
    root.render(<AdministrationPage client={client} tenantId={Number(scopeRuntime.current().scope.tenantId)} />);
  } else if (route.path === '/platform/organizations') {
    root.render(<OrganizationsPage client={client} />);
  } else if (route.path === '/platform/settings') {
    root.render(<SettingsPage client={client} tenantId={Number(scopeRuntime.current().scope.tenantId)} />);
  } else if (route.path === '/platform/system') {
    root.render(<AdministrationPage client={client} tenantId={Number(scopeRuntime.current().scope.tenantId)} systemAdmin />);
  } else if (route.kind === 'knowledge-base' && route.path.split('/').filter(Boolean).length === 2) {
    const knowledgeBaseId = decodeURIComponent(route.path.split('/')[2]!);
    root.render(<KnowledgeDocumentsPage client={client} knowledgeBaseId={knowledgeBaseId} onOpenDocument={(document) => window.location.assign(`/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}/documents/${encodeURIComponent(document.id)}`)} />);
  } else if (route.path === '/platform/creatChat' || route.path.startsWith('/platform/chat/')) {
    root.render(<ChatRoutePage client={client} scopeController={scopeController} />);
  } else if (route.path === '/platform/integrations') {
    root.render(<IntegrationsRoutePage client={client} />);
  } else {
    root.render(<KnowledgeBasesPage client={client} scopeController={scopeController} />);
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
      root.render(<JoinPage client={client} onAuthenticated={(next) => {
        session = { credential: { kind: 'bearer', accessToken: next.token, refreshToken: next.refreshToken }, tenantId: null };
        persistBrowserCredential(window.localStorage, session.credential);
        window.location.assign(nextPathAfterAuth());
      }} />);
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
