import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button, Card, Status } from '@weknora/ui';
import { loadKnowledgeBases, type KnowledgeBaseListState } from './knowledge-bases/list.ts';
import { SemanticPage } from './semantic/SemanticPage.tsx';

interface KnowledgeBasesPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
}
// Semantic deep-link (W02): #/semantic/:kbId/:documentId mounts the
// semantic status + evidence flow without rewriting the KB navigation.
function semanticRouteFromHash(): { kbId: string; documentId: string } | null {
  const match = window.location.hash.match(/^#\/semantic\/([^/]+)\/([^/]+)$/);
  if (!match) return null;
  return { kbId: decodeURIComponent(match[1]), documentId: decodeURIComponent(match[2]) };
}

export function KnowledgeBasesPage({ client, scopeController }: KnowledgeBasesPageProps) {
  // Hooks stay unconditional (Rules of Hooks); the semantic route decides
  // CONTENT, not the hook shape.
  const [semanticRoute, setSemanticRoute] = useState(() => semanticRouteFromHash());
  useEffect(() => {
    const onHashChange = () => setSemanticRoute(semanticRouteFromHash());
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  }, []);
  const semanticContent = semanticRoute ? (
    <SemanticPage
      key={semanticRoute.kbId + '/' + semanticRoute.documentId}
      knowledgeBaseId={semanticRoute.kbId}
      documentId={semanticRoute.documentId}
    />
  ) : null;
  const [reloadToken, setReloadToken] = useState(0);
  const [state, setState] = useState<KnowledgeBaseListState>({ status: 'error', message: 'Loading…' });
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'knowledge-bases'), [scope.scope]);

  useEffect(() => {
    let active = true;
    setState({ status: 'error', message: 'Loading…' });
    void loadKnowledgeBases(client, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setState(next);
    });
    return () => { active = false; };
  }, [client, reloadToken, scopeController, scope.scope, scope.signal]);

  if (semanticContent) {
    return semanticContent;
  }

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">React migration seam</p>
          <h1>Knowledge bases</h1>
          <p className="wk-muted">Live data from GET /api/v1/knowledge-bases</p>
        </div>
        <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>Reload</Button>
      </header>
      <Card>
        <p className="wk-debug">scope key: {JSON.stringify(queryKey)}</p>
        {state.status === 'error' && state.message === 'Loading…' ? <Status>Loading knowledge bases…</Status> : null}
        {state.status === 'error' && state.message !== 'Loading…' ? (
          <>
            <Status tone="error">{state.message}</Status>
            <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>Try again</Button>
          </>
        ) : null}
        {state.status === 'success' ? (
          state.items.length === 0 ? <Status>No knowledge bases returned by the backend.</Status> : (
            <ul className="wk-list">
              {state.items.map((item) => <li key={item.id}><strong>{item.name}</strong><span>{item.id}</span></li>)}
            </ul>
          )
        ) : null}
      </Card>
    </main>
  );
}
