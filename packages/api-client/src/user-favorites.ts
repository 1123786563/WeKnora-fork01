import type { ClientRequest } from './client.ts';

/**
 * User resource favorites API (internal/handler/user_resource_favorite.go,
 * internal/router/routes_agent.go RegisterUserFavoriteRoutes). Per-user,
 * per-tenant star state for kb/agent resources: the endpoints derive
 * (user_id, tenant_id) from the auth context and are Viewer+. Responses ride
 * the standard success envelope and are unwrapped/validated here like every
 * other module.
 *
 * Field names mirror the backend DTO exactly (types.UserResourceFavorite in
 * internal/types/user_resource_favorite.go). created_at is the DB timestamp
 * serialized by gin's JSON encoder (RFC3339 string); only its string-ness is
 * enforced. resource_type is validated against the same service allowlist
 * (service.ErrFavoriteInvalidType) so a malformed row cannot reach UI state.
 */
export type UserFavoriteResourceType = 'kb' | 'agent';

export interface UserFavorite {
  user_id: string;
  tenant_id: number;
  resource_type: UserFavoriteResourceType;
  resource_id: string;
  created_at: string;
}

type RecordValue = Record<string, unknown>;
function record(value: unknown, path: string): RecordValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path} must be an object`);
  return value as RecordValue;
}
function required(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}
function text(value: unknown, path: string): string {
  if (typeof value !== 'string') throw new Error(`${path} must be a string`);
  return value;
}
function numberValue(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) throw new Error(`${path} must be a finite number`);
  return value;
}
function resourceType(value: unknown, path: string): UserFavoriteResourceType {
  if (value !== 'kb' && value !== 'agent') throw new Error(`${path} must be "kb" or "agent"`);
  return value;
}
function successfulData(value: unknown, path: string): unknown {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) throw new Error(`${path}.data is required`);
  return envelope.data;
}
function expectSuccess(value: unknown, path: string): void {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
}
function id(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}

function parseFavorite(value: unknown, path: string): UserFavorite {
  const row = record(value, path);
  return {
    user_id: required(row.user_id, `${path}.user_id`),
    tenant_id: numberValue(row.tenant_id, `${path}.tenant_id`),
    resource_type: resourceType(row.resource_type, `${path}.resource_type`),
    resource_id: required(row.resource_id, `${path}.resource_id`),
    created_at: text(row.created_at, `${path}.created_at`),
  };
}

export function createUserFavoritesApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    /** GET /user/favorites?type=… — this user's starred ids for one resource type, newest first. */
    async list(type: UserFavoriteResourceType, signal?: AbortSignal): Promise<UserFavorite[]> {
      const suffix = `type=${resourceType(type, 'type')}`;
      const data = successfulData(await request({ method: 'GET', path: `/api/v1/user/favorites?${suffix}`, ...(signal === undefined ? {} : { signal }) }), '/user/favorites');
      if (!Array.isArray(data)) throw new Error('/user/favorites.data must be an array');
      return data.map((item, index) => parseFavorite(item, `/user/favorites.data[${index}]`));
    },
    /** POST /user/favorites {type,id} — upsert a star (idempotent server-side). */
    async add(type: UserFavoriteResourceType, resourceId: string, signal?: AbortSignal): Promise<void> {
      expectSuccess(await request({
        method: 'POST',
        path: '/api/v1/user/favorites',
        body: { type: resourceType(type, 'type'), id: required(resourceId, 'resourceId') },
        ...(signal === undefined ? {} : { signal }),
      }), '/user/favorites POST');
    },
    /** DELETE /user/favorites/:type/:id — remove a star (no-op when absent). */
    async remove(type: UserFavoriteResourceType, resourceId: string, signal?: AbortSignal): Promise<void> {
      expectSuccess(await request({
        method: 'DELETE',
        path: `/api/v1/user/favorites/${resourceType(type, 'type')}/${id(resourceId, 'resourceId')}`,
        ...(signal === undefined ? {} : { signal }),
      }), '/user/favorites DELETE');
    },
  };
}

export type UserFavoritesApi = ReturnType<typeof createUserFavoritesApi>;
