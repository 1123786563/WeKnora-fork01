import assert from 'node:assert/strict';
import test from 'node:test';

import { displayGraphEdges, filterGraphNodes, graphEdgeEndpoints, graphFrontierNodes, graphHighlightSets, graphNeighborStatus, graphNodeRadius, graphQueryParams, growGraphFrontier, layoutGraphNodes, mergeGraphData, WIKI_GRAPH_TYPES, zoomGraphViewport } from './graph.ts';

const graph = {
  nodes: [
    { slug: 'docs/start', title: 'Start', page_type: 'summary', link_count: 3 },
      { slug: 'docs/next', title: 'Next', page_type: 'entity', link_count: 2 },
  ],
  edges: [{ source: 'docs/start', target: 'docs/next' }],
  meta: { mode: 'overview', total: 2, returned: 2, truncated: false },
};

test('filters graph nodes by title, slug, and page type without changing edges', () => {
  const result = filterGraphNodes(graph, { query: 'start', types: ['summary'] });
  assert.deepEqual(result.nodes.map((node) => node.slug), ['docs/start']);
  assert.deepEqual(result.edges, []);
});

test('lays out every graph node at a bounded non-overlapping display position', () => {
  const positions = layoutGraphNodes(graph.nodes, 640, 360);
  assert.equal(positions.length, 2);
  assert.ok(positions.every((position) => position.x >= 32 && position.x <= 608 && position.y >= 32 && position.y <= 328));
  assert.notDeepEqual(positions[0], positions[1]);
});

test('builds a type-filter query without losing the active ego center', () => {
  assert.deepEqual(graphQueryParams('ego', 'docs/next', 2, 'entity'), {
    mode: 'ego', center: 'docs/next', depth: 2, limit: 500, types: ['entity'],
  });
});

test('omits the type query when every Vue graph legend type is active', () => {
  assert.deepEqual(graphQueryParams('overview', '', 1, WIKI_GRAPH_TYPES), {
    mode: 'overview', limit: 500,
  });
});

test('preserves a multi-type graph legend selection in the API query', () => {
  assert.deepEqual(graphQueryParams('overview', '', 1, ['summary', 'entity']), {
    mode: 'overview', limit: 500, types: ['summary', 'entity'],
  });
});

test('deduplicates reciprocal Vue graph links while preserving bidirectional arrow intent', () => {
  assert.deepEqual(displayGraphEdges([
    { source: 'docs/start', target: 'docs/next' },
    { source: 'docs/next', target: 'docs/start' },
    { source: 'docs/third', target: 'docs/next' },
  ]), [
    { source: 'docs/start', target: 'docs/next', bidirectional: true },
    { source: 'docs/third', target: 'docs/next', bidirectional: false },
  ]);
});

test('shortens edge ends to the node boundary so Vue-style arrows stay outside the circles', () => {
  const source = { slug: 'docs/start', x: 0, y: 0 };
  const target = { slug: 'docs/next', x: 100, y: 0 };
  const sourceRadius = graphNodeRadius(3);
  const targetRadius = graphNodeRadius(2);
  const ends = graphEdgeEndpoints(source, target, sourceRadius, targetRadius);
  // Each end is pulled back by the node radius + the Vue 4px arrow margin.
  assert.ok(Math.abs(ends.x1 - (sourceRadius + 4)) < 1e-9);
  assert.equal(ends.y1, 0);
  assert.ok(Math.abs(ends.x2 - (100 - targetRadius - 4)) < 1e-9);
  assert.equal(ends.y2, 0);
  // Endpoints must leave the node circles: arrows (~9.6px) stay visible.
  assert.ok(ends.x1 > 0);
  assert.ok(ends.x2 < 100);
});

test('keeps edge geometry stable for coincident nodes', () => {
  const same = { slug: 'docs/start', x: 40, y: 40 };
  const ends = graphEdgeEndpoints(same, { ...same }, 8, 8);
  assert.equal(ends.x1, 40);
  assert.equal(ends.y1, 40);
  assert.equal(ends.x2, 40);
  assert.equal(ends.y2, 40);
});

test('merges bloom results without duplicating nodes or edges and preserves familiar state', () => {
  const merged = mergeGraphData(graph, {
    nodes: [
      { slug: 'docs/start', title: 'Start', page_type: 'summary', link_count: 3, familiar: true },
      { slug: 'docs/third', title: 'Third', page_type: 'concept', link_count: 1 },
    ],
    edges: [
      { source: 'docs/start', target: 'docs/next' },
      { source: 'docs/next', target: 'docs/third' },
    ],
    meta: { mode: 'ego', total: 3, returned: 2, truncated: false },
  });
  assert.deepEqual(merged.nodes.map((node) => node.slug), ['docs/start', 'docs/next', 'docs/third']);
  assert.equal(merged.nodes[0]?.familiar, true);
  assert.equal(merged.edges.length, 2);
  assert.equal(merged.meta.returned, 3);
});

