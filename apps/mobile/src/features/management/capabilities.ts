export interface MobileCapability {
  key: string;
  label: string;
  support: 'core' | 'read-only' | 'management' | 'unsupported';
  reason?: string;
}

export const MOBILE_CAPABILITIES: readonly MobileCapability[] = [
  { key: 'knowledge', label: 'Knowledge bases and files', support: 'core' },
  { key: 'chat', label: 'Chat, SSE recovery and citations', support: 'core' },
  { key: 'attachments', label: 'Chat attachments', support: 'core' },
  { key: 'approvals', label: 'Tool approvals', support: 'core' },
  { key: 'identity', label: 'Members, roles and audit', support: 'management', reason: 'Tenant context is server-owned; writes require an owner/admin role and remain server-authorized.' },
  { key: 'organizations', label: 'Organizations and join requests', support: 'management', reason: 'Organization membership and writes remain server-authorized.' },
  { key: 'configuration', label: 'Agents, models, MCP and skills', support: 'read-only', reason: 'Mobile does not persist configuration writes.' },
  { key: 'wiki-faq', label: 'Wiki and FAQ', support: 'management', reason: 'Owner/admin editing uses server version and permission checks.' },
  { key: 'sandbox', label: 'Sandbox terminal', support: 'unsupported', reason: 'The ticket/WebSocket surface is not exposed as an arbitrary mobile shell.' },
  { key: 'offline-writes', label: 'Offline write queue', support: 'unsupported', reason: 'The migration explicitly does not add offline synchronization.' },
  { key: 'embed-admin', label: 'Embed and IM administration', support: 'unsupported', reason: 'Mobile does not embed the administration iframe.' },
] as const;

export function mobileCapability(key: string): MobileCapability | undefined {
  return MOBILE_CAPABILITIES.find((capability) => capability.key === key);
}
