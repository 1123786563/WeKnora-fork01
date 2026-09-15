export type DataSourceErrorKind = 'forbidden' | 'auth' | 'not-found' | 'error';

export interface DataSourceErrorState {
  kind: DataSourceErrorKind;
  message: string;
}

export function classifyDataSourceError(value: unknown, fallback = 'Failed to load resources'): DataSourceErrorState {
  const row = typeof value === 'object' && value !== null ? value as Record<string, unknown> : {};
  const raw = value instanceof Error ? value.message : typeof row.message === 'string' ? row.message : typeof row.error === 'string' ? row.error : '';
  const code = row.code === undefined ? '' : String(row.code);
  const lower = `${raw} ${code}`.toLowerCase();
  if (lower.includes('status=403') || lower.includes('forbidden') || code === '1061004') return { kind: 'forbidden', message: raw || fallback };
  if (lower.includes('status=401') || lower.includes('unauthorized') || lower.includes('authentication') || lower.includes('invalid auth') || code === '1061005') return { kind: 'auth', message: raw || fallback };
  if (lower.includes('not found') || lower.includes('not_found') || code === '1061003') return { kind: 'not-found', message: raw || fallback };
  return { kind: 'error', message: raw || fallback };
}
