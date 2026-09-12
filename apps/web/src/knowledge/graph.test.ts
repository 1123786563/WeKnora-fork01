import assert from 'node:assert/strict';
import test from 'node:test';

import { filterGraphNodes, graphQueryParams, layoutGraphNodes } from './graph.ts';

const graph = {
  nodes: [
    { slug: 'docs/start', title: 'Start', page_type: 'summary', link_count: 3 },
    { slug: 'docs/next', title: 'Next', page_type: 'entity', link_count: 1 },
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
