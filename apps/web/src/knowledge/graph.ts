import type { WikiGraphData, WikiGraphEdge, WikiGraphNode, WikiGraphQueryParams } from '@weknora/api-client';

export interface GraphFilter {
  query?: string;
  types?: readonly string[];
}

export interface GraphNodePosition {
  slug: string;
  x: number;
  y: number;
}

export const WIKI_GRAPH_TYPES = ['summary', 'entity', 'concept', 'synthesis', 'comparison', 'index'] as const;

export interface DisplayGraphEdge extends WikiGraphEdge {
  bidirectional: boolean;
}

/**
 * Mirrors Vue WikiBrowser's graph renderer: reciprocal links share one SVG
 * line and receive an arrow at both ends. Keeping this derived from the API
 * edges means the backend contract stays unchanged.
 */
export function displayGraphEdges(edges: readonly WikiGraphEdge[]): DisplayGraphEdge[] {
  const seen = new Set<string>();
  const directed = new Set(edges.map((edge) => `${edge.source}\u0000${edge.target}`));
  const result: DisplayGraphEdge[] = [];
  for (const edge of edges) {
    const pair = [edge.source, edge.target].sort().join('\u0000');
    if (seen.has(pair)) continue;
    seen.add(pair);
    const bidirectional = directed.has(`${edge.target}\u0000${edge.source}`);
    result.push({ ...edge, bidirectional });
  }
  return result;
}

export function graphQueryParams(
  mode: 'overview' | 'ego',
  center: string,
  depth: number,
  type: string | readonly string[],
): WikiGraphQueryParams {
  const types = typeof type === 'string' ? (type === 'all' ? [] : [type]) : type.filter(Boolean);
  return {
    mode,
    ...(mode === 'ego' && center ? { center, depth } : {}),
    limit: 500,
    ...(types.length === 0 || types.length === WIKI_GRAPH_TYPES.length && WIKI_GRAPH_TYPES.every((item) => types.includes(item)) ? {} : { types: [...types] }),
  };
}

export function filterGraphNodes(graph: WikiGraphData, filter: GraphFilter = {}): WikiGraphData {
  const query = filter.query?.trim().toLocaleLowerCase() ?? '';
  const types = new Set((filter.types ?? []).filter(Boolean));
  const nodes = graph.nodes.filter((node) => {
    const matchesType = types.size === 0 || types.has(node.page_type);
    const matchesQuery = query === '' || `${node.title} ${node.slug}`.toLocaleLowerCase().includes(query);
    return matchesType && matchesQuery;
  });
  const slugs = new Set(nodes.map((node) => node.slug));
  return { ...graph, nodes, edges: graph.edges.filter((edge) => slugs.has(edge.source) && slugs.has(edge.target)) };
}

export function mergeGraphData(base: WikiGraphData, incoming: WikiGraphData): WikiGraphData {
  const nodes = new Map(base.nodes.map((node) => [node.slug, node]));
  for (const node of incoming.nodes) {
    const existing = nodes.get(node.slug);
    nodes.set(node.slug, existing ? { ...existing, ...node, familiar: Boolean(existing.familiar || node.familiar) } : node);
  }
  const edges = new Map(base.edges.map((edge) => [`${edge.source}\u2192${edge.target}`, edge]));
  for (const edge of incoming.edges) edges.set(`${edge.source}\u2192${edge.target}`, edge);
  const merged = [...nodes.values()];
  const familiarCount = merged.filter((node) => node.familiar).length;
  return {
    nodes: merged,
    edges: [...edges.values()],
    meta: {
      ...base.meta,
      returned: nodes.size,
      total: Math.max(base.meta.total, incoming.meta.total),
      truncated: Boolean(base.meta.truncated || incoming.meta.truncated),
      ...(familiarCount > 0 ? { familiar_count: familiarCount } : {}),
    },
  };
}

export function graphFrontierNodes(graph: WikiGraphData | null, center: string): WikiGraphNode[] {
  if (!graph || graph.meta.mode !== 'ego') return [];
  const degree = new Map<string, number>();
  for (const edge of graph.edges) {
    degree.set(edge.source, (degree.get(edge.source) ?? 0) + 1);
    degree.set(edge.target, (degree.get(edge.target) ?? 0) + 1);
  }
  return graph.nodes.filter((node) => isGraphFrontierCandidate(node, center, degree.get(node.slug) ?? 0));
}

/** Vue WikiBrowser keeps index/log super-nodes out of the batch expansion. */
function isGraphFrontierCandidate(node: WikiGraphNode, center: string, visibleDegree: number): boolean {
  if (node.slug === center) return false;
  if (node.page_type === 'index' || node.page_type === 'log') return false;
  return node.link_count > visibleDegree;
}

