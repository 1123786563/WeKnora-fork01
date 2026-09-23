import React, { useState } from 'react';

export type CraftTaskRole = 'owner' | 'collaborator' | 'viewer';
export interface CraftTaskMember { user_id: string; role: CraftTaskRole }

export interface CraftAccessProps {
  role: CraftTaskRole;
  members: readonly CraftTaskMember[];
  onGrant(userId: string, role: 'collaborator' | 'viewer'): void | Promise<void>;
  onRevoke(userId: string): void | Promise<void>;
}

// This panel projects Task roles; the API enforces every grant and read.
export function CraftAccess({ role, members, onGrant, onRevoke }: CraftAccessProps) {
  const [userId, setUserId] = useState('');
  const [grantRole, setGrantRole] = useState<'collaborator' | 'viewer'>('viewer');
  return <section aria-label="Task access">
    <h3>Task access</h3>
    <ul>{members.map(member => <li key={member.user_id}>
      <span>{member.user_id}: {member.role}</span>
      {role === 'owner' && member.role !== 'owner' && <button type="button" onClick={() => onRevoke(member.user_id)}>Revoke</button>}
    </li>)}</ul>
    {role === 'owner' && <form onSubmit={event => {
      event.preventDefault();
      const trimmed = userId.trim();
      if (trimmed) { void onGrant(trimmed, grantRole); setUserId(''); }
    }}>
      <label>User ID <input value={userId} onChange={event => setUserId(event.target.value)} required /></label>
      <label>Role <select value={grantRole} onChange={event => setGrantRole(event.target.value as 'collaborator' | 'viewer')}>
        <option value="viewer">Viewer</option>
        <option value="collaborator">Collaborator</option>
      </select></label>
      <button type="submit">Add member</button>
    </form>}
  </section>;
}
