import type { ChatMessage, ChatSession } from '@weknora/contracts';

export interface ScrollMetrics {
  scrollTop: number;
  scrollHeight: number;
  clientHeight: number;
}

export function shouldStickToBottom(metrics: ScrollMetrics, threshold = 96): boolean {
  return metrics.scrollHeight - (metrics.scrollTop + metrics.clientHeight) <= threshold;
}

export function scrollTopAfterPrepend(previousTop: number, previousHeight: number, nextHeight: number): number {
  return previousTop + Math.max(0, nextHeight - previousHeight);
}

export function appendMessages(current: readonly ChatMessage[], incoming: readonly ChatMessage[]): ChatMessage[] {
  const byId = new Map<string, ChatMessage>();
  for (const message of [...current, ...incoming]) if (!byId.has(message.id)) byId.set(message.id, message);
  return [...byId.values()].sort((a, b) => {
    const left = a.created_at ?? '';
    const right = b.created_at ?? '';
    return left.localeCompare(right) || a.id.localeCompare(b.id);
  });
}

export function hasOlderMessages(batch: readonly ChatMessage[], pageSize: number): boolean {
  return batch.length >= pageSize;
}

export function sessionPageCount(total: number, pageSize: number): number {
  return pageSize > 0 && total > 0 ? Math.ceil(total / pageSize) : 1;
}

/** True while a turn streams but no assistant answer content has arrived yet. */
export function shouldShowTypingIndicator(messages: readonly ChatMessage[], streaming: boolean): boolean {
  return streaming && !messages.some((message) => message.role === 'assistant' && message.content.trim() !== '');
}

export function hasSessionChanged(previousSessionId: string | null, nextSessionId: string | null): boolean {
  return previousSessionId !== nextSessionId;
}

export type SessionDateGroupKey = 'pinned' | 'today' | 'yesterday' | 'last7Days' | 'last30Days' | 'older' | 'all';
export interface SessionGroup { key: SessionDateGroupKey; label: string; items: ChatSession[]; }

function dateBucket(value: string | undefined, now = new Date()): Exclude<SessionDateGroupKey, 'pinned'> {
  if (!value) return 'older';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'older';
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const day = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  const oneDay = 24 * 60 * 60 * 1000;
  if (day >= today) return 'today';
  if (day >= today - oneDay) return 'yesterday';
  if (day >= today - 7 * oneDay) return 'last7Days';
  if (day >= today - 30 * oneDay) return 'last30Days';
  return 'older';
}

export function sessionGroups(sessions: readonly ChatSession[], now = new Date(), mode: 'none' | 'date' = 'date'): SessionGroup[] {
  if (mode === 'none') {
    const pinned = sessions.filter((session) => session.is_pinned);
    const rest = sessions.filter((session) => !session.is_pinned);
    return [
      ...(pinned.length ? [{ key: 'pinned' as const, label: 'pinned', items: pinned }] : []),
      ...(rest.length ? [{ key: 'all' as const, label: '', items: rest }] : []),
    ];
  }
  const keys: SessionDateGroupKey[] = ['pinned', 'today', 'yesterday', 'last7Days', 'last30Days', 'older'];
  const buckets = new Map(keys.map((key) => [key, [] as ChatSession[]]));
  for (const session of sessions) buckets.get(session.is_pinned ? 'pinned' : dateBucket(session.updated_at ?? session.created_at, now))!.push(session);
  return keys.filter((key) => buckets.get(key)!.length > 0).map((key) => ({ key, label: key, items: buckets.get(key)! }));
}