import { createRoot } from 'react-dom/client';
import { createWeKnoraClient } from '@weknora/api-client';
import { Status } from '@weknora/ui';
import { KnowledgeBasesPage } from './App.tsx';
import { LoginPage } from './auth/LoginPage.tsx';
import { readLegacyPlatformSession } from './platform/legacy-session.ts';
import { createBrowserTransport } from './platform/http.ts';
import { createWebScopeRuntime } from './platform/scope-runtime.ts';
import { resolveRoute } from './routes.tsx';
import { ChatRoutePage } from './chat/ChatRoutePage.tsx';
import { IntegrationsRoutePage } from './integrations/IntegrationsRoutePage.tsx';
import './styles.css';

const route = resolveRoute(window.location.pathname);
let session = route.kind === 'embed' ? { credential: { kind: 'anonymous' } as const, tenantId: null } : readLegacyPlatformSession();
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? '';
const scopeRuntime = createWebScopeRuntime(apiBaseUrl || window.location.origin, null, session.tenantId);
const scopeController = scopeRuntime.controller;

const client = createWeKnoraClient({
  baseURL: apiBaseUrl,
  transport: createBrowserTransport({
    credential: () => session.credential,
    tenantId: session.tenantId,
    locale: navigator.language,
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
    window.localStorage.setItem('weknora_token', next.token);
    window.localStorage.setItem('weknora_refresh_token', next.refreshToken);
    window.location.assign('/platform/knowledge-bases');
  }} />);
} else if (route.path === '/platform/creatChat' || route.path.startsWith('/platform/chat/')) {
  root.render(<ChatRoutePage client={client} scopeController={scopeController} />);
} else if (route.path === '/platform/integrations') {
  root.render(<IntegrationsRoutePage client={client} />);
} else {
  root.render(<KnowledgeBasesPage client={client} scopeController={scopeController} />);
}
