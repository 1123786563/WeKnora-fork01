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

export type AppBadgeTone = 'neutral' | 'success' | 'warning' | 'danger';

/** Vue AppsView riskTheme/riskLabel parity for the reviewed action catalog. */
export function appRisk(value: unknown): { label: string; tone: AppBadgeTone } {
  const risk = String(value ?? '').trim();
  const labels: Record<string, string> = { read: '只读', write: '写入', send: '发送', delete: '删除' };
  const tones: Record<string, AppBadgeTone> = { read: 'success', write: 'warning', send: 'danger', delete: 'danger' };
  return { label: labels[risk] ?? (risk || '—'), tone: tones[risk] ?? 'neutral' };
}

/** Vue shortDigest: a 12-character digest preview, not the generic ID preview. */
export function appDigest(value: unknown): string {
  const digest = String(value ?? '').trim();
  return digest.length > 12 ? `${digest.slice(0, 12)}…` : digest || '—';
}

/** Vue installationStateLabel parity, including the explicit disabled state. */
export function installationState(value: unknown): { label: string; tone: AppBadgeTone } {
  const state = String(value ?? '').trim();
  if (state === 'active') return { label: '活跃', tone: 'success' };
  if (state === 'disabled') return { label: '已停用', tone: 'neutral' };
  return { label: state ? `状态：${state}` : '—', tone: 'neutral' };
}
