import { useEffect, useRef, useState } from 'react';
import type { ChatToolApprovalPrompt } from './page.tsx';
import type { ChatCopyTable } from './chat-copy.ts';

/** Serialized draft shown in the args editor when the card expands. */
export function initialApprovalArgsDraft(approval: Pick<ChatToolApprovalPrompt, 'arguments'>): string {
  return JSON.stringify(approval.arguments ?? {}, null, 2);
}

export type ApprovalArgsParseResult =
  | { ok: true; args: Record<string, unknown> }
  | { ok: false; error: string };

/** Live editing status mirroring the Vue ToolApprovalCard contract. */
export interface ApprovalArgsStatus {
  /** Empty drafts count as valid (Vue isJsonValid). */
  valid: boolean;
  /** Trimmed draft differs from the initial args (Vue argsDirty). */
  dirty: boolean;
}

/**
 * Vue ToolApprovalCard validates on every keystroke: an empty draft stays
 * valid, anything else must parse as JSON, and "dirty" flags edits away from
 * the original arguments so the card can show the argsModified hint.
 */
export function approvalArgsStatus(draft: string, initialDraft: string): ApprovalArgsStatus {
  const trimmed = draft.trim();
  let valid = true;
  if (trimmed) {
    try { JSON.parse(trimmed); } catch { valid = false; }
  }
  return { valid, dirty: trimmed !== initialDraft.trim() };
}

/** Parses the textarea draft; only JSON objects are accepted as tool arguments. */
export function parseApprovalArgsInput(draft: string, copy?: Pick<ChatCopyTable, 'approvalInvalidJson' | 'approvalArgsObject'>): ApprovalArgsParseResult {
  const trimmed = draft.trim();
  if (!trimmed) return { ok: true, args: {} };
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch (cause) {
    return { ok: false, error: cause instanceof Error ? `${copy?.approvalInvalidJson ?? 'Invalid JSON'}: ${cause.message}` : (copy?.approvalInvalidJson ?? 'Invalid JSON') };
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    return { ok: false, error: copy?.approvalArgsObject ?? 'Arguments must be a JSON object' };
  }
  return { ok: true, args: parsed as Record<string, unknown> };
}

export type ApprovalResolutionPayload =
  | { ok: true; decision: 'approve' | 'reject'; modifiedArgs?: Record<string, unknown>; reason?: string }
  | { ok: false; error: string };

/** Builds the payload for onResolveToolApproval; approve validates the edited args. */
export function approvalResolution(decision: 'approve' | 'reject', draft: string, expanded: boolean, copy?: Pick<ChatCopyTable, 'approvalInvalidJson' | 'approvalArgsObject' | 'approvalRejectedReason'>): ApprovalResolutionPayload {
  if (decision === 'reject') {
    // Vue sends the localized agentStream.toolApproval.userRejected reason.
    const reason = copy?.approvalRejectedReason;
    return reason ? { ok: true, decision: 'reject', reason } : { ok: true, decision: 'reject' };
  }
  if (!expanded) return { ok: true, decision: 'approve' };
  const parsed = parseApprovalArgsInput(draft, copy);
  if (!parsed.ok) return parsed;
  return { ok: true, decision: 'approve', modifiedArgs: parsed.args };
}

/** Vue countdown default: agentStream.toolApproval uses a 10 minute window. */
export const APPROVAL_DEFAULT_TIMEOUT_SECONDS = 600;

/**
 * Vue ToolApprovalCard deadline math: (requestedAt || 0) * 1000 +
 * (timeoutSeconds || 600) * 1000, floor to whole seconds and clamp at 0 so an
 * expired request pins the timer at 0 instead of going negative.
 */
export function approvalCountdownSeconds(nowMs: number, requestedAt?: number, timeoutSeconds?: number): number {
  const deadline = (requestedAt || 0) * 1000 + (timeoutSeconds || APPROVAL_DEFAULT_TIMEOUT_SECONDS) * 1000;
  return Math.max(0, Math.floor((deadline - nowMs) / 1000));
}

/**
 * Vue timerClass thresholds: critical at <= 30s, warning at <= 120s,
 * unstyled otherwise. The wk-timer-* classes are the DOM/test hooks.
 */
export function approvalTimerClass(secondsLeft: number): '' | 'wk-timer-warning' | 'wk-timer-critical' {
  if (secondsLeft <= 30) return 'wk-timer-critical';
  if (secondsLeft <= 120) return 'wk-timer-warning';
  return '';
}

/**
 * Vue formatCountdown: below 60s renders the localized countdownShort copy
 * ({seconds} placeholder), otherwise m:ss with zero-padded seconds.
 */
export function formatApprovalCountdown(secondsLeft: number, copy?: Pick<ChatCopyTable, 'approvalCountdownShort'>): string {
  if (secondsLeft < 60) {
    const template = copy?.approvalCountdownShort ?? '{seconds}s';
    return template.replace('{seconds}', String(secondsLeft));
  }
  const minutes = Math.floor(secondsLeft / 60);
  const rest = secondsLeft % 60;
  return `${minutes}:${rest.toString().padStart(2, '0')}`;
}

