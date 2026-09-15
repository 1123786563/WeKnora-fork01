import type { ChatApproval, ChatOAuthApproval } from '@weknora/domain/chat/reducer';

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
