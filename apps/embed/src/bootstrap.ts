import type { EmbedSession } from '@weknora/api-client';

export interface StoredEmbedSession extends EmbedSession {
  agentId?: string;
}

export function channelIdFromPath(pathname: string): string {
  const match = /^\/embed\/([^/]+)\/?$/.exec(pathname);
  if (!match) return '';
  try { return decodeURIComponent(match[1]); } catch { return ''; }
}

export function createVisitorId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID();
  return `visitor-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function readVisitorId(channelId: string, storage: Storage | undefined = typeof localStorage === 'undefined' ? undefined : localStorage): string {
  if (!storage || !channelId) return createVisitorId();
  const key = `weknora-embed-visitor:${channelId}`;
  try {
    const existing = storage.getItem(key)?.trim();
    if (existing) return existing;
    const created = createVisitorId();
    storage.setItem(key, created);
    return created;
  } catch { return createVisitorId(); }
}

export function readStoredSession(channelId: string, storage: Storage | undefined = typeof localStorage === 'undefined' ? undefined : localStorage): StoredEmbedSession | null {
  if (!storage || !channelId) return null;
  try {
    const parsed: unknown = JSON.parse(storage.getItem(`weknora-embed-session:${channelId}`) ?? 'null');
    if (parsed && typeof parsed === 'object' && typeof (parsed as EmbedSession).id === 'string' && typeof (parsed as EmbedSession).signature === 'string') {
      const agentId = typeof (parsed as StoredEmbedSession).agentId === 'string' ? (parsed as StoredEmbedSession).agentId : undefined;
      return { id: (parsed as EmbedSession).id, signature: (parsed as EmbedSession).signature, ...(agentId ? { agentId } : {}) };
    }
  } catch { /* malformed local state is ignored */ }
  return null;
}

export function writeStoredSession(channelId: string, session: StoredEmbedSession | null, storage: Storage | undefined = typeof localStorage === 'undefined' ? undefined : localStorage): void {
  if (!storage || !channelId) return;
  try {
    const key = `weknora-embed-session:${channelId}`;
    if (session) storage.setItem(key, JSON.stringify(session));
    else storage.removeItem(key);
  } catch { /* persistence is best effort */ }
}

export function parentOriginFromReferrer(referrer: string): string {
  try {
    const url = new URL(referrer);
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return '';
    return url.origin === 'null' ? '' : url.origin;
  } catch { return ''; }
}
