import { createRoot } from 'react-dom/client';
import { createWeKnoraClient } from '@weknora/api-client';
import { KnowledgeBasesPage } from './App.tsx';
import { readLegacyPlatformSession } from './platform/legacy-session.ts';
import { createBrowserTransport } from './platform/http.ts';
import { createWebScopeRuntime } from './platform/scope-runtime.ts';
import './styles.css';

const session = readLegacyPlatformSession();
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? '';
const scopeRuntime = createWebScopeRuntime(apiBaseUrl || window.location.origin, null, session.tenantId);
const scopeController = scopeRuntime.controller;

const client = createWeKnoraClient({
  baseURL: apiBaseUrl,
  transport: createBrowserTransport({
    credential: session.credential,
    tenantId: session.tenantId,
    locale: navigator.language,
  }),
});

// Migration seam: the legacy Vue route is /platform/knowledge-bases.
// This entry only proves the React list slice; it does not claim Vue migration completion.
createRoot(document.getElementById('root')!).render(
  <KnowledgeBasesPage client={client} scopeController={scopeController} />,
);
