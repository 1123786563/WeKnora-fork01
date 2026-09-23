# Validation Report — Architecture Guard Integration

- Plan: `docs/plans/2026-09-24-issue-140-architectureguard-integration.md`
- Code revision validated: `6bc188d3da8fbdc4dde08cf9ece78b677ae0e36c`
- Report-only HEAD: `4b85e89efb5c2eb9889d89c5fcbc2db546ed01cd`
- Status: **PASS with one bounded evidence limitation**
- Working tree at start and end: clean before this report was created; only this assigned report is new.

## Commands and evidence

1. `git status --short && git rev-parse HEAD && git rev-parse 6bc188d3da8fbdc4dde08cf9ece78b677ae0e36c` — HEAD was `4b85e89efb5c2eb9889d89c5fcbc2db546ed01cd`; requested implementation commit resolves exactly to `6bc188d3da8fbdc4dde08cf9ece78b677ae0e36c`. Initial status was empty.
2. `go test -count=1 ./tools/architectureguard/...` — PASS (`ok`, 0.216s).
3. `go test -count=1 ./tools/architectureguard -run 'TestGuardCleanAtHead|TestDiscoverRealRepoRouteTotals' -v` — PASS. Both tests passed; real route assertion confirms 575 literal, 69 API-key, 0 handle = 644.
4. `go run ./tools/architectureguard --root .` — PASS; output: `literal=575 apiKeyRoute=69 handle=0 total=644 | redis=23 lite=23 | hooks=58 | modules=17`, followed by `OK (0 violations)`.
5. `go test -count=1 ./tools/architectureguard/...` was not rerun after running the CLI; no tracked source was changed. The only write was this assigned report.

## Isolated route injection evidence

`TestRunRejectsUnownedRouteFile` in `tools/architectureguard/check_test.go` creates a `t.TempDir()` fixture with an injected `RegisterNewthingRoutes` route registration. It first confirms the declared entry is covered, then calls `Run` without a manifest entry and asserts `route-file-coverage` names `internal/router/routes_newthing.go`. This passed in the package run above. The fixture is isolated and does not alter the checkout.

The separate `TestDiscoverRealRepoRouteTotals` asserts exact discovered totals against 575/69/0/644. There is no single existing fixture test that injects a route and checks the real-repository total assertion together: `Run` itself summarizes arbitrary fixture counts but does not enforce a global expected count. Thus evidence confirms unowned-route coverage failure for injected routes and exact count enforcement against the real repository, but not a combined synthetic “injected route makes the count assertion fail” case.

## Reused same-revision evidence

The implementation report `.superpowers/sdd/2026-09-24-issue-140-implementation/task-architectureguard-report.md` records `go test -count=1 ./...` passing and `git diff --check` passing on implementation commit `6bc188d3da8fbdc4dde08cf9ece78b677ae0e36c`. This is same-revision evidence, so the full suite was not rerun. It records only macOS duplicate `-lc++` linker warnings.

## Acceptance and risks

- Route discovery counts: PASS (575 / 69 / 0; total 644).
- Whole-repository guard diagnostics: PASS (zero).
- Injected unowned route coverage behavior: PASS through isolated temporary fixture.
- Full Go suite: PASS by same-revision implementation evidence; not independently rerun.
- No auth/API, migration, or cancellation acceptance applies to this architectureguard integration.
- Bounded limitation: the injected fixture is not wired to the real-repository hardcoded total test as one combined test; separate coverage and count assertions provide evidence for each behavior.
