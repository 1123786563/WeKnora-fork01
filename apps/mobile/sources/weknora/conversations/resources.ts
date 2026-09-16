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
  if (String(workspace.target_id) !== selection.targetId) throw new Error('workspace target ownership could not be verified');
  if (target.owner_id !== identity.userId || target.state !== 'active') throw new Error('target owner is not active for this user');
  if (String(agent.id) !== selection.agentId || String(target.id) !== selection.targetId || String(workspace.id) !== selection.workspaceRef) throw new Error('resource identity mismatch');
  return { ...selection, agentName: id(agent.name, 'agent.name') };
}
