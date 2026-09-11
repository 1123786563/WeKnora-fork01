# T01 Router / handler / permission 静态审计 — 2026-09-11

## Inventory boundary

The generated API matrix contains 452 unique method/path rows:

| Classification | Rows |
|---|---:|
| `registered-route` | 344 |
| `swagger` | 64 |
| `special-route` | 32 |
| `client-used` | 12 |
| **Total** | **452** |

The comparison column contains 285 rows present in both the Swagger inventory
and registered routes, 76 Swagger-only rows, and 91 implementation-only rows.
The separate route-parity inventory contains 53 page/entry rows, including
the file proxy and sandbox terminal WebSocket.

## Handler and permission authority

For registered routes, the authority remains the Gin registration in
`internal/router/routes_*.go` and `internal/router/files.go`, the handler DTO
parsers, middleware/RBAC guards, and focused router tests. The matrix therefore
does not mark a Swagger-only row as runtime-compatible merely because a client
could be generated.

`go test ./internal/router -count=1` passed. This suite covers the central
API-key policy self-check, platform/tenant/agent/knowledge/FAQ/chunker/sandbox
capability declarations, tenant and organization role matrices, cross-tenant
KB/file/wiki denial cases, scoped API-key behavior, and static Web/Embed
fallback behavior.

The focused router test inventory includes 86 router tests in the package;
46 are directly named for RBAC, API-key, capability, route, permission,
cross-tenant, or protected-resource behavior. These tests prove registration,
guard declarations, and selected negative/positive authorization paths. They
do not prove every handler DTO branch, every registered endpoint's live HTTP
response, or production role/tenant behavior.

## Remaining T01 review items

- Review the 76 Swagger-only rows against current handler registration before
  adding a shared client method.
- Review each of the 91 implementation-only rows for DTO, response envelope,
  middleware and target-task coverage; this includes special file, Embed, IM,
  and terminal routes.
- Run authenticated live smoke for representative read, mutation, binary,
  SSE, and WebSocket routes. No production identity or data was used in this
  static audit.

This artifact is static/router evidence only. It does not upgrade T01 to
`accepted` and does not replace the OpenAPI Generator trial evidence.
