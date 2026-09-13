import { useEffect, useMemo, useState } from 'react';
import type { WikiPage as WikiPageModel, WikiPageRevision, WeKnoraClient } from '@weknora/api-client';
import { diffWikiRevision } from '@weknora/domain/wiki/diff';
import { Button, Card, Status } from '@weknora/ui';
import { saveWikiPage, type WikiSaveState } from './editor.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { pagerState } from '../pagination.ts';

const WIKI_PAGE_SIZE = 50;

export function WikiPage({ client, knowledgeBaseId, initialSlug }: { client: WeKnoraClient; knowledgeBaseId: string; initialSlug?: string }) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState('');
  const [pages, setPages] = useState<WikiPageModel[]>([]);
  const [pageTotal, setPageTotal] = useState(0);
  const [selected, setSelected] = useState<WikiPageModel | null>(null);
  const [title, setTitle] = useState('');
  const [slug, setSlug] = useState('');
  const [summary, setSummary] = useState('');
  const [content, setContent] = useState('');
  const [version, setVersion] = useState(1);
  const [state, setState] = useState<{ status: 'loading' | 'error' | 'success'; message?: string }>({ status: 'loading' });
  const [saveState, setSaveState] = useState<WikiSaveState | null>(null);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [revisions, setRevisions] = useState<WikiPageRevision[]>([]);
  const [revision, setRevision] = useState<WikiPageRevision | null>(null);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const [reverting, setReverting] = useState(false);

  async function loadPages() {
    setState({ status: 'loading' });
    try {
      // Must-fix #3: server-backed pagination replaces the 50-entry hard cap.
      const response = await client.wiki.list(knowledgeBaseId, { page, page_size: WIKI_PAGE_SIZE, keyword: keyword || undefined });
      setPages(response.pages);
      setPageTotal(response.total ?? response.pages.length);
      const requested = initialSlug?.trim();
      const requestedPage = requested ? response.pages.find((page) => page.slug === requested) : undefined;
      if (requestedPage) choose(requestedPage);
      setState({ status: 'success' });
    }
    catch (error) { setState({ status: 'error', message: error instanceof Error ? error.message : 'Unable to load Wiki pages' }); }
  }
  useEffect(() => { void loadPages(); }, [client, knowledgeBaseId, keyword, initialSlug, page]);
  useEffect(() => { setPage(1); }, [keyword]);

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

  async function openHistory() {
    if (!selected) return;
    setHistoryOpen(true); setHistoryLoading(true); setHistoryError(null); setRevision(null);
    try { setRevisions((await client.wiki.revisions(knowledgeBaseId, selected.slug, { limit: 50, offset: 0 })).revisions); }
    catch (error) { setHistoryError(error instanceof Error ? error.message : 'Unable to load Wiki history'); }
    finally { setHistoryLoading(false); }
  }
  async function chooseRevision(item: WikiPageRevision) {
    setRevision(item); setHistoryLoading(true); setHistoryError(null);
    try { setRevision(await client.wiki.getRevision(knowledgeBaseId, item.slug, item.version)); }
    catch (error) { setHistoryError(error instanceof Error ? error.message : 'Unable to load Wiki revision'); }
    finally { setHistoryLoading(false); }
  }
  async function revertRevision() {
    if (!selected || !revision || !window.confirm(`Revert ${selected.title} to version ${revision.version}?`)) return;
    setReverting(true); setHistoryError(null);
    try { const page = await client.wiki.revert(knowledgeBaseId, selected.slug, revision.version); choose(page); setHistoryOpen(false); setSaveState({ status: 'saved', page }); await loadPages(); }
    catch (error) { setHistoryError(error instanceof Error ? error.message : 'Unable to revert Wiki revision'); }
    finally { setReverting(false); }
  }
  const pager = pagerState(pageTotal, page, WIKI_PAGE_SIZE);
  const revisionDiff = useMemo(() => revision && selected ? diffWikiRevision({ title: revision.title, summary: revision.summary, content: revision.content ?? '' }, { title: selected.title, summary: selected.summary, content: selected.content }) : [], [revision, selected]);

  return <main className="wk-page wk-wiki-page"><header className="wk-header"><div><p className="wk-eyebrow">Knowledge base · {knowledgeBaseId}</p><h1>{t('wikiBrowser.page.title')}</h1><p className="wk-muted">{t('wikiBrowser.page.subtitle')}</p></div><Button type="button" onClick={newPage}>{t('wikiBrowser.page.new')}</Button></header><Card><div className="wk-wiki-layout"><aside className="wk-wiki-sidebar"><div className="wk-wiki-sidebar-header"><label className="wk-wiki-search" role="search"><span className="wk-sr-only">{t('wikiBrowser.page.search')}</span><span aria-hidden="true">⌕</span><input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder={t('wikiBrowser.page.searchPlaceholder')} /></label></div><nav className="wk-wiki-page-list" aria-label="Wiki pages">{pages.map((page) => <button className={`wk-wiki-page-item${selected?.id === page.id ? ' is-active' : ''}`} key={page.id} type="button" onClick={() => choose(page)}><span className="wk-wiki-page-item-title">{page.title}</span><span className="wk-wiki-page-item-summary">{page.summary || '—'}</span><span className="wk-wiki-page-item-meta">v{page.version}</span></button>)}{state.status === 'loading' ? <Status>Loading Wiki pages…</Status> : null}{state.status === 'error' ? <Status tone="error">{state.message}</Status> : null}{state.status === 'success' && pages.length === 0 ? <div className="wk-wiki-empty"><span className="wk-wiki-empty-icon" aria-hidden="true">▧</span><strong>{t('wikiBrowser.emptyTitle')}</strong><span>{t('wikiBrowser.emptyDesc')}</span></div> : null}</nav></aside><form className="wk-wiki-editor" onSubmit={save}><label>Title <input value={title} onChange={(event) => setTitle(event.target.value)} required /></label><label>Slug <input value={slug} onChange={(event) => setSlug(event.target.value)} required disabled={Boolean(selected)} /></label><label>Summary <input value={summary} onChange={(event) => setSummary(event.target.value)} /></label><label>Content <textarea value={content} onChange={(event) => setContent(event.target.value)} rows={14} required /></label><div className="wk-list-actions"><span>{selected ? `Version ${version}` : 'New page'}</span><Button type="submit">{selected ? 'Save version' : 'Create page'}</Button>{selected ? <><Button type="button" onClick={() => void reloadSelected()}>Reload latest</Button><Button type="button" onClick={() => void openHistory()}>History</Button></> : null}</div>{saveState?.status === 'conflict' ? <Status tone="warning">{saveState.message}</Status> : null}{saveState?.status === 'error' ? <Status tone="error">{saveState.message}</Status> : null}{saveState?.status === 'saved' ? <Status tone="success">Saved as version {saveState.page.version}.</Status> : null}</form></div>{pager.total > WIKI_PAGE_SIZE ? <nav className="wk-pagination" aria-label={t('wikiBrowser.page.title')}><Button type="button" disabled={!pager.hasPrevious} onClick={() => setPage((value) => value - 1)}>{t('wikiBrowser.page.previous')}</Button><span>{t('wikiBrowser.page.page', { page: pager.page, total: pager.total })}</span><Button type="button" disabled={!pager.hasNext} onClick={() => setPage((value) => value + 1)}>{t('wikiBrowser.page.next')}</Button></nav> : null}</Card>{historyOpen && selected ? <Card className="wk-wiki-history"><div className="wk-header"><div><h2>Revision history</h2><p className="wk-muted">Current page is version {selected.version}; historical snapshots are immutable.</p></div><Button type="button" onClick={() => setHistoryOpen(false)}>Close</Button></div>{historyError ? <Status tone="error">{historyError}</Status> : null}{historyLoading && !revision ? <Status>Loading revisions…</Status> : null}{!historyLoading && revisions.length === 0 ? <Status>No historical revisions.</Status> : null}<div className="wk-wiki-history-layout"><nav aria-label="Wiki revisions"><ul className="wk-list">{revisions.map((item) => <li key={item.id}><button className="wk-document-link" type="button" onClick={() => void chooseRevision(item)}>Version {item.version}</button><span>{item.edit_source ?? 'user'}</span></li>)}</ul></nav><div>{revision ? <><div className="wk-list-actions"><strong>v{revision.version} → v{selected.version}</strong><Button type="button" onClick={() => void revertRevision()} loading={reverting}>Revert to v{revision.version}</Button></div>{revisionDiff.length === 0 ? <Status>No changes.</Status> : <div className="wk-diff">{revisionDiff.map((section) => <section key={section.field}><h3>{section.field}</h3><pre>{section.lines.map((line, index) => <span key={`${section.field}-${index}`} className={`wk-diff-${line.type}`}>{line.type === 'add' ? '+' : line.type === 'del' ? '-' : ' '}{line.text}{'\n'}</span>)}</pre></section>)}</div>}</> : <Status>Select a revision to inspect its diff.</Status>}</div></div></Card> : null}</main>;
}
