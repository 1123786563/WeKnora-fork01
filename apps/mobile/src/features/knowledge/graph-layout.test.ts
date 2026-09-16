import assert from 'node:assert/strict';
import test from 'node:test';
import { graphNodePositions } from './graph-layout.ts';

const nodes = [
  { slug: 'a', title: 'A', page_type: 'summary', link_count: 1, familiar: false },
  { slug: 'b', title: 'B', page_type: 'summary', link_count: 1, familiar: false },
] as any;

test('native graph layout is deterministic and highlights the center node', () => {
  assert.deepEqual(graphNodePositions(nodes, 'b').map(({ slug, left, top, tone }) => ({ slug, left, top, tone })), [
    { slug: 'a', left: 0, top: 0, tone: 'muted' },
    { slug: 'b', left: 50, top: 0, tone: 'primary' },
  ]);
});