/**
 * Vue caps the whole-frontier operation at 6 parallel ego fetches so a huge
 * frontier cannot hammer the backend; individual failures are ignored so one
 * broken node does not sink the batch. Returns null when nothing was fetched.
 */
export const GRAPH_GROW_FRONTIER_CONCURRENCY = 6;

export async function growGraphFrontier(
  graph: WikiGraphData,
  center: string,
  fetchEgo: (slug: string) => Promise<WikiGraphData>,
): Promise<WikiGraphData | null> {
  const frontier = graphFrontierNodes(graph, center);
  if (frontier.length === 0) return null;
  let cursor = 0;
  let merged: WikiGraphData | null = null;
  async function worker(): Promise<void> {
    while (cursor < frontier.length) {
      const slug = frontier[cursor++]!.slug;
      try {
        const incoming = await fetchEgo(slug);
        merged = mergeGraphData(merged ?? graph, incoming);
      } catch {
        // One slow/broken node must not sink the whole batch (Vue behavior).
      }
    }
  }
  await Promise.all(
    Array.from({ length: Math.min(GRAPH_GROW_FRONTIER_CONCURRENCY, frontier.length) }, () => worker()),
  );
  return merged;
}

/**
 * Vue drawer neighbor accounting: compares the undirected degree inside the
 * current subgraph with the KB-wide link_count and classifies the gap so the
 * drawer can disable bloom/expand correctly.
 */
export interface GraphNeighborStatus {
  visible: number;
  total: number;
  hidden: number;
  isEgoCenter: boolean;
  isOverview: boolean;
  fullyExplored: boolean;
  canBloom: boolean;
}

export function graphNeighborStatus(graph: WikiGraphData, slug: string): GraphNeighborStatus | null {
  const node = graph.nodes.find((item) => item.slug === slug);
  if (!node) return null;
  const neighbors = new Set<string>();
  for (const edge of graph.edges) {
    if (edge.source === slug) neighbors.add(edge.target);
    else if (edge.target === slug) neighbors.add(edge.source);
  }
  const visible = neighbors.size;
  const total = node.link_count || 0;
  const hidden = Math.max(0, total - visible);
  const isEgoCenter = graph.meta.mode === 'ego' && graph.meta.center === slug;
  const isOverview = graph.meta.mode === 'overview';
  const fullyExplored = total === 0 || visible >= total || isEgoCenter;
  return {
    visible,
    total,
    hidden,
    isEgoCenter,
    isOverview,
    fullyExplored,
    // Bloom is additive and ego-only: the center already has everything
    // reachable, and with no hidden neighbors there is nothing to add.
    canBloom: !isEgoCenter && hidden > 0,
  };
}

/** Vue node radius: logarithmic in link_count, clamped to [8, 24]. */
export function graphNodeRadius(linkCount: number): number {
  return Math.max(8, Math.min(24, 8 + Math.log(linkCount + 1) * 4));
}

/**
 * Vue WikiBrowser applyHighlight/clearHighlight contract (L4558-4635):
 * a selection (click/search, or the ego-center preselect) is the primary
 * focus; while a selection exists, hovering another node adds it as a
 * secondary focus. With no selection the hovered node is the only focus.
 * Edges incident to either focus light up; nodes are enlarged (r+3 /
 * stroke-width 3), their undirected neighbors stay fully opaque, and every
 * other node fades to opacity 0.2. Returning null means "nothing to
 * highlight" — the renderer keeps the plain edge/node styling.
 */
export interface GraphHighlightSets {
  /** focus slugs — drawn enlarged like Vue's r+3 / stroke-width 3 circles */
  enlargedNodes: Set<string>;
  /** focus nodes plus their undirected neighbors — kept at full opacity */
  litNodes: Set<string>;
  /** "source-target" keys of edges incident to either focus slug */
  litEdges: Set<string>;
}

export function graphHighlightSets(
  edges: ReadonlyArray<{ source: string; target: string }>,
  selectedSlug: string | null | undefined,
  hoveredSlug: string | null | undefined,
): GraphHighlightSets | null {
  const primary = selectedSlug || hoveredSlug || null;
  if (!primary) return null;
  const secondary = selectedSlug && hoveredSlug && hoveredSlug !== selectedSlug ? hoveredSlug : null;
  const focus = secondary ? new Set([primary, secondary]) : new Set([primary]);
  const litEdges = new Set<string>();
  const neighbors = new Map<string, Set<string>>();
  for (const edge of edges) {
    if (!neighbors.has(edge.source)) neighbors.set(edge.source, new Set());
    if (!neighbors.has(edge.target)) neighbors.set(edge.target, new Set());
    neighbors.get(edge.source)!.add(edge.target);
    neighbors.get(edge.target)!.add(edge.source);
    if (focus.has(edge.source) || focus.has(edge.target)) litEdges.add(`${edge.source}-${edge.target}`);
  }
  const litNodes = new Set<string>(focus);
  for (const slug of focus) {
    for (const neighbor of neighbors.get(slug) ?? []) litNodes.add(neighbor);
  }
  return { enlargedNodes: focus, litNodes, litEdges };
}

