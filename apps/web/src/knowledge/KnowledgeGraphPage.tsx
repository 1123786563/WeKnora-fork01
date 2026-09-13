import { useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent, type WheelEvent as ReactWheelEvent } from 'react';
import type { WikiGraphData, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { filterGraphNodes, graphFrontierNodes, graphQueryParams, layoutGraphNodes, mergeGraphData, type GraphViewport, WIKI_GRAPH_TYPES, zoomGraphViewport } from './graph.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';

export function KnowledgeGraphPage({ client, knowledgeBaseId, slug }: { client: WeKnoraClient; knowledgeBaseId: string; slug?: string }) {
  const t = createTranslator(useAppLocale());

  const [graph, setGraph] = useState<WikiGraphData | null>(null);
  const [status, setStatus] = useState<{ kind: 'loading' | 'success' | 'error'; message?: string }>({ kind: 'loading' });
  const [mode, setMode] = useState<'overview' | 'ego'>(() => slug ? 'ego' : 'overview');
  const [center, setCenter] = useState(slug ?? '');
  const [depth, setDepth] = useState(1);
  const [query, setQuery] = useState('');
  const [selectedTypes, setSelectedTypes] = useState<string[]>(() => [...WIKI_GRAPH_TYPES]);
  const [searchResults, setSearchResults] = useState<Array<{ title: string; slug: string }>>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [drawerNode, setDrawerNode] = useState<{ slug: string; title: string; page_type: string; link_count: number } | null>(null);
  const [drawerPage, setDrawerPage] = useState<{ title: string; summary: string; content: string; version: number } | null>(null);
  const [drawerStatus, setDrawerStatus] = useState<'idle' | 'loading' | 'error'>('idle');
  const [viewport, setViewport] = useState<GraphViewport>({ x: 0, y: 0, scale: 1 });
  const [dragPositions, setDragPositions] = useState<Record<string, { x: number; y: number }>>({});
  const gesture = useRef<{ kind: 'pan' | 'node'; pointerId: number; startX: number; startY: number; originX: number; originY: number; slug?: string } | null>(null);
  const dragged = useRef(false);

  async function load(nextMode: 'overview' | 'ego', nextCenter?: string) {
    setStatus({ kind: 'loading' });
    try {
      const result = await client.wiki.graph(knowledgeBaseId, {
        ...graphQueryParams(nextMode, nextCenter ?? '', depth, selectedTypes),
      });
      setGraph(result);
      setMode(nextMode);
      setCenter(nextCenter ?? '');
      setStatus({ kind: 'success' });
    } catch (error) {
      setStatus({ kind: 'error', message: error instanceof Error ? error.message : t('knowledgeBase.graph.loadFailed') });
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

  async function bloomNeighbors(anchorSlug: string) {
    if (!anchorSlug || !graph) return;
    if (mode !== 'ego') {
      await load('ego', anchorSlug);
      return;
    }
    setStatus({ kind: 'loading' });
    try {
      const incoming = await client.wiki.graph(knowledgeBaseId, graphQueryParams('ego', anchorSlug, depth, selectedTypes));
      setGraph(mergeGraphData(graph, incoming));
      setStatus({ kind: 'success' });
    } catch (error) {
      setStatus({ kind: 'error', message: error instanceof Error ? error.message : t('knowledgeBase.graph.loadFailed') });
    }
  }

  async function growFrontier() {
    const candidates = graphFrontierNodes(graph, center);
    if (!graph || candidates.length === 0) return;
    setStatus({ kind: 'loading' });
    try {
      const incoming = await Promise.all(candidates.map((node) => client.wiki.graph(knowledgeBaseId, graphQueryParams('ego', node.slug, depth, selectedTypes))));
      setGraph(incoming.reduce((current, next) => mergeGraphData(current, next), graph));
      setStatus({ kind: 'success' });
    } catch (error) {
      setStatus({ kind: 'error', message: error instanceof Error ? error.message : t('knowledgeBase.graph.loadFailed') });
    }
  }

  useEffect(() => { void load(mode, mode === 'ego' ? center : undefined); }, [client, knowledgeBaseId, selectedTypes, depth]);
  useEffect(() => {
    const keyword = query.trim();
    if (keyword.length < 2) { setSearchResults([]); setSearchLoading(false); return; }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      setSearchLoading(true);
      void client.wiki.list(knowledgeBaseId, { page: 1, page_size: 20, keyword }).then((result) => {
        if (!cancelled) setSearchResults(result.pages.map((page) => ({ title: page.title, slug: page.slug })));
      }).catch(() => {
        if (!cancelled) setSearchResults([]);
      }).finally(() => { if (!cancelled) setSearchLoading(false); });
    }, 250);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [client, knowledgeBaseId, query]);

  useEffect(() => {
    if (!drawerNode) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setDrawerNode(null);
        setDrawerPage(null);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [drawerNode]);

  const visible = useMemo(() => graph ? filterGraphNodes(graph, { query, types: selectedTypes }) : null, [graph, query, selectedTypes]);
  const positions = useMemo(() => visible ? layoutGraphNodes(visible.nodes, 760, 420) : [], [visible]);
  const displayPositions = useMemo(() => positions.map((position) => ({ ...position, ...dragPositions[position.slug] })), [positions, dragPositions]);
  const positionBySlug = useMemo(() => new Map(displayPositions.map((position) => [position.slug, position])), [displayPositions]);
  const toggleGraphType = (graphType: string) => {
    setSelectedTypes((current) => current.includes(graphType) ? current.filter((item) => item !== graphType) : [...current, graphType]);
  };
  const graphTypeLabel = (graphType: string) => {
    const labels: Record<string, string> = {
      summary: t('wikiBrowser.filterSummary'),
      entity: t('wikiBrowser.filterEntity'),
      concept: t('wikiBrowser.filterConcept'),
      synthesis: t('wikiBrowser.filterSynthesis'),
      comparison: t('wikiBrowser.filterComparison'),
      index: t('wikiBrowser.indexTitle'),
    };
    return labels[graphType] ?? graphType;
  };
  const frontier = useMemo(() => graphFrontierNodes(graph, center), [graph, center]);
  const graphStatusCard = useMemo(() => {
    if (!graph || status.kind !== 'success') return null;
    const isEgo = graph.meta.mode === 'ego';
    const centerTitle = graph.meta.center ? graph.nodes.find((node) => node.slug === graph.meta.center)?.title ?? graph.meta.center : '';
    return {
      title: t(isEgo ? 'wikiBrowser.cardEgoTitle' : 'wikiBrowser.cardOverviewTitle'),
      primary: isEgo ? centerTitle : t('wikiBrowser.cardOverviewPrimary', { returned: graph.nodes.length, total: graph.meta.total }),
      secondary: graph.meta.truncated ? t('wikiBrowser.cardOverviewHintTruncated') : isEgo ? t('wikiBrowser.cardRelatedNodes', { count: graph.nodes.length }) : t('wikiBrowser.cardOverviewHintFull'),
    };
  }, [graph, status.kind, t]);

  function svgPoint(event: { clientX: number; clientY: number; currentTarget: SVGElement }) {
    const svg = event.currentTarget instanceof SVGSVGElement ? event.currentTarget : event.currentTarget.ownerSVGElement;
    if (!svg) return { x: 0, y: 0 };
    const rect = svg.getBoundingClientRect();
    return { x: ((event.clientX - rect.left) / rect.width) * 760, y: ((event.clientY - rect.top) / rect.height) * 420 };
  }

  function beginPan(event: ReactPointerEvent<SVGSVGElement>) {
    if (event.target !== event.currentTarget) return;
    const point = svgPoint(event);
    gesture.current = { kind: 'pan', pointerId: event.pointerId, startX: point.x, startY: point.y, originX: viewport.x, originY: viewport.y };
    dragged.current = false;
    event.currentTarget.setPointerCapture(event.pointerId);
  }

  function beginNodeDrag(event: ReactPointerEvent<SVGGElement>, slug: string) {
    const point = svgPoint(event);
    const position = positionBySlug.get(slug);
    if (!position) return;
    gesture.current = { kind: 'node', pointerId: event.pointerId, startX: point.x, startY: point.y, originX: position.x, originY: position.y, slug };
    dragged.current = false;
    event.stopPropagation();
    (event.currentTarget.ownerSVGElement ?? event.currentTarget).setPointerCapture(event.pointerId);
  }

  function moveGraphGesture(event: ReactPointerEvent<SVGSVGElement>) {
    const current = gesture.current;
    if (!current || current.pointerId !== event.pointerId) return;
    const point = svgPoint(event);
    const dx = point.x - current.startX;
    const dy = point.y - current.startY;
    if (Math.abs(dx) + Math.abs(dy) > 3) dragged.current = true;
    if (current.kind === 'pan') setViewport((value) => ({ ...value, x: current.originX + dx, y: current.originY + dy }));
    else if (current.slug) setDragPositions((value) => ({ ...value, [current.slug!]: { x: current.originX + dx / viewport.scale, y: current.originY + dy / viewport.scale } }));
  }

  function endGraphGesture(event: ReactPointerEvent<SVGSVGElement>) {
    if (gesture.current?.pointerId === event.pointerId) gesture.current = null;
  }

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Knowledge base · {knowledgeBaseId}</p>
          <h1>{t('knowledgeBase.graph.title')}</h1>
          <p className="wk-muted">{t('wikiBrowser.tabGraphTip')}</p>
        </div>
        <div className="wk-list-actions">
          {mode === 'ego' ? <Button type="button" onClick={() => void load('overview')}>{t('wikiBrowser.backToOverview')}</Button> : null}
          {frontier.length > 0 ? <Button type="button" onClick={() => void growFrontier()} disabled={status.kind === 'loading'} title={t('wikiBrowser.growFrontierTitle', { count: frontier.length })}>{t('wikiBrowser.growFrontier', { count: frontier.length })}</Button> : null}
          <Button type="button" onClick={() => setViewport({ x: 0, y: 0, scale: 1 })}>{t('wikiBrowser.fitView')}</Button>
          <Button type="button" onClick={() => void load(mode, center || undefined)} disabled={status.kind === 'loading'}>{t('common.refresh')}</Button>
        </div>
      </header>
      <Card>
        <div className="wk-toolbar" role="search">
          {status.kind === 'success' ? <><label>{t('wikiBrowser.page.search')} <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('wikiBrowser.searchPlaceholder')} aria-autocomplete="list" aria-controls="wk-graph-search-results" />{searchLoading ? <Status>{t('wikiBrowser.loading')}</Status> : null}{searchResults.length > 0 ? <ul id="wk-graph-search-results" className="wk-graph-search-results" aria-label={t('wikiBrowser.page.search')}>{searchResults.map((result) => <li key={result.slug}><button type="button" onClick={() => { setQuery(result.slug); void openNode({ slug: result.slug, title: result.title, page_type: 'page', link_count: 0 }); void load('ego', result.slug); }}>{result.title}<span>{result.slug}</span></button></li>)}</ul> : null}</label>
          <label>{t('knowledgeBase.graph.depth')} <select value={String(depth)} onChange={(event) => setDepth(Number(event.target.value))}><option value="1">1</option><option value="2">2</option><option value="3">3</option></select></label>
          <div className="wk-graph-type-filters" role="group" aria-label={t('knowledgeBase.graph.type')}>
            {WIKI_GRAPH_TYPES.map((graphType) => <button key={graphType} type="button" className={selectedTypes.includes(graphType) ? 'is-selected' : ''} aria-pressed={selectedTypes.includes(graphType)} onClick={() => toggleGraphType(graphType)}><span className={`wk-graph-legend-dot is-${graphType}`} aria-hidden="true" />{graphTypeLabel(graphType)}</button>)}
          </div>
          <details className="wk-graph-help">
            <summary>{t('wikiBrowser.helpButtonTitle')}</summary>
            <dl>
              <div><dt>{t('wikiBrowser.helpClickAction')}</dt><dd>{t('wikiBrowser.helpClickDesc')}</dd></div>
              <div><dt>{t('wikiBrowser.helpDblClickAction')}</dt><dd>{t('wikiBrowser.helpDblClickDesc')}</dd></div>
              <div><dt>{t('wikiBrowser.helpShiftClickAction')}</dt><dd>{t('wikiBrowser.helpShiftClickDesc')}</dd></div>
              <div><dt>{t('wikiBrowser.helpDragAction')}</dt><dd>{t('wikiBrowser.helpDragDesc')}</dd></div>
              <div><dt>{t('wikiBrowser.helpZoomAction')}</dt><dd>{t('wikiBrowser.helpZoomDesc')}</dd></div>
            </dl>
          </details></> : null}
        </div>
        {status.kind === 'loading' ? <Status>{t('wikiBrowser.graphEmpty')}</Status> : null}
        {status.kind === 'error' ? <><Status tone="error">{status.message}</Status><Button type="button" onClick={() => void load(mode, center || undefined)}>{t('common.retry')}</Button></> : null}
        {status.kind === 'success' && visible ? <>
          {graphStatusCard ? <div className="wk-graph-status-card"><div className="wk-graph-status-card-header"><span aria-hidden="true">◌</span><strong>{graphStatusCard.title}</strong></div><div className="wk-graph-status-card-primary">{graphStatusCard.primary}</div><div className="wk-graph-status-card-secondary">{graphStatusCard.secondary}</div></div> : null}
          {visible.nodes.length === 0 ? <Status>{t('wikiBrowser.graphNoData')}</Status> : <>
            <svg className="wk-knowledge-graph" viewBox="0 0 760 420" role="img" aria-label={t('knowledgeBase.graph.ariaLinks')} onPointerDown={beginPan} onPointerMove={moveGraphGesture} onPointerUp={endGraphGesture} onPointerCancel={endGraphGesture} onWheel={(event: ReactWheelEvent<SVGSVGElement>) => { event.preventDefault(); const point = svgPoint(event); setViewport((value) => zoomGraphViewport(value, event.deltaY < 0 ? 1.15 : 0.87, point)); }}>
              <g transform={`translate(${viewport.x} ${viewport.y}) scale(${viewport.scale})`}>
                {visible.edges.map((edge) => { const source = positionBySlug.get(edge.source); const target = positionBySlug.get(edge.target); return source && target ? <line key={`${edge.source}-${edge.target}`} x1={source.x} y1={source.y} x2={target.x} y2={target.y} className="wk-knowledge-graph-edge" /> : null; })}
                {visible.nodes.map((node, index) => { const position = displayPositions[index]!; return <g key={node.slug} className={`wk-knowledge-graph-node is-${node.page_type}${node.familiar ? ' is-familiar' : ''}`} role="button" tabIndex={0} aria-label={`${node.title} · ${node.slug}`} onPointerDown={(event) => beginNodeDrag(event, node.slug)} onClick={() => { if (!dragged.current) void openNode(node); }} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); void openNode(node); } }}><circle cx={position.x} cy={position.y} r={Math.min(24, 10 + Math.log2(node.link_count + 1) * 4)} /><text x={position.x} y={position.y + 40} textAnchor="middle">{node.title.length > 24 ? `${node.title.slice(0, 23)}…` : node.title}</text></g>; })}
              </g>
            </svg>
            <ul className="wk-list" aria-label={t('wikiBrowser.tabGraph')}>{visible.nodes.map((node) => <li key={node.slug}><div className="wk-list-item-copy"><strong>{node.title}</strong><span>{node.slug} · {node.page_type} · {node.link_count} {t('knowledgeBase.graph.links')}{node.familiar ? ` · ${t('wikiBrowser.legendFamiliar')}` : ''}</span></div><Button type="button" onClick={() => { void openNode(node); void load('ego', node.slug); }}>{t('wikiBrowser.expandNeighbors')}</Button></li>)}</ul>
            {visible.nodes.some((node) => node.familiar) ? <p className="wk-graph-familiar-legend"><span className="wk-graph-familiar-ring" aria-hidden="true" />{t('wikiBrowser.legendFamiliar')}</p> : null}
          </>}
        </> : null}
      </Card>
      {drawerNode ? <aside className="wk-graph-drawer" aria-label={drawerNode.title} role="dialog" aria-modal="true"><div className="wk-header"><div><h2>{drawerNode.title}</h2><p className="wk-muted">{drawerNode.page_type} · {drawerNode.link_count} {t('knowledgeBase.graph.links')}</p></div><Button type="button" onClick={() => { setDrawerNode(null); setDrawerPage(null); }}>{t('common.close')}</Button></div>{drawerStatus === 'loading' ? <Status>{t('wikiBrowser.loading')}</Status> : null}{drawerStatus === 'error' ? <Status tone="error">{t('wikiBrowser.revisionLoadFailed')}</Status> : null}{drawerPage ? <><p className="wk-muted">{drawerPage.summary || '—'} · {t('wikiBrowser.version', { ver: drawerPage.version })}</p><pre className="wk-graph-drawer-content">{drawerPage.content}</pre><div className="wk-list-actions"><Button type="button" onClick={() => void bloomNeighbors(drawerNode.slug)}>{t('wikiBrowser.bloomNeighbors')}</Button><Button type="button" onClick={() => void load('ego', drawerNode.slug)}>{t('wikiBrowser.expandNeighbors')}</Button></div></> : null}</aside> : null}
    </main>
  );
}
