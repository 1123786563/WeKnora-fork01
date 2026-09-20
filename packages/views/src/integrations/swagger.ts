/**
 * Swagger docs entry helpers (SP14 Task 2).
 *
 * The backend mounts /swagger/*any only outside gin release mode
 * (internal/router/router.go) and advertises that fact through
 * GET /system/info → `swagger_enabled` (internal/handler/system.go). UI
 * surfaces that link to the docs must (a) point at the gin-swagger index —
 * the historical "/docs" URL was dangling — and (b) render the entry only
 * when the flag explicitly confirms the route exists.
 */

/** Absolute docs URL for an apiBaseUrl of any trailing-slash spelling. */
export function swaggerDocsUrl(apiBaseUrl: string): string {
  return apiBaseUrl.replace(/\/+$/, '') + '/swagger/index.html';
}

/**
 * Only an explicit `true` shows the docs entry. `undefined` (older backend
 * without the field, or a failed system-info fetch) and `false` (release
 * build) both hide it — a hidden link beats another dead one.
 */
export function shouldShowSwaggerDocs(swaggerEnabled: boolean | undefined): boolean {
  return swaggerEnabled === true;
}
