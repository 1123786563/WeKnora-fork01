// Pure spotlight-geometry tests (no DOM): the placement math ported from
// frontend/src/components/SpotlightGuide.vue cardStyle/backdropPieces.
import assert from 'node:assert/strict';
import test from 'node:test';

import {
  computeBackdropPieces,
  computeCardStyle,
  GUIDE_BACKDROP_COLOR,
  GUIDE_CARD_WIDTH,
} from './geometry.ts';

const VIEW = { vw: 1280, vh: 800, size: { width: 340, height: 220 } };

test('target-less steps render a centered card above the fold like the Vue cardStyle', () => {
  const style = computeCardStyle(VIEW.vw, VIEW.vh, null, VIEW.size, undefined);
  assert.equal(style.width, '340px');
  assert.equal(style.left, String((VIEW.vw - 340) / 2) + 'px');
  assert.equal(style.top, String(Math.max(16, VIEW.vh * 0.32 - 110)) + 'px');
});

test('preferred placement wins when the card fits without overlapping the hole', () => {
  const hole = { x: 0, y: 300, width: 200, height: 60 };
  const right = computeCardStyle(VIEW.vw, VIEW.vh, hole, VIEW.size, 'right');
  assert.equal(right.left, String(hole.x + hole.width + 16) + 'px');
  assert.equal(right.top, String(hole.y + hole.height / 2 - 110) + 'px');

  const left = computeCardStyle(VIEW.vw, VIEW.vh, { x: 1200, y: 300, width: 80, height: 60 }, VIEW.size, 'left');
  assert.equal(left.left, String(1200 - 340 - 16) + 'px');
});

test('a sidebar-sized hole keeps the card clear and clamped to the viewport', () => {
  const hole = { x: 0, y: 300, width: 240, height: 44 };
  const style = computeCardStyle(VIEW.vw, VIEW.vh, hole, VIEW.size, 'right');
  const left = parseInt(style.left, 10);
  const top = parseInt(style.top, 10);
  assert.ok(left >= 16 && top >= 16, 'clamped to EDGE');
  assert.ok(left + GUIDE_CARD_WIDTH <= VIEW.vw - 16, 'right edge inside viewport');
  const intersects = !(left >= hole.x + hole.width || left + GUIDE_CARD_WIDTH <= hole.x || top >= hole.y + hole.height || top + VIEW.size.height <= hole.y);
  assert.ok(!intersects, 'card must not overlap the hole');
});

test('when every placement overlaps, the card parks below the hole exactly like the Vue fallback', () => {
  // Degenerate near-fullscreen hole: all four candidates overlap, so the
  // algorithm falls back to horizontally-centered, below-the-hole positions
  // clamped into the viewport (SpotlightGuide.vue lines 248-250).
  const hole = { x: 0, y: 0, width: 1200, height: 700 };
  const style = computeCardStyle(VIEW.vw, VIEW.vh, hole, VIEW.size, 'right');
  assert.equal(style.left, String(Math.min(Math.max(16, (VIEW.vw - 340) / 2), VIEW.vw - 340 - 16)) + 'px');
  assert.equal(style.top, String(Math.min(Math.max(16, hole.y + hole.height + 16), VIEW.vh - VIEW.size.height - 16)) + 'px');
});

test('backdrop pieces tile the viewport around the hole', () => {
  const hole = { x: 100, y: 100, width: 200, height: 100 };
  const pieces = computeBackdropPieces(VIEW.vw, VIEW.vh, hole);
  assert.equal(pieces.length, 4);
  const [topPiece, bottomPiece, leftPiece, rightPiece] = pieces;
  assert.equal(topPiece.height, '100px');
  assert.equal(bottomPiece.height, String(VIEW.vh - 200) + 'px');
  assert.equal(leftPiece.width, '100px');
  assert.equal(rightPiece.width, String(VIEW.vw - 300) + 'px');
});

test('backdrop color and card width match the Vue constants', () => {
  assert.equal(GUIDE_CARD_WIDTH, 340);
  assert.equal(GUIDE_BACKDROP_COLOR, 'rgba(15, 18, 22, 0.58)');
});
