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
  const result: DisplayGraphEdge[] = [];
  for (const edge of edges) {
    const pair = [edge.source, edge.target].sort().join('\u0000');
    if (seen.has(pair)) continue;
    seen.add(pair);
    const bidirectional = edges.some((candidate) => candidate.source === edge.target && candidate.target === edge.source);
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
  return {
    nodes: [...nodes.values()],
    edges: [...edges.values()],
    meta: {
      ...base.meta,
      returned: nodes.size,
      total: Math.max(base.meta.total, incoming.meta.total),
      truncated: Boolean(base.meta.truncated || incoming.meta.truncated),
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
  return graph.nodes.filter((node) => node.slug !== center && node.page_type !== 'index' && node.page_type !== 'log' && node.link_count > (degree.get(node.slug) ?? 0));
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
