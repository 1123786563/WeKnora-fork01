export function organizationRoleLabel(role: string): string { return ({ admin: 'Admin', editor: 'Editor', viewer: 'Viewer' } as Record<string, string>)[role] ?? role; }

/** Vue OrganizationSettingsModal only exposes join-request review to admins. */
export function organizationSettingsSections(mode: 'create' | 'edit', canManage: boolean): string[] {
  if (mode === 'create') return ['basic', 'permissions'];
  return ['basic', 'members', ...(canManage ? ['requests'] : []), 'shares', 'agents', 'invite'];
}
