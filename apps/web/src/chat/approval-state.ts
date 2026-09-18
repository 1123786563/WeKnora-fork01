import { responseType, type ChatStreamEvent } from '@weknora/contracts';
import type { ChatApproval, ChatOAuthApproval } from '@weknora/domain/chat/reducer';

/** Vue ToolApprovalCard countdown inputs, mirrored from the SSE payload. */
export interface ApprovalTiming {
  /** Unix seconds the approval was requested (payload requested_at). */
  requestedAt?: number;
  /** Approval window in seconds (payload timeout_seconds). */
  timeoutSeconds?: number;
}

function eventPayload(event: ChatStreamEvent): Record<string, unknown> {
  return typeof event.data === 'object' && event.data !== null ? event.data as Record<string, unknown> : event;
}

function numberOf(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

/**
 * The SSE tool_approval_required payload carries the Vue countdown inputs
 * (requested_at in unix seconds, timeout_seconds), but the domain reducer
 * drops them — so the web layer re-extracts them from the raw stream event.
 * Returns null for non-approval events.
 */
export function extractApprovalTiming(event: ChatStreamEvent): ApprovalTiming | null {
  if (responseType(event) !== 'tool_approval_required') return null;
  const payload = eventPayload(event);
  const timing: ApprovalTiming = {};
  const requestedAt = numberOf(payload.requested_at);
  const timeoutSeconds = numberOf(payload.timeout_seconds);
  if (requestedAt !== undefined) timing.requestedAt = requestedAt;
  if (timeoutSeconds !== undefined) timing.timeoutSeconds = timeoutSeconds;
  return timing;
}

/** Merges remembered countdown inputs onto approval prompts by pendingId. */
export function withApprovalTiming<T extends { pendingId: string }>(approvals: readonly T[], timing: ReadonlyMap<string, ApprovalTiming>): Array<T & ApprovalTiming> {
  return approvals.map((approval) => {
    const found = timing.get(approval.pendingId);
    return found ? { ...approval, ...found } : { ...approval };
  });
}

export function applyToolApprovalResolution(
  approvals: Readonly<Record<string, ChatApproval>>,
  pendingId: string,
  decision: string,
): Record<string, ChatApproval> {
  const current = approvals[pendingId];
  if (!current) return { ...approvals };
  return {
    ...approvals,
    [pendingId]: { ...current, status: 'resolved', decision },
  };
}

export function applyOAuthApprovalResolution(
  approvals: Readonly<Record<string, ChatOAuthApproval>>,
  pendingId: string,
  authorized: boolean,
): Record<string, ChatOAuthApproval> {
  const current = approvals[pendingId];
  if (!current) return { ...approvals };
  return {
    ...approvals,
    [pendingId]: { ...current, status: 'resolved', authorized },
  };
}

export function applyOAuthApprovalCancellation(
  approvals: Readonly<Record<string, ChatOAuthApproval>>,
  pendingId: string,
  reason: string,
): Record<string, ChatOAuthApproval> {
  const current = approvals[pendingId];
  if (!current) return { ...approvals };
  return {
    ...approvals,
    [pendingId]: { ...current, status: 'resolved', authorized: false, reason },
  };
}
