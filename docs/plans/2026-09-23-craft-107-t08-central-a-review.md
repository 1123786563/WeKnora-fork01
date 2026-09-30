# T08 central support A independent review

Reviewed 2026-09-23. Read-only source scope: `internal/container/container.go`, `internal/container/craft_access_wiring.go`, and `internal/container/craft_access_wiring_test.go` in the integration worktree at HEAD `929fb287e5e0ed29c6637b118fbdf0d009232728`. The incremental manifest SHA-256 is `a16eb84d5881324b76b12949dc4bc248d8cd7f6babb5bc81572608bbc42bde97`; the tracked `container.go` diff SHA-256 is `a787f71a3a43d37e923a642a014859ad470b9be28d6fd7b29e904466cbc8a911`. All three live file hashes match that manifest. I did not run OCR or repeat the implementer's Go tests.

Authority: approved Craft web-artifact Spec stories 28–31 and all-operation role rule; Issue #126 acceptance; `CONTEXT.md` Task roles; ADR-0004 Task/Session identity; T00 constrained feature-route contract; T08 central A plan/report; reviewed T08 lane and membership-migration records. This is a review of central support A, not a verdict on the pending central support B list/direct/session/version/preview/download enforcement.

## Verdict

- **Scoped Spec compliance: conditional pass.** The provider graph exposes one `*CraftAccessService` as the concrete service and as `craft.TaskAccessChecker`. The access feature is registered before `router.NewRouter` is provided. T00's registry constrains its three route patterns, and the production session router mounts feature routes on the authenticated Viewer+/chat-capability `/api/v1/sessions` group. The service, rather than tenant-admin status, decides Task membership. Grant/revoke audit is exercised against persisted SQLite in the focused test. The production boot/route claim still lacks a test that traverses the real container and router (finding A-2).
- **Code quality: needs correction for repeated assembly (finding A-1).** The new registration writes to a process-global, single-use feature registry; a second assembly in the same process fails after the first mount, and a failed assembly can retain its registration. This is particularly relevant to test isolation and retryable boot.
- **Issue #126 overall: not yet verified.** The previously recorded T08 integration gate remains: central support B must enforce current membership on all existing content paths, including list, direct read, preview issuance/redemption, and download bytes, then test revocation and audit through authenticated HTTP.

## Findings

### A-1 — Medium — New boot registration is not isolated per container/router

**Evidence / affected symbols:** `BuildContainer` invokes `registerCraftAccessFeature` on every assembly (`internal/container/container.go:1043`). That function registers `"access"` via the package-global `session.RegisterCraftFeatureRoute` (`internal/container/craft_access_wiring.go:34-35`, `internal/handler/session/craft_features.go:108-113`). The registry rejects the duplicate name and rejects every registration after `Mount` freezes it (`craft_features.go:47-55,59-74`). Production route construction mounts that same global registry in `RegisterCraftSessionRoutes` (`internal/handler/session/craft.go:155-158`). The new test uses a separate local registry, so it cannot expose this state transition.

**Impact:** A second `BuildContainer` in one process fails at this new invoke, even with an otherwise valid database. If the first build registers the feature but fails before the router mounts, retry also fails with a duplicate. A mounted router retains the first assembly's service closure, making later isolated router/container tests unsafe. A normal one-time process boot is unaffected.

**Smallest defensible correction:** Make feature registration and mounting belong to the container/router instance, threading a fresh `CraftFeatureRoutes` through the existing route assembly, while preserving duplicate-name and post-mount rejection within that instance. Add a two-assembly or failed-first-assembly test that checks the second router uses its own service.

### A-2 — Medium — Focused test does not exercise the production provider and authentication assembly

**Evidence / affected test:** `TestCraftAccessFeatureUsesPersistentPolicyOnConstrainedRoutes` directly calls `NewCraftAccessService`, `craftTaskAccessChecker`, and `wireCraftAccessFeature` with a local registry (`internal/container/craft_access_wiring_test.go:24-29`). It mounts that registry on a raw Gin group and injects user/tenant identity from `X-Test-User` in test middleware (`:30-37`). It never invokes `BuildContainer`, `registerCraftAccessFeature`, `router.NewRouter`, or the production auth/API-key policy group. The existing T00 `TestCraftFeatureProductionRouterPolicyAndEscape` proves that the generic feature registry can inherit the auth chain, but it registers a synthetic `sources` feature, not this access feature.

**Impact:** The focused test would still pass if the new provider or invoke lines were removed, moved after router construction, or made dependent on a missing/cyclic provider. It proves the handler/service behavior and route patterns, but does not prove that an authenticated production router actually exposes those routes or that duplicate/missing provider errors are handled at boot.

**Smallest defensible correction:** Add a focused assembly test that resolves the concrete service and checker from the same dig container, invokes the real access registration, mounts a router through the production session auth/API-key route seam, and asserts the access route table and denied unauthenticated/API-key requests. Keep the persisted owner grant/revoke/audit assertions already present; test the boot failure paths for missing and duplicate registration separately. If full `BuildContainer` has external startup effects, extract and test the small production composition boundary rather than replacing it with a manually wired local registry.

## Conforming evidence and limits

- `service.NewCraftAccessService` is provided once, then `craftTaskAccessChecker` returns that same pointer as the narrow port (`container.go:490-491`; `craft_access_wiring.go:13-15`). The graph adds no cycle because the access constructor takes only `*gorm.DB`; later consumers can take the checker without depending on the Craft session service.
- `wireCraftAccessFeature` rejects a nil service or registrar and propagates registration errors. `dig.Invoke` is wrapped by `must`, so a missing dependency or duplicate registration fails assembly rather than silently producing a nil access handler.
- The access route paths use `/:id` for GET and `/:session_id` for POST, matching the existing Gin method trees. `CraftFeatureRoutes.Mount` validates the `/craft/` segment before forwarding. `RegisterSessionRoutes` supplies the guarded `/sessions` group; `RegisterCraftSessionRoutes` mounts features only when the Craft session handler exists.
- The focused SQLite test proves tenant admin denial, owner grant/revoke, Viewer member-list access while granted, denial after revoke, unauthenticated denial under its test identity middleware, removal of the grant row, and two successful grant/revoke audit rows. It does not establish denial audit on existing content routes or cover central support B.