test('finds expandable ego nodes while excluding the center and super-nodes', () => {
  const frontier = graphFrontierNodes({
    ...graph,
    meta: { ...graph.meta, mode: 'ego', center: 'docs/start' },
    nodes: [
      ...graph.nodes,
      { slug: 'docs/index', title: 'Index', page_type: 'index', link_count: 20 },
      { slug: 'docs/log', title: 'Log', page_type: 'log', link_count: 20 },
    ],
  }, 'docs/start');
  assert.deepEqual(frontier.map((node) => node.slug), ['docs/next']);
});

test('zooms around the pointer anchor and clamps the Vue viewport scale', () => {
  const zoomed = zoomGraphViewport({ x: 0, y: 0, scale: 1 }, 2, { x: 100, y: 80 });
  assert.deepEqual(zoomed, { x: -100, y: -80, scale: 2 });
  assert.equal(zoomGraphViewport(zoomed, 10, { x: 0, y: 0 }).scale, 2.5);
  assert.equal(zoomGraphViewport(zoomed, 0.01, { x: 0, y: 0 }).scale, 0.6);
});

test('grows the frontier with a concurrency-capped fan-out that merges every ego response', async () => {
  const base = {
    nodes: [
      { slug: 'center', title: 'Center', page_type: 'summary', link_count: 3 },
      { slug: 'a', title: 'A', page_type: 'entity', link_count: 2 },
      { slug: 'b', title: 'B', page_type: 'concept', link_count: 2 },
    ],
    edges: [
      { source: 'center', target: 'a' },
      { source: 'center', target: 'b' },
    ],
    meta: { mode: 'ego' as const, total: 3, returned: 3, truncated: false, center: 'center' },
  };
  let inFlight = 0;
  let peak = 0;
  const fetched: string[] = [];
  const fetchEgo = async (slug: string) => {
    inFlight += 1;
    peak = Math.max(peak, inFlight);
    await new Promise((resolve) => setTimeout(resolve, 5));
    fetched.push(slug);
    inFlight -= 1;
    return {
      nodes: [
        { slug, title: slug.toUpperCase(), page_type: 'entity', link_count: 2 },
        { slug: `${slug}-n`, title: 'New', page_type: 'concept', link_count: 0 },
      ],
      edges: [{ source: slug, target: `${slug}-n` }],
      meta: { mode: 'ego' as const, total: 5, returned: 2, truncated: false, center: slug },
    };
  };
  const merged = await growGraphFrontier(base, 'center', fetchEgo);
  assert.deepEqual([...fetched].sort(), ['a', 'b']);
  assert.ok(peak <= 6, 'Vue caps the whole-frontier fan-out at 6 parallel requests');
  assert.deepEqual(merged?.nodes.map((node) => node.slug).sort(), ['a', 'a-n', 'b', 'b-n', 'center']);
  assert.equal(merged?.edges.length, 4);
  assert.equal(merged?.meta.returned, 5);
});

test('growing an exhausted frontier reports nothing to merge', async () => {
  const base = {
    nodes: [{ slug: 'solo', title: 'Solo', page_type: 'summary', link_count: 0 }],
    edges: [],
    meta: { mode: 'ego' as const, total: 1, returned: 1, truncated: false, center: 'solo' },
  };
  const merged = await growGraphFrontier(base, 'solo', async () => {
    throw new Error('must not fetch');
  });
  assert.equal(merged, null);
});

test('growing the frontier ignores individual ego fetch failures like Vue', async () => {
  const base = {
    nodes: [
      { slug: 'center', title: 'Center', page_type: 'summary', link_count: 2 },
      { slug: 'bad', title: 'Bad', page_type: 'entity', link_count: 2 },
      { slug: 'good', title: 'Good', page_type: 'entity', link_count: 2 },
    ],
    edges: [
      { source: 'center', target: 'bad' },
      { source: 'center', target: 'good' },
    ],
    meta: { mode: 'ego' as const, total: 3, returned: 3, truncated: false, center: 'center' },
  };
  const merged = await growGraphFrontier(base, 'center', async (slug) => {
    if (slug === 'bad') throw new Error('boom');
    return {
      nodes: [{ slug: 'good-kid', title: 'Kid', page_type: 'concept', link_count: 0 }],
      edges: [],
      meta: { mode: 'ego' as const, total: 4, returned: 1, truncated: false, center: slug },
    };
  });
  assert.deepEqual(merged?.nodes.map((node) => node.slug).sort(), ['bad', 'center', 'good', 'good-kid']);
  assert.equal(merged?.meta.returned, 4);
});

