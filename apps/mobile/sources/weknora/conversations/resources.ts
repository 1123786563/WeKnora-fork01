import type { ProductAuthSession } from '@weknora/api-client';
import type { ProductIdentity } from '../platform/product-session';

export interface ProductSessionResourceSelection {
  spaceId: string;
  agentId: string;
  targetId: string;
  workspaceRef: string;
}

export interface VerifiedProductSessionResources extends ProductSessionResourceSelection {
  agentName: string;
}

type Row = Record<string, unknown>;
function row(value: unknown, path: string): Row {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path} must be an object`);
  const root = value as Row;
  if (root.success !== true || !root.data || typeof root.data !== 'object' || Array.isArray(root.data)) throw new Error(`${path} must be a successful envelope`);
  return root.data as Row;
}
function id(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}
function tenant(value: unknown, expected: string | null, path: string): void {
  if (!expected || (typeof value !== 'string' && typeof value !== 'number') || String(value) !== expected) throw new Error(`${path} tenant ownership could not be verified`);
}
function pathID(value: string): string { return encodeURIComponent(id(value, 'resource id')); }
function exact(value: unknown, expected: string, path: string): void {
  if (typeof value !== 'string' && typeof value !== 'number') throw new Error(`${path} relationship could not be verified`);
  if (String(value) !== expected) throw new Error(`${path} relationship could not be verified`);
}
function member(value: unknown, userID: string, path: string): void {
  if (Array.isArray(value)) {
    const found = value.some((item) => {
      if (typeof item === 'string') return item === userID;
      if (!item || typeof item !== 'object') return false;
      const row = item as Row;
      return String(row.user_id ?? row.userId ?? row.id ?? '') === userID && row.status !== 'revoked' && row.status !== 'inactive';
    });
    if (!found) throw new Error(`${path} membership could not be verified`);
    return;
  }
  if (value !== undefined && value !== userID) throw new Error(`${path} membership could not be verified`);
}

async function get(session: ProductAuthSession, path: string, signal: AbortSignal): Promise<Row> {
  const result = await session.transport.send({ method: 'GET', url: `${session.baseURL}${path}`, headers: { accept: 'application/json' }, signal });
  if (result.status < 200 || result.status >= 300) throw new Error(`RESOURCE_HTTP_${result.status}`);
  return row(result.body, path);
}

/** Resolve every execution resource from product APIs and reject unverifiable ownership. */
export async function resolveProductSessionResources(
  session: ProductAuthSession,
  selection: ProductSessionResourceSelection,
  identity: ProductIdentity,
  signal: AbortSignal,
): Promise<VerifiedProductSessionResources> {
  if (!identity.userId || !identity.tenantId) throw new Error('PRODUCT_SCOPE_REQUIRED');
  const [space, agent, target, workspace] = await Promise.all([
    get(session, `/api/v1/organizations/${pathID(selection.spaceId)}`, signal),
    get(session, `/api/v1/agents/${pathID(selection.agentId)}`, signal),
    get(session, `/api/v1/execution-targets/${pathID(selection.targetId)}`, signal),
    get(session, `/api/v1/execution-workspaces/${pathID(selection.workspaceRef)}`, signal),
  ]);
  tenant(space.tenant_id ?? space.owner_tenant_id, identity.tenantId, 'space');
  tenant(agent.tenant_id, identity.tenantId, 'agent');
  tenant(target.tenant_id, identity.tenantId, 'target');
  tenant(workspace.tenant_id, identity.tenantId, 'workspace');
  exact(space.id, selection.spaceId, 'space id');
  exact(agent.id, selection.agentId, 'agent id');
  exact(target.id, selection.targetId, 'target id');
  exact(workspace.id, selection.workspaceRef, 'workspace id');
  member(space.members ?? space.member_ids ?? space.memberships, identity.userId, 'space');
  if (space.owner_id !== undefined && String(space.owner_id) !== identity.userId && space.members === undefined && space.member_ids === undefined && space.memberships === undefined) throw new Error('space owner/member could not be verified');
  if (agent.owner_id !== undefined && String(agent.owner_id) !== identity.userId) throw new Error('agent owner could not be verified');
  if (agent.space_id !== undefined) exact(agent.space_id, selection.spaceId, 'agent space');
  if (target.space_id !== undefined) exact(target.space_id, selection.spaceId, 'target space');
  if (target.workspace_ref !== undefined) exact(target.workspace_ref, selection.workspaceRef, 'target workspace');
  if (workspace.space_id !== undefined) exact(workspace.space_id, selection.spaceId, 'workspace space');
  if (workspace.owner_id !== undefined && String(workspace.owner_id) !== identity.userId) throw new Error('workspace owner could not be verified');
  if (String(workspace.target_id) !== selection.targetId) throw new Error('workspace target ownership could not be verified');
  if (target.owner_id !== identity.userId || target.state !== 'active') throw new Error('target owner is not active for this user');
  if (String(agent.id) !== selection.agentId || String(target.id) !== selection.targetId || String(workspace.id) !== selection.workspaceRef) throw new Error('resource identity mismatch');
  return { ...selection, agentName: id(agent.name, 'agent.name') };
}
