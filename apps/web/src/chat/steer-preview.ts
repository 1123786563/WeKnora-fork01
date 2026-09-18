import type { ChatMessage } from '@weknora/api-client';

import type { SteerMentionItem } from './steer-submit.ts';

/*
 * R475-A3 — Vue inject optimistic preview parity.
 *
 * Vue chat/index.vue handleSteerMsg (delivery === 'inject') calls
 * previewSteerMessage (frontend/src/utils/steerStreamFork.ts) right before the
 * POST /steer goes out: the typed draft surfaces immediately as a user bubble
 * flagged `_steerPending`, so the input never vanishes between submit and the
 * SSE receipt. The receipt (user_message_injected) replaces the optimistic
 * row; a failed enqueue keeps it on screen flagged `_steerFailed` (the chip
 * retry re-runs the same inject); a new_run degrade-to-send, a confirmed stop
 * or a session switch discards it.
 *
 * React equivalents on the immutable ChatMessage[] state:
 * - `steer_pending` → Vue `_steerPending`
 * - `steer_failed`  → Vue `_steerFailed`
 * - `steer_id` / `isSteer` → same field names the stream feed already uses.
 *
 * The optimistic row reuses the SSE injected-row id shape
 * (`injected-<steerId>`, ChatRoutePage.createStreamFeed) so the receipt
 * naturally replaces it by id even before steer-id matching runs.
 */

/** Row id the SSE feed uses for an injected steer user message. */
export function injectedSteerRowId(steerId: string): string {
  return `injected-${steerId}`;
}

/**
 * Append the optimistic pending user bubble for an inject submission
 * (idempotent). A retry over an existing row clears the failed flag — Vue
 * handleSteerMsg re-previews then runs `delete preview._steerFailed`.
 */
export function previewSteerUserMessage(messages: readonly ChatMessage[], input: {
  sessionId: string;
  steerId: string;
  content: string;
  mentionedItems?: readonly SteerMentionItem[];
}): ChatMessage[] {
  const existingIndex = messages.findIndex((row) => row.role === 'user' && row.steer_id === input.steerId);
  if (existingIndex >= 0) {
    const existing = messages[existingIndex];
    if (existing.steer_failed === undefined) return [...messages];
    const settled = { ...existing };
    delete settled.steer_failed;
    return messages.map((row, index) => (index === existingIndex ? settled : row));
  }
  const row: ChatMessage = {
    id: injectedSteerRowId(input.steerId),
    session_id: input.sessionId,
    role: 'user',
    content: input.content,
    is_completed: true,
    steer_id: input.steerId,
    isSteer: true,
    steer_pending: true,
    ...(input.mentionedItems && input.mentionedItems.length > 0 ? { mentioned_items: [...input.mentionedItems] } : {}),
  };
  return [...messages, row];
}

/** The enqueue failed: keep the bubble, flag it (Vue preview._steerFailed = true). */
export function markSteerPreviewFailed(messages: readonly ChatMessage[], steerId: string): ChatMessage[] {
  return messages.map((row) => row.steer_id === steerId && row.steer_pending ? { ...row, steer_failed: true } : row);
}

/** The server already injected it: settle the optimistic row (Vue deletes _steerPending). */
export function clearSteerPreviewPending(messages: readonly ChatMessage[], steerId: string): ChatMessage[] {
  return messages.map((row) => {
    if (row.steer_id !== steerId) return row;
    const next = { ...row };
    delete next.steer_pending;
    delete next.steer_failed;
    return next;
  });
}

/**
 * Remove pending preview rows. Matching is by steer id and, optionally, by row
 * id — the SSE receipt knows the row id shape even when an older backend
 * issued its own steer id. No ids at all drops every pending preview (Vue
 * handleStopConfirmed / session switch).
 */
export function discardSteerPreviews(messages: readonly ChatMessage[], steerIds?: readonly string[], rowIds?: readonly string[]): ChatMessage[] {
  return messages.filter((row) => {
    if (!row.steer_pending) return true;
    if (steerIds === undefined && rowIds === undefined) return false;
    const bySteerId = steerIds?.includes(String(row.steer_id)) ?? false;
    const byRowId = rowIds?.includes(row.id) ?? false;
    return !(bySteerId || byRowId);
  });
}

/**
 * Receipt reconcile (Vue reconcileSteerMessageId): the POST answered with the
 * server steer id. When the SSE receipt already landed the persisted row the
 * optimistic duplicate goes away; otherwise the optimistic row rebases onto the
 * server id (both `steer_id` and the `injected-<id>` row id) so the SSE
 * receipt replaces it by id.
 */
export function reconcileSteerPreview(messages: readonly ChatMessage[], clientSteerId: string, serverSteerId: string): ChatMessage[] {
  if (!serverSteerId || serverSteerId === clientSteerId) return [...messages];
  const receiptLanded = messages.some((row) => row.role === 'user' && row.steer_id === serverSteerId && !row.steer_pending);
  if (receiptLanded) {
    return messages.filter((row) => !(row.role === 'user' && row.steer_id === clientSteerId && row.steer_pending));
  }
  return messages.map((row) => row.role === 'user' && row.steer_id === clientSteerId
    ? { ...row, steer_id: serverSteerId, id: injectedSteerRowId(serverSteerId) }
    : row);
}
