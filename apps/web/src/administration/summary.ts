export function roleLabel(role: string): string {
  return ({ owner: 'Owner', admin: 'Admin', contributor: 'Contributor', viewer: 'Viewer' } as Record<string, string>)[role] ?? role;
}

export function canManageTenant(role: string | undefined): boolean {
  const normalized = role?.trim().toLowerCase();
  return normalized === 'owner' || normalized === 'admin';
}

export function canViewAudit(role: string | undefined): boolean {
  return canManageTenant(role);
}

export function isEditableMember(member: { user_id: string; role: string }, currentUserId: string): boolean {
  return member.role !== 'owner' && member.user_id !== currentUserId;
}

export function tenantRoleFromMemberships(memberships: unknown[] | undefined, tenantId: number): string | undefined {
  const membership = memberships?.find((candidate) => {
    if (!candidate || typeof candidate !== 'object') return false;
    const record = candidate as Record<string, unknown>;
    return Number(record.tenant_id ?? record.tenantId) === tenantId;
  });
  const role = membership && typeof membership === 'object' ? (membership as Record<string, unknown>).role : undefined;
  return typeof role === 'string' && role.trim() ? role : undefined;
}

export function invitationIsOpen(status: string): boolean { return status === 'pending'; }

export const SYSTEM_ADMIN_PANEL_KEYS = ['system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log'] as const;
export type SystemAdminPanelKey = typeof SYSTEM_ADMIN_PANEL_KEYS[number];

export function systemAdminPanelKeys(systemAdmin: boolean): SystemAdminPanelKey[] {
  return systemAdmin ? [...SYSTEM_ADMIN_PANEL_KEYS] : [];
}
