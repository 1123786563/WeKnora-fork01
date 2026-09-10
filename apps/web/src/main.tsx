import { createRoot } from 'react-dom/client';
import { createWeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain/scope';
import { KnowledgeBasesPage } from './App.tsx';
import { readLegacyPlatformSession } from './platform/legacy-session.ts';
import { createBrowserTransport } from './platform/http.ts';
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
