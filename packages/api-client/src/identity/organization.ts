import { parseActionSuccessResponse, type ActionSuccessResponse } from '@weknora/contracts';
import { array, dataArray, dataRecord, encoded, numberValue, optionalNumber, optionalString, query, record, stringValue, success, withSignal, type IdentityRequest, type JsonRecord } from './common.ts';

export type OrganizationRole = 'admin' | 'editor' | 'viewer';
export type OrganizationPermission = 'admin' | 'editor' | 'viewer';
export interface Organization { id: string; name: string; description: string; owner_id: string; owner_tenant_id: number; /** Whether the current tenant already has a pending role-upgrade request (org detail endpoint, Vue OrganizationSettingsModal.vue). */ has_pending_upgrade?: boolean; [key: string]: unknown }
export interface OrganizationPage { items: Organization[]; total: number; resourceCounts?: JsonRecord }
export interface OrganizationMember { id: string; user_id: string; username: string; email: string; role: OrganizationRole; tenant_id: number; tenant_name?: string; joined_at: string; [key: string]: unknown }
export interface OrganizationMemberPage { items: OrganizationMember[]; total: number }
export interface OrganizationJoinRequest { id: string; user_id: string; username: string; email: string; message: string; request_type: string; requested_role: string; status: string; created_at: string; [key: string]: unknown }
export interface OrganizationJoinRequestPage { items: OrganizationJoinRequest[]; total: number }
export interface OrganizationShare { id: string; [key: string]: unknown }
export interface OrganizationSharedResource { share_id: string; organization_id: string; permission: string; source_tenant_id: number; [key: string]: unknown }
export interface TenantInviteCandidate { tenant_id: number; tenant_name: string; representative_user_id: string; representative_username: string; representative_email: string; [key: string]: unknown }

function organization(value: unknown, path: string): Organization {
  const row = record(value, path);
  // has_pending_upgrade normalizes like the Vue modal's `|| false` read.
  return { ...row, id: stringValue(row.id, `${path}.id`), name: stringValue(row.name, `${path}.name`), description: typeof row.description === 'string' ? row.description : '', owner_id: stringValue(row.owner_id, `${path}.owner_id`), owner_tenant_id: numberValue(row.owner_tenant_id, `${path}.owner_tenant_id`), has_pending_upgrade: row.has_pending_upgrade === true };
}
function member(value: unknown, path: string): OrganizationMember {
  const row = record(value, path);
  return { ...row, id: stringValue(row.id, `${path}.id`), user_id: stringValue(row.user_id, `${path}.user_id`), username: stringValue(row.username, `${path}.username`), email: stringValue(row.email, `${path}.email`), role: stringValue(row.role, `${path}.role`) as OrganizationRole, tenant_id: numberValue(row.tenant_id, `${path}.tenant_id`), ...(typeof row.tenant_name === 'string' ? { tenant_name: row.tenant_name } : {}), joined_at: stringValue(row.joined_at, `${path}.joined_at`) };
}
function parseResource(value: unknown, path: string): OrganizationShare { const row = record(value, path); return { ...row, id: stringValue(row.id, `${path}.id`) }; }
function responseData(value: unknown, path: string): unknown { return success(value, path).data; }
function action(value: unknown): ActionSuccessResponse { return parseActionSuccessResponse(value); }
function parseOrganizationPage(value: unknown): OrganizationPage { const data = dataRecord(value, '/organizations'); const rows = array(data.organizations, '/organizations.data.organizations'); return { items: rows.map((item, index) => organization(item, `/organizations.data.organizations[${index}]`)), total: numberValue(data.total, '/organizations.data.total'), ...(data.resource_counts === undefined ? {} : { resourceCounts: record(data.resource_counts, '/organizations.data.resource_counts') }) }; }
function parseMemberPage(value: unknown): OrganizationMemberPage { const data = dataRecord(value, '/organizations/members'); const rows = array(data.members, '/organizations/members.data.members'); return { items: rows.map((item, index) => member(item, `/organizations/members.data.members[${index}]`)), total: numberValue(data.total, '/organizations/members.data.total') }; }
function parseJoinPage(value: unknown): OrganizationJoinRequestPage { const data = dataRecord(value, '/organizations/join-requests'); const rows = array(data.requests, '/organizations/join-requests.data.requests'); return { items: rows.map((item, index) => { const row = record(item, `/organizations/join-requests.data.requests[${index}]`); return { ...row, id: stringValue(row.id, 'join-request.id'), user_id: stringValue(row.user_id, 'join-request.user_id'), username: stringValue(row.username, 'join-request.username'), email: stringValue(row.email, 'join-request.email'), message: typeof row.message === 'string' ? row.message : '', request_type: stringValue(row.request_type, 'join-request.request_type'), requested_role: stringValue(row.requested_role, 'join-request.requested_role'), status: stringValue(row.status, 'join-request.status'), created_at: stringValue(row.created_at, 'join-request.created_at') }; }), total: numberValue(data.total, '/organizations/join-requests.data.total') }; }
function parseArrayOfRecords(value: unknown, path: string): JsonRecord[] { return dataArray(value, path).map((item, index) => record(item, `${path}.data[${index}]`)); }

