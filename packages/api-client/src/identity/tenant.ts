import { parseActionSuccessResponse, type ActionSuccessResponse } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';
import { action, array, dataArray, dataRecord, encoded, numberValue, optionalNumber, optionalString, query, record, stringValue, stringValueAllowEmpty, success, withSignal, type IdentityRequest, type JsonRecord } from './common.ts';

export type TenantRole = 'owner' | 'admin' | 'contributor' | 'viewer';
export type TenantMemberStatus = 'active' | 'invited' | 'suspended';
export type TenantInvitationStatus = 'pending' | 'accepted' | 'declined' | 'revoked' | 'expired';
export type AuditOutcome = 'accepted' | 'success' | 'failed' | 'partial' | 'canceled' | 'denied';

export interface TenantMember { user_id: string; email: string; username: string; avatar?: string; role: TenantRole; status: TenantMemberStatus; invited_by?: string | null; joined_at: string }
export interface TenantMemberPage { items: TenantMember[]; total: number; page: number; pageSize: number }
export interface TenantInvitation { id: number; tenant_id: number; tenant_name?: string; invitee_user_id: string; invitee_email?: string; invitee_name?: string; invited_by?: string | null; inviter_email?: string; inviter_name?: string; role: TenantRole; status: TenantInvitationStatus; message?: string; expires_at: string; responded_at?: string | null; created_at: string; invite_url?: string; is_share_link?: boolean; accepted_count?: number }
export interface TenantInvitationPage { items: TenantInvitation[]; total: number; page: number; pageSize: number }
export interface Membership { tenantId: number; role: TenantRole; status: string; joinedAt: string; tenantName?: string }
export interface AuditLog { id: number; tenant_id: number; actor_user_id: string; actor_role: string; action: string; scope_type: string; scope_id: string; target_type: string; target_id: string; target_user_id: string; request_path: string; request_method: string; outcome: AuditOutcome; details: Record<string, unknown> | string | null; created_at: string }
export interface AuditLogPage { items: AuditLog[]; nextCursor: number }

function parseMember(value: unknown, path: string): TenantMember {
  const row = record(value, path);
  const role = stringValue(row.role, `${path}.role`) as TenantRole;
  const status = stringValue(row.status, `${path}.status`) as TenantMemberStatus;
  return { user_id: stringValue(row.user_id, `${path}.user_id`), email: stringValue(row.email, `${path}.email`), username: stringValue(row.username, `${path}.username`), ...(optionalString(row.avatar, `${path}.avatar`) === undefined ? {} : { avatar: row.avatar as string }), role, status, ...(row.invited_by === null ? { invited_by: null } : optionalString(row.invited_by, `${path}.invited_by`) === undefined ? {} : { invited_by: row.invited_by as string }), joined_at: stringValue(row.joined_at, `${path}.joined_at`) };
}

function parseInvitation(value: unknown, path: string): TenantInvitation {
  const row = record(value, path);
  const role = stringValue(row.role, `${path}.role`) as TenantRole;
  const status = stringValue(row.status, `${path}.status`) as TenantInvitationStatus;
  if (typeof row.invitee_user_id !== 'string') throw new Error(`${path}.invitee_user_id must be a string`);
  return { id: numberValue(row.id, `${path}.id`), tenant_id: numberValue(row.tenant_id, `${path}.tenant_id`), ...(optionalString(row.tenant_name, `${path}.tenant_name`) === undefined ? {} : { tenant_name: row.tenant_name as string }), invitee_user_id: row.invitee_user_id, ...(optionalString(row.invitee_email, `${path}.invitee_email`) === undefined ? {} : { invitee_email: row.invitee_email as string }), ...(optionalString(row.invitee_name, `${path}.invitee_name`) === undefined ? {} : { invitee_name: row.invitee_name as string }), ...(row.invited_by === null ? { invited_by: null } : optionalString(row.invited_by, `${path}.invited_by`) === undefined ? {} : { invited_by: row.invited_by as string }), ...(optionalString(row.inviter_email, `${path}.inviter_email`) === undefined ? {} : { inviter_email: row.inviter_email as string }), ...(optionalString(row.inviter_name, `${path}.inviter_name`) === undefined ? {} : { inviter_name: row.inviter_name as string }), role, status, ...(optionalString(row.message, `${path}.message`) === undefined ? {} : { message: row.message as string }), expires_at: stringValue(row.expires_at, `${path}.expires_at`), ...(row.responded_at === null ? { responded_at: null } : optionalString(row.responded_at, `${path}.responded_at`) === undefined ? {} : { responded_at: row.responded_at as string }), created_at: stringValue(row.created_at, `${path}.created_at`), ...(optionalString(row.invite_url, `${path}.invite_url`) === undefined ? {} : { invite_url: row.invite_url as string }), ...(row.is_share_link === undefined ? {} : { is_share_link: row.is_share_link === true }), ...(row.accepted_count === undefined ? {} : { accepted_count: numberValue(row.accepted_count, `${path}.accepted_count`) }) };
}

