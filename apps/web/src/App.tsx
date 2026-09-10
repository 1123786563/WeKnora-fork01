import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button, Card, Status } from '@weknora/ui';
import { deleteKnowledgeBase, loadKnowledgeBases, saveKnowledgeBase, type KnowledgeBaseListState } from './knowledge-bases/list.ts';

interface KnowledgeBasesPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
}
export function KnowledgeBasesPage({ client, scopeController }: KnowledgeBasesPageProps) {
  const [reloadToken, setReloadToken] = useState(0);
  const [state, setState] = useState<KnowledgeBaseListState>({ status: 'error', message: 'Loading…' });
  const [name, setName] = useState('');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [mutationError, setMutationError] = useState<string | null>(null);
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

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMutationError(null);
    try {
      await saveKnowledgeBase(client, editingId, { name, type: 'document' });
      setName('');
      setEditingId(null);
      setReloadToken((value) => value + 1);
    } catch (error) { setMutationError(error instanceof Error ? error.message : 'Save failed'); }
  }

  async function remove(id: string) {
    setMutationError(null);
    try { await deleteKnowledgeBase(client, id); setReloadToken((value) => value + 1); }
    catch (error) { setMutationError(error instanceof Error ? error.message : 'Delete failed'); }
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
        <form className="wk-form" onSubmit={save}>
          <label>Knowledge base name <input value={name} onChange={(event) => setName(event.target.value)} required /></label>
          <Button type="submit">{editingId ? 'Save changes' : 'Create knowledge base'}</Button>
          {editingId ? <Button type="button" onClick={() => { setEditingId(null); setName(''); }}>Cancel</Button> : null}
        </form>
        {mutationError ? <Status tone="error">{mutationError}</Status> : null}
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
              {state.items.map((item) => <li key={item.id}><strong>{item.name}</strong><span>{item.id}</span><Button type="button" onClick={() => { setEditingId(item.id); setName(item.name); }}>Edit</Button><Button type="button" onClick={() => void remove(item.id)}>Delete</Button></li>)}
            </ul>
          )
        ) : null}
      </Card>
    </main>
  );
}
