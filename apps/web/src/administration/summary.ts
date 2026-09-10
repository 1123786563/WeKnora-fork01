export function roleLabel(role: string): string {
  return ({ owner: 'Owner', admin: 'Admin', contributor: 'Contributor', viewer: 'Viewer' } as Record<string, string>)[role] ?? role;
}

export function invitationIsOpen(status: string): boolean { return status === 'pending'; }
