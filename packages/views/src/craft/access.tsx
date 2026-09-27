import React, { useState } from 'react';

export type CraftTaskRole = 'owner' | 'collaborator' | 'viewer';
export interface CraftTaskMember { user_id: string; role: CraftTaskRole }

export interface CraftAccessProps {
  role: CraftTaskRole;
  members: readonly CraftTaskMember[];
  onGrant(userId: string, role: 'collaborator' | 'viewer'): void | Promise<void>;
  onRevoke(userId: string): void | Promise<void>;
}

type Feedback = { kind: 'pending' | 'success' | 'error'; message: string } | null;

// Server refusals (ApiError with status) and transport failures say different
// things; the distinction is what makes the message actionable.
export function errorHint(error: unknown): string {
  const status = (error as { status?: number } | null)?.status;
  if (typeof status === 'number') {
    return status >= 500
      ? 'The server could not complete the request. Try again later.'
      : 'The server refused the request. Check the member ID and your permission.';
  }
  return 'Check your connection and try again.';
}

// This panel projects Task roles; the API enforces every grant and read.
export function CraftAccess({ role, members, onGrant, onRevoke }: CraftAccessProps) {
  const [userId, setUserId] = useState('');
  const [grantRole, setGrantRole] = useState<'collaborator' | 'viewer'>('viewer');
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<Feedback>(null);
  const isOwner = role === 'owner';
  const isPending = pendingAction !== null;

  // One shared action lifecycle: pending feedback, the caller's authoritative
  // callback, then success/error feedback. The thrown error is bound so the
  // hint can distinguish transport failures from server refusals.
  async function runAction(key: string, pendingMessage: string, action: () => Promise<void>, successMessage: () => string, errorMessage: (error: unknown) => string) {
    setPendingAction(key);
    setFeedback({ kind: 'pending', message: pendingMessage });
    try {
      await action();
      setFeedback({ kind: 'success', message: successMessage() });
    } catch (error) {
      setFeedback({ kind: 'error', message: errorMessage(error) });
    } finally {
      setPendingAction(null);
    }
  }

  async function grant(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = userId.trim();
    if (!isOwner || !trimmed || isPending) return;
    await runAction(
      'grant',
      'Adding member…',
      async () => {
        await onGrant(trimmed, grantRole);
        setUserId('');
      },
      () => 'Member added.',
      error => `Member could not be added. ${errorHint(error)}`,
    );
  }

  async function revoke(memberId: string) {
    if (!isOwner || isPending) return;
    await runAction(
      `revoke:${memberId}`,
      'Revoking member…',
      async () => { await onRevoke(memberId); },
      () => `Access revoked for ${memberId}.`,
      error => `Access for ${memberId} could not be revoked. ${errorHint(error)}`,
    );
  }

  return <section aria-label="Task access" className="wk-craft-access">
    <h3>Task access</h3>
    <ul className="wk-craft-access-members">
      {members.map(member => {
        const actionPending = pendingAction === `revoke:${member.user_id}`;
        return <li key={member.user_id} className="wk-craft-access-member">
          <span className="wk-craft-access-member-id">{member.user_id}: {member.role}</span>
          {isOwner && member.role !== 'owner' && <button type="button" aria-label={`Revoke access for ${member.user_id}`} disabled={isPending} onClick={() => void revoke(member.user_id)}>
            {actionPending ? 'Revoking…' : 'Revoke'}
          </button>}
        </li>;
      })}
    </ul>
    {isOwner && <form onSubmit={event => void grant(event)} className="wk-craft-access-form">
      <label className="wk-craft-access-field wk-craft-access-field-user">User ID <input value={userId} onChange={event => setUserId(event.target.value)} required disabled={isPending} /></label>
      <label className="wk-craft-access-field wk-craft-access-field-role">Role <select value={grantRole} onChange={event => setGrantRole(event.target.value as 'collaborator' | 'viewer')} disabled={isPending}>
        <option value="viewer">Viewer</option>
        <option value="collaborator">Collaborator</option>
      </select></label>
      <button type="submit" disabled={isPending}>{pendingAction === 'grant' ? 'Adding…' : 'Add member'}</button>
    </form>}
    {feedback && <p role={feedback.kind === 'error' ? 'alert' : 'status'} aria-live={feedback.kind === 'error' ? 'assertive' : 'polite'} aria-atomic="true" className="wk-craft-access-feedback" data-kind={feedback.kind}>{feedback.message}</p>}
  </section>;
}
