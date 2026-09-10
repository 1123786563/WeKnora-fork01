import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { filterKnowledgeBases, type KnowledgeBaseCreatorFilter } from '@weknora/domain';
import { Button, Card, Status } from '@weknora/ui';
import { deleteKnowledgeBase, loadKnowledgeBases, saveKnowledgeBase, type KnowledgeBaseListState } from './knowledge-bases/list.ts';

interface KnowledgeBasesPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
}
export function KnowledgeBasesPage({ client, scopeController }: KnowledgeBasesPageProps) {
  const [reloadToken, setReloadToken] = useState(0);
  const [state, setState] = useState<KnowledgeBaseListState>({ status: 'error', message: 'Loading…' });
  const [editingId, setEditingId] = useState<string | null>(null);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [type, setType] = useState<'document' | 'faq'>('document');
  const [query, setQuery] = useState('');
  const [creator, setCreator] = useState<KnowledgeBaseCreatorFilter>('all');
  const [page, setPage] = useState(1);
  const [saving, setSaving] = useState(false);
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'knowledge-bases'), [scope.scope]);
  const filtered = useMemo(() => state.status === 'success'
    ? filterKnowledgeBases(state.items, { query, creator, currentUserId: scope.scope.userId ?? undefined, page, pageSize: 12 })
    : null, [creator, page, query, scope.scope.userId, state]);

  useEffect(() => {
    let active = true;
    setState({ status: 'error', message: 'Loading…' });
    void loadKnowledgeBases(client, scope.signal, { creator }).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setState(next);
    });
    return () => { active = false; };
  }, [client, creator, reloadToken, scopeController, scope.scope, scope.signal]);

  useEffect(() => { setPage(1); }, [creator, query]);

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMutationError(null);
    setSaving(true);
    try {
      await saveKnowledgeBase(client, editingId, { name, type });
      setName('');
      setEditingId(null);
      setType('document');
      setReloadToken((value) => value + 1);
    } catch (error) { setMutationError(error instanceof Error ? error.message : 'Save failed'); }
    finally { setSaving(false); }
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
        <div className="wk-toolbar" role="search">
          <label>Search <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Name or description" /></label>
          <label>Creator <select value={creator} onChange={(event) => setCreator(event.target.value as KnowledgeBaseCreatorFilter)}><option value="all">All</option><option value="mine">Mine</option><option value="others">Others</option></select></label>
        </div>
        <form className="wk-form" onSubmit={save}>
          <label>Knowledge base name <input value={name} onChange={(event) => setName(event.target.value)} required /></label>
          <label>Type <select value={type} onChange={(event) => setType(event.target.value as 'document' | 'faq')}><option value="document">Document</option><option value="faq">FAQ</option></select></label>
          <Button type="submit" loading={saving}>{editingId ? 'Save changes' : 'Create knowledge base'}</Button>
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
          !filtered || filtered.total === 0 ? <Status>No knowledge bases match the current filters.</Status> : (
            <>
            <ul className="wk-list">
              {filtered.items.map((item) => <li key={item.id}><div className="wk-list-item-copy"><strong>{item.name}</strong><span>{item.type === 'faq' ? 'FAQ' : 'Document'}{item.source ? ` · ${String(item.source)}` : ''}{item.permission ? ` · ${String(item.permission)}` : ''}</span></div><div className="wk-list-item-actions"><Button type="button" onClick={() => { setEditingId(item.id); setName(item.name); setType(item.type === 'faq' ? 'faq' : 'document'); }}>Edit</Button><Button type="button" onClick={() => void remove(item.id)}>Delete</Button></div></li>)}
            </ul>
            {filtered.pageCount > 1 ? <nav className="wk-pagination" aria-label="Knowledge base pages"><Button type="button" disabled={filtered.page <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))}>Previous</Button><span>Page {filtered.page} of {filtered.pageCount}</span><Button type="button" disabled={filtered.page >= filtered.pageCount} onClick={() => setPage((value) => value + 1)}>Next</Button></nav> : null}
            </>
          )
        ) : null}
      </Card>
    </main>
  );
}
