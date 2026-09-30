# T08 Central A Fix 1 Report

## Scope

Addressed central A review findings A-1 and A-2. The feature-route registry is now created per dig container and passed through `RouterParams` into the production session-route assembly. The process-global Craft feature registry and its registration helper were removed. Access feature registration remains duplicate-checked and fail-closed on the owning registry. Existing variadic route registration APIs preserve callers that do not supply Craft features.

The fix1 Review Package is [here](2026-09-23-craft-107-t08-central-a-fix1-review-package.patch). It contains the central A changes from the prior base through the observed commit plus the current uncommitted router seam refinement. Its SHA-256 is `6770f61a7ac9477a3d1931b4bdc16e039d860059da2620de3a245041f2841f6c`.

## RED → GREEN and tests

- RED: `go test ./internal/container -run '^TestCraftAccessFeatureRegistriesAreAssemblyOwned$' -count=1` — exit 1 before the per-registry API existed; the prior wiring accepted a registrar callback and had no way to pass an assembly-owned registry.
- GREEN: `go test ./internal/container ./internal/router ./internal/handler/session -run 'TestCraftAccess|TestCraftFeature|TestCraftHTTPRoutes' -count=1` — exit 0. This passed against the current source and covers:
  - Two dig assemblies each resolve a concrete service and the same instance as `craft.TaskAccessChecker`; their route closures retain separate SQLite databases, and a duplicate registration in one registry does not block a second assembly.
  - Missing dig dependencies and duplicate feature registration fail closed.
  - The production `RouterParams` → `registerSessionRoutes` → `RegisterSessionRoutes` seam mounts the access feature inside the Viewer+/chat API-key guarded sessions group. Anonymous context gets 401, a scoped API key without chat gets 403, a tenant admin without Task grant gets 403, and an owner can grant/revoke while audit rows persist.
  - T00 production route policy and escape validation remain covered.
- `go test ./internal/application/service -run 'TestCraft(T08|AccessMigration)' -count=1` — exit 0.
- `git diff --check` — exit 0.

Container tests emitted the existing linker warning `ignoring duplicate libraries: '-lc++'`.

The auth route test exercises the production RBAC/API-key authorizer and session route construction; test middleware supplies the caller context instead of running the full JWT validation middleware. The full `BuildContainer` startup was not invoked because it starts unrelated application providers and external services. The dig provider/invoke and `RouterParams` handoff are exercised separately at their production seams.

## Shared worktree commit event

I did not run `git commit`. While the shared integration worktree was being edited, it advanced from base `929fb287e5e0ed29c6637b118fbdf0d009232728` to commit `a5e9195acd6500c085c85d60c852148e7bbbbf34`; the root coordinator confirmed it did not create that commit. I did not rewrite it.

- Parent: `929fb287e5e0ed29c6637b118fbdf0d009232728`
- Observed commit: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- Author/committer: `wuyj <wuyj@yinhai.com>`
- Author/committer time: `2026-09-23T19:32:33+08:00`
- Subject: `feat(craft): 实现任务权限管理的后端核心功能`

That commit includes the initial central A files and other task-owned deliverables. Three small refinements made afterward remain uncommitted in the shared worktree: `router.go` delegates through the RouterParams seam helper; `routes_chat.go` defines that helper; the router test now invokes the same helper. The current integration worktree has not been committed by this agent.

## Current source hashes

SHA-256 at report creation:

| File | SHA-256 |
| --- | --- |
| `internal/container/container.go` | `50e43e6f0d10014e6a50118797ac86444800a1274a8f9b36fd4ce99dbceaadbc` |
| `internal/container/craft_access_wiring.go` | `e0f887826e0938f6d5aef3824336d7ab50759f2ae910ab68fe6cbf0748a5fa56` |
| `internal/container/craft_access_wiring_test.go` | `c97e81f220bafe0bed95e7925e8db11627c2d00f1ef06962ddf66dd387056846` |
| `internal/container/craft_features.go` | `c259438800c392818c2566f09fde0935720c17516e2d1e5d5ea41a4d61c618e0` |
| `internal/handler/session/craft_features.go` | `9bdfb389baa21b7ab7420995c70a72ed00ec2851599c43a2d7ace79bbd6298c5` |
| `internal/handler/session/craft.go` | `2b5c20515037632fd8ab9e4cfba396d24a98d7d70e7c572cb4b868aa8cbde847` |
| `internal/router/routes_chat.go` | `12a87b392b598c13c1ef904749d4216e5dc133fd2e52fa9f7805636af78c85fe` |
| `internal/router/router.go` | `1c3c9feab36a381c8abec7800f58d3937cba3bc2143ee369d5c33ff8dd3d075d` |
| `internal/router/routes_craft_features_test.go` | `d1ae88017694443ecb5e1e099abd51ba11039d135727a26685bb9076abd7a170` |

## Remaining scope and review

Central support B remains outstanding: authorized Task filtering for list/direct/session/version/preview/download paths and persisted revocation behavior across those authenticated HTTP journeys. Do not mark T08 complete from this fix1 result. The independent review must evaluate the Review Package together with the current source hashes and the uncommitted router seam refinement; no independent fix1 review has yet been recorded.
