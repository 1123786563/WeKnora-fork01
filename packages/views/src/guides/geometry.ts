// Pure spotlight geometry for the welcome tour, ported from
// frontend/src/components/SpotlightGuide.vue (constants lines 57-62,
// computeHighlightHole lines 118-145, backdropPieces lines 164-185,
// cardStyle lines 210-251). No DOM types beyond structural element access,
// so the math is unit-testable without jsdom.

export const GUIDE_CARD_WIDTH = 340;
export const GUIDE_GAP = 16;
export const GUIDE_EDGE = 16;
export const GUIDE_PAD = 8;
export const GUIDE_HOLE_RADIUS = 8;
export const GUIDE_BACKDROP_COLOR = 'rgba(15, 18, 22, 0.58)';

export interface GuideHoleRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface GuideBox {
  left: number;
  top: number;
  width: number;
  height: number;
  right: number;
  bottom: number;
}

export type GuidePlacementName = 'right' | 'left' | 'bottom' | 'top';

function overlaps(a: GuideBox, b: GuideBox): boolean {
  return !(a.right <= b.left || a.left >= b.right || a.bottom <= b.top || a.top >= b.bottom);
}

/**
 * Vue computeHighlightHole: inflate the target rect up to PAD px into the
 * free space around neighboring siblings, clamped to the viewport.
 */
export function computeHighlightHole(
  el: {
    previousElementSibling: Element | null;
    nextElementSibling: Element | null;
  },
  rect: { top: number; left: number; width: number; height: number; bottom: number; right: number },
  viewport: { getComputedStyle: (el: unknown) => { marginBottom: string } },
  vw: number,
  vh: number,
): GuideHoleRect {
  let above = GUIDE_PAD;
  const prev = el.previousElementSibling;
  if (prev) {
    above = Math.max(0, rect.top - prev.getBoundingClientRect().bottom);
  }
  let below = GUIDE_PAD;
  const next = el.nextElementSibling;
  if (next) {
    below = Math.max(0, next.getBoundingClientRect().top - rect.bottom);
  } else {
    const marginBottom = parseFloat(viewport.getComputedStyle(el).marginBottom) || 0;
    below = Math.max(0, GUIDE_PAD - marginBottom);
  }

  const inset = Math.min(GUIDE_PAD, above, below);
  let x = rect.left - inset;
  let y = rect.top - inset;
  let width = rect.width + inset * 2;
  let height = rect.height + inset * 2;

  if (x < 0) {
    width += x;
    x = 0;
  }
  if (y < 0) {
    height += y;
    y = 0;
  }
  const rightOverflow = x + width - vw;
  if (rightOverflow > 0) {
    width -= rightOverflow;
  }
  const bottomOverflow = y + height - vh;
  if (bottomOverflow > 0) {
    height -= bottomOverflow;
  }
  return { x, y, width, height };
}

/**
 * Vue cardStyle: prefer the step placement, fall through the remaining sides
 * while the card would overlap the hole, then park below the hole;
 * target-less steps get a centered card slightly above the fold.
 */
export function computeCardStyle(
  vw: number,
  vh: number,
  hole: GuideHoleRect | null,
  cardSize: { width: number; height: number },
  placement?: GuidePlacementName,
): { width: string; left: string; top: string } {
  const width = Math.min(GUIDE_CARD_WIDTH, vw - GUIDE_EDGE * 2);
  const height = cardSize.height;

  if (!hole) {
    return {
      width: String(width) + 'px',
      left: String((vw - width) / 2) + 'px',
      top: String(Math.max(GUIDE_EDGE, vh * 0.32 - height / 2)) + 'px',
    };
  }

  const holeBox: GuideBox = {
    left: hole.x,
    top: hole.y,
    right: hole.x + hole.width,
    bottom: hole.y + hole.height,
    width: hole.width,
    height: hole.height,
  };
  const all: GuidePlacementName[] = ['right', 'left', 'bottom', 'top'];
  const preferred = placement ?? 'right';
  const order: GuidePlacementName[] = [preferred, ...all.filter((p) => p !== preferred)];

  const candidates: Record<GuidePlacementName, { left: number; top: number }> = {
    right: { left: holeBox.right + GUIDE_GAP, top: hole.y + hole.height / 2 - height / 2 },
    left: { left: holeBox.left - width - GUIDE_GAP, top: hole.y + hole.height / 2 - height / 2 },
    bottom: { left: hole.x + hole.width / 2 - width / 2, top: holeBox.bottom + GUIDE_GAP },
    top: { left: hole.x + hole.width / 2 - width / 2, top: holeBox.top - height - GUIDE_GAP },
  };

  for (const place of order) {
    const candidate = candidates[place];
    const left = Math.min(Math.max(GUIDE_EDGE, candidate.left), vw - width - GUIDE_EDGE);
    const top = Math.min(Math.max(GUIDE_EDGE, candidate.top), vh - height - GUIDE_EDGE);
    const cardBox: GuideBox = { left, top, right: left + width, bottom: top + height, width, height };
    if (!overlaps(cardBox, holeBox)) {
      return { width: String(width) + 'px', left: String(left) + 'px', top: String(top) + 'px' };
    }
  }

  const left = Math.min(Math.max(GUIDE_EDGE, (vw - width) / 2), vw - width - GUIDE_EDGE);
  const top = Math.min(Math.max(GUIDE_EDGE, holeBox.bottom + GUIDE_GAP), vh - height - GUIDE_EDGE);
  return { width: String(width) + 'px', left: String(left) + 'px', top: String(top) + 'px' };
}

/** Vue backdropPieces: the four rectangles around the hole. */
export function computeBackdropPieces(vw: number, vh: number, hole: GuideHoleRect): Array<{ top: string; left: string; width: string; height: string }> {
  return [
    { top: '0px', left: '0px', width: String(vw) + 'px', height: String(hole.y) + 'px' },
    {
      top: String(hole.y + hole.height) + 'px',
      left: '0px',
      width: String(vw) + 'px',
      height: String(Math.max(0, vh - hole.y - hole.height)) + 'px',
    },
    { top: String(hole.y) + 'px', left: '0px', width: String(hole.x) + 'px', height: String(hole.height) + 'px' },
    {
      top: String(hole.y) + 'px',
      left: String(hole.x + hole.width) + 'px',
      width: String(Math.max(0, vw - hole.x - hole.width)) + 'px',
      height: String(hole.height) + 'px',
    },
  ];
}