/**
 * Vue WikiBrowser fitGraphToView (L1092-1137): frame the bounding box of the
 * visible nodes with a 60px padding, clamp the scale to [0.2, 2], and center
 * the box — shifted 240px left of center while the drawer is open so the
 * node under focus stays clear of the 480px panel. Returns the viewport the
 * renderer can tween to.
 */
export function fitGraphViewport(
  positions: ReadonlyArray<{ x: number; y: number }>,
  width: number,
  height: number,
  drawerVisible: boolean,
): GraphViewport {
  if (positions.length === 0) return { x: 0, y: 0, scale: 1 };
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const node of positions) {
    minX = Math.min(minX, node.x);
    minY = Math.min(minY, node.y);
    maxX = Math.max(maxX, node.x);
    maxY = Math.max(maxY, node.y);
  }
  const cx = (minX + maxX) / 2;
  const cy = (minY + maxY) / 2;
  const padding = 60;
  const boxWidth = Math.max(maxX - minX, 100) + padding * 2;
  const boxHeight = Math.max(maxY - minY, 100) + padding * 2;
  const scaleX = width / boxWidth;
  const scaleY = height / boxHeight;
  const scale = Math.max(0.2, Math.min(2, Math.min(scaleX, scaleY)));
  const targetCx = width / 2 - (drawerVisible ? 240 : 0);
  const targetCy = height / 2;
  return { x: targetCx - cx * scale, y: targetCy - cy * scale, scale };
}

/** Vue WikiBrowser setEdgePositions margin: keep end markers clear of node circles. */
export const GRAPH_EDGE_ARROW_MARGIN = 4;

export interface GraphEdgeEndpoints {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

/**
 * Mirrors Vue WikiBrowser setEdgePositions: shorten each edge end by the node
 * radius plus a 4px margin so end markers (≈9.6px long at markerWidth 8 ×
 * stroke-width 1.2) sit outside the node circles instead of being painted over
 * by them — without this both arrows hide under the nodes and edges render as
 * plain directionless lines.
 */
export function graphEdgeEndpoints(
  source: GraphNodePosition,
  target: GraphNodePosition,
  sourceRadius: number,
  targetRadius: number,
): GraphEdgeEndpoints {
  const dx = target.x - source.x;
  const dy = target.y - source.y;
  const dist = Math.hypot(dx, dy) || 1;
  const ux = dx / dist;
  const uy = dy / dist;
  const startRadius = sourceRadius + GRAPH_EDGE_ARROW_MARGIN;
  const endRadius = targetRadius + GRAPH_EDGE_ARROW_MARGIN;
  return {
    x1: source.x + ux * startRadius,
    y1: source.y + uy * startRadius,
    x2: target.x - ux * endRadius,
    y2: target.y - uy * endRadius,
  };
}

export interface GraphViewport {
  x: number;
  y: number;
  scale: number;
}

export function zoomGraphViewport(viewport: GraphViewport, factor: number, anchor: { x: number; y: number }): GraphViewport {
  const scale = Math.min(2.5, Math.max(0.6, viewport.scale * factor));
  const ratio = scale / viewport.scale;
  return { scale, x: anchor.x - (anchor.x - viewport.x) * ratio, y: anchor.y - (anchor.y - viewport.y) * ratio };
}

export function layoutGraphNodes(nodes: readonly WikiGraphNode[], width: number, height: number): GraphNodePosition[] {
  const safeWidth = Math.max(width, 64);
  const safeHeight = Math.max(height, 64);
  const centerX = safeWidth / 2;
  const centerY = safeHeight / 2;
  const radius = Math.max(0, Math.min(safeWidth, safeHeight) / 2 - 32);
  if (nodes.length === 1) return [{ slug: nodes[0]!.slug, x: centerX, y: centerY }];
  return nodes.map((node, index) => {
    const angle = (2 * Math.PI * index) / nodes.length - Math.PI / 2;
    return { slug: node.slug, x: centerX + radius * Math.cos(angle), y: centerY + radius * Math.sin(angle) };
  });
}