test('drawer neighbor status classifies the ego center as fully explored', () => {
  const ego = {
    nodes: [
      { slug: 'center', title: 'Center', page_type: 'summary', link_count: 5 },
      { slug: 'near', title: 'Near', page_type: 'entity', link_count: 1 },
    ],
    edges: [{ source: 'center', target: 'near' }],
    meta: { mode: 'ego' as const, total: 2, returned: 2, truncated: false, center: 'center' },
  };
  const centerStatus = graphNeighborStatus(ego, 'center');
  assert.equal(centerStatus?.visible, 1);
  assert.equal(centerStatus?.total, 5);
  assert.equal(centerStatus?.hidden, 4);
  assert.equal(centerStatus?.isEgoCenter, true);
  assert.equal(centerStatus?.fullyExplored, true, 'dead refs are unreachable, not loadable');
  assert.equal(centerStatus?.canBloom, false);

  const leaf = graphNeighborStatus(ego, 'near');
  assert.equal(leaf?.canBloom, false, 'no hidden neighbors means nothing to bloom');

  const overview = {
    nodes: [
      { slug: 'hub', title: 'Hub', page_type: 'summary', link_count: 9 },
      { slug: 'near', title: 'Near', page_type: 'entity', link_count: 1 },
    ],
    edges: [{ source: 'hub', target: 'near' }],
    meta: { mode: 'overview' as const, total: 2, returned: 2, truncated: true },
  };
  const hub = graphNeighborStatus(overview, 'hub');
  assert.equal(hub?.isOverview, true);
  assert.equal(hub?.fullyExplored, false);
  assert.equal(hub?.canBloom, true, 'formula matches Vue; the bloom button itself is ego-only and stays hidden');
});

test('node radii follow the Vue logarithmic clamp for rings and labels', () => {
  assert.equal(graphNodeRadius(0), 8);
  assert.equal(Math.round(graphNodeRadius(2) * 100) / 100, 12.39);
  assert.equal(graphNodeRadius(100000), 24);
});

test('highlight sets follow the Vue applyHighlight focus contract', () => {
  const edges = [
    { source: 'a', target: 'b' },
    { source: 'c', target: 'b' },
    { source: 'd', target: 'e' },
  ];
  // Hover with no selection: the hovered node is the only focus.
  const hoverOnly = graphHighlightSets(edges, null, 'b');
  assert.ok(hoverOnly);
  assert.deepEqual([...hoverOnly.enlargedNodes], ['b']);
  assert.deepEqual([...hoverOnly.litNodes].sort(), ['a', 'b', 'c'], 'undirected neighbors stay lit');
  assert.deepEqual([...hoverOnly.litEdges].sort(), ['a-b', 'c-b'], 'both in- and out-edges light up');
  assert.equal(hoverOnly.litEdges.has('d-e'), false, 'unrelated edges stay dim');

  // Selection + hover: both are focus nodes; selection stays primary.
  const both = graphHighlightSets(edges, 'a', 'c');
  assert.ok(both);
  assert.deepEqual([...both.enlargedNodes].sort(), ['a', 'c']);
  assert.deepEqual([...both.litNodes].sort(), ['a', 'b', 'c']);
  assert.deepEqual([...both.litEdges].sort(), ['a-b', 'c-b']);
  assert.equal(both.litEdges.has('d-e'), false);

  // Hovering the selected node collapses to a single focus (Vue passes no
  // hoverSlug when hover === selection).
  const same = graphHighlightSets(edges, 'b', 'b');
  assert.ok(same);
  assert.deepEqual([...same.enlargedNodes], ['b']);
  assert.deepEqual([...same.litNodes].sort(), ['a', 'b', 'c']);

  // No hover and no selection means nothing to highlight.
  assert.equal(graphHighlightSets(edges, null, null), null);
  assert.equal(graphHighlightSets(edges, '', ''), null);

  // Incoming edges to the focus light up too (undirected adjacency).
  const incoming = graphHighlightSets([{ source: 'x', target: 'focus' }], 'focus', null);
  assert.ok(incoming);
  assert.deepEqual([...incoming.litEdges], ['x-focus']);
  assert.deepEqual([...incoming.litNodes].sort(), ['focus', 'x']);
});
