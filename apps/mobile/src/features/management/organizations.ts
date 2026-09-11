export function canManageOrganization(organization: { my_role?: unknown; [key: string]: unknown }): boolean {
  return organization.my_role === 'admin';
}

export function validateOrganizationDraft(name: string, _description: string): string[] {
  return name.trim() ? [] : ['Name is required'];
}
