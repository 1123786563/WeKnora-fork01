import { useEffect, useState } from 'react';
import type { WikiPage as WikiPageModel, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { saveWikiPage, type WikiSaveState } from './editor.ts';

export function WikiPage({ client, knowledgeBaseId }: { client: WeKnoraClient; knowledgeBaseId: string }) {
  const [keyword, setKeyword] = useState('');
  const [pages, setPages] = useState<WikiPageModel[]>([]);
  const [selected, setSelected] = useState<WikiPageModel | null>(null);
  const [title, setTitle] = useState('');
  const [slug, setSlug] = useState('');
  const [summary, setSummary] = useState('');
  const [content, setContent] = useState('');
  const [version, setVersion] = useState(1);
  const [state, setState] = useState<{ status: 'loading' | 'error' | 'success'; message?: string }>({ status: 'loading' });
  const [saveState, setSaveState] = useState<WikiSaveState | null>(null);

  async function loadPages() {
    setState({ status: 'loading' });
    try { const response = await client.wiki.list(knowledgeBaseId, { page: 1, page_size: 50, keyword: keyword || undefined }); setPages(response.pages); setState({ status: 'success' }); }
    catch (error) { setState({ status: 'error', message: error instanceof Error ? error.message : 'Unable to load Wiki pages' }); }
  }
  useEffect(() => { void loadPages(); }, [client, knowledgeBaseId, keyword]);

  function choose(page: WikiPageModel) { setSelected(page); setTitle(page.title); setSlug(page.slug); setSummary(page.summary); setContent(page.content); setVersion(page.version); setSaveState(null); }
  function newPage() { setSelected(null); setTitle(''); setSlug(''); setSummary(''); setContent(''); setVersion(1); setSaveState(null); }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected) {
      try { const page = await client.wiki.create(knowledgeBaseId, { title, slug, summary, content, version: 1 }); choose(page); setSaveState({ status: 'saved', page }); await loadPages(); }
      catch (error) { setSaveState({ status: 'error', message: error instanceof Error ? error.message : 'Unable to create Wiki page' }); }
      return;
    }
    const result = await saveWikiPage(client.wiki, knowledgeBaseId, selected.slug, { title, content, summary, version });
    setSaveState(result);
    if (result.status === 'saved') { choose(result.page); await loadPages(); }
  }

  async function reloadSelected() {
    if (!selected) return;
    try { choose(await client.wiki.get(knowledgeBaseId, selected.slug)); }
    catch (error) { setSaveState({ status: 'error', message: error instanceof Error ? error.message : 'Unable to reload Wiki page' }); }
  }

  return <main className="wk-page wk-wiki-page"><header className="wk-header"><div><p className="wk-eyebrow">Knowledge base · {knowledgeBaseId}</p><h1>Wiki</h1><p className="wk-muted">Versioned pages with server-side optimistic concurrency.</p></div><Button type="button" onClick={newPage}>New page</Button></header><Card><div className="wk-toolbar" role="search"><label>Search <input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="Page title or slug" /></label></div><div className="wk-wiki-layout"><nav aria-label="Wiki pages"><ul className="wk-list">{pages.map((page) => <li key={page.id}><button className="wk-document-link" type="button" onClick={() => choose(page)}>{page.title}</button><span>v{page.version}</span></li>)}</ul>{state.status === 'loading' ? <Status>Loading Wiki pages…</Status> : null}{state.status === 'error' ? <Status tone="error">{state.message}</Status> : null}{state.status === 'success' && pages.length === 0 ? <Status>No Wiki pages yet.</Status> : null}</nav><form className="wk-wiki-editor" onSubmit={save}><label>Title <input value={title} onChange={(event) => setTitle(event.target.value)} required /></label><label>Slug <input value={slug} onChange={(event) => setSlug(event.target.value)} required disabled={Boolean(selected)} /></label><label>Summary <input value={summary} onChange={(event) => setSummary(event.target.value)} /></label><label>Content <textarea value={content} onChange={(event) => setContent(event.target.value)} rows={14} required /></label><div className="wk-list-actions"><span>{selected ? `Version ${version}` : 'New page'}</span><Button type="submit">{selected ? 'Save version' : 'Create page'}</Button>{selected ? <Button type="button" onClick={() => void reloadSelected()}>Reload latest</Button> : null}</div>{saveState?.status === 'conflict' ? <Status tone="warning">{saveState.message}</Status> : null}{saveState?.status === 'error' ? <Status tone="error">{saveState.message}</Status> : null}{saveState?.status === 'saved' ? <Status tone="success">Saved as version {saveState.page.version}.</Status> : null}</form></div></Card></main>;
}
