# T08 central support A fix 1 independent review

**Scope and identity:** Reviewed the central backend delta from `929fb287e5e0ed29c6637b118fbdf0d009232728` to `a5e9195acd6500c085c85d60c852148e7bbbbf34`, plus the current uncommitted refinements in `internal/router/router.go`, `routes_chat.go`, and `routes_craft_features_test.go`. The committed range also contains T08 UI and other task documents; this verdict covers only central A backend assembly. I read the fix plan/report, prior central A review, checkpoint JSON and review-package patch, approved Craft Spec, CONTEXT.md, ADR-0004/0009, #126/T08 plan, and relevant route tests. All nine current central source hashes match the checkpoint/report; the review-package SHA-256 is `6770f61a7ac9477a3d1931b4bdc16e039d860059da2620de3a245041f2841f6c`. The three router refinements remain uncommitted. I did not run OCR or repeat the reported Go tests.

## Verdict

- **Scoped Spec compliance: PASS for central support A.** The access feature is registered on a per-container registry before router construction, routed through `RouterParams` to the authenticated sessions group, and backed by the same persistent `CraftAccessService` instance exposed as `craft.TaskAccessChecker`. Route authorization and owner grant/revoke/audit behavior are exercised at the production session-route/policy seam. The full #126 acceptance remains pending central support B enforcement of existing list, direct read, preview, and download paths.
- **Code quality: PASS for this fix scope.** Prior A-1 and A-2 are addressed at the tested composition seams. No new blocking correctness, security, concurrency, or data-consistency finding was established in the central A delta. The tests do not start full `BuildContainer` or validate a real JWT, so the verdict is confined to the inspected provider, route, and policy seams.

## Prior finding disposition

### A-1 — Resolved: feature registry is assembly owned

`BuildContainer` provides `session.NewCraftFeatureRoutes` (`internal/container/container.go:491-493`); `registerCraftAccessFeature` receives that pointer through dig and registers before the router provider (`container.go:1045-1050`, `craft_access_wiring.go:17-33`). `RouterParams` requires the registry and `NewRouter` forwards the same instance through `registerSessionRoutes` to `RegisterCraftSessionRoutes` (`internal/router/router.go:27-33, 364`; `routes_chat.go:95-112, 205-210`; `internal/handler/session/craft.go:155-158`). The process-global feature-registration helper was removed. `TestCraftAccessFeatureRegistriesAreAssemblyOwned` creates two dig assemblies with separate databases, confirms their service/checker identity, verifies that a duplicate in one registry does not poison the other, and mounts both (`internal/container/craft_access_wiring_test.go:85-151`). This covers the earlier second-assembly failure and closure contamination.

### A-2 — Resolved at the production composition seams

The new container test invokes dig registration and checks missing-provider and duplicate-registration failures (`craft_access_wiring_test.go:109-151`). The router test passes an access feature through the actual `RouterParams` helper into `RegisterSessionRoutes`, using its production Viewer+ guard and API-key authorizer (`internal/router/routes_craft_features_test.go:102-158`; `routes_chat.go:98-112`). It checks anonymous 401, scoped key without chat 403, tenant admin without Task grant 403, owner grant, Viewer read while granted, revocation denial, and persisted grant/audit outcomes (`routes_craft_features_test.go:173-201`). Test middleware injects caller context; JWT verification and all of `BuildContainer` are not executed. That limitation does not negate the specific dig-to-route and route-policy seams the prior finding asked to prove.

## Evidence and remaining gate

The report records exit 0 for focused container/router/session tests, focused service tests, and uncommitted `git diff --check`; the current central source hashes match its checkpoint. I independently checked that the uncommitted router diff has no whitespace errors. A `git diff --check` over the whole committed range reports trailing whitespace in an unrelated T05 validation document; it is outside central A source and this verdict. The checkpoint and review package include both the commit and the three later router edits, so an empty `HEAD` diff alone would have missed the latter.

Central support B must still bind current Task membership to every existing content path and test revocation, preview redemption, download bytes, and audit through authenticated HTTP before T08/#126 can be marked verified. This report does not review T08 UI or claim that overall Issue #126 is complete.
