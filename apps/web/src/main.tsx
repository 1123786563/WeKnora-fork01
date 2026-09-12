import { createRoot } from 'react-dom/client';
import { createJsonTransport, createWeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain/scope';
import { KnowledgeBasesPage } from './App.tsx';
import { CraftRoutes } from './features/craft/routes.tsx';
import { authorizationHeader, readLegacyPlatformSession } from './platform/legacy-session.ts';
import './styles.css';

const session = readLegacyPlatformSession();
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? '';
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
createRoot(document.getElementById('root')!).render(
  window.location.pathname === '/craft' || window.location.pathname.startsWith('/craft/') ? (
    <CraftRoutes client={client} scopeController={scopeController} session={session} apiBaseUrl={apiBaseUrl} />
  ) : (
    <KnowledgeBasesPage client={client} scopeController={scopeController} />
  ),
);
