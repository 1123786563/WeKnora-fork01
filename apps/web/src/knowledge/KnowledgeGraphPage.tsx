import { useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent, type WheelEvent as ReactWheelEvent } from 'react';
import type { WikiGraphData, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Input, Status } from '@weknora/ui';
import { renderChatMarkdown } from '@weknora/views';
import { displayGraphEdges, filterGraphNodes, graphFrontierNodes, graphNeighborStatus, graphNodeRadius, graphQueryParams, growGraphFrontier, layoutGraphNodes, mergeGraphData, type GraphViewport, WIKI_GRAPH_TYPES, zoomGraphViewport } from './graph.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';

/* Tailwind migration: static per-type classes replacing the former
   .wk-graph-legend-dot.is-* / .wk-knowledge-graph-node.is-* css rules.
   Colors mirror the Vue WikiBrowser canvas (nodeColorMap / legend dots). */
const GRAPH_TYPE_DOT_BG: Record<string, string> = {
  summary: 'bg-[#0052d9]',
  entity: 'bg-[#2ba471]',
  concept: 'bg-[#e37318]',
  synthesis: 'bg-[#0594fa]',
  comparison: 'bg-[#d54941]',
  index: 'bg-[#8c8c8c]',
};

const GRAPH_NODE_FILL: Record<string, string> = {
  summary: '#0052d9',
  entity: '#2ba471',
  concept: '#e37318',
  synthesis: '#0594fa',
  comparison: '#d54941',
  index: '#8c8c8c',
};

/* Vue legend shows only the five content types; index stays in the
   server-side allow-list (Vue graphFilterTypes seeds it) but has no toggle. */
