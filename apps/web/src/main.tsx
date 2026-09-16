import { createRoot } from 'react-dom/client';
import { createJsonTransport, createWeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain/scope';
import { KnowledgeBasesPage, AppsPage } from './App.tsx';
import { CraftRoutes } from './features/craft/routes.tsx';
import { IntegrationsPage } from './integrations/IntegrationsPage.tsx';
import { parseIntegrationRoute } from './integrations/route.ts';
import { authorizationHeader, readLegacyPlatformSession } from './platform/legacy-session.ts';
import './styles.css';
import { AuthRoutes } from './auth/AuthPages.tsx';
import { guardRoute, nextPathAfterAuth, resolveRoute } from './routes.tsx';
import { resolveApiBaseUrl, wailsBridgeFromWindow } from './platform/desktop-bridge.ts';

const session = readLegacyPlatformSession();
const apiBaseUrl = await resolveApiBaseUrl({
  injected: window.__WEKNORA_API_BASE__,
  bridge: wailsBridgeFromWindow(window),
  configured: import.meta.env.VITE_API_BASE_URL ?? '',
  origin: window.location.origin,
});
const scopeController = createScopeController({
  origin: apiBaseUrl || window.location.origin,
  userId: null,
  tenantId: session.tenantId,
});

const client = createWeKnoraClient({
  baseURL: apiBaseUrl,
  transport: createJsonTransport(async (input, init) => {
    const headers = new Headers(init?.headers);
    const authorization = authorizationHeader(session.credential);
    if (authorization) headers.set('Authorization', authorization);
    if (session.tenantId) headers.set('X-Tenant-ID', session.tenantId);
    return fetch(input, { ...init, headers });
  }),
});

// Migration seam: the legacy Vue route is /platform/knowledge-bases.
// This entry only proves the React list slice; it does not claim Vue migration completion.
// W05 craft mounts at /craft and /craft/:sessionId through the same assembly
// (legacy session + shared scope + shared client) — no second router.
const route = resolveRoute(`${window.location.pathname}${window.location.search}`);
const decision = guardRoute(`${window.location.pathname}${window.location.search}`, {
  authenticated: session.credential.kind === 'bearer',
  tenantId: session.tenantId,
});

if (decision.kind === 'redirect') {
  window.history.replaceState({}, document.title, decision.to);
}

function replaceAuthTarget(path: string): void {
  window.location.assign(path === '/platform/knowledge-bases' ? nextPathAfterAuth(window.location.search) : path);
}

function renderNotFound(path: string) {
  return <main className="wk-page"><h1>404</h1><p>Page not found: {path}</p></main>;
}

createRoot(document.getElementById('root')!).render(
  decision.kind === 'redirect' && decision.reason === 'authentication-required' ? (
    <AuthRoutes client={client} navigation={{ replace: replaceAuthTarget }} />
  ) : decision.kind === 'redirect' && decision.to === '/onboarding/workspace' ? (
    <AuthRoutes client={client} navigation={{ replace: (path) => window.location.assign(path) }} />
  ) : decision.kind === 'not-found' ? (
    renderNotFound(route.path)
  ) : route.kind === 'craft' ? (
    <CraftRoutes client={client} scopeController={scopeController} session={session} apiBaseUrl={apiBaseUrl} />
  ) : route.kind === 'integration' && parseIntegrationRoute(window.location.href) !== null ? (
    <IntegrationsPage
      route={parseIntegrationRoute(window.location.href)!}
      onNavigate={(path) => { window.history.pushState(null, '', path); window.location.assign(path); }}
    />
  ) : (
    route.kind === 'login' || route.kind === 'onboarding' ? <AuthRoutes client={client} navigation={{ replace: replaceAuthTarget }} /> :
    route.kind === 'apps' ? <AppsPage client={client} scopeController={scopeController} /> :
    route.kind === 'platform' ? <KnowledgeBasesPage client={client} scopeController={scopeController} /> :
    renderNotFound(route.path)
  ),
);
