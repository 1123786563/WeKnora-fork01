// SP11 Task 9 — analytics date-range helpers.
// The dashboard's date pickers and the /api/v1/analytics endpoints speak
// YYYY-MM-DD strings bucketed by UTC date, so every boundary computed here is
// anchored to the UTC day (not the browser's local midnight). Extracted as
// pure functions so the 30-day default, the inverted fallback and the 366-day
// cap stay unit-testable without mounting the page.
export interface AnalyticsDateRange {
  startTime: string;
  endTime: string;
}

/** Inclusive cap on the selectable window (a leap year plus a day). */
export const ANALYTICS_RANGE_MAX_DAYS = 366;

const DAY_MS = 86_400_000;

/** Date → 'YYYY-MM-DD' in UTC. */
export function toISODate(d: Date): string {
  return d.toISOString().slice(0, 10);
}

/** Default window: [today − 30 days, today] at the UTC day boundary. */
export function defaultAnalyticsRange(now = new Date()): AnalyticsDateRange {
  const end = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  const start = new Date(end);
  start.setUTCDate(start.getUTCDate() - 30);
  return { startTime: toISODate(start), endTime: toISODate(end) };
}

function parseISODateMs(value: string): number | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
  const ms = Date.parse(`${value}T00:00:00Z`);
  return Number.isNaN(ms) ? null : ms;
}

/**
 * Sanitize user-supplied bounds: malformed or inverted input falls back to the
 * default window; spans beyond 366 days are truncated by moving startTime
 * forward (endTime stays anchored — "up to now" is the natural reading of a
 * dashboard window).
 */
export function clampAnalyticsRange(startTime: string, endTime: string, now = new Date()): AnalyticsDateRange {
  const startMs = parseISODateMs(startTime);
  const endMs = parseISODateMs(endTime);
  if (startMs === null || endMs === null || endMs < startMs) return defaultAnalyticsRange(now);
  if (endMs - startMs > ANALYTICS_RANGE_MAX_DAYS * DAY_MS) {
    return { startTime: toISODate(new Date(endMs - ANALYTICS_RANGE_MAX_DAYS * DAY_MS)), endTime: toISODate(new Date(endMs)) };
  }
  return { startTime, endTime };
}