function parsePage(value: unknown, path: string): TenantMemberPage {
  const data = dataRecord(value, path);
  const raw = array(data.members, `${path}.data.members`);
  return { items: raw.map((item, index) => parseMember(item, `${path}.data.members[${index}]`)), total: numberValue(data.total, `${path}.data.total`), page: optionalNumber(data.page, `${path}.data.page`) ?? 1, pageSize: optionalNumber(data.page_size, `${path}.data.page_size`) ?? raw.length };
}

function parseInvitationPage(value: unknown, path: string): TenantInvitationPage {
  const data = dataRecord(value, path);
  const raw = array(data.invitations, `${path}.data.invitations`);
  return { items: raw.map((item, index) => parseInvitation(item, `${path}.data.invitations[${index}]`)), total: numberValue(data.total, `${path}.data.total`), page: optionalNumber(data.page, `${path}.data.page`) ?? 1, pageSize: optionalNumber(data.page_size, `${path}.data.page_size`) ?? raw.length };
}

function parseMembership(value: unknown, path: string): Membership {
  const data = dataRecord(value, path);
  const membership = record(data.membership, `${path}.data.membership`);
  return { tenantId: numberValue(membership.tenant_id, `${path}.data.membership.tenant_id`), role: stringValue(membership.role, `${path}.data.membership.role`) as TenantRole, status: stringValue(membership.status, `${path}.data.membership.status`), joinedAt: stringValue(membership.joined_at, `${path}.data.membership.joined_at`), ...(optionalString(data.tenant_name, `${path}.data.tenant_name`) === undefined ? {} : { tenantName: data.tenant_name as string }) };
}

function parseAudit(value: unknown, path: string): AuditLogPage {
  const envelope = success(value, path);
  const rows = array(envelope.data, `${path}.data`);
  const items = rows.map((item, index): AuditLog => {
    const itemPath = `${path}.data[${index}]`;
    const row = record(item, itemPath);
    const details = row.details;
    if (details !== null && details !== undefined && typeof details !== 'string' && (typeof details !== 'object' || Array.isArray(details))) {
      throw new Error(`${itemPath}.details must be an object, string, or null`);
    }
    return {
      id: numberValue(row.id, `${itemPath}.id`),
      tenant_id: numberValue(row.tenant_id, `${itemPath}.tenant_id`),
      actor_user_id: stringValueAllowEmpty(row.actor_user_id, `${itemPath}.actor_user_id`),
      actor_role: stringValueAllowEmpty(row.actor_role, `${itemPath}.actor_role`),
      action: stringValue(row.action, `${itemPath}.action`),
      scope_type: stringValueAllowEmpty(row.scope_type, `${itemPath}.scope_type`),
      scope_id: stringValueAllowEmpty(row.scope_id, `${itemPath}.scope_id`),
      target_type: stringValueAllowEmpty(row.target_type, `${itemPath}.target_type`),
      target_id: stringValueAllowEmpty(row.target_id, `${itemPath}.target_id`),
      target_user_id: stringValueAllowEmpty(row.target_user_id, `${itemPath}.target_user_id`),
      request_path: stringValueAllowEmpty(row.request_path, `${itemPath}.request_path`),
      request_method: stringValueAllowEmpty(row.request_method, `${itemPath}.request_method`),
      outcome: stringValue(row.outcome, `${itemPath}.outcome`) as AuditOutcome,
      details: details === undefined ? null : details as Record<string, unknown> | string | null,
      created_at: stringValue(row.created_at, `${itemPath}.created_at`),
    };
  });
  return { items, nextCursor: optionalNumber(envelope.next_cursor, `${path}.next_cursor`) ?? 0 };
}

