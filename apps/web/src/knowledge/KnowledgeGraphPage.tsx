import { useEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type PointerEvent as ReactPointerEvent, type WheelEvent as ReactWheelEvent } from 'react';
import type { WikiGraphData, WeKnoraClient } from '@weknora/api-client';
import { Button, Dialog, Input, Status } from '@weknora/ui';
import { renderChatMarkdown } from '@weknora/views';
import { displayGraphEdges, filterGraphNodes, graphEdgeEndpoints, fitGraphViewport, graphFrontierNodes, graphHighlightSets, graphNeighborStatus, graphNodeRadius, graphQueryParams, growGraphFrontier, layoutGraphNodes, mergeGraphData, type GraphViewport, WIKI_GRAPH_TYPES, zoomGraphViewport } from './graph.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { navigate } from '../platform/navigation.ts';
import { DocumentsBreadcrumb, ParserHint, type DocumentsBreadcrumbTab, type KBChromeListItem } from '../documents/DocumentsPageChrome.tsx';
import '../documents/documents.td.css';
import { computeSupportedFileTypes, computeUnsupportedFileTypes } from '../documents/page-chrome.ts';
import { KnowledgeSettingsPage, type KnowledgeSettingsSectionKey } from '../knowledge-settings/KnowledgeSettingsPage.tsx';
import { canUploadKnowledgeDocuments, kbWikiTabFallbackPath, resolveKBSurfaceTabs, type KBSurfaceKB, type KBSurfaceMe, type KBSurfaceTab } from './permissions.ts';
import { useKbDetailGuideTrigger } from '../../../../packages/views/src/guides/use-kb-detail-guide-trigger.ts';

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

  // Vue KnowledgeBase.vue header inputs (kbInfo + me + KB switcher list +
  // parser engines for the KBInfoPopover file types). The graph surface shares
  // the documents page chrome, so it sources the same data as KnowledgeDocumentsPage.
  const [kbMeta, setKbMeta] = useState<KBSurfaceKB | null>(null);
  const [kbList, setKbList] = useState<KBChromeListItem[]>([]);
  const [canManage, setCanManage] = useState(false);
  const [parserEngines, setParserEngines] = useState<{ Name: string; FileTypes?: string[]; Available?: boolean }[]>([]);
  // Vue WikiBrowser renders the graph SVG at the measured .wiki-graph size
  // (renderGraph reads clientWidth/clientHeight). Track the surface box so the
  // node layout and viewBox match the real full-bleed area instead of a fixed
  // 760×420 card.
  const [surfaceSize, setSurfaceSize] = useState({ width: 760, height: 420 });
  const surfaceRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const el = surfaceRef.current;
    if (!el || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver((entries) => {
      const rect = entries[0]?.contentRect;
      if (rect && rect.width >= 1 && rect.height >= 1) setSurfaceSize({ width: Math.round(rect.width), height: Math.round(rect.height) });
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    let active = true;
    void Promise.all([
      client.knowledgeBases.settings.get(knowledgeBaseId),
      client.auth.me().catch(() => null),
      // Vue KBSwitcherDropdown input: the tenant KB list behind the crumb menu.
      client.knowledgeBases.list().catch(() => []),
      client.knowledgeBases.settings.parserEngines().catch(() => ({ data: [] })),
    ] as const)
      .then(([kb, me, list, engines]) => {
        if (!active) return;
        setKbMeta(kb as KBSurfaceKB);
        setCanManage(canUploadKnowledgeDocuments(kb as KBSurfaceKB, me as KBSurfaceMe | null));
        setKbList((list as { id: unknown; name: unknown }[]).map((item) => ({ id: String(item.id), name: String(item.name) })));
        setParserEngines((engines.data ?? []) as { Name: string; FileTypes?: string[]; Available?: boolean }[]);
      })
      .catch(() => {
        if (active) setKbMeta(null);
      });
    return () => { active = false; };
  }, [client, knowledgeBaseId]);

  // Vue KnowledgeBase.vue:2741 + 339-345 — the one-shot kbDetail welcome tour
  // arms on detail-page entry regardless of the active tab (the Vue computed
  // never reads it), so a ?tab=graph deep link arms too. The KB metadata this
  // page already loads carries knowledge_count (filled server-side by GET
  // /knowledge-bases/:id), so the empty-KB check costs no extra request; the
  // shell guide host owns dismissal + welcome-tour gating.
  useKbDetailGuideTrigger({
    knowledgeBaseId,
    kbType: typeof kbMeta?.type === 'string' ? kbMeta.type : null,
    canEdit: canManage,
    // kbMeta settling decides readiness; an unexpectedly missing knowledge_count
    // keeps the tour disarmed instead of guessing an empty KB.
    documentsLoading: kbMeta === null || typeof kbMeta.knowledge_count !== 'number',
    documentCount: typeof kbMeta?.knowledge_count === 'number' ? kbMeta.knowledge_count : 0,
  });

  const supportedFileTypes = useMemo(() => {
    const rules = (kbMeta?.chunking_config as { parser_engine_rules?: { file_types: string[]; engine: string }[] } | null | undefined)?.parser_engine_rules ?? [];
    return [...computeSupportedFileTypes(parserEngines, rules)];
  }, [kbMeta, parserEngines]);

  // Vue unsupportedFileTypes computed (KnowledgeBase.vue:224-234): every type
  // advertised by a tenant engine minus the supported set, sorted — drives the
  // .parser-hint warning line under the header on every KB tab (graph included).
  const parserRules = useMemo(() => {
    const chunking = kbMeta?.chunking_config as { parser_engine_rules?: unknown } | null | undefined;
    return Array.isArray(chunking?.parser_engine_rules)
      ? (chunking.parser_engine_rules as { file_types: string[]; engine: string }[])
      : [];
  }, [kbMeta]);
  const unsupportedFileTypes = useMemo(() => computeUnsupportedFileTypes(parserEngines, parserRules), [parserEngines, parserRules]);

  // Vue title row (KnowledgeBase.vue L2359-2380): wiki KBs render the third
  // crumb level as the 文档 / Wiki / 图谱 breadcrumb-tab row; the active graph
  // tab carries the tabGraphTip concept-clarification tooltip (Vue t-tooltip).
  // The row exists only when the KB enables the wiki (Vue isWiki gate —
  // resolveKBSurfaceTabs returns nothing for a non-wiki KB even when graph
  // extraction is on, permissions.ts); an empty/absent list falls back to the
  // plain 文档 crumb inside DocumentsBreadcrumb. /knowledgeBase/<id>?tab=… is
  // the canonical KB route form (routes.tsx knowledgeBaseView) — the same URLs
  // the documents page nav links to.
  const kbBasePath = `/knowledgeBase/${encodeURIComponent(knowledgeBaseId)}`;
  const resolvedTabs = kbMeta ? resolveKBSurfaceTabs(kbMeta) : undefined;
  const kbTabs: DocumentsBreadcrumbTab[] | undefined = resolvedTabs
    ? resolvedTabs.map((tab: KBSurfaceTab) => ({
      key: tab,
      label: tab === 'documents'
        ? t('knowledgeEditor.wikiBrowser.tabDocuments')
        : tab === 'wiki'
          ? 'Wiki' /* Vue template renders the wiki tab as the literal "Wiki" (KnowledgeBase.vue L2365) */
          : t('knowledgeEditor.wikiBrowser.tabGraph'),
      href: tab === 'documents' ? kbBasePath : `${kbBasePath}?tab=${tab}`,
      active: tab === 'graph',
      title: tab === 'graph' ? t('knowledgeEditor.wikiBrowser.tabGraphTip') : undefined,
    }))
    : undefined;

  // Vue keeps a ?tab=wiki|graph URL readable only for wiki KBs: the template
  // gates .wiki-main-area on isWiki and renders the documents branch otherwise
  // (KnowledgeBase.vue:2412/2418). The React tab pages are separate routes, so
  // a non-wiki deep link falls back to the canonical documents URL instead of
  // erroring against wiki-only APIs (same view the URL shows in Vue).
  const wikiTabFallbackPath = useMemo(() => (kbMeta ? kbWikiTabFallbackPath(kbMeta) : undefined), [kbMeta]);
  useEffect(() => {
    if (wikiTabFallbackPath) navigate(wikiTabFallbackPath, 'replace');
  }, [wikiTabFallbackPath]);

  // Vue ⚙ opens the in-place KB settings surface without leaving the page
  // (KnowledgeBase.vue:2388 → uiStore.openKBSettings → the editor modal).
  // React hosts the settled KB settings content (KnowledgeSettingsPage, the
  // same surface the /settings route renders) inside a Dialog overlay.
  const [settingsOpen, setSettingsOpen] = useState(false);
  // Vue uiStore.openKBSettings(kbId, 'parser') seeds the drawer at a section;
  // the parser-hint banner routes there while the ⚙ gear opens the default.
  const [settingsSection, setSettingsSection] = useState<KnowledgeSettingsSectionKey | undefined>(undefined);
  function openParserSettings() {
    setSettingsSection('parser');
    setSettingsOpen(true);
  }

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
  // Vue renders the search as a t-select (filterable) whose popup can be
  // expanded/collapsed — the React port must track the same open/active state.
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchActive, setSearchActive] = useState(-1);
  const [drawerNode, setDrawerNode] = useState<{ slug: string; title: string; page_type: string; link_count: number } | null>(null);
  const [drawerPage, setDrawerPage] = useState<{ title: string; summary: string; content: string; version: number } | null>(null);
  const [drawerStatus, setDrawerStatus] = useState<'idle' | 'loading' | 'error'>('idle');
  const [viewport, setViewport] = useState<GraphViewport>({ x: 0, y: 0, scale: 1 });
  const [dragPositions, setDragPositions] = useState<Record<string, { x: number; y: number }>>({});
  const gesture = useRef<{ kind: 'pan' | 'node'; pointerId: number; startX: number; startY: number; originX: number; originY: number; slug?: string; captured?: boolean } | null>(null);
  const dragged = useRef(false);
  // Vue pendingSingleClick (L4186-4220): the first of two taps on the same
  // node opens the drawer, the second belongs to the dblclick ego pivot —
  // without this both taps would fetch the page twice.
  const lastNodeTap = useRef<{ slug: string; time: number } | null>(null);
  // Vue WikiBrowser selection/hover highlight state (graphSelectedSlug /
  // graphHighlightSlug, L3735/3738): a click/search/ego-preselect selects a
  // node, hovering another node adds a secondary focus, and a near-stationary
  // click on the canvas background clears selection + drawer + highlight.
  const [selectedSlug, setSelectedSlug] = useState<string | null>(null);
  const [hoveredSlug, setHoveredSlug] = useState<string | null>(null);
  // Shared mouseleave debounce timer so sliding the pointer between nodes
  // doesn't flash through the fully-unhighlighted state (Vue L3721-3723).
  const hoverLeaveTimer = useRef<number | null>(null);
  // Outside-pointerdown closes the expanded search popup, like the Vue t-select.
  const searchShellRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => () => {
    if (hoverLeaveTimer.current !== null) window.clearTimeout(hoverLeaveTimer.current);
  }, []);

  // Vue node mouseenter (L4121-4131): cancel any pending leave, then hover.
  function enterNodeHover(slug: string) {
    if (hoverLeaveTimer.current !== null) {
      window.clearTimeout(hoverLeaveTimer.current);
      hoverLeaveTimer.current = null;
    }
    setHoveredSlug(slug);
  }

  // Vue node mouseleave (L4143-4158): 60ms debounced; with a selection the
  // highlight falls back to the selection only — derive that in render.
  function leaveNodeHover() {
    if (hoverLeaveTimer.current !== null) window.clearTimeout(hoverLeaveTimer.current);
    hoverLeaveTimer.current = window.setTimeout(() => {
      hoverLeaveTimer.current = null;
      setHoveredSlug(null);
    }, 60);
  }

  async function load(nextMode: 'overview' | 'ego', nextCenter?: string) {
    setStatus({ kind: 'loading' });
    try {
      const result = await client.wiki.graph(knowledgeBaseId, {
        ...graphQueryParams(nextMode, nextCenter ?? '', depth, selectedTypes),
      });
      setGraph(result);
      setMode(nextMode);
      setCenter(nextCenter ?? '');
      // Vue preselects the ego center after a fresh ego render (L3231-3238,
      // "preselect the center so the highlight / drawer context matches") and
      // going back to overview clears the selection with it.
      setSelectedSlug(nextMode === 'ego' && nextCenter ? nextCenter : null);
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

  // Single "jump to this slug" entry point for the search select, mirroring
  // Vue handleGraphSearchSelect: open the drawer and pivot to the ego view,
  // then clear the keyword (Vue resets graphSearchValue after ~300ms).
  function selectSearchResult(result: { title: string; slug: string }) {
    setSearchOpen(false);
    setQuery('');
    setSearchResults([]);
    void openNode({ slug: result.slug, title: result.title, page_type: 'page', link_count: 0 });
    void load('ego', result.slug);
  }

  // Vue t-select keyboard contract: arrows open + highlight, Enter commits the
  // highlighted (or first) match, Escape collapses the popup.
  function onSearchKeyDown(event: ReactKeyboardEvent<HTMLInputElement>) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      setSearchOpen(true);
      const count = searchOptions.length;
      if (count === 0) return;
      setSearchActive((current) => event.key === 'ArrowDown' ? Math.min(current + 1, count - 1) : Math.max(current - 1, -1));
    } else if (event.key === 'Enter') {
      const active = searchActive >= 0 ? searchOptions[searchActive] : searchOptions[0];
      if (active) {
        event.preventDefault();
        selectSearchResult(active);
      }
    } else if (event.key === 'Escape') {
      setSearchOpen(false);
    }
  }

  useEffect(() => { void load(mode, mode === 'ego' ? center : undefined); }, [client, knowledgeBaseId, selectedTypes]);
  // Vue handleGraphRemoteSearch (WikiBrowser.vue L4680-4712): any non-empty
  // keyword debounces 200ms into a remote wiki search; an empty keyword just
  // clears the keyword results so the dropdown falls back to the top-500
  // snapshot. Typing never touches the canvas — see the visible memo below.
  useEffect(() => {
    const keyword = query.trim();
    if (!keyword) { setSearchResults([]); setSearchLoading(false); return; }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      setSearchLoading(true);
      void client.wiki.list(knowledgeBaseId, { page: 1, page_size: 20, keyword }).then((result) => {
        if (!cancelled) setSearchResults(result.pages.map((page) => ({ title: page.title, slug: page.slug })));
      }).catch(() => {
        if (!cancelled) setSearchResults([]);
      }).finally(() => { if (!cancelled) setSearchLoading(false); });
    }, 200);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [client, knowledgeBaseId, query]);

  // With an empty keyword the Vue select falls back to the overview top-500
  // snapshot (graphSearchEffectiveOptions, WikiBrowser.vue L4669) so the
  // expanded popup is browsable without typing. Mirror it with the loaded
  // graph nodes ranked by link_count.
  const searchDefaultOptions = useMemo(() => graph ? [...graph.nodes].sort((a, b) => b.link_count - a.link_count).map((node) => ({ title: node.title, slug: node.slug })) : [], [graph]);
  const searchOptions = query.trim().length > 0 ? searchResults : searchDefaultOptions;
  useEffect(() => {
    if (!searchOpen) return;
    const onPointerDown = (event: PointerEvent) => {
      if (searchShellRef.current && event.target instanceof Node && !searchShellRef.current.contains(event.target)) setSearchOpen(false);
    };
    document.addEventListener('pointerdown', onPointerDown);
    return () => document.removeEventListener('pointerdown', onPointerDown);
  }, [searchOpen]);
  useEffect(() => { setSearchActive(-1); }, [searchOpen, query]);

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

  // Vue search never filters the canvas: handleGraphRemoteSearch only swaps
  // dropdown options (WikiBrowser.vue L4680-4712), so typing leaves the node
  // set untouched. The canvas narrows only through the type allow-list
  // (graphFilterTypesToArray, re-fetched server-side), which the selectedTypes
  // load effect mirrors — keep query out of this filter.
  const visible = useMemo(() => graph ? filterGraphNodes(graph, { types: selectedTypes }) : null, [graph, selectedTypes]);
  // Vue graphReady (WikiBrowser.vue renderGraph, L3759-3764 + L4349): the
  // overlays (search shell, ? help, legend chips/actions, status card) mount
  // only after the renderer actually drew nodes — renderGraph returns early
  // for a node-less payload, so a 0-node graph keeps them ALL hidden behind
  // the 暂无图谱数据 empty state.
  const graphReady = status.kind === 'success' && visible !== null && visible.nodes.length > 0;
  const positions = useMemo(() => visible ? layoutGraphNodes(visible.nodes, surfaceSize.width, surfaceSize.height) : [], [visible, surfaceSize.width, surfaceSize.height]);
  const displayPositions = useMemo(() => positions.map((position) => ({ ...position, ...dragPositions[position.slug] })), [positions, dragPositions]);
  const positionBySlug = useMemo(() => new Map(displayPositions.map((position) => [position.slug, position])), [displayPositions]);
  const displayEdges = useMemo(() => visible ? displayGraphEdges(visible.edges) : [], [visible]);
  // Node radii per slug, needed to shorten edge ends like Vue setEdgePositions.
  const linkCountBySlug = useMemo(() => new Map((visible?.nodes ?? []).map((node) => [node.slug, node.link_count])), [visible]);
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
  // Vue applyHighlight input (L4558): the selection is the primary focus and
  // the hovered node becomes a secondary focus while a selection exists; with
  // no selection the hover alone drives the highlight. Null = plain styling.
  const highlight = useMemo(
    () => (visible ? graphHighlightSets(visible.edges, selectedSlug, hoveredSlug) : null),
    [visible, selectedSlug, hoveredSlug],
  );
  // Node lookup for the Vue edge stroke rule: a lit edge takes the type color
  // of the focus node driving it (hover wins over selection, L4605-4608).
  const nodeBySlug = useMemo(() => new Map((visible?.nodes ?? []).map((node) => [node.slug, node])), [visible]);
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
    // Child elements carry ownerSVGElement; the svg root itself has none.
    // (Deliberately not `instanceof SVGSVGElement`: that interface must exist
    // on the global scope or the gesture handlers throw before registering.)
    const svg = event.currentTarget.ownerSVGElement ?? event.currentTarget;
    if (!svg) return { x: 0, y: 0 };
    const rect = svg.getBoundingClientRect();
    // The viewBox mirrors the measured surface size, so the mapping is 1:1.
    return { x: ((event.clientX - rect.left) / rect.width) * surfaceSize.width, y: ((event.clientY - rect.top) / rect.height) * surfaceSize.height };
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
    // No setPointerCapture here: capturing during pointerdown retargets the
    // following pointer events (and the compat click) to the svg root, so a
    // plain tap never activates the node in real browsers. Capture is taken
    // lazily in moveGraphGesture once the pointer actually moves.
    event.stopPropagation();
  }

  function moveGraphGesture(event: ReactPointerEvent<SVGSVGElement>) {
    const current = gesture.current;
    if (!current || current.pointerId !== event.pointerId) return;
    const point = svgPoint(event);
    const dx = point.x - current.startX;
    const dy = point.y - current.startY;
    if (Math.abs(dx) + Math.abs(dy) > 3) dragged.current = true;
    if (current.kind === 'pan') setViewport((value) => ({ ...value, x: current.originX + dx, y: current.originY + dy }));
    else if (current.slug) {
      // Take capture only once the drag is real so the svg keeps receiving
      // moves outside its bounds; a stationary tap never captures and its
      // pointerup stays on the node.
      if (dragged.current && !current.captured) {
        current.captured = true;
        try {
          event.currentTarget.setPointerCapture(event.pointerId);
        } catch {
          // Pointer already gone (released before the move dispatch) — the
          // gesture simply stops receiving moves.
        }
      }
      setDragPositions((value) => ({ ...value, [current.slug!]: { x: current.originX + dx / viewport.scale, y: current.originY + dy / viewport.scale } }));
    }
  }

  // Single activation path for canvas nodes: pointerdown registers the
  // gesture, and a pointerup that never exceeded the drag threshold is the
  // tap. (The former <g> onClick never fired in real browsers — capturing on
  // pointerdown retargeted the click to the svg root — and jsdom's
  // fireEvent-style dispatches bypass capture, which is why tests stayed
  // green.) Shift+tap blooms like Vue shift+click (L4191-4200), and the
  // second tap of a double-tap is left to onDoubleClick (Vue pendingSingleClick).
  function endGraphGesture(event: ReactPointerEvent<SVGSVGElement>) {
    const current = gesture.current;
    if (!current || current.pointerId !== event.pointerId) return;
    gesture.current = null;
    if (current.kind !== 'node' || !current.slug || dragged.current) return;
    const slug = current.slug;
    const now = Date.now();
    const isSecondTap = lastNodeTap.current?.slug === slug && now - lastNodeTap.current.time < 300;
    lastNodeTap.current = { slug, time: now };
    if (isSecondTap) return;
    const node = (visible?.nodes ?? []).find((item) => item.slug === slug);
    if (!node) return;
    if (event.shiftKey) {
      void bloomNeighbors(slug);
      return;
    }
    setSelectedSlug(slug);
    void openNode(node);
  }

  // pointercancel means the gesture was taken over (scroll/pointer loss) —
  // never treat it as a tap.
  function cancelGraphGesture(event: ReactPointerEvent<SVGSVGElement>) {
    if (gesture.current?.pointerId === event.pointerId) gesture.current = null;
  }

  // Vue flyTo (WikiBrowser.vue L4482-4499): a 600ms cubic ease-out tween to
  // the target pan/zoom. Falls back to an immediate jump when rAF is absent.
  function flyViewportTo(target: GraphViewport, duration = 600) {
    const start = viewport;
    const t0 = typeof performance !== 'undefined' ? performance.now() : Date.now();
    const frame = typeof requestAnimationFrame === 'function'
      ? (step: (now: number) => void) => requestAnimationFrame(step)
      : null;
    const ease = (t: number) => 1 - Math.pow(1 - t, 3);
    if (!frame) { setViewport(target); return; }
    const step = (now: number) => {
      const total = typeof performance !== 'undefined' ? performance.now() - t0 : Date.now() - t0;
      const p = Math.min(1, duration > 0 ? total / duration : 1);
      const e = ease(p);
      setViewport({
        x: start.x + (target.x - start.x) * e,
        y: start.y + (target.y - start.y) * e,
        scale: start.scale + (target.scale - start.scale) * e,
      });
      if (p < 1) frame(step);
    };
    frame(step);
  }

  return (
    /* Vue KnowledgeBase.vue renders the graph surface full-bleed: the page is
       a flex column (document-header 24px/32px/0 + .wiki-main-area flex:1) and
       .wiki-graph fills that area (width/height 100%). .knowledge-layout uses
       margin 0 16px 0 4px + padding 24px 32px 0 + gap 20px with a trailing
       0-height sibling, so the content column sits at x+36/x-28 with a 20px
       bottom inset — mirrored here so the graph area lands on the same box. */
    /* Vue App.vue also renders text with -webkit-font-smoothing: antialiased
       (inherited); the platform shell lacks it, so the page root re-arms it
       to keep glyph rasterization identical. */
    <div className="knowledge-layout">
      <div className="document-header">
        <div className="document-header-title">
        <DocumentsBreadcrumb
          t={t}
          knowledgeBaseId={knowledgeBaseId}
          kbName={typeof kbMeta?.name === 'string' ? kbMeta.name : null}
          kbList={kbList}
          kbMeta={{
            type: typeof kbMeta?.type === 'string' ? kbMeta.type : undefined,
            description: typeof kbMeta?.description === 'string' ? kbMeta.description : undefined,
            createdAt: typeof kbMeta?.created_at === 'string' ? kbMeta.created_at.slice(0, 10) : undefined,
          }}
          supportedFileTypes={supportedFileTypes}
          canManage={canManage}
          onOpenSettings={() => { setSettingsSection(undefined); setSettingsOpen(true); }}
          tabs={kbTabs}
        />
        {/* Vue keeps the document upload subtitle under every tab — the
            document-subtitle line is unconditional in KnowledgeBase.vue, and
            .document-header-title's 4px gap offsets it from the title row. */}
        <p className="document-subtitle">{t('knowledgeEditor.document.subtitle')}</p>
        {/* Vue parser-hint warning line (KnowledgeBase.vue:2458-2465): types
            advertised by an engine but unresolved by the KB rules render the
            banner + 前往配置 link on every KB detail tab. The wrapper margin
            (collapsing with the hint's own 2px) yields Vue's 6px gap after
            the .document-header-title 4px offset above. */}
        <ParserHint t={t} types={unsupportedFileTypes} onConfigure={openParserSettings} />
        </div>
      </div>
      {/* Vue .wiki-main-area: flex:1 / min-height:0 / overflow:hidden with no
          border or radius — the Card chrome (rounded-card + border) is a
          React-only addition, so the surface renders as a plain flex region. */}
      <div className="wiki-main-area">
      <div className="flex min-h-0 h-full w-full flex-1 flex-col overflow-hidden">
        <div ref={surfaceRef} data-testid="knowledge-graph-surface" className="relative min-h-[420px] flex-1 overflow-hidden bg-white max-[720px]:min-h-[26rem]">
          {status.kind === 'success' && graph && visible && visible.nodes.length > 0 ? (
            <svg className="absolute inset-0 block h-full w-full cursor-grab touch-none select-none active:cursor-grabbing" viewBox={`0 0 ${surfaceSize.width} ${surfaceSize.height}`} role="img" aria-label={t('knowledgeBase.graph.ariaLinks')} onPointerDown={beginPan} onPointerMove={moveGraphGesture} onPointerUp={endGraphGesture} onPointerCancel={cancelGraphGesture} onClick={(event) => {
              // Vue setupPanZoom mouseup (L4544-4553): a near-stationary click
              // on the svg background clears the selection, the drawer, and
              // the highlight with it.
              if (event.target === event.currentTarget && !dragged.current) {
                setSelectedSlug(null);
                setHoveredSlug(null);
                setDrawerNode(null);
                setDrawerPage(null);
              }
            }} onWheel={(event: ReactWheelEvent<SVGSVGElement>) => { event.preventDefault(); const point = svgPoint(event); setViewport((value) => zoomGraphViewport(value, event.deltaY < 0 ? 1.15 : 0.87, point)); }}>
              <defs>
                <marker id="wk-graph-arrow-end" viewBox="0 0 10 6" refX="10" refY="3" markerWidth="8" markerHeight="6" orient="auto">
                  <path d="M0,0 L10,3 L0,6 L2,3 Z" fill="#c0c4cc" />
                </marker>
                <marker id="wk-graph-arrow-start" viewBox="0 0 10 6" refX="0" refY="3" markerWidth="8" markerHeight="6" orient="auto">
                  <path d="M10,0 L0,3 L10,6 L8,3 Z" fill="#c0c4cc" />
                </marker>
                {/* Vue highlight arrows (WikiBrowser.vue L3911-3927): same
                    geometry as the plain markers but filled brand blue. */}
                <marker id="wk-graph-arrow-end-hl" viewBox="0 0 10 6" refX="10" refY="3" markerWidth="8" markerHeight="6" orient="auto">
                  <path d="M0,0 L10,3 L0,6 L2,3 Z" fill="#0052d9" />
                </marker>
                <marker id="wk-graph-arrow-start-hl" viewBox="0 0 10 6" refX="0" refY="3" markerWidth="8" markerHeight="6" orient="auto">
                  <path d="M10,0 L0,3 L10,6 L8,3 Z" fill="#0052d9" />
                </marker>
              </defs>
              <g transform={`translate(${viewport.x} ${viewport.y}) scale(${viewport.scale})`}>
                {displayEdges.map((edge) => {
                  const source = positionBySlug.get(edge.source);
                  const target = positionBySlug.get(edge.target);
                  if (!source || !target) return null;
                  // Vue setEdgePositions: stop each end at the node circle boundary so arrows stay visible.
                  const ends = graphEdgeEndpoints(
                    source,
                    target,
                    graphNodeRadius(linkCountBySlug.get(edge.source) ?? 0),
                    graphNodeRadius(linkCountBySlug.get(edge.target) ?? 0),
                  );
                  // Vue applyHighlight edge rule (L4601-4619): edges incident to
                  // a focus node light up (stroke = focus type color, opacity
                  // 0.9, width 2, #0052d9 hl arrows on both ends of a
                  // reciprocal pair); every other edge fades to opacity 0.08 /
                  // width 1 with the plain markers. The hover focus colors an
                  // edge it touches before the selection does.
                  const lit = highlight?.litEdges.has(`${edge.source}-${edge.target}`) ?? false;
                  const focusSlug = lit && highlight
                    ? (hoveredSlug && hoveredSlug !== selectedSlug && (edge.source === hoveredSlug || edge.target === hoveredSlug)
                      ? hoveredSlug
                      : (selectedSlug ?? hoveredSlug)!)
                    : null;
                  const stroke = focusSlug ? (GRAPH_NODE_FILL[nodeBySlug.get(focusSlug)?.page_type ?? ''] ?? '#0052d9') : '#c0c4cc';
                  return <line key={`${edge.source}-${edge.target}`} {...ends} markerEnd={showArrows ? `url(#wk-graph-arrow-end${lit ? '-hl' : ''})` : undefined} markerStart={showArrows && edge.bidirectional ? `url(#wk-graph-arrow-start${lit ? '-hl' : ''})` : undefined} className="stroke-[#c0c4cc] [stroke-width:1.2] [stroke-opacity:0.4]" style={highlight ? { stroke, strokeWidth: lit ? 2 : 1, strokeOpacity: lit ? 0.9 : 0.08, transition: 'stroke 0.2s, stroke-width 0.2s, stroke-opacity 0.2s' } : undefined} />;
                })}
                {visible.nodes.map((node, index) => {
                  const position = displayPositions[index]!;
                  const radius = graphNodeRadius(node.link_count);
                  const fill = GRAPH_NODE_FILL[node.page_type] ?? '#8c8c8c';
                  // Vue expansion hint ring: dashed, type-colored, only when
                  // undisplayed neighbors remain and this is not the ego center.
                  const neighborCount = adjacencyBySlug.get(node.slug)?.size ?? 0;
                  const isEgoCenter = graph.meta.mode === 'ego' && graph.meta.center === node.slug;
                  const showExpansionRing = Math.max(0, node.link_count - neighborCount) > 0 && !isEgoCenter;
                  // Vue applyHighlight node rule (L4569-4596): focus nodes grow
                  // by r+3 / stroke-width 3, lit neighbors (and the focus set)
                  // stay at full opacity, everything else fades to 0.2.
                  const enlarged = highlight?.enlargedNodes.has(node.slug) ?? false;
                  const dimmed = highlight ? !highlight.litNodes.has(node.slug) : false;
                  return <g key={node.slug} className="group/node cursor-pointer outline-none" role="button" tabIndex={0} aria-label={`${node.title} · ${node.slug}`} style={{ opacity: dimmed ? 0.2 : 1, transition: 'opacity 0.2s' }} onPointerDown={(event) => beginNodeDrag(event, node.slug)} onMouseEnter={() => enterNodeHover(node.slug)} onMouseLeave={leaveNodeHover} onDoubleClick={() => void load('ego', node.slug)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelectedSlug(node.slug); void openNode(node); } }}>
                    {showExpansionRing ? <circle cx={position.x} cy={position.y} r={radius + 3} className="node-expansion-ring [fill:none] [stroke-width:1.5] [stroke-dasharray:3_3]" style={{ stroke: fill, opacity: 0.55 }} aria-hidden="true" /> : null}
                    {node.familiar ? <circle cx={position.x} cy={position.y} r={radius + 7} className="wk-graph-familiar-ring [fill:none] [stroke:#0052d9] [stroke-width:2]" style={{ opacity: 0.9 }} aria-hidden="true" /> : null}
                    {selectedSlug === node.slug ? <circle cx={position.x} cy={position.y} r={radius + 5} className="wk-graph-active-ring pointer-events-none [fill:none]" style={{ stroke: fill, strokeWidth: 2, transformOrigin: position.x + "px " + position.y + "px", animation: 'wk-node-active-pulse 1.5s cubic-bezier(0.25, 0.46, 0.45, 0.94) infinite' }} aria-hidden="true" /> : null}
                    <circle cx={position.x} cy={position.y} r={enlarged ? radius + 3 : radius} style={{ fill, strokeWidth: enlarged ? 3 : 2, transition: 'r 0.2s, stroke-width 0.2s, opacity 0.2s' }} className="[stroke:#fff] [stroke-width:2]" />
                    <text x={position.x} y={position.y + radius + 14} textAnchor="middle" className="pointer-events-none text-[11px] [fill:#66758b]">{node.title.length > 14 ? `${node.title.slice(0, 14)}…` : node.title}</text>
                    {mode === 'ego' && !isEgoCenter && Math.max(0, node.link_count - neighborCount) > 0 ? <g className="node-bloom-btn pointer-events-none opacity-0 transition-opacity group-hover/node:pointer-events-auto group-hover/node:opacity-100" onPointerDown={(event) => event.stopPropagation()} onClick={(event) => { event.stopPropagation(); void bloomNeighbors(node.slug); }}>
                      <circle cx={position.x + Math.SQRT1_2 * (radius + 6)} cy={position.y - Math.SQRT1_2 * (radius + 6)} r={8} className="[fill:#fff] [stroke:#0052d9] [stroke-width:1.5]" />
                      <line x1={position.x + Math.SQRT1_2 * (radius + 6)} x2={position.x + Math.SQRT1_2 * (radius + 6)} y1={position.y - Math.SQRT1_2 * (radius + 6) - 4} y2={position.y - Math.SQRT1_2 * (radius + 6) + 4} className="stroke-[#0052d9] [stroke-width:1.8] [stroke-linecap:round]" />
                      <line x1={position.x + Math.SQRT1_2 * (radius + 6) - 4} x2={position.x + Math.SQRT1_2 * (radius + 6) + 4} y1={position.y - Math.SQRT1_2 * (radius + 6)} y2={position.y - Math.SQRT1_2 * (radius + 6)} className="stroke-[#0052d9] [stroke-width:1.8] [stroke-linecap:round]" />
                    </g> : null}
                  </g>;
                })}
              </g>
            </svg>
          ) : null}
          {graphReady ? <div role="search" className="absolute left-4 top-4 z-10 flex w-80 max-w-[calc(100%-2rem)] flex-col gap-3 max-[720px]:left-2 max-[720px]:top-2">
            <div className="flex items-center gap-2">
              {/* Vue renders the graph search as a t-select (filterable):
                  search prefix icon + suffix chevron that rotates when the
                  popup expands, 32px control height, and an empty keyword
                  falls back to the overview snapshot. WikiBrowser.vue L12-17. */}
              <div ref={searchShellRef} className="relative min-w-0 flex-1">
                <div className="flex h-8 items-center rounded-[4px] border border-line-input bg-white/95 pl-2 pr-1 shadow-[0_1px_10px_rgba(0,0,0,0.05)] transition-colors focus-within:border-accent">
                  <svg viewBox="0 0 16 16" aria-hidden="true" className="size-4 shrink-0 text-faint"><circle cx="7" cy="7" r="4.6" fill="none" stroke="currentColor" strokeWidth="1.4" /><path d="M10.4 10.4 14 14" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" /></svg>
                  <Input value={query} onChange={(event) => setQuery(event.target.value)} onFocus={() => setSearchOpen(true)} onKeyDown={onSearchKeyDown} placeholder={t('wikiBrowser.searchPlaceholder')} role="combobox" aria-expanded={searchOpen} aria-controls="wk-graph-search-results" aria-autocomplete="list" aria-activedescendant={searchActive >= 0 ? `wk-graph-search-option-${searchActive}` : undefined} aria-label={t('wikiBrowser.page.search')} className="h-8 min-w-0 flex-1 rounded-[4px]! border-0! bg-transparent! px-1! shadow-none! outline-none! focus-visible:outline-none!" />
                  <button type="button" tabIndex={-1} aria-hidden="true" data-testid="graph-search-chevron" className="flex size-6 shrink-0 cursor-pointer items-center justify-center border-0 bg-transparent p-0 text-faint" onClick={() => setSearchOpen((open) => !open)}>
                    <svg viewBox="0 0 10 6" aria-hidden="true" className={`size-[10px] transition-transform duration-150 ${searchOpen ? 'rotate-180' : ''}`}><path d="M1 1l4 4 4-4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" /></svg>
                  </button>
                </div>
                {searchOpen ? <ul id="wk-graph-search-results" role="listbox" aria-label={t('wikiBrowser.page.search')} className="absolute left-0 top-[calc(100%+4px)] z-[3] m-0 max-h-[16rem] w-full list-none overflow-auto rounded-[6px] border border-[#d8e0eb] bg-white p-1 shadow-[0_8px_20px_rgba(31,52,84,.12)]">
                  {searchLoading ? <li className="flex h-8 items-center px-2 text-[13px] text-faint">{t('wikiBrowser.loading')}</li> : null}
                  {!searchLoading && searchOptions.length === 0 ? <li className="flex h-8 items-center px-2 text-[13px] text-faint">{t('common.empty')}</li> : null}
                  {searchOptions.map((option, index) => <li key={option.slug} id={`wk-graph-search-option-${index}`} role="option" aria-selected={index === searchActive}><button type="button" className={`flex h-8 w-full cursor-pointer items-center truncate rounded-[3px] border-0 bg-transparent px-2 text-left text-[13px] text-ink hover:bg-[#f3f3f3] focus-visible:bg-[#f3f3f3] focus-visible:outline-none ${index === searchActive ? 'bg-[#f3f3f3]' : ''}`} onMouseEnter={() => setSearchActive(index)} onClick={() => selectSearchResult(option)}>{option.title}</button></li>)}
                </ul> : null}
              </div>
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
          {graphReady ? <div data-testid="knowledge-graph-legend" className="absolute right-4 top-4 z-10 flex flex-col gap-3 rounded-[6px] border border-line-neutral bg-white p-[10px_12px] opacity-95 shadow-[0_1px_10px_rgba(0,0,0,0.05)] transition-all duration-300 max-[720px]:right-2 max-[720px]:top-2" style={drawerNode ? { right: 'calc(480px + 16px)' } : undefined}>
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
              <button type="button" className="flex cursor-pointer select-none items-center gap-1.5 border-0 bg-transparent p-0 text-left text-[11px] leading-[14px] text-muted-strong transition-colors hover:text-primary" title="Fit to View" onClick={() => flyViewportTo(fitGraphViewport(displayPositions, surfaceSize.width, surfaceSize.height, Boolean(drawerNode)))}>
                <span className="inline-flex size-[14px] shrink-0 items-center justify-center text-[13px] text-faint" aria-hidden="true">◎</span>
                <span>{t('wikiBrowser.fitView')}</span>
              </button>
              <button type="button" className="flex cursor-pointer select-none items-center gap-1.5 border-0 bg-transparent p-0 text-left text-[11px] leading-[14px] text-muted-strong transition-colors hover:text-primary" onClick={() => setShowArrows((value) => !value)} aria-pressed={showArrows}>
                {/* Vue toggleArrows icon semantics: browse-off while arrows are shown, browse when hidden. */}
                <span className="inline-flex size-[14px] shrink-0 items-center justify-center text-faint" aria-hidden="true">
                  <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M1.6 8s2.4-4.2 6.4-4.2S14.4 8 14.4 8 12 12.2 8 12.2 1.6 8 1.6 8Z" />
                    <circle cx="8" cy="8" r="2.1" />
                    {showArrows ? <line x1="3.2" y1="13" x2="12.8" y2="3" /> : null}
                  </svg>
                </span>
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
          {/* Vue .wiki-graph-empty: absolute 100%×100% + padding 60px 20px in
              a content-box (TDesign reset), so the padded box overflows the
              canvas by exactly the padding and centers the icon inside the
              unpadded content area. Mirrored with w-full/h-full + content-box. */}
          {status.kind === 'loading' ? <div className="wiki-graph-empty absolute left-0 top-0 z-20 flex h-full w-full flex-col items-center justify-center bg-white p-[60px_20px] text-center [box-sizing:content-box]"><Status>{t('wikiBrowser.graphEmpty')}</Status></div> : null}
          {status.kind === 'error' ? <div className="wiki-graph-empty absolute left-0 top-0 z-20 flex h-full w-full flex-col items-center justify-center gap-2 bg-white p-[60px_20px] text-center [box-sizing:content-box]"><Status tone="error">{status.message}</Status><Button type="button" onClick={() => void load(mode, center || undefined)}>{t('common.retry')}</Button></div> : null}
          {status.kind === 'success' && visible && visible.nodes.length === 0 ? <div className="wiki-graph-empty absolute left-0 top-0 z-20 flex h-full w-full flex-col items-center justify-center bg-white p-[60px_20px] text-center [box-sizing:content-box]">
            {/* Vue empty icon: 64px circle (bg #f3f3f3) wrapping a 48px
                t-icon-chart-bubble, both tinted text-color-placeholder. */}
            <span className="mb-4 flex size-16 shrink-0 items-center justify-center rounded-full bg-[#f3f3f3] text-[rgba(0,0,0,0.4)]" aria-hidden="true">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" strokeLinejoin="miter" aria-hidden="true">
                <path d="M21 21H3V3" />
                <path d="M18 6C18 7.10457 17.1046 8 16 8C14.8954 8 14 7.10457 14 6C14 4.89543 14.8954 4 16 4C17.1046 4 18 4.89543 18 6Z" />
                <path d="M13 14C13 15.6569 11.6569 17 10 17C8.34315 17 7 15.6569 7 14C7 12.3431 8.34315 11 10 11C11.6569 11 13 12.3431 13 14Z" />
                <path d="M19 15C18.4477 15 18 14.5523 18 14C18 13.4477 18.4477 13 19 13C19.5523 13 20 13.4477 20 14C20 14.5523 19.5523 15 19 15Z" />
              </svg>
            </span>
            <p className="m-0 text-[13px] leading-[18px] text-[rgba(0,0,0,0.4)]">{t('wikiBrowser.graphNoData')}</p>
          </div> : null}
        </div>
      </div>
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
      {/* Vue ⚙ opens the KB settings overlay in place (uiStore.openKBSettings);
          the Dialog re-hosts the settled KB settings content (KnowledgeSettingsPage,
          the same surface /knowledgeBase/<id>/settings renders) instead of
          navigating away from the graph. Gate matches Vue canManage — the gear
          that opens it only renders for managers. */}
      {settingsOpen ? (
        <Dialog
          open
          title={t('knowledgeBase.settings')}
          closeLabel={t('common.close')}
          onClose={() => setSettingsOpen(false)}
          className="h-[min(85vh,750px)] w-[min(1000px,90vw)]! max-h-[min(750px,85vh)]! overflow-auto"
        >
          <KnowledgeSettingsPage client={client} knowledgeBaseId={knowledgeBaseId} role={canManage ? 'admin' : 'viewer'} initialSection={settingsSection} />
        </Dialog>
      ) : null}
      </div>
      {/* Vue DocContent 宿主（关闭态 0 高度，参与 knowledge-layout gap 布局）。 */}
      <div className="doc_content" />
    </div>
  );
}
