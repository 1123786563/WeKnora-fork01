import { useEffect, useMemo, useState } from 'react';
import type { WikiGraphData, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { filterGraphNodes, graphQueryParams, layoutGraphNodes } from './graph.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';

export function KnowledgeGraphPage({ client, knowledgeBaseId, slug }: { client: WeKnoraClient; knowledgeBaseId: string; slug?: string }) {
  const t = createTranslator(useAppLocale());

  const [graph, setGraph] = useState<WikiGraphData | null>(null);
  const [status, setStatus] = useState<{ kind: 'loading' | 'success' | 'error'; message?: string }>({ kind: 'loading' });
  const [mode, setMode] = useState<'overview' | 'ego'>(() => slug ? 'ego' : 'overview');
  const [center, setCenter] = useState(slug ?? '');
  const [depth, setDepth] = useState(1);
  const [query, setQuery] = useState('');
  const [type, setType] = useState('all');
  const [drawerNode, setDrawerNode] = useState<{ slug: string; title: string; page_type: string; link_count: number } | null>(null);
  const [drawerPage, setDrawerPage] = useState<{ title: string; summary: string; content: string; version: number } | null>(null);
  const [drawerStatus, setDrawerStatus] = useState<'idle' | 'loading' | 'error'>('idle');

  async function load(nextMode: 'overview' | 'ego', nextCenter?: string) {
    setStatus({ kind: 'loading' });
    try {
      const result = await client.wiki.graph(knowledgeBaseId, {
        ...graphQueryParams(nextMode, nextCenter ?? '', depth, type),
      });
      setGraph(result);
      setMode(nextMode);
      setCenter(nextCenter ?? '');
      setStatus({ kind: 'success' });
    } catch (error) {
      setStatus({ kind: 'error', message: error instanceof Error ? error.message : 'Unable to load the knowledge graph' });
    }
  }

  async function openNode(node: typeof drawerNode) {
    if (!node) return;
    setDrawerNode(node);
    setDrawerPage(null);
    setDrawerStatus('loading');
    try {
      const page = await client.wiki.get(knowledgeBaseId, node.slug);
      setDrawerPage(page);
      setDrawerStatus('idle');
    } catch {
      setDrawerStatus('error');
    }
  }

  useEffect(() => { void load(mode, mode === 'ego' ? center : undefined); }, [client, knowledgeBaseId, type, depth]);

  const visible = useMemo(() => graph ? filterGraphNodes(graph, { query }) : null, [graph, query]);
  const positions = useMemo(() => visible ? layoutGraphNodes(visible.nodes, 760, 420) : [], [visible]);
  const positionBySlug = useMemo(() => new Map(positions.map((position) => [position.slug, position])), [positions]);
  const types = useMemo(() => [...new Set(graph?.nodes.map((node) => node.page_type) ?? [])].sort(), [graph]);

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Knowledge base · {knowledgeBaseId}</p>
          <h1>{t('knowledgeBase.graph.title')}</h1>
          <p className="wk-muted">Server-backed Wiki links with bounded overview and neighbor expansion.</p>
        </div>
        <div className="wk-list-actions">
          {mode === 'ego' ? <Button type="button" onClick={() => void load('overview')}>Back to overview</Button> : null}
          <Button type="button" onClick={() => void load(mode, center || undefined)} disabled={status.kind === 'loading'}>Reload</Button>
        </div>
      </header>
      <Card>
        <div className="wk-toolbar" role="search">
          <label>Find node <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Title or slug" /></label>
          <label>Type <select value={type} onChange={(event) => setType(event.target.value)}><option value="all">All types</option>{types.map((item) => <option key={item} value={item}>{item}</option>)}</select></label>
          <label>Depth <select value={String(depth)} onChange={(event) => setDepth(Number(event.target.value))}><option value="1">1</option><option value="2">2</option><option value="3">3</option></select></label>
        </div>
        {status.kind === 'loading' ? <Status>Loading knowledge graph…</Status> : null}
        {status.kind === 'error' ? <><Status tone="error">{status.message}</Status><Button type="button" onClick={() => void load(mode, center || undefined)}>Try again</Button></> : null}
        {status.kind === 'success' && visible ? <>
          <p className="wk-muted">Showing {visible.nodes.length} of {graph?.meta.total ?? visible.nodes.length} nodes{graph?.meta.truncated ? ' · overview is bounded; expand a node for neighbors' : ''}.</p>
          {visible.nodes.length === 0 ? <Status>No graph nodes match the current filter.</Status> : <>
            <svg className="wk-knowledge-graph" viewBox="0 0 760 420" role="img" aria-label="Knowledge graph links">
              {visible.edges.map((edge) => { const source = positionBySlug.get(edge.source); const target = positionBySlug.get(edge.target); return source && target ? <line key={`${edge.source}-${edge.target}`} x1={source.x} y1={source.y} x2={target.x} y2={target.y} className="wk-knowledge-graph-edge" /> : null; })}
              {visible.nodes.map((node, index) => { const position = positions[index]!; return <g key={node.slug} className="wk-knowledge-graph-node" role="button" tabIndex={0} aria-label={`${node.title} · ${node.slug}`} onClick={() => { void openNode(node); void load('ego', node.slug); }} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { void openNode(node); void load('ego', node.slug); } }}><circle cx={position.x} cy={position.y} r={Math.min(24, 10 + Math.log2(node.link_count + 1) * 4)} /><text x={position.x} y={position.y + 40} textAnchor="middle">{node.title.length > 24 ? `${node.title.slice(0, 23)}…` : node.title}</text></g>; })}
            </svg>
            <ul className="wk-list" aria-label="Knowledge graph nodes">{visible.nodes.map((node) => <li key={node.slug}><div className="wk-list-item-copy"><strong>{node.title}</strong><span>{node.slug} · {node.page_type} · {node.link_count} links{node.familiar ? ' · familiar' : ''}</span></div><Button type="button" onClick={() => { void openNode(node); void load('ego', node.slug); }}>Expand neighbors</Button></li>)}</ul>
          </>}
        </> : null}
      </Card>
      {drawerNode ? <aside className="wk-graph-drawer" aria-label={drawerNode.title} role="dialog"><div className="wk-header"><div><h2>{drawerNode.title}</h2><p className="wk-muted">{drawerNode.page_type} · {drawerNode.link_count} links</p></div><Button type="button" onClick={() => { setDrawerNode(null); setDrawerPage(null); }}>Close</Button></div>{drawerStatus === 'loading' ? <Status>Loading page…</Status> : null}{drawerStatus === 'error' ? <Status tone="error">Unable to load this Wiki page.</Status> : null}{drawerPage ? <><p className="wk-muted">{drawerPage.summary || '—'} · v{drawerPage.version}</p><pre className="wk-graph-drawer-content">{drawerPage.content}</pre><Button type="button" onClick={() => void load('ego', drawerNode.slug)}>Expand neighbors</Button></> : null}</aside> : null}
    </main>
  );
}