function parseCount(value: unknown): { pendingCount: number } { return { pendingCount: numberValue(dataRecord(value, '/me/invitations/pending-count').pending_count, '/me/invitations/pending-count.data.pending_count') }; }

export function createTenantMembersApi(request: IdentityRequest) {
  return {
    async list(tenantId: number, params: { q?: string; page?: number; pageSize?: number } = {}, signal?: AbortSignal): Promise<TenantMemberPage> {
      return parsePage(await request(withSignal({ method: 'GET', path: query(`/api/v1/tenants/${encoded(tenantId, 'tenantId')}/members`, [['q', params.q?.trim() || undefined], ['page', params.page], ['page_size', params.pageSize]]) }, signal)), '/tenants/members');
    },
    async add(tenantId: number, input: { email: string; role: TenantRole }, signal?: AbortSignal): Promise<TenantMember> {
      const data = dataRecord(await request(withSignal({ method: 'POST', path: `/api/v1/tenants/${encoded(tenantId, 'tenantId')}/members`, body: input }, signal)), '/tenants/members');
      return parseMember(data, '/tenants/members.data');
    },
    async updateRole(tenantId: number, userId: string, role: TenantRole, signal?: AbortSignal): Promise<ActionSuccessResponse> {
      const result = await request(withSignal({ method: 'PUT', path: `/api/v1/tenants/${encoded(tenantId, 'tenantId')}/members/${encoded(userId, 'userId')}`, body: { role } }, signal));
      return parseActionSuccessResponse(result);
    },
    async remove(tenantId: number, userId: string, signal?: AbortSignal): Promise<void> {
      action(await request(withSignal({ method: 'DELETE', path: `/api/v1/tenants/${encoded(tenantId, 'tenantId')}/members/${encoded(userId, 'userId')}` }, signal)));
    },
    async leave(tenantId: number, signal?: AbortSignal): Promise<void> {
      action(await request(withSignal({ method: 'POST', path: `/api/v1/tenants/${encoded(tenantId, 'tenantId')}/leave`, body: {} }, signal)));
    },
  };
}

export function createTenantInvitationsApi(request: IdentityRequest) {
  const listTenant = async (tenantId: number, options: { includeTerminal?: boolean; page?: number; pageSize?: number } = {}, signal?: AbortSignal) => parseInvitationPage(await request(withSignal({ method: 'GET', path: query(`/api/v1/tenants/${encoded(tenantId, 'tenantId')}/invitations`, [['include_terminal', options.includeTerminal ? 'true' : undefined], ['page', options.page], ['page_size', options.pageSize]]) }, signal)), '/tenants/invitations');
  const listMine = async (options: { includeTerminal?: boolean } = {}, signal?: AbortSignal) => parseInvitationPage(await request(withSignal({ method: 'GET', path: query('/api/v1/me/invitations', [['include_terminal', options.includeTerminal ? 'true' : undefined]]) }, signal)), '/me/invitations');
  return {
    listTenant,
    async create(tenantId: number, input: { email: string; role: TenantRole; message?: string }, signal?: AbortSignal): Promise<TenantInvitation | TenantMember> {
      const data = dataRecord(await request(withSignal({ method: 'POST', path: `/api/v1/tenants/${encoded(tenantId, 'tenantId')}/invitations`, body: input }, signal)), '/tenants/invitations');
      return data.id !== undefined ? parseInvitation(data, '/tenants/invitations.data') : parseMember(data, '/tenants/invitations.data');
    },
    async revoke(tenantId: number, invitationId: number, signal?: AbortSignal): Promise<void> { action(await request(withSignal({ method: 'DELETE', path: `/api/v1/tenants/${encoded(tenantId, 'tenantId')}/invitations/${encoded(invitationId, 'invitationId')}` }, signal))); },
    listMine,
    async pendingCount(signal?: AbortSignal) { return parseCount(await request(withSignal({ method: 'GET', path: '/api/v1/me/invitations/pending-count' }, signal))); },
    async accept(invitationId: number, signal?: AbortSignal) { return parseMembership(await request(withSignal({ method: 'POST', path: `/api/v1/me/invitations/${encoded(invitationId, 'invitationId')}/accept` }, signal)), '/me/invitations/accept'); },
    async acceptByToken(token: string, signal?: AbortSignal) { return parseMembership(await request(withSignal({ method: 'POST', path: '/api/v1/me/invitations/accept-by-token', body: { token } }, signal)), '/me/invitations/accept-by-token'); },
    async decline(invitationId: number, signal?: AbortSignal): Promise<void> { action(await request(withSignal({ method: 'POST', path: `/api/v1/me/invitations/${encoded(invitationId, 'invitationId')}/decline` }, signal))); },
    async createInviteLink(tenantId: number, input: { role: TenantRole; message?: string }, signal?: AbortSignal): Promise<TenantInvitation> { const data = dataRecord(await request(withSignal({ method: 'POST', path: `/api/v1/tenants/${encoded(tenantId, 'tenantId')}/invite-links`, body: input }, signal)), '/tenants/invite-links'); return parseInvitation(data, '/tenants/invite-links.data'); },
  };
}

