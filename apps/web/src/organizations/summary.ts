export function organizationRoleLabel(role: string): string { return ({ admin: 'Admin', editor: 'Editor', viewer: 'Viewer' } as Record<string, string>)[role] ?? role; }
