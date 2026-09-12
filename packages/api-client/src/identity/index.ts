import type { ClientRequest } from '../client.ts';
import { createOrganizationApi } from './organization.ts';
import { createTenantAdminApi, createTenantAuditLogApi, createTenantInvitationsApi, createTenantMembersApi } from './tenant.ts';

export function createIdentityApi(request: (input: ClientRequest) => Promise<unknown>) {
  const members = createTenantMembersApi(request);
  const invitations = createTenantInvitationsApi(request);
  const auditLog = createTenantAuditLogApi(request);
  const admin = createTenantAdminApi(request);
  return {
    tenants: { members, invitations, auditLog, admin },
    invitations,
    organizations: createOrganizationApi(request),
  };
}

export type IdentityApi = ReturnType<typeof createIdentityApi>;
export * from './tenant.ts';
export * from './organization.ts';
