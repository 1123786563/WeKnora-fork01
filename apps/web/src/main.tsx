import { createRoot } from 'react-dom/client';
import { createRefreshCoordinator, createWeKnoraClient, type Credential } from '@weknora/api-client';
import { Status } from '@weknora/ui';
import { KnowledgeBasesPage } from './App.tsx';
import { LoginPage } from './auth/LoginPage.tsx';
import { JoinPage } from './auth/JoinPage.tsx';
import { parseOIDCCallbackHash } from './auth/oidc.ts';
import { readLegacyPlatformSession } from './platform/legacy-session.ts';
import { createBrowserTransport } from './platform/http.ts';
import { createBrowserCredentialAdapter, persistBrowserCredential } from './platform/credentials.ts';
import { createWebScopeRuntime } from './platform/scope-runtime.ts';
import { resolveRoute } from './routes.tsx';
import { ChatRoutePage } from './chat/ChatRoutePage.tsx';
import { IntegrationsRoutePage } from './integrations/IntegrationsRoutePage.tsx';
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
const scopeRuntime = createWebScopeRuntime(apiBaseUrl || window.location.origin, null, session.tenantId);
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
    tenantId: session.tenantId,
    locale: navigator.language,
    shouldRefresh: (request) => !request.url.endsWith('/api/v1/auth/refresh'),
    refresh: refreshCoordinator ? async () => { await refreshCoordinator.refresh(); } : undefined,
  }),
});

// Migration seam: the legacy Vue route is /platform/knowledge-bases.
// This entry only proves the React list slice; it does not claim Vue migration completion.
const root = createRoot(document.getElementById('root')!);
if (route.kind === 'embed') {
  root.render(<main className="wk-page"><Status tone="error">Embed must use its isolated entrypoint.</Status></main>);
} else if (route.kind === 'login') {
  root.render(<LoginPage client={client} onAuthenticated={(next) => {
    session = { credential: { kind: 'bearer', accessToken: next.token, refreshToken: next.refreshToken }, tenantId: null };
    persistBrowserCredential(window.localStorage, session.credential);
    window.location.assign('/platform/knowledge-bases');
  }} apiBaseUrl={apiBaseUrl} initialError={initialLoginError} initialMode={route.mode} />);
} else if (route.kind === 'join') {
  root.render(<JoinPage client={client} onAuthenticated={(next) => {
    session = { credential: { kind: 'bearer', accessToken: next.token, refreshToken: next.refreshToken }, tenantId: null };
    persistBrowserCredential(window.localStorage, session.credential);
    window.location.assign('/platform/knowledge-bases');
  }} />);
} else if (route.path === '/platform/creatChat' || route.path.startsWith('/platform/chat/')) {
  root.render(<ChatRoutePage client={client} scopeController={scopeController} />);
} else if (route.path === '/platform/integrations') {
  root.render(<IntegrationsRoutePage client={client} />);
} else {
  root.render(<KnowledgeBasesPage client={client} scopeController={scopeController} />);
}
