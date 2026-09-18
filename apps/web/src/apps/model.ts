export type AppRow = Record<string, unknown>;

/* actionControls lives in actionState.ts (verbatim Vue port); the approval
   polling backoff lives in pollBackoff.ts. Risk/state labels resolve through
   the generated apps i18n block (@weknora/i18n apps.*), not hardcoded copy. */

function object(value: unknown): AppRow {
  return value && typeof value === 'object' ? value as AppRow : {};
}

export function appRows(value: unknown): AppRow[] {
  if (Array.isArray(value)) return value.map(object);
  const root = object(value);
  const data = root.data;
  if (Array.isArray(data)) return data.map(object);
  const nested = object(data);
  for (const candidate of [nested.items, nested.rows, root.items, root.rows]) {
    if (Array.isArray(candidate)) return candidate.map(object);
  }
  return [];
}

export function appErrorMessage(cause: unknown): string {
  return cause instanceof Error && cause.message ? cause.message : '应用页面加载失败';
}

export function appStatus(value: unknown, fallback = '—'): string {
  const text = String(value ?? '').trim();
  return text || fallback;
}

export function appShort(value: unknown): string {
  const text = String(value ?? '').trim();
  return text.length > 18 ? `${text.slice(0, 18)}…` : text || '—';
}

/** Vue shortDigest: a 12-character digest preview, not the generic ID preview. */
export function appDigest(value: unknown): string {
  const digest = String(value ?? '').trim();
  return digest.length > 12 ? `${digest.slice(0, 12)}…` : digest || '—';
}
