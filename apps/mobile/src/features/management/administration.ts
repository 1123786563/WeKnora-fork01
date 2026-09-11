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
