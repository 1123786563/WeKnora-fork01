export type AppRow = Record<string, unknown>;

export function actionControls(state: unknown, canDrive: boolean): { approve: boolean; execute: boolean } {
  return { approve: canDrive && state === 'awaiting_approval', execute: canDrive && state === 'authorized' };
}

export function authorizationStatus(state: unknown): { poll: boolean } {
  return { poll: state === 'pending' || state === 'authorizing' || state === 'verifying' };
}

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