export function createOrganizationApi(request: IdentityRequest) {
  const organizationPath = (id: string) => `/api/v1/organizations/${encoded(id, 'organizationId')}`;
  const shareMutation = async (method: string, path: string, body?: unknown, signal?: AbortSignal) => action(await request(withSignal({ method, path, ...(body === undefined ? {} : { body }) }, signal)));
  return {
    async list(signal?: AbortSignal) { return parseOrganizationPage(await request(withSignal({ method: 'GET', path: '/api/v1/organizations' }, signal))); },
    async get(id: string, signal?: AbortSignal) { return organization(responseData(await request(withSignal({ method: 'GET', path: organizationPath(id) }, signal)), '/organizations/:id'), '/organizations/:id.data'); },
    async create(input: Record<string, unknown>, signal?: AbortSignal) { return organization(responseData(await request(withSignal({ method: 'POST', path: '/api/v1/organizations', body: input }, signal)), '/organizations'), '/organizations.data'); },
    async update(id: string, input: Record<string, unknown>, signal?: AbortSignal) { return organization(responseData(await request(withSignal({ method: 'PUT', path: organizationPath(id), body: input }, signal)), '/organizations/:id'), '/organizations/:id.data'); },
    async remove(id: string, signal?: AbortSignal) { await shareMutation('DELETE', organizationPath(id), undefined, signal); },
    async join(input: { invite_code: string }, signal?: AbortSignal) { return organization(responseData(await request(withSignal({ method: 'POST', path: '/api/v1/organizations/join', body: input }, signal)), '/organizations/join'), '/organizations/join.data'); },
    async submitJoinRequest(input: { invite_code: string; message?: string; role?: OrganizationRole }, signal?: AbortSignal) { await shareMutation('POST', '/api/v1/organizations/join-request', input, signal); },
    async preview(inviteCode: string, signal?: AbortSignal) { return record(responseData(await request(withSignal({ method: 'GET', path: `/api/v1/organizations/preview/${encoded(inviteCode, 'inviteCode')}` }, signal)), '/organizations/preview'), '/organizations/preview.data'); },
    async search(q = '', limit = 20, signal?: AbortSignal) { const root = success(await request(withSignal({ method: 'GET', path: query('/api/v1/organizations/search', [['q', q || undefined], ['limit', limit]]) }, signal)), '/organizations/search'); const rows = array(root.data, '/organizations/search.data'); return { items: rows.map((item, index) => record(item, `/organizations/search.data[${index}]`)), total: numberValue(root.total, '/organizations/search.total') }; },
    async joinById(organizationId: string, input: { message?: string; role?: OrganizationRole } = {}, signal?: AbortSignal) { return organization(responseData(await request(withSignal({ method: 'POST', path: '/api/v1/organizations/join-by-id', body: { organization_id: organizationId, ...input } }, signal)), '/organizations/join-by-id'), '/organizations/join-by-id.data'); },
    async leave(id: string, signal?: AbortSignal) { await shareMutation('POST', `${organizationPath(id)}/leave`, {}, signal); },
    async requestRoleUpgrade(id: string, input: { requested_role: OrganizationRole; message?: string }, signal?: AbortSignal) { return record(responseData(await request(withSignal({ method: 'POST', path: `${organizationPath(id)}/request-upgrade`, body: input }, signal)), '/organizations/request-upgrade'), '/organizations/request-upgrade.data'); },
    async generateInviteCode(id: string, signal?: AbortSignal) { const data = dataRecord(await request(withSignal({ method: 'POST', path: `${organizationPath(id)}/invite-code`, body: {} }, signal)), '/organizations/invite-code'); return { inviteCode: stringValue(data.invite_code, '/organizations/invite-code.data.invite_code') }; },
    members: {
      async list(id: string, signal?: AbortSignal) { return parseMemberPage(await request(withSignal({ method: 'GET', path: `${organizationPath(id)}/members` }, signal))); },
      async updateRole(id: string, tenantId: number, input: { role: OrganizationRole }, signal?: AbortSignal) { await shareMutation('PUT', `${organizationPath(id)}/members/${encoded(tenantId, 'tenantId')}`, input, signal); },
      async remove(id: string, tenantId: number, signal?: AbortSignal) { await shareMutation('DELETE', `${organizationPath(id)}/members/${encoded(tenantId, 'tenantId')}`, undefined, signal); },
    },
    joinRequests: {
      async list(id: string, signal?: AbortSignal) { return parseJoinPage(await request(withSignal({ method: 'GET', path: `${organizationPath(id)}/join-requests` }, signal))); },
      async review(id: string, requestId: string, input: { approved: boolean; message?: string; role?: OrganizationRole }, signal?: AbortSignal) { await shareMutation('PUT', `${organizationPath(id)}/join-requests/${encoded(requestId, 'requestId')}/review`, input, signal); },
    },
    knowledgeBaseShares: {
      async create(kbId: string, input: { organization_id: string; permission: OrganizationPermission }, signal?: AbortSignal) { return parseResource(responseData(await request(withSignal({ method: 'POST', path: `/api/v1/knowledge-bases/${encoded(kbId, 'knowledgeBaseId')}/shares`, body: input }, signal)), '/knowledge-bases/shares'), '/knowledge-bases/shares.data'); },
      async list(kbId: string, signal?: AbortSignal) { const data = dataRecord(await request(withSignal({ method: 'GET', path: `/api/v1/knowledge-bases/${encoded(kbId, 'knowledgeBaseId')}/shares` }, signal)), '/knowledge-bases/shares'); return { items: parseArrayOfRecords({ success: true, data: data.shares }, '/knowledge-bases/shares'), total: numberValue(data.total, '/knowledge-bases/shares.data.total') }; },
      async updatePermission(kbId: string, shareId: string, permission: OrganizationPermission, signal?: AbortSignal) { await shareMutation('PUT', `/api/v1/knowledge-bases/${encoded(kbId, 'knowledgeBaseId')}/shares/${encoded(shareId, 'shareId')}`, { permission }, signal); },
      async remove(kbId: string, shareId: string, signal?: AbortSignal) { await shareMutation('DELETE', `/api/v1/knowledge-bases/${encoded(kbId, 'knowledgeBaseId')}/shares/${encoded(shareId, 'shareId')}`, undefined, signal); },
      async listForOrganization(id: string, signal?: AbortSignal) { const data = dataRecord(await request(withSignal({ method: 'GET', path: `${organizationPath(id)}/shares` }, signal)), '/organizations/shares'); const rows = array(data.shares, '/organizations/shares.data.shares'); return { items: rows.map((item, index) => parseResource(item, `/organizations/shares.data.shares[${index}]`)), total: numberValue(data.total, '/organizations/shares.data.total') }; },
      async listShared(signal?: AbortSignal) { return parseArrayOfRecords(await request(withSignal({ method: 'GET', path: '/api/v1/shared-knowledge-bases' }, signal)), '/shared-knowledge-bases'); },
      async listInOrganization(id: string, signal?: AbortSignal) { return parseArrayOfRecords(await request(withSignal({ method: 'GET', path: `${organizationPath(id)}/shared-knowledge-bases` }, signal)), '/organizations/shared-knowledge-bases'); },
    },
    agentShares: {
      async create(agentId: string, input: { organization_id: string; permission: OrganizationPermission }, signal?: AbortSignal) { return parseResource(responseData(await request(withSignal({ method: 'POST', path: `/api/v1/agents/${encoded(agentId, 'agentId')}/shares`, body: input }, signal)), '/agents/shares'), '/agents/shares.data'); },
      async list(agentId: string, signal?: AbortSignal) { const data = dataRecord(await request(withSignal({ method: 'GET', path: `/api/v1/agents/${encoded(agentId, 'agentId')}/shares` }, signal)), '/agents/shares'); return { items: parseArrayOfRecords({ success: true, data: data.shares }, '/agents/shares'), total: numberValue(data.total, '/agents/shares.data.total') }; },
      async remove(agentId: string, shareId: string, signal?: AbortSignal) { await shareMutation('DELETE', `/api/v1/agents/${encoded(agentId, 'agentId')}/shares/${encoded(shareId, 'shareId')}`, undefined, signal); },
      async listForOrganization(id: string, signal?: AbortSignal) { const data = dataRecord(await request(withSignal({ method: 'GET', path: `${organizationPath(id)}/agent-shares` }, signal)), '/organizations/agent-shares'); const rows = array(data.shares, '/organizations/agent-shares.data.shares'); return { items: rows.map((item, index) => parseResource(item, `/organizations/agent-shares.data.shares[${index}]`)), total: numberValue(data.total, '/organizations/agent-shares.data.total') }; },
      async listShared(signal?: AbortSignal) { return parseArrayOfRecords(await request(withSignal({ method: 'GET', path: '/api/v1/shared-agents' }, signal)), '/shared-agents'); },
      async listInOrganization(id: string, signal?: AbortSignal) { return parseArrayOfRecords(await request(withSignal({ method: 'GET', path: `${organizationPath(id)}/shared-agents` }, signal)), '/organizations/shared-agents'); },
      async setDisabledByMe(agentId: string, disabled: boolean, signal?: AbortSignal) { await shareMutation('POST', '/api/v1/shared-agents/disabled', { agent_id: agentId, disabled }, signal); },
    },
    async searchTenantsForInvite(id: string, q: string, limit = 10, signal?: AbortSignal) { const rows = dataArray(await request(withSignal({ method: 'GET', path: query(`${organizationPath(id)}/search-tenants`, [['q', q], ['limit', limit]]) }, signal)), '/organizations/search-tenants'); return rows.map((item, index) => { const row = record(item, `/organizations/search-tenants.data[${index}]`); return { ...row, tenant_id: numberValue(row.tenant_id, 'tenant_id'), tenant_name: stringValue(row.tenant_name, 'tenant_name'), representative_user_id: stringValue(row.representative_user_id, 'representative_user_id'), representative_username: stringValue(row.representative_username, 'representative_username'), representative_email: stringValue(row.representative_email, 'representative_email') } as TenantInviteCandidate; }); },
    async inviteMember(id: string, input: { tenant_id?: number; representative_user_id?: string; user_id?: string; role: OrganizationRole }, signal?: AbortSignal) { await shareMutation('POST', `${organizationPath(id)}/invite`, input, signal); },
  };
}

export type OrganizationApi = ReturnType<typeof createOrganizationApi>;
