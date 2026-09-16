import type { TenantRole } from '@weknora/api-client';

export function canManageTenant(role: string | undefined): boolean {
  const normalized = role?.trim().toLowerCase();
  return normalized === 'owner' || normalized === 'admin';
}

export function isRemovableMember(member: { role: string; status?: string }): boolean {
  return member.role !== 'owner';
}

export function validateInvite(email: string, role: TenantRole): string[] {
  const errors: string[] = [];
  if (!email.trim()) errors.push('Email is required');
  if (role === 'owner') errors.push('Owner invitations are not allowed');
  return errors;
}

const AUDIT_ACTIONS = new Set(['rbac.invitation_sent', 'rbac.invitation_revoked', 'rbac.member_removed', 'rbac.member_role_updated', 'rbac.member_added']);
const AUDIT_OUTCOMES = new Set(['success', 'denied', 'failure', 'error', 'pending']);
const AUDIT_ROLES = new Set(['owner', 'admin', 'contributor', 'viewer', 'system_admin']);

export function administrationAuditActionKey(action: string): string | null {
  return AUDIT_ACTIONS.has(action) ? `mobileAdministration.audit.action.${action}` : null;
}

export function administrationAuditOutcomeKey(outcome: string): string | null {
  return AUDIT_OUTCOMES.has(outcome) ? `mobileAdministration.audit.outcome.${outcome}` : null;
}

export function administrationAuditActorKey(role: string): string | null {
  return AUDIT_ROLES.has(role) ? `mobileAdministration.audit.actor.${role}` : null;
}
