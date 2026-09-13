// Tenant app-connector API (T15): the frontend face of the T13 route table
// under /api/v1/apps. Every call goes through @/utils/request's get/post, so
// the tenant scope always arrives from the authenticated context (the
// X-Tenant-ID header) and never from a client-supplied parameter.
//
// Wire projections mirror the backend view structs exactly:
//   - connections: id/kind/state/owner_id — the backend view deliberately has
//     NO credential field, and neither does this type;
//   - catalog entries: the reviewed action definitions reachable through this
//     tenant's live bindings (published, scope-bearing, provider-matching);
//   - action detail: the frozen approval snapshot (state, digest, target,
//     args content, display connection name) plus the fence an approval must
//     echo — never a runtime address, secret reference or internal alias.
import { get, post } from '@/utils/request'

/** Optional per-call options; today only request cancellation (space epoch). */
export interface AppConnectorCallOptions {
  signal?: AbortSignal
}

/** One installed app version (GET /apps/installations). */
export interface AppInstallationView {
  id: string
  app_key: string
  version: string
  state: string
  scopes: string[]
}

/** One reviewed, tenant-reachable action definition (GET /apps/catalog). */
export interface OCCatalogEntry {
  action_id: string
  app_id: string
  app_version: string
  provider: string
  risk: string
  connection_id: string
  schema_digest: string
  input_schema: string | Record<string, unknown>
  required_scopes: string[]
  published: boolean
}

/** One connection projection (GET /apps/connections). No credential field. */
export interface AppConnectionView {
  id: string
  kind: 'personal' | 'space' | string
  state: string
  owner_id?: string | null
  /** Live authorization generation (R18): the fence a revoke must echo. */
  auth_version: number
}

/** Authorization-attempt status (POST/GET authorization-attempts). */
export interface OCAuthorizationAttemptView {
  attempt_id: string
  status: string
  connection_id: string
  expires_at: string
}

/** The frozen action snapshot an approval binds to (GET /apps/actions/:id). */
export interface AppActionView {
  id: string
  state: string
  digest: string
  target: string
  content: string
  connection_name: string
  /** Frozen risk category from the persisted snapshot (R18). */
  risk: string
}

export interface AppActionDetailView {
  action: AppActionView
  expected_version: number
}

export async function listInstallations(options: AppConnectorCallOptions = {}): Promise<AppInstallationView[]> {
  const response: any = await get('/api/v1/apps/installations', options.signal ? { signal: options.signal } : undefined)
  return Array.isArray(response?.data) ? response.data : []
}

export async function listOCCatalog(options: AppConnectorCallOptions = {}): Promise<OCCatalogEntry[]> {
  const response: any = await get('/api/v1/apps/catalog', options.signal ? { signal: options.signal } : undefined)
  return Array.isArray(response?.data) ? response.data : []
}

export async function listConnections(options: AppConnectorCallOptions = {}): Promise<AppConnectionView[]> {
  const response: any = await get('/api/v1/apps/connections', options.signal ? { signal: options.signal } : undefined)
  return Array.isArray(response?.data) ? response.data : []
}

// BeginOCAuthorization starts one single-use, correlate-able authorization
// attempt. T13-F-3 (binding): the backend returns NO authorization URL —
// only the attempt id to poll. The UI must NEVER fabricate or link one.
export async function beginAuthorizationAttempt(
  connectionId: string,
): Promise<OCAuthorizationAttemptView> {
  const response: any = await post(
    `/api/v1/apps/connections/${encodeURIComponent(connectionId)}/authorization-attempts`,
    {},
  )
  const data = response?.data ?? response
  return {
    attempt_id: data?.attempt_id ?? '',
    status: data?.status ?? 'pending',
    connection_id: connectionId,
    expires_at: data?.expires_at ?? '',
  }
}

export async function getAuthorizationAttempt(
  attemptId: string,
  options: AppConnectorCallOptions = {},
): Promise<OCAuthorizationAttemptView> {
  const response: any = await get(
    `/api/v1/apps/authorization-attempts/${encodeURIComponent(attemptId)}`,
    options.signal ? { signal: options.signal } : undefined,
  )
  const data = response?.data ?? response
  return {
    attempt_id: data?.attempt_id ?? attemptId,
    status: data?.status ?? '',
    connection_id: data?.connection_id ?? '',
    expires_at: data?.expires_at ?? '',
  }
}

// RevokeConnection is the A02 local revocation. The server CAS-checks
// auth_version against expectedVersion; the connection list DTO carries the
// live auth_version (R18), so callers echo the value they just read and
// surface a 409 VERSION_CONFLICT honestly when it went stale.
// The local transaction is the authorization authority — a 200 means the
// connection is locally revoked; the REMOTE cleanup (token/connection
// deletion upstream) runs asynchronously afterwards.
export async function revokeConnection(
  connectionId: string,
  expectedVersion: number,
): Promise<AppConnectionView> {
  const response: any = await post(
    `/api/v1/apps/connections/${encodeURIComponent(connectionId)}/revoke`,
    { expected_version: expectedVersion },
  )
  return response?.data ?? response
}

export async function getAction(
  actionId: string,
  options: AppConnectorCallOptions = {},
): Promise<AppActionDetailView> {
  const response: any = await get(
    `/api/v1/apps/actions/${encodeURIComponent(actionId)}`,
    options.signal ? { signal: options.signal } : undefined,
  )
  return response?.data ?? response
}

// ApproveAction binds a human decision to the CURRENT snapshot: the digest
// and fence must echo what the server returned, so the caller always re-reads
// the action after the response instead of optimistically marking success.
export async function approveAction(
  actionId: string,
  digest: string,
  expectedVersion: number,
): Promise<AppActionDetailView> {
  const response: any = await post(
    `/api/v1/apps/actions/${encodeURIComponent(actionId)}/approve`,
    { digest, expected_version: expectedVersion },
  )
  return response?.data ?? response
}

// ExecuteAction consumes the approval and dispatches. An unobservable
// provider outcome parks the action in unknown — the ONLY resolution is a
// provider query, never a resend (phase one offers no retry button at all).
export async function executeAction(actionId: string): Promise<AppActionDetailView> {
  const response: any = await post(
    `/api/v1/apps/actions/${encodeURIComponent(actionId)}/execute`,
    {},
  )
  return response?.data ?? response
}

// PrepareOCAction is the trusted OC prepare path (T13). The body admits ONLY
// connection_id/action_id/input — risk, version, runtime and alias are
// derived server-side. Not wired to a view yet; kept so the API layer maps
// the full T13 route table.
export async function prepareOCAction(
  connectionId: string,
  actionId: string,
  input: Record<string, unknown>,
): Promise<AppActionDetailView> {
  const response: any = await post('/api/v1/apps/oc/actions/prepare', {
    connection_id: connectionId,
    action_id: actionId,
    input,
  })
  return response?.data ?? response
}
