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

// This panel projects Task roles; the API enforces every grant and read.
export function CraftAccess({ role, members, onGrant, onRevoke }: CraftAccessProps) {
  const [userId, setUserId] = useState('');
  const [grantRole, setGrantRole] = useState<'collaborator' | 'viewer'>('viewer');
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<Feedback>(null);
  const isOwner = role === 'owner';
  const isPending = pendingAction !== null;

  async function grant(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = userId.trim();
    if (!isOwner || !trimmed || isPending) return;
    setPendingAction('grant');
    setFeedback({ kind: 'pending', message: 'Adding member…' });
    try {
      await onGrant(trimmed, grantRole);
      setUserId('');
      setFeedback({ kind: 'success', message: 'Member added. The access list will update when refreshed.' });
    } catch {
      setFeedback({ kind: 'error', message: 'Member could not be added. Check your connection and try again.' });
    } finally {
      setPendingAction(null);
    }
  }

  async function revoke(memberId: string) {
    if (!isOwner || isPending) return;
    setPendingAction(`revoke:${memberId}`);
    setFeedback({ kind: 'pending', message: 'Revoking member…' });
    try {
      await onRevoke(memberId);
      setFeedback({ kind: 'success', message: `Access revoked for ${memberId}. The access list will update when refreshed.` });
    } catch {
      setFeedback({ kind: 'error', message: `Access for ${memberId} could not be revoked. Try again.` });
    } finally {
      setPendingAction(null);
    }
  }

  return <section aria-label="Task access" style={{ minWidth: 0 }}>
    <h3>Task access</h3>
    <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
      {members.map(member => {
        const actionPending = pendingAction === `revoke:${member.user_id}`;
        return <li key={member.user_id} style={{ alignItems: 'center', display: 'flex', flexWrap: 'wrap', gap: '0.5rem', justifyContent: 'space-between', minWidth: 0, padding: '0.5rem 0' }}>
          <span style={{ minWidth: 0, overflowWrap: 'anywhere' }}>{member.user_id}: {member.role}</span>
          {isOwner && member.role !== 'owner' && <button type="button" aria-label={`Revoke access for ${member.user_id}`} disabled={isPending} onClick={() => void revoke(member.user_id)}>
            {actionPending ? 'Revoking…' : 'Revoke'}
          </button>}
        </li>;
      })}
    </ul>
    {isOwner && <form onSubmit={event => void grant(event)} style={{ display: 'flex', flexWrap: 'wrap', gap: '0.75rem', minWidth: 0 }}>
      <label style={{ display: 'grid', flex: '1 1 14rem', gap: '0.3rem', minWidth: 0 }}>User ID <input value={userId} onChange={event => setUserId(event.target.value)} required disabled={isPending} style={{ boxSizing: 'border-box', maxWidth: '100%', minWidth: 0, width: '100%' }} /></label>
      <label style={{ display: 'grid', flex: '1 1 10rem', gap: '0.3rem', minWidth: 0 }}>Role <select value={grantRole} onChange={event => setGrantRole(event.target.value as 'collaborator' | 'viewer')} disabled={isPending}>
        <option value="viewer">Viewer</option>
        <option value="collaborator">Collaborator</option>
      </select></label>
      <button type="submit" disabled={isPending}>{pendingAction === 'grant' ? 'Adding…' : 'Add member'}</button>
    </form>}
    {feedback && <p role={feedback.kind === 'error' ? 'alert' : 'status'} aria-live="polite" aria-atomic="true" data-state={feedback.kind}>{feedback.message}</p>}
  </section>;
}