export function createTenantAuditLogApi(request: IdentityRequest) {
  return {
    async list(tenantId: number, params: { afterId?: number; limit?: number; action?: string; outcome?: AuditOutcome; actor?: string } = {}, signal?: AbortSignal): Promise<AuditLogPage> {
      const path = query(`/api/v1/tenants/${encoded(tenantId, 'tenantId')}/audit-log`, [
        ['after_id', params.afterId], ['limit', params.limit], ['action', params.action], ['outcome', params.outcome], ['actor', params.actor],
      ]);
      return parseAudit(await request(withSignal({ method: 'GET', path }, signal)), '/tenants/audit-log');
    },
  };
}

/** Port of Vue CreateTenantDialog -> createTenant (POST /api/v1/tenants,
 *  internal/handler/tenant.go:89-90: name required 1-128, description max 512).
 *  Tenant self-service may be disabled server-side; failures surface as errors. */
export function createTenantAdminApi(request: IdentityRequest) {
  return {
    async create(input: { name: string; description?: string }, signal?: AbortSignal): Promise<Record<string, unknown> & { id: number | string }> {
      const name = typeof input.name === 'string' ? input.name.trim() : '';
      if (name.length < 1 || name.length > 128) throw new Error('name must be 1-128 characters');
      if (input.description !== undefined && input.description.length > 512) throw new Error('description must be at most 512 characters');
      const data = dataRecord(await request(withSignal({ method: 'POST', path: '/api/v1/tenants', body: { name, ...(input.description === undefined ? {} : { description: input.description }) } }, signal)), 'POST /tenants data');
      const id = data.id;
      if ((typeof id !== 'string' || !id.trim()) && (typeof id !== 'number' || !Number.isSafeInteger(id) || id <= 0)) throw new Error('tenant.id is required');
      return data as Record<string, unknown> & { id: number | string };
    },
    /** Port of Vue deleteTenant (DELETE /api/v1/tenants/:id). Owner+ per RBAC
     *  (routes_auth_tenant.go:18). Irreversible — deletes the workspace and
     *  all associated data. */
    async deleteTenant(tenantId: number, signal?: AbortSignal): Promise<void> {
      if (typeof tenantId !== 'number' || !Number.isSafeInteger(tenantId) || tenantId <= 0) throw new Error('tenantId must be a positive safe integer');
      await request(withSignal({ method: 'DELETE', path: `/api/v1/tenants/${tenantId}` }, signal));
    },
  };
}

export type TenantMembersApi = ReturnType<typeof createTenantMembersApi>;
export type TenantInvitationsApi = ReturnType<typeof createTenantInvitationsApi>;
export type TenantAuditLogApi = ReturnType<typeof createTenantAuditLogApi>;
