import assert from 'node:assert/strict';
import test from 'node:test';

import { displayGraphEdges, filterGraphNodes, graphFrontierNodes, graphQueryParams, layoutGraphNodes, mergeGraphData, WIKI_GRAPH_TYPES, zoomGraphViewport } from './graph.ts';

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
