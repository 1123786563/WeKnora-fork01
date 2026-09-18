import type { SteerMentionItem } from './steer-submit.ts';

/*
 * R473-A2 — Vue steer queue parity (frontend/src/views/chat/index.vue
 * `steerQueue` ref). A follow-up queued while a steer-capable agent turn runs
 * is tracked locally so the composer can render one chip per pending
 * after-message (Input-field.vue .steer-queue) with promote / remove / retry
 * affordances, and so the completed turn can consume the backlog:
 *
 * - `pending`  — POST /steer still in flight (Vue item.pending); actions stay
 *   disabled until the server-issued queue entry exists.
 * - `queued`   — accepted by the server; it fires as a follow-up turn once the
 *   current run exits (the server owns the backlog).
 * - `failed`   — the enqueue POST failed (Vue item.failed); the chip keeps a
 *   retry affordance.
 * - `awaitingIdleSend` — the server answered new_run while a stream was still
 *   attached (Vue item.awaitingIdleSend): nothing is queued server-side, so
 *   the host itself sends the message once the current stream settles, one
 *   item per completed turn (flushSteerAfterTurn).
 */

export type WebSteerQueueStatus = 'pending' | 'queued' | 'failed';

export interface WebSteerQueueItem {
  steerId: string;
  content: string;
  status: WebSteerQueueStatus;
  mentionedItems?: SteerMentionItem[];
  awaitingIdleSend?: boolean;
}

/** Server-side backlog entry from GET /steer (contracts SteerQueueItem). */
export interface SteerServerQueueItem {
  steer_id: string;
  content: string;
  delivery?: string;
}

export function enqueueSteerItem(queue: readonly WebSteerQueueItem[], input: {
  steerId: string;
  content: string;
  mentionedItems?: SteerMentionItem[];
}): WebSteerQueueItem[] {
  return [...queue, {
    steerId: input.steerId,
    content: input.content,
    status: 'pending' as const,
    ...(input.mentionedItems && input.mentionedItems.length > 0 ? { mentionedItems: input.mentionedItems } : {}),
  }];
}

/**
 * The POST resolved: the item is live on the server. Vue handleSteerMsg
 * rebases `queued.steer_id` onto res.steer_id because an older backend may
 * issue its own id.
 */
export function settleSteerItem(queue: readonly WebSteerQueueItem[], steerId: string, serverSteerId?: string): WebSteerQueueItem[] {
  const nextId = serverSteerId && serverSteerId.trim() ? serverSteerId : steerId;
  return queue.map((item) => item.steerId === steerId ? { ...item, steerId: nextId, status: 'queued' as const } : item);
}

/** Vue new_run while still streaming: send locally once the turn completes. */
export function markSteerAwaitingIdleSend(queue: readonly WebSteerQueueItem[], steerId: string): WebSteerQueueItem[] {
  return queue.map((item) => item.steerId === steerId ? { ...item, status: 'queued' as const, awaitingIdleSend: true } : item);
}

/** The enqueue failed; the chip keeps the retry affordance (Vue item.failed). */
export function failSteerItem(queue: readonly WebSteerQueueItem[], steerId: string): WebSteerQueueItem[] {
  return queue.map((item) => item.steerId === steerId ? { ...item, status: 'failed' as const } : item);
}

/** Consumed by the run (injected / follow-up claim), promoted or cancelled. */
export function dropSteerItem(queue: readonly WebSteerQueueItem[], steerId: string): WebSteerQueueItem[] {
  return queue.filter((item) => item.steerId !== steerId);
}

/** Vue stop confirmed / session switch: the whole queue is cancelled. */
export function clearSteerQueue(): WebSteerQueueItem[] {
  return [];
}

/** First item the host must send itself after the current turn (one per turn). */
export function nextSteerIdleSend(queue: readonly WebSteerQueueItem[]): WebSteerQueueItem | undefined {
  return queue.find((item) => item.awaitingIdleSend && item.status !== 'failed' && item.status !== 'pending');
}

/**
 * Vue hydrateSteerQueue: the server list owns the backlog — items it no
 * longer returns moved into the follow-up run and leave the queue, while
 * locally failed entries (never accepted server-side) and awaiting-idle-send
 * / still-pending entries survive with their local flags.
 */
export function syncSteerQueueFromServer(queue: readonly WebSteerQueueItem[], serverItems: readonly SteerServerQueueItem[]): WebSteerQueueItem[] {
  const serverIds = new Set(serverItems.map((item) => item.steer_id));
  const adopted = serverItems.map((item): WebSteerQueueItem => {
    const local = queue.find((entry) => entry.steerId === item.steer_id);
    return {
      steerId: item.steer_id,
      content: item.content,
      status: 'queued' as const,
      ...(local?.mentionedItems && local.mentionedItems.length > 0 ? { mentionedItems: local.mentionedItems } : {}),
    };
  });
  const preserved = queue.filter((item) =>
    !serverIds.has(item.steerId)
    && (item.status === 'failed' || item.awaitingIdleSend || item.status === 'pending'));
  return [...adopted, ...preserved];
}

/** Queue projection consumed by the composer chip strip. */
export function steerQueueChips(queue: readonly WebSteerQueueItem[]): Array<{ steerId: string; content: string; status: WebSteerQueueStatus }> {
  return queue.map((item) => ({ steerId: item.steerId, content: item.content, status: item.status }));
}
