import { useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent, type WheelEvent as ReactWheelEvent } from 'react';
import type { WikiGraphData, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Input, Status } from '@weknora/ui';
import { renderChatMarkdown } from '@weknora/views';
import { displayGraphEdges, filterGraphNodes, graphFrontierNodes, graphQueryParams, layoutGraphNodes, mergeGraphData, type GraphViewport, WIKI_GRAPH_TYPES, zoomGraphViewport } from './graph.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';

/* Tailwind migration: static per-type classes replacing the former
   .wk-graph-legend-dot.is-* / .wk-knowledge-graph-node.is-* css rules. */
const GRAPH_TYPE_DOT_BG: Record<string, string> = {
  summary: 'bg-[#0052d9]',
  entity: 'bg-[#2ba471]',
  concept: 'bg-[#e37318]',
  synthesis: 'bg-[#0594fa]',
  comparison: 'bg-[#d54941]',
  index: 'bg-[#7a5af8]',
};

const GRAPH_NODE_CIRCLE: Record<string, string> = {
  summary: '[fill:#0052d9] [stroke:#003b9a]',
  entity: '[fill:#2ba471] [stroke:#147d54]',
  concept: '[fill:#e37318] [stroke:#b4570d]',
  synthesis: '[fill:#0594fa] [stroke:#0575c5]',
  comparison: '[fill:#d54941] [stroke:#a52f29]',
  index: '[fill:#7a5af8] [stroke:#5b3ec4]',
};

