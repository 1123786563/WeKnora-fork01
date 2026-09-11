export function canManageOrganization(organization: { my_role?: unknown; [key: string]: unknown }): boolean {
  return organization.my_role === 'admin';
}

export function canCreateOrganization(workspaceRole: unknown): boolean {
  return workspaceRole === 'owner' || workspaceRole === 'admin';
}

export function validateOrganizationDraft(name: string, _description: string): string[] {
  return name.trim() ? [] : ['Name is required'];
}

export type OrganizationSharedResourceKind = 'knowledge-base' | 'agent';

export function shareResourceId(
  share: { [key: string]: unknown },
  kind: OrganizationSharedResourceKind,
): string | null {
  const key = kind === 'knowledge-base' ? 'knowledge_base_id' : 'agent_id';
  const value = share[key];
  return typeof value === 'string' && value.trim() ? value : null;
}

export function shareResourceLabel(
  share: { [key: string]: unknown },
  kind: OrganizationSharedResourceKind,
): string {
  const key = kind === 'knowledge-base' ? 'knowledge_base_name' : 'agent_name';
  const value = share[key];
  if (typeof value === 'string' && value.trim()) return value;
  return shareResourceId(share, kind) || (kind === 'knowledge-base' ? 'Unnamed knowledge base' : 'Unnamed agent');
}
