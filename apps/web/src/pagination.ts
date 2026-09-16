// Shared server-backed pager state, matching the documents-list pager pattern
// (page state + page_size param + Previous/Next nav). Used to replace the
// FAQ/Wiki 50-entry hard caps with real pagination.
export interface PagerState {
  page: number;
  pageSize: number;
  total: number;
  start: number;
  end: number;
  hasPrevious: boolean;
  hasNext: boolean;
}

export function pagerState(total: number, page: number, pageSize: number): PagerState {
  const safeTotal = Number.isFinite(total) && total > 0 ? Math.floor(total) : 0;
  const pageCount = Math.max(1, Math.ceil(safeTotal / pageSize));
  const safePage = Math.min(Math.max(1, Math.floor(page) || 1), pageCount);
  const start = safeTotal === 0 ? 0 : (safePage - 1) * pageSize + 1;
  const end = Math.min(safePage * pageSize, safeTotal);
  return {
    page: safePage,
    pageSize,
    total: safeTotal,
    start,
    end,
    hasPrevious: safePage > 1,
    hasNext: safePage * pageSize < safeTotal,
  };
}

/** Client-side page slice (fallback path when a response arrives unpaginated). */
export function slicePage<T>(items: readonly T[], page: number, pageSize: number): T[] {
  const state = pagerState(items.length, page, pageSize);
  return items.slice(state.start - 1, state.end);
}
