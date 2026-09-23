# T08 Central Support A Report

## Scope and result

Implemented central T08 access service and route wiring in integration worktree at HEAD `929fb287e5e0ed29c6637b118fbdf0d009232728`. The container provides one `*service.CraftAccessService` and exposes that same instance as `craft.TaskAccessChecker`. Before `router.NewRouter` is provided, a dig invoke registers the access feature through `container.RegisterCraftFeature`; registration errors fail assembly. The feature mounts only `GET /:id/craft/access`, `POST /:session_id/craft/access`, and `POST /:session_id/craft/access/revoke` through the constrained Craft route registry.

No commit was created. No T08 lane files, migration, T01 session service, T14 code, or frontend files were changed.

## TDD and verification evidence

- RED: `go test ./internal/container -run '^TestCraftAccessFeatureUsesPersistentPolicyOnConstrainedRoutes$' -count=1` failed to build because `craftTaskAccessChecker` and `wireCraftAccessFeature` did not exist.
- GREEN: the same focused test passed after wiring. It uses the migration-backed SQLite fixture, a local feature registry, and an authenticated identity context. A tenant admin without an explicit grant receives 403; the owner grants the Viewer role; the Viewer can list members; the admin and the Viewer after revoke receive 403; a request without user identity receives 401. It verifies the grant row is removed, both successful grant/revoke audit rows persist, the three route patterns are present, and no other feature route is registered.
- `go test ./internal/container ./internal/router ./internal/handler/session -run 'TestCraftAccess|TestCraftFeature|TestCraftHTTPRoutes' -count=1` — PASS (all three packages).
- `go test ./internal/application/service -run 'TestCraft(T08|AccessMigration)' -count=1` — PASS.
- `git diff --check` — PASS (exit 0).

The targeted tests emitted only the existing linker warning `ignoring duplicate libraries: '-lc++'`.

## Route and authority boundary

Production registration occurs in `BuildContainer` before the router provider is installed. `RegisterCraftFeature` feeds the construction-time constrained feature registry; the existing Craft session router mounts that registry inside its authenticated `/api/v1/sessions` group. Service authorization is authoritative: tenant admin role alone did not allow access in the real service HTTP test. The test uses a focused Gin middleware to populate authenticated tenant/user context rather than booting the complete production router; route inheritance is separately covered by `TestCraftFeatureProductionRouterPolicyAndEscape`.

## Remaining integration

Central support B still needs to apply `ListAccessibleTaskIDs` to central Task listing and the checker to direct session reads, session input/run paths, version reads/downloads, and preview issuance. Verify those guarded paths via production authenticated HTTP/audit journeys before marking T08 verified. The test's identity middleware is a focused seam, so a full production-router journey remains outstanding.

## Changed file hashes

SHA-256 at completion:

| File | SHA-256 |
| --- | --- |
| `internal/container/container.go` | `e913b555ff869a350f72ffc073d7adaff5fb9858c7b8253f8d650afb7546a3f8` |
| `internal/container/craft_access_wiring.go` | `166ed73cc3a9e77cffb5c368d56025ef723e2b103b98e28cb6e2d1d3a8ccbc96` |
| `internal/container/craft_access_wiring_test.go` | `1271cbd7561003ed7f7d85ab7de3df483f0f706ce827afad1530a2d328411c37` |

Execution role/model/reasoning metadata was not exposed to this agent runtime, so it is not inferred here.
