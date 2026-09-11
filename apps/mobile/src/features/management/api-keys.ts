export function canManageApiKeys(role: string | undefined): boolean {
  return role?.trim().toLowerCase() === 'owner';
}

export function validateApiKeyDraft(name: string, capabilities: readonly string[]): string[] {
  const errors: string[] = [];
  if (!name.trim()) errors.push('Name is required');
  if (capabilities.length === 0) errors.push('Select at least one capability');
  return errors;
}

export const MOBILE_API_KEY_CAPABILITIES = [
  'retrieve',
  'chat',
  'read_agents',
  'ingest',
  'manage_knowledge_bases',
] as const;