export interface ToolApprovalCardProps {
  approval: ChatToolApprovalPrompt;
  busy: boolean;
  onResolve?: (pendingId: string, decision: 'approve' | 'reject', modifiedArgs?: Record<string, unknown>, reason?: string) => Promise<void>;
  copy?: ChatCopyTable;
}

/** Expandable approval card: view/edit the tool call arguments as JSON before approving. */
export function ToolApprovalCard({ approval, busy, onResolve, copy }: ToolApprovalCardProps) {
  const [initialDraft] = useState(() => initialApprovalArgsDraft(approval));
  const [draft, setDraft] = useState(initialDraft);
  const [argsError, setArgsError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [now, setNow] = useState(() => Date.now());
  const submittingRef = useRef(false);
  const pending = approval.status === 'pending' && Boolean(onResolve);
  // Vue ToolApprovalCard ticks a 1s interval while unresolved to advance its
  // countdown; expiry never auto-rejects nor disables the actions.
  useEffect(() => {
    if (!pending) return;
    setNow(Date.now());
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [pending]);
  // Vue ToolApprovalCard validates on every keystroke: invalid JSON disables
  // the approve action, edits away from the original args show the
  // argsModified hint.
  const argsStatus = approvalArgsStatus(draft, initialDraft);
  const secondsLeft = approvalCountdownSeconds(now, approval.requestedAt, approval.timeoutSeconds);
  const timerClass = approvalTimerClass(secondsLeft);

  async function resolve(decision: 'approve' | 'reject') {
    if (!onResolve || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    const expanded = pending;
    const resolution = approvalResolution(decision, draft, expanded, copy);
    if (!resolution.ok) {
      setArgsError(resolution.error);
      submittingRef.current = false;
      setSubmitting(false);
      return;
    }
    setArgsError(null);
    try {
      await onResolve(approval.pendingId, resolution.decision, resolution.modifiedArgs, resolution.reason);
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  /* chat.css → utilities: .wk-chat-approval-* card family; the wk-* classes
     remain DOM/test hooks. The approval-scoped button family overrides the
     base .wk-list-actions button recipe (which itself lives in utilities). */
  /* T15：旧栈 utility 串语义化为 .wk-chat-approval-btn（含 enabled:hover /
     disabled 态，views-chat-u.css）。 */
  const approvalButton = 'wk-chat-approval-btn';
  return <div className="wk-chat-action-card wk-chat-approval-card wk-vc-tool-approval-1">
    <strong className="wk-vc-tool-approval-2">{copy?.approvalTitle ?? 'Tool approval'}: {approval.toolName ?? 'unknown tool'}</strong>
    {pending ? <div className="wk-chat-approval-editor wk-vc-tool-approval-3">
      <details className="wk-chat-approval-args wk-vc-tool-approval-4" onToggle={() => setArgsError(null)}>
        <summary className="wk-chat-approval-args-toggle wk-ease-ease wk-vc-tool-approval-5">{copy?.approvalViewArgs ?? 'View arguments'}</summary>
        <textarea
          className="wk-chat-approval-args-input wk-vc-tool-approval-6"
          rows={6}
          spellCheck={false}
          aria-label={`参数 ${approval.toolName ?? approval.pendingId}`}
          value={draft}
          onChange={(event) => { setDraft(event.target.value); setArgsError(null); }}
        />
        {argsError ? <p role="alert" className="wk-chat-approval-error wk-vc-tool-approval-7">{argsError}</p> : null}
      </details>
      {/* Vue shows the live args status both collapsed and expanded; one
          always-visible line after the editor covers both placements. */}
      {!argsStatus.valid ? <p role="alert" className="wk-chat-approval-invalid wk-vc-tool-approval-7">{copy?.approvalInvalidJson ?? 'Invalid JSON'}</p>
        : argsStatus.dirty ? <p role="status" className="wk-chat-approval-dirty wk-vc-tool-approval-8">{copy?.approvalArgsModified ?? 'Modified'}</p> : null}
      <div className="wk-list-actions wk-vc-tool-approval-9">
        <button type="button" disabled={busy || submitting} className={approvalButton} onClick={() => void resolve('reject')}>{copy?.approvalReject ?? 'Reject'}</button>
        <button type="button" disabled={busy || submitting || !argsStatus.valid} className={approvalButton} onClick={() => void resolve('approve')}>{copy?.approvalApprove ?? 'Approve'}</button>
        {/* Vue inline layout: "reject · approve · <timer>"; the timer stays
            visible pinned at 0 after expiry and disappears once resolved. */}
        <span aria-hidden="true" className="wk-vc-tool-approval-10">·</span>
        <span
          className={`wk-chat-approval-timer wk-vc-tool-approval-12 ${timerClass === 'wk-timer-warning' ? 'wk-timer-warning wk-vc-tool-approval-13' : timerClass === 'wk-timer-critical' ? 'wk-timer-critical wk-vc-tool-approval-14' : 'wk-vc-tool-approval-11'}`}
        >
          {formatApprovalCountdown(secondsLeft, copy)}
        </span>
      </div>
    </div> : <small className="wk-vc-tool-approval-11">{approval.decision ? `${copy?.approvalResolved ?? 'Resolved'}: ${approval.decision}` : (copy?.approvalResolved ?? 'Resolved')}</small>}
  </div>;
}