export function KnowledgeGraphPage({ client, knowledgeBaseId, slug }: { client: WeKnoraClient; knowledgeBaseId: string; slug?: string }) {
  const t = createTranslator(useAppLocale());

  const [graph, setGraph] = useState<WikiGraphData | null>(null);
  const [status, setStatus] = useState<{ kind: 'loading' | 'success' | 'error'; message?: string }>({ kind: 'loading' });
  const [mode, setMode] = useState<'overview' | 'ego'>(() => slug ? 'ego' : 'overview');
  const [center, setCenter] = useState(slug ?? '');
  // Vue WikiBrowser keeps the ego depth internal; there is no visible depth control.
  const depth = 1;
  const [query, setQuery] = useState('');
  const [selectedTypes, setSelectedTypes] = useState<string[]>(() => [...WIKI_GRAPH_TYPES]);
  const [showArrows, setShowArrows] = useState(true);
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

  useEffect(() => { void load(mode, mode === 'ego' ? center : undefined); }, [client, knowledgeBaseId, selectedTypes]);
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
  const displayEdges = useMemo(() => visible ? displayGraphEdges(visible.edges) : [], [visible]);
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
    <main className="wk-page mx-auto box-border max-w-[960px] px-[1.25rem] py-12">
      <header className="wk-header mb-6 flex items-start justify-between gap-4">
        <div>
          <p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">{t('common.knowledgeBases')} · {knowledgeBaseId}</p>
          <h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{t('knowledgeBase.graph.title')}</h1>
          <p className="wk-muted text-muted">{t('wikiBrowser.tabGraphTip')}</p>
        </div>
        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
          {mode === 'ego' ? <Button type="button" onClick={() => void load('overview')}>{t('wikiBrowser.backToOverview')}</Button> : null}
          {frontier.length > 0 ? <Button type="button" onClick={() => void growFrontier()} disabled={status.kind === 'loading'} title={t('wikiBrowser.growFrontierTitle', { count: frontier.length })}>{t('wikiBrowser.growFrontier', { count: frontier.length })}</Button> : null}
          <Button type="button" onClick={() => setViewport({ x: 0, y: 0, scale: 1 })}>{t('wikiBrowser.fitView')}</Button>
          <Button type="button" variant="text" size="small" aria-pressed={showArrows} onClick={() => setShowArrows((value) => !value)} title={showArrows ? t('wikiBrowser.hideArrows') : t('wikiBrowser.showArrows')}>
            {showArrows ? t('wikiBrowser.hideArrows') : t('wikiBrowser.showArrows')}
          </Button>
          <Button type="button" onClick={() => void load(mode, center || undefined)} disabled={status.kind === 'loading'}>{t('common.refresh')}</Button>
        </div>
      </header>
      <Card className="relative overflow-hidden p-0">
        <div data-testid="knowledge-graph-surface" className="relative min-h-[32rem] p-4 max-[720px]:min-h-[26rem]">
        <div data-testid="knowledge-graph-legend" className="wk-toolbar mb-4 flex flex-wrap items-end gap-3 max-[720px]:gap-2" role="search">
          {status.kind === 'success' ? <><label className="relative grid gap-1">{t('wikiBrowser.page.search')} <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('wikiBrowser.searchPlaceholder')} aria-autocomplete="list" aria-controls="wk-graph-search-results" className="rounded-control border border-line-strong p-[0.55rem]" />{searchLoading ? <Status>{t('wikiBrowser.loading')}</Status> : null}{searchResults.length > 0 ? <ul id="wk-graph-search-results" className="absolute z-[3] m-[.35rem_0_0] max-h-[14rem] list-none overflow-auto rounded-[6px] border border-[#d8e0eb] bg-white p-[.25rem] shadow-[0_8px_20px_rgba(31,52,84,.12)] w-[min(24rem,100%)]" aria-label={t('wikiBrowser.page.search')}>{searchResults.map((result) => <li key={result.slug}><button type="button" className="flex w-full cursor-pointer flex-col items-start gap-[.15rem] rounded-[4px] border-0 bg-transparent p-[.5rem_.6rem] text-left text-[#27364d] hover:bg-[#eef5ff] hover:outline-none focus-visible:bg-[#eef5ff] focus-visible:outline-none" onClick={() => { setQuery(result.slug); void openNode({ slug: result.slug, title: result.title, page_type: 'page', link_count: 0 }); void load('ego', result.slug); }}>{result.title}<span className="text-[.75rem] text-[#718096]">{result.slug}</span></button></li>)}</ul> : null}</label>
          <div className="flex flex-wrap items-center gap-[.35rem]" role="group" aria-label={t('knowledgeBase.graph.type')}>
            {WIKI_GRAPH_TYPES.map((graphType) => <button key={graphType} type="button" className="inline-flex cursor-pointer items-center gap-[.35rem] rounded-pill border border-[#d8e0eb] bg-white p-[.35rem_.6rem] text-[#52627a] hover:border-[#2e6de6] hover:text-[#1849a9] focus-visible:border-[#2e6de6] focus-visible:text-[#1849a9] aria-pressed:bg-[#eef4ff] aria-pressed:border-[#2e6de6] aria-pressed:text-[#1849a9]" aria-pressed={selectedTypes.includes(graphType)} onClick={() => toggleGraphType(graphType)}><span className={`inline-block size-[.55rem] rounded-full ${GRAPH_TYPE_DOT_BG[graphType] ?? 'bg-[#98a2b3]'}`} aria-hidden="true" />{graphTypeLabel(graphType)}</button>)}
          </div>
          <details className="mt-2 text-[.875rem] text-[#52627a]">
            <summary className="inline-block cursor-pointer">{t('wikiBrowser.helpButtonTitle')}</summary>
            <dl className="mt-2 min-w-[15rem] max-w-[20rem] rounded-[6px] border border-[#d8e0eb] bg-white p-[.65rem_.8rem] shadow-[0_8px_20px_rgba(31,52,84,.12)]">
              <div className="mb-2 select-none text-[11px] font-normal uppercase leading-[14px] tracking-[.04em] text-muted">{t('wikiBrowser.helpTitle')}</div>
              <div className="flex flex-col gap-[6px]">
                <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-foreground">{t('wikiBrowser.helpClickAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpClickDesc')}</dd></div>
                <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-foreground">{t('wikiBrowser.helpDblClickAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpDblClickDesc')}</dd></div>
                <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-foreground">{t('wikiBrowser.helpShiftClickAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpShiftClickDesc')}</dd></div>
                <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-foreground">{t('wikiBrowser.helpHoverPlusAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpHoverPlusDesc')}</dd></div>
                <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-foreground">{t('wikiBrowser.helpDragAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpDragDesc')}</dd></div>
                <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-foreground">{t('wikiBrowser.helpPanAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpPanDesc')}</dd></div>
                <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-foreground">{t('wikiBrowser.helpZoomAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpZoomDesc')}</dd></div>
              </div>
            </dl>
          </details></> : null}
        </div>
        {status.kind === 'loading' ? <Status>{t('wikiBrowser.graphEmpty')}</Status> : null}
        {status.kind === 'error' ? <><Status tone="error">{status.message}</Status><Button type="button" onClick={() => void load(mode, center || undefined)}>{t('common.retry')}</Button></> : null}
        {status.kind === 'success' && visible ? <>
          {graphStatusCard ? <div className="my-[.75rem] max-w-[28rem] rounded-[8px] border border-[#d8e0eb] bg-white p-[.75rem_.9rem] shadow-[0_8px_20px_rgba(31,52,84,.1)]"><div className="flex items-center gap-[.4rem] text-[.8rem] text-[#52627a]"><span aria-hidden="true">◌</span><strong>{graphStatusCard.title}</strong></div><div className="mt-[.25rem] truncate text-[1rem] font-semibold text-[#1d2939]">{graphStatusCard.primary}</div><div className="mt-[.25rem] text-[.8rem] text-[#667085]">{graphStatusCard.secondary}</div></div> : null}
          {visible.nodes.length === 0 ? <Status>{t('wikiBrowser.graphNoData')}</Status> : <>
            <svg className="block w-full min-h-[24rem] max-[720px]:min-h-[18rem] my-4 border border-line rounded-card bg-[#fbfcfe] cursor-grab touch-none select-none active:cursor-grabbing" viewBox="0 0 760 420" role="img" aria-label={t('knowledgeBase.graph.ariaLinks')} onPointerDown={beginPan} onPointerMove={moveGraphGesture} onPointerUp={endGraphGesture} onPointerCancel={endGraphGesture} onWheel={(event: ReactWheelEvent<SVGSVGElement>) => { event.preventDefault(); const point = svgPoint(event); setViewport((value) => zoomGraphViewport(value, event.deltaY < 0 ? 1.15 : 0.87, point)); }}>
              <defs>
                <marker id="wk-graph-arrow-end" viewBox="0 0 10 6" refX="10" refY="3" markerWidth="8" markerHeight="6" orient="auto">
                  <path d="M0,0 L10,3 L0,6 L2,3 Z" className="fill-[#c0c4cc]" />
                </marker>
                <marker id="wk-graph-arrow-start" viewBox="0 0 10 6" refX="0" refY="3" markerWidth="8" markerHeight="6" orient="auto">
                  <path d="M10,0 L0,3 L10,6 L8,3 Z" className="fill-[#c0c4cc]" />
                </marker>
              </defs>
              <g transform={`translate(${viewport.x} ${viewport.y}) scale(${viewport.scale})`}>
                {displayEdges.map((edge) => { const source = positionBySlug.get(edge.source); const target = positionBySlug.get(edge.target); return source && target ? <line key={`${edge.source}-${edge.target}`} x1={source.x} y1={source.y} x2={target.x} y2={target.y} markerEnd={showArrows ? 'url(#wk-graph-arrow-end)' : undefined} markerStart={showArrows && edge.bidirectional ? 'url(#wk-graph-arrow-start)' : undefined} className="stroke-[#c0c4cc] [stroke-width:1.2] [stroke-opacity:0.4]" /> : null; })}
                {visible.nodes.map((node, index) => { const position = displayPositions[index]!; const radius = Math.min(24, 10 + Math.log2(node.link_count + 1) * 4); const circleClass = `${GRAPH_NODE_CIRCLE[node.page_type] ?? '[fill:#98a2b3] [stroke:#667085]'} [stroke-width:2]`; const expandable = frontier.some((item) => item.slug === node.slug); return <g key={node.slug} className="cursor-pointer outline-none [&:hover_circle]:[fill:#6941c6] [&:focus_circle]:[fill:#6941c6]" role="button" tabIndex={0} aria-label={`${node.title} · ${node.slug}`} onPointerDown={(event) => beginNodeDrag(event, node.slug)} onClick={() => { if (!dragged.current) void openNode(node); }} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); void openNode(node); } }}><circle cx={position.x} cy={position.y} r={radius} className={circleClass} />{expandable ? <circle cx={position.x} cy={position.y} r={radius + 7} className="wk-graph-expansion-ring [fill:none] [stroke:#0052d9] [stroke-width:1.5] [stroke-dasharray:4_3]" aria-hidden="true" /> : null}{node.familiar ? <circle cx={position.x} cy={position.y} r={radius + 7} className="wk-graph-familiar-ring [fill:none] [stroke:#0052d9] [stroke-width:2]" aria-hidden="true" /> : null}<text x={position.x} y={position.y + 40} textAnchor="middle" className="[fill:#27364d] text-[12px] pointer-events-none">{node.title.length > 24 ? `${node.title.slice(0, 23)}…` : node.title}</text></g>; })}
              </g>
            </svg>
            <ul className="wk-list m-0 list-none p-0" aria-label={t('wikiBrowser.tabGraph')}>{visible.nodes.map((node) => <li key={node.slug} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{node.title}</strong><span className="font-mono text-[0.8rem] text-muted">{node.slug} · {node.page_type} · {node.link_count} {t('knowledgeBase.graph.links')}{node.familiar ? ` · ${t('wikiBrowser.legendFamiliar')}` : ''}</span></div><Button type="button" onClick={() => { void openNode(node); void load('ego', node.slug); }}>{t('wikiBrowser.expandNeighbors')}</Button></li>)}</ul>
            {visible.nodes.some((node) => node.familiar) ? <p className="mt-2 flex items-center gap-[.4rem] text-[#52627a]"><span className="inline-block size-[.75rem] rounded-full border-2 border-[#0052d9]" aria-hidden="true" />{t('wikiBrowser.legendFamiliar')}</p> : null}
          </>}
        </> : null}
        </div>
      </Card>
      {drawerNode ? <aside className="mt-4 rounded-[8px] border border-[#d8e0eb] bg-white p-4 shadow-[0_12px_32px_rgba(31,52,84,.14)]" aria-label={drawerNode.title} role="dialog" aria-modal="true"><div className="wk-header mb-[.75rem]! flex items-start justify-between gap-4"><div><h2>{drawerNode.title}</h2><p className="wk-muted text-muted">{drawerNode.page_type} · {drawerNode.link_count} {t('knowledgeBase.graph.links')}</p></div><Button type="button" onClick={() => { setDrawerNode(null); setDrawerPage(null); }}>{t('common.close')}</Button></div>{drawerStatus === 'loading' ? <Status>{t('wikiBrowser.loading')}</Status> : null}{drawerStatus === 'error' ? <Status tone="error">{t('wikiBrowser.revisionLoadFailed')}</Status> : null}{drawerPage ? <><p className="wk-muted text-muted">{drawerPage.summary || '—'} · {t('wikiBrowser.version', { ver: drawerPage.version })}</p><div data-testid="knowledge-graph-reader" className="wk-reader-body max-h-[24rem] overflow-auto leading-[1.6]" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(drawerPage.content) }} /><div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Button type="button" onClick={() => void bloomNeighbors(drawerNode.slug)}>{t('wikiBrowser.bloomNeighbors')}</Button><Button type="button" onClick={() => void load('ego', drawerNode.slug)}>{t('wikiBrowser.expandNeighbors')}</Button></div></> : null}</aside> : null}
    </main>
  );
}
