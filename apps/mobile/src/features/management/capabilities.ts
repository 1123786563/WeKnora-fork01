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
  { key: 'api-keys', label: 'Workspace API keys', support: 'management', reason: 'Only the workspace owner can mint or revoke keys; tokens are displayed once and never persisted by mobile.' },
  { key: 'system-runtime', label: 'System runtime queues', support: 'unsupported', reason: 'System-admin queue/task controls remain on Web/Desktop and are not exposed as a mobile mutation surface.' },
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

export function projectMobileCapability(
  capability: MobileCapability,
  serverCapabilities: Record<string, { supported: boolean; reason?: string }>,
): MobileCapability {
  const serverKey = capability.key === 'organizations' ? 'organizations' : capability.key === 'api-keys' ? 'integrations.api' : capability.key === 'sandbox' ? 'settings.sandbox' : undefined;
  if (!serverKey || serverCapabilities[serverKey] === undefined) return capability;
  const server = serverCapabilities[serverKey];
  return server.supported ? capability : { ...capability, support: 'unsupported', reason: server.reason || 'Disabled by the server deployment' };
}