const LEGEND_GRAPH_TYPES = ['summary', 'entity', 'concept', 'synthesis', 'comparison'] as const;

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
    if (!graph || mode !== 'ego') return;
    setStatus({ kind: 'loading' });
    try {
      const merged = await growGraphFrontier(graph, center, (nodeSlug) => client.wiki.graph(knowledgeBaseId, graphQueryParams('ego', nodeSlug, depth, selectedTypes)));
      if (merged) setGraph(merged);
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
  // Undirected adjacency within the current subgraph (Vue builds this once
  // per render to drive the dashed expansion hint rings).
  const adjacencyBySlug = useMemo(() => {
    const adjacency = new Map<string, Set<string>>();
    if (!visible) return adjacency;
    for (const edge of visible.edges) {
      if (!adjacency.has(edge.source)) adjacency.set(edge.source, new Set());
      if (!adjacency.has(edge.target)) adjacency.set(edge.target, new Set());
      adjacency.get(edge.source)!.add(edge.target);
      adjacency.get(edge.target)!.add(edge.source);
    }
    return adjacency;
  }, [visible]);
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

  // Vue status card: header (icon + mode name), primary (page title in ego /
  // "returned / total" in overview), optional secondary (type badge + count
  // or truncation hint). Related count excludes the ego center itself.
  const graphStatusCard = useMemo(() => {
    if (!graph || status.kind !== 'success') return null;
    const meta = graph.meta;
    if (meta.mode === 'ego' && meta.center) {
      const centerNode = graph.nodes.find((node) => node.slug === meta.center);
      const centerTitle = centerNode?.title || meta.center;
      const typeLabel = centerNode ? graphTypeLabel(centerNode.page_type) : '';
      const relatedCount = Math.max(0, meta.returned - 1);
      const secondaryParts = [typeLabel, t('wikiBrowser.cardRelatedNodes', { count: relatedCount })].filter(Boolean);
      return {
        icon: '◎',
        title: t('wikiBrowser.cardEgoTitle'),
        primary: centerTitle,
        secondary: secondaryParts.join(' · '),
      };
    }
    if (meta.mode === 'overview') {
      return {
        icon: '◌',
        title: t('wikiBrowser.cardOverviewTitle'),
        primary: t('wikiBrowser.cardOverviewPrimary', { returned: meta.returned, total: meta.total }),
        secondary: meta.truncated ? t('wikiBrowser.cardOverviewHintTruncated') : t('wikiBrowser.cardOverviewHintFull'),
      };
    }
    return null;
  }, [graph, status.kind, t]);

  // Legend familiar entry mirrors Vue graphFamiliarCount (meta.familiar_count).
  const familiarCount = graph?.meta.familiar_count ?? (graph ? graph.nodes.filter((node) => node.familiar).length : 0);

  // Drawer neighbor hint (Vue graphDrawerNeighborHint): classify the gap
  // between visible degree and KB-wide link_count for the open drawer page.
  const drawerNeighbor = useMemo(() => drawerNode && graph ? graphNeighborStatus(graph, drawerNode.slug) : null, [drawerNode, graph]);
  const drawerNeighborHint = useMemo(() => {
    if (!drawerNeighbor) return '';
    if (drawerNeighbor.total === 0) return t('wikiBrowser.neighborsNone');
    if (drawerNeighbor.visible >= drawerNeighbor.total) return t('wikiBrowser.neighborsAllShown', { total: drawerNeighbor.total });
    if (drawerNeighbor.isEgoCenter) return t('wikiBrowser.neighborsCenterUnreachable', { visible: drawerNeighbor.visible, total: drawerNeighbor.total, hidden: drawerNeighbor.hidden });
    if (drawerNeighbor.isOverview) return t('wikiBrowser.neighborsOverviewHidden', { visible: drawerNeighbor.visible, total: drawerNeighbor.total, hidden: drawerNeighbor.hidden });
    return t('wikiBrowser.neighborsProgress', { visible: drawerNeighbor.visible, total: drawerNeighbor.total, hidden: drawerNeighbor.hidden });
  }, [drawerNeighbor, t]);

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
      </header>
      <Card className="relative overflow-hidden p-0">
        <div data-testid="knowledge-graph-surface" className="relative min-h-[500px] overflow-hidden bg-white max-[720px]:min-h-[26rem]">
          {status.kind === 'success' && graph && visible && visible.nodes.length > 0 ? (
            <svg className="absolute inset-0 block h-full w-full cursor-grab touch-none select-none active:cursor-grabbing" viewBox="0 0 760 420" role="img" aria-label={t('knowledgeBase.graph.ariaLinks')} onPointerDown={beginPan} onPointerMove={moveGraphGesture} onPointerUp={endGraphGesture} onPointerCancel={endGraphGesture} onWheel={(event: ReactWheelEvent<SVGSVGElement>) => { event.preventDefault(); const point = svgPoint(event); setViewport((value) => zoomGraphViewport(value, event.deltaY < 0 ? 1.15 : 0.87, point)); }}>
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
                {visible.nodes.map((node, index) => {
                  const position = displayPositions[index]!;
                  const radius = graphNodeRadius(node.link_count);
                  const fill = GRAPH_NODE_FILL[node.page_type] ?? '#8c8c8c';
                  // Vue expansion hint ring: dashed, type-colored, only when
                  // undisplayed neighbors remain and this is not the ego center.
                  const neighborCount = adjacencyBySlug.get(node.slug)?.size ?? 0;
                  const isEgoCenter = graph.meta.mode === 'ego' && graph.meta.center === node.slug;
                  const showExpansionRing = Math.max(0, node.link_count - neighborCount) > 0 && !isEgoCenter;
                  return <g key={node.slug} className="group/node cursor-pointer outline-none" role="button" tabIndex={0} aria-label={`${node.title} · ${node.slug}`} onPointerDown={(event) => beginNodeDrag(event, node.slug)} onClick={(event) => { if (event.shiftKey) { void bloomNeighbors(node.slug); return; } if (!dragged.current) void openNode(node); }} onDoubleClick={() => void load('ego', node.slug)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); void openNode(node); } }}>
                    {showExpansionRing ? <circle cx={position.x} cy={position.y} r={radius + 3} className="node-expansion-ring [fill:none] [stroke-width:1.5] [stroke-dasharray:3_3]" style={{ stroke: fill, opacity: 0.55 }} aria-hidden="true" /> : null}
                    {node.familiar ? <circle cx={position.x} cy={position.y} r={radius + 7} className="wk-graph-familiar-ring [fill:none] [stroke:#0052d9] [stroke-width:2]" style={{ opacity: 0.9 }} aria-hidden="true" /> : null}
                    <circle cx={position.x} cy={position.y} r={radius} style={{ fill }} className="[stroke:#fff] [stroke-width:2]" />
                    <text x={position.x} y={position.y + radius + 14} textAnchor="middle" className="pointer-events-none text-[11px] [fill:#66758b]">{node.title.length > 14 ? `${node.title.slice(0, 14)}…` : node.title}</text>
                    {mode === 'ego' && !isEgoCenter && Math.max(0, node.link_count - neighborCount) > 0 ? <g className="node-bloom-btn pointer-events-none opacity-0 transition-opacity group-hover/node:pointer-events-auto group-hover/node:opacity-100" onClick={(event) => { event.stopPropagation(); void bloomNeighbors(node.slug); }}>
                      <circle cx={position.x + Math.SQRT1_2 * (radius + 6)} cy={position.y - Math.SQRT1_2 * (radius + 6)} r={8} className="[fill:#fff] [stroke:#0052d9] [stroke-width:1.5]" />
                      <line x1={position.x + Math.SQRT1_2 * (radius + 6)} x2={position.x + Math.SQRT1_2 * (radius + 6)} y1={position.y - Math.SQRT1_2 * (radius + 6) - 4} y2={position.y - Math.SQRT1_2 * (radius + 6) + 4} className="stroke-[#0052d9] [stroke-width:1.8] [stroke-linecap:round]" />
                      <line x1={position.x + Math.SQRT1_2 * (radius + 6) - 4} x2={position.x + Math.SQRT1_2 * (radius + 6) + 4} y1={position.y - Math.SQRT1_2 * (radius + 6)} y2={position.y - Math.SQRT1_2 * (radius + 6)} className="stroke-[#0052d9] [stroke-width:1.8] [stroke-linecap:round]" />
                    </g> : null}
                  </g>;
                })}
              </g>
            </svg>
          ) : null}
          {status.kind === 'success' ? <div role="search" className="absolute left-4 top-4 z-10 flex w-80 max-w-[calc(100%-2rem)] flex-col gap-3 max-[720px]:left-2 max-[720px]:top-2">
            <div className="flex items-center gap-2">
              <label className="relative grid min-w-0 flex-1 gap-1">{t('wikiBrowser.page.search')} <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('wikiBrowser.searchPlaceholder')} aria-autocomplete="list" aria-controls="wk-graph-search-results" className="rounded-[4px] border border-line-neutral bg-white/95 p-[0.55rem] shadow-[0_1px_10px_rgba(0,0,0,0.05)]" />{searchLoading ? <Status>{t('wikiBrowser.loading')}</Status> : null}{searchResults.length > 0 ? <ul id="wk-graph-search-results" className="absolute z-[3] m-[.35rem_0_0] max-h-[14rem] list-none overflow-auto rounded-[6px] border border-[#d8e0eb] bg-white p-[.25rem] shadow-[0_8px_20px_rgba(31,52,84,.12)] w-[min(24rem,100%)]" aria-label={t('wikiBrowser.page.search')}>{searchResults.map((result) => <li key={result.slug}><button type="button" className="flex w-full cursor-pointer flex-col items-start gap-[.15rem] rounded-[4px] border-0 bg-transparent p-[.5rem_.6rem] text-left text-[#27364d] hover:bg-[#eef5ff] hover:outline-none focus-visible:bg-[#eef5ff] focus-visible:outline-none" onClick={() => { setQuery(result.slug); void openNode({ slug: result.slug, title: result.title, page_type: 'page', link_count: 0 }); void load('ego', result.slug); }}>{result.title}<span className="text-[.75rem] text-[#718096]">{result.slug}</span></button></li>)}</ul> : null}</label>
              <details className="relative shrink-0">
                <summary className="inline-flex size-8 cursor-pointer select-none items-center justify-center text-[18px] text-faint transition-colors hover:text-primary [&::-webkit-details-marker]:hidden" title={t('wikiBrowser.helpButtonTitle')} aria-label={t('wikiBrowser.helpButtonTitle')}>?</summary>
                <dl className="absolute right-0 top-[calc(100%+8px)] z-20 m-0 min-w-[240px] max-w-[320px] rounded-[6px] border border-line-neutral bg-white p-[.65rem_.8rem] shadow-[0_8px_20px_rgba(31,52,84,.12)]">
                  <div className="mb-2 select-none text-[11px] font-normal uppercase leading-[14px] tracking-[.04em] text-faint">{t('wikiBrowser.helpTitle')}</div>
                  <div className="flex flex-col gap-[6px]">
                    <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-ink">{t('wikiBrowser.helpClickAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpClickDesc')}</dd></div>
                    <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-ink">{t('wikiBrowser.helpDblClickAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpDblClickDesc')}</dd></div>
                    <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-ink">{t('wikiBrowser.helpShiftClickAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpShiftClickDesc')}</dd></div>
                    <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-ink">{t('wikiBrowser.helpHoverPlusAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpHoverPlusDesc')}</dd></div>
                    <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-ink">{t('wikiBrowser.helpDragAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpDragDesc')}</dd></div>
                    <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-ink">{t('wikiBrowser.helpPanAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpPanDesc')}</dd></div>
                    <div className="grid grid-cols-[110px_1fr] gap-3 text-[12px] leading-4"><dt className="whitespace-nowrap font-medium text-ink">{t('wikiBrowser.helpZoomAction')}</dt><dd className="m-0 text-muted">{t('wikiBrowser.helpZoomDesc')}</dd></div>
                  </div>
                </dl>
              </details>
            </div>
          </div> : null}
          {status.kind === 'success' ? <div data-testid="knowledge-graph-legend" className="absolute right-4 top-4 z-10 flex flex-col gap-3 rounded-[6px] border border-line-neutral bg-white p-[10px_12px] opacity-95 shadow-[0_1px_10px_rgba(0,0,0,0.05)] transition-all duration-300 max-[720px]:right-2 max-[720px]:top-2" style={drawerNode ? { right: 'calc(480px + 16px)' } : undefined}>
            <div className="flex flex-col gap-2">
              {LEGEND_GRAPH_TYPES.map((graphType) => {
                const enabled = selectedTypes.includes(graphType);
                return <button key={graphType} type="button" className={`flex cursor-pointer items-center gap-2 border-0 bg-transparent p-0 text-left text-[11px] transition-colors ${enabled ? 'text-muted-strong hover:text-ink' : 'text-faint line-through opacity-50'}`} onClick={() => toggleGraphType(graphType)}>
                  <span className={`inline-block size-[10px] shrink-0 rounded-full ${GRAPH_TYPE_DOT_BG[graphType] ?? 'bg-[#98a2b3]'}`} aria-hidden="true" />
                  {graphTypeLabel(graphType)}
                </button>;
              })}
              {familiarCount > 0 ? <div className="flex items-center gap-2 text-[11px] text-muted-strong">
                <span className="inline-block size-[10px] shrink-0 rounded-full border-2 border-[#0052d9] bg-transparent" aria-hidden="true" />
                {t('wikiBrowser.legendFamiliar')}
              </div> : null}
            </div>
            <div className="-mx-3 h-px bg-line-neutral" aria-hidden="true" />
            <div className="flex flex-col gap-2">
              <button type="button" className="flex cursor-pointer select-none items-center gap-1.5 border-0 bg-transparent p-0 text-left text-[11px] leading-[14px] text-muted-strong transition-colors hover:text-primary" title="Fit to View" onClick={() => setViewport({ x: 0, y: 0, scale: 1 })}>
                <span className="inline-flex size-[14px] shrink-0 items-center justify-center text-[13px] text-faint" aria-hidden="true">◎</span>
                <span>{t('wikiBrowser.fitView')}</span>
              </button>
              <button type="button" className="flex cursor-pointer select-none items-center gap-1.5 border-0 bg-transparent p-0 text-left text-[11px] leading-[14px] text-muted-strong transition-colors hover:text-primary" onClick={() => setShowArrows((value) => !value)} aria-pressed={showArrows}>
                <span className="inline-flex size-[14px] shrink-0 items-center justify-center text-[13px] text-faint" aria-hidden="true">{showArrows ? '⌀' : '→'}</span>
                <span>{showArrows ? t('wikiBrowser.hideArrows') : t('wikiBrowser.showArrows')}</span>
              </button>
              {mode === 'ego' && frontier.length > 0 ? <button type="button" className="flex cursor-pointer select-none items-center gap-1.5 border-0 bg-transparent p-0 text-left text-[11px] leading-[14px] text-muted-strong transition-colors hover:text-primary" title={t('wikiBrowser.growFrontierTitle', { count: frontier.length })} onClick={() => void growFrontier()}>
                <span className="inline-flex size-[14px] shrink-0 items-center justify-center text-[13px] text-faint" aria-hidden="true">◌</span>
                <span>{t('wikiBrowser.growFrontier', { count: frontier.length })}</span>
              </button> : null}
              {mode === 'ego' ? <button type="button" className="flex cursor-pointer select-none items-center gap-1.5 border-0 bg-transparent p-0 text-left text-[11px] leading-[14px] text-muted-strong transition-colors hover:text-primary" onClick={() => void load('overview')}>
                <span className="inline-flex size-[14px] shrink-0 items-center justify-center text-[13px] text-faint" aria-hidden="true">↩</span>
                <span>{t('wikiBrowser.backToOverview')}</span>
              </button> : null}
            </div>
            {graphStatusCard ? <div className="flex select-none flex-col gap-1 border-t border-dashed border-line-neutral pt-2">
              <div className="flex items-center gap-1 text-[11px] leading-[14px] text-faint"><span aria-hidden="true">{graphStatusCard.icon}</span><span className="font-medium">{graphStatusCard.title}</span></div>
              <div className="overflow-hidden text-ellipsis whitespace-nowrap text-[12px] leading-4 text-ink" title={graphStatusCard.primary}>{graphStatusCard.primary}</div>
              {graphStatusCard.secondary ? <div className="text-[11px] leading-[14px] text-muted-strong">{graphStatusCard.secondary}</div> : null}
            </div> : null}
          </div> : null}
          {status.kind === 'loading' ? <div className="wiki-graph-empty absolute inset-0 z-20 flex flex-col items-center justify-center bg-white p-[60px_20px] text-center"><Status>{t('wikiBrowser.graphEmpty')}</Status></div> : null}
          {status.kind === 'error' ? <div className="wiki-graph-empty absolute inset-0 z-20 flex flex-col items-center justify-center gap-2 bg-white p-[60px_20px] text-center"><Status tone="error">{status.message}</Status><Button type="button" onClick={() => void load(mode, center || undefined)}>{t('common.retry')}</Button></div> : null}
          {status.kind === 'success' && visible && visible.nodes.length === 0 ? <div className="wiki-graph-empty absolute inset-0 z-20 flex flex-col items-center justify-center bg-white p-[60px_20px] text-center text-muted">
            <span className="mb-4 flex size-16 items-center justify-center rounded-full bg-surface-alt text-[48px] leading-none text-faint" aria-hidden="true">◌</span>
            <p className="m-0 text-[13px] text-faint">{t('wikiBrowser.graphNoData')}</p>
          </div> : null}
        </div>
      </Card>
      {drawerNode ? <aside className="fixed right-0 top-0 z-40 h-full w-[min(480px,100%)] overflow-auto border-l border-line-neutral bg-white p-4 shadow-[-4px_0_16px_rgba(0,0,0,0.08)]" aria-label={drawerNode.title} role="dialog" aria-modal="true">
        <div className="wk-header mb-[.75rem]! flex items-start justify-between gap-4">
          <div>
            <h2>{drawerNode.title}</h2>
            <p className="wk-muted text-muted">{drawerNode.page_type} · {drawerNode.link_count} {t('knowledgeBase.graph.links')}</p>
          </div>
          <Button type="button" onClick={() => { setDrawerNode(null); setDrawerPage(null); }}>{t('common.close')}</Button>
        </div>
        {drawerStatus === 'loading' ? <Status>{t('wikiBrowser.loading')}</Status> : null}
        {drawerStatus === 'error' ? <Status tone="error">{t('wikiBrowser.revisionLoadFailed')}</Status> : null}
        {drawerPage ? <>
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <span className="inline-flex items-center rounded-pill border border-line-neutral px-2 py-[1px] text-[11px] text-muted-strong">{graphTypeLabel(drawerNode.page_type)}</span>
            <span className="text-[12px] text-muted">{t('wikiBrowser.version', { ver: drawerPage.version })}</span>
            {mode === 'ego' && center !== drawerNode.slug ? <Button type="button" size="small" className="ml-auto!" disabled={drawerNeighbor ? !drawerNeighbor.canBloom : false} onClick={() => void bloomNeighbors(drawerNode.slug)}>{t('wikiBrowser.bloomNeighbors')}</Button> : null}
            {mode !== 'ego' || center !== drawerNode.slug ? <Button type="button" size="small" className={mode === 'ego' ? '' : 'ml-auto!'} onClick={() => void load('ego', drawerNode.slug)}>{t('wikiBrowser.expandNeighbors')}</Button> : null}
          </div>
          {drawerNeighborHint ? <p className="mb-4 select-none text-[12px] leading-4 text-muted-strong">{drawerNeighborHint}</p> : null}
          <div data-testid="knowledge-graph-reader" className="wk-reader-body max-h-[24rem] overflow-auto leading-[1.6]" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(drawerPage.content) }} />
        </> : null}
      </aside> : null}
    </main>
  );
}
