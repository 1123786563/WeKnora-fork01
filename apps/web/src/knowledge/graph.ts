import type { WikiGraphData, WikiGraphNode, WikiGraphQueryParams } from '@weknora/api-client';

export interface GraphFilter {
  query?: string;
  types?: readonly string[];
}

export interface GraphNodePosition {
  slug: string;
  x: number;
  y: number;
}

export function graphQueryParams(
  mode: 'overview' | 'ego',
  center: string,
  depth: number,
  type: string,
): WikiGraphQueryParams {
  return {
    mode,
    ...(mode === 'ego' && center ? { center, depth } : {}),
    limit: 500,
    ...(type === 'all' ? {} : { types: [type] }),
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
