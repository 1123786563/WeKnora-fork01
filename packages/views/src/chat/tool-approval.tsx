import { useState } from 'react';
import type { ChatToolApprovalPrompt } from './page.tsx';

/** Serialized draft shown in the args editor when the card expands. */
export function initialApprovalArgsDraft(approval: Pick<ChatToolApprovalPrompt, 'arguments'>): string {
  return JSON.stringify(approval.arguments ?? {}, null, 2);
}

export type ApprovalArgsParseResult =
  | { ok: true; args: Record<string, unknown> }
  | { ok: false; error: string };

/** Parses the textarea draft; only JSON objects are accepted as tool arguments. */
export function parseApprovalArgsInput(draft: string): ApprovalArgsParseResult {
  const trimmed = draft.trim();
  if (!trimmed) return { ok: true, args: {} };
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch (cause) {
    return { ok: false, error: cause instanceof Error ? `Invalid JSON: ${cause.message}` : 'Invalid JSON' };
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    return { ok: false, error: 'Arguments must be a JSON object' };
  }
  return { ok: true, args: parsed as Record<string, unknown> };
}

export type ApprovalResolutionPayload =
  | { ok: true; decision: 'approve' | 'reject'; modifiedArgs?: Record<string, unknown> }
  | { ok: false; error: string };

/** Builds the payload for onResolveToolApproval; approve validates the edited args. */
export function approvalResolution(decision: 'approve' | 'reject', draft: string, expanded: boolean): ApprovalResolutionPayload {
  if (decision === 'reject') return { ok: true, decision: 'reject' };
  if (!expanded) return { ok: true, decision: 'approve' };
  const parsed = parseApprovalArgsInput(draft);
  if (!parsed.ok) return parsed;
  return { ok: true, decision: 'approve', modifiedArgs: parsed.args };
}

export interface ToolApprovalCardProps {
  approval: ChatToolApprovalPrompt;
  busy: boolean;
  onResolve?: (pendingId: string, decision: 'approve' | 'reject', modifiedArgs?: Record<string, unknown>) => Promise<void>;
}

/** Expandable approval card: view/edit the tool call arguments as JSON before approving. */
export function ToolApprovalCard({ approval, busy, onResolve }: ToolApprovalCardProps) {
  const [draft, setDraft] = useState(() => initialApprovalArgsDraft(approval));
  const [argsError, setArgsError] = useState<string | null>(null);
  const pending = approval.status === 'pending' && Boolean(onResolve);

  async function resolve(decision: 'approve' | 'reject') {
    if (!onResolve) return;
    const expanded = pending;
    const resolution = approvalResolution(decision, draft, expanded);
    if (!resolution.ok) {
      setArgsError(resolution.error);
      return;
    }
    setArgsError(null);
    await onResolve(approval.pendingId, resolution.decision, resolution.modifiedArgs);
  }

  return <div className="wk-chat-action-card wk-chat-approval-card">
    <strong>Tool approval: {approval.toolName ?? 'unknown tool'}</strong>
    {pending ? <div className="wk-chat-approval-editor">
      <details className="wk-chat-approval-args" onToggle={() => setArgsError(null)}>
        <summary className="wk-chat-approval-args-toggle">查看参数</summary>
        <textarea
          className="wk-chat-approval-args-input"
          rows={6}
          spellCheck={false}
          aria-label={`参数 ${approval.toolName ?? approval.pendingId}`}
          value={draft}
          onChange={(event) => { setDraft(event.target.value); setArgsError(null); }}
        />
        {argsError ? <p role="alert" className="wk-chat-approval-error">{argsError}</p> : null}
      </details>
      <div className="wk-list-actions">
        <button type="button" disabled={busy} onClick={() => void resolve('approve')}>同意</button>
        <button type="button" disabled={busy} onClick={() => void resolve('reject')}>拒绝</button>
      </div>
    </div> : <small>{approval.decision ? `Resolved: ${approval.decision}` : 'Resolved'}</small>}
  </div>;
}
