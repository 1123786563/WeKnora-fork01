export type MobileCapabilityMode = 'native-write' | 'native-read' | 'web-handoff' | 'unsupported';
export type MobileCapabilityRole = 'owner' | 'admin' | 'contributor' | 'viewer' | 'system_admin';
export type ManagementRoute =
  | '/management/configuration'
  | '/management/administration'
  | '/management/api-keys'
  | '/management/organizations';

export interface MobileCapability {
  key: string;
  label: string;
  mode: MobileCapabilityMode;
  reason: string;
  requiredRoles: readonly MobileCapabilityRole[];
}

/** Stable message keys keep server supplied capability reasons as data while
 * allowing the built-in catalog copy to follow the active mobile locale. */
export const MOBILE_CAPABILITY_MESSAGE_KEYS: Readonly<Record<string, { label: string; reason: string }>> = Object.freeze(
  Object.fromEntries([
    'knowledge', 'chat', 'attachments', 'approvals', 'identity', 'api-keys', 'system-runtime',
    'organizations', 'configuration', 'skills', 'wiki-faq', 'sandbox', 'offline-writes', 'embed-admin',
  ].map((key) => [key, { label: `mobileManagement.capability.${key}.label`, reason: `mobileManagement.capability.${key}.reason` }])),
);

export function capabilityMessageKeys(key: string): { label: string; reason: string } | undefined {
  return MOBILE_CAPABILITY_MESSAGE_KEYS[key];
}

export function capabilityModeMessageKey(mode: MobileCapabilityMode): string {
  if (mode === 'native-write') return 'mobileManagement.mode.nativeWrite';
  if (mode === 'native-read') return 'mobileManagement.mode.nativeRead';
  if (mode === 'web-handoff') return 'mobileManagement.mode.webHandoff';
  return 'mobileManagement.mode.unsupported';
}

const TENANT_ROLES: readonly MobileCapabilityRole[] = ['owner', 'admin', 'contributor', 'viewer'];

export const MOBILE_CAPABILITIES: readonly MobileCapability[] = [
  { key: 'knowledge', label: 'Knowledge bases and files', mode: 'native-write', reason: 'Native knowledge and file screens submit changes through the authenticated server API.', requiredRoles: TENANT_ROLES },
  { key: 'chat', label: 'Chat, SSE recovery and citations', mode: 'native-write', reason: 'Authenticated tenant members can send messages and recover streams from the native chat surface.', requiredRoles: TENANT_ROLES },
  { key: 'attachments', label: 'Chat attachments', mode: 'native-write', reason: 'Native attachment selection and upload use the authenticated tenant-scoped file API.', requiredRoles: TENANT_ROLES },
  { key: 'approvals', label: 'Tool approvals', mode: 'native-write', reason: 'Approval decisions are submitted by the authenticated conversation participant and remain server-authorized.', requiredRoles: TENANT_ROLES },
  { key: 'identity', label: 'Members, roles and audit', mode: 'native-write', reason: 'Tenant context is server-owned; writes require an owner/admin role and remain server-authorized.', requiredRoles: ['owner', 'admin'] },
  { key: 'api-keys', label: 'Workspace API keys', mode: 'native-write', reason: 'Only the workspace owner can mint or revoke keys; tokens are displayed once and never persisted by mobile.', requiredRoles: ['owner'] },
  { key: 'system-runtime', label: 'System runtime queues', mode: 'web-handoff', reason: 'System-admin queue/task controls remain on Web/Desktop and are not exposed as a native mobile mutation surface.', requiredRoles: ['system_admin'] },
  { key: 'organizations', label: 'Organizations and join requests', mode: 'native-write', reason: 'Organization membership and writes require an organization admin and remain server-authorized.', requiredRoles: ['admin'] },
  { key: 'configuration', label: 'Agents, models and MCP', mode: 'native-write', reason: 'Native forms persist Agent, Model and MCP configuration through server-authorized APIs; credentials remain write-only.', requiredRoles: TENANT_ROLES },
  { key: 'skills', label: 'Skills', mode: 'native-read', reason: 'Mobile can inspect the server-owned Skill list; catalog installation remains on Web/Desktop until its native sandbox flow is implemented.', requiredRoles: TENANT_ROLES },
  { key: 'wiki-faq', label: 'Wiki and FAQ', mode: 'web-handoff', reason: 'The management hub has no workspace context for a reference editor; use the Web surface for this management entry.', requiredRoles: ['owner', 'admin'] },
  { key: 'sandbox', label: 'Sandbox terminal', mode: 'unsupported', reason: 'The ticket/WebSocket surface is not exposed as an arbitrary mobile shell.', requiredRoles: TENANT_ROLES },
  { key: 'offline-writes', label: 'Offline write queue', mode: 'unsupported', reason: 'The migration explicitly does not add offline synchronization.', requiredRoles: TENANT_ROLES },
  { key: 'embed-admin', label: 'Embed and IM administration', mode: 'web-handoff', reason: 'Embed and IM administration remains a Web/desktop flow; mobile does not embed the administration iframe.', requiredRoles: ['owner', 'admin'] },
] as const;

const MANAGEMENT_ROUTES: Readonly<Partial<Record<string, ManagementRoute>>> = {
  configuration: '/management/configuration',
  identity: '/management/administration',
  'api-keys': '/management/api-keys',
  organizations: '/management/organizations',
};

export type MobileCapabilityAction =
  | { kind: 'route'; route: ManagementRoute }
  | { kind: 'status'; label: string };

export function capabilityModeLabel(mode: MobileCapabilityMode): string {
  if (mode === 'native-write') return '原生可写';
  if (mode === 'native-read') return '只读';
  if (mode === 'web-handoff') return '转 Web';
  return '不支持';
}

export function capabilityAction(capability: MobileCapability): MobileCapabilityAction {
  const route = MANAGEMENT_ROUTES[capability.key];
  if (route && (capability.mode === 'native-write' || capability.mode === 'native-read')) return { kind: 'route', route };
  return { kind: 'status', label: capabilityModeLabel(capability.mode) };
}

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
  return server.supported ? capability : { ...capability, mode: 'unsupported', reason: server.reason || 'Disabled by the server deployment' };
}
