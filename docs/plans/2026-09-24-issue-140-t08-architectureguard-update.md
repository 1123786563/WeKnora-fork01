# T08 Career route-count guard update

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep architectureguard's exact route count current after reviewed T08/#146 Opportunity backend routes are integrated.

**Architecture:** Career already owns `internal/router/routes_career.go` via its in-place manifest, so no new module or ownership transfer is needed. Update only the guard's current route-count expectation and dated architecture extension after verifying the actual three T08 routes. Preserve the original F0 16/633 snapshot and prior T07 17/644 checkpoint as historical facts.

**Tech Stack:** Go guard tests and architecture documentation.

**Sources:** T08 backend task report and reviewed integrated code; `docs/architecture/moves/career.yaml`, `tools/architectureguard/discovery_test.go`, `docs/architecture/moves/README.md`. Do not execute until T08 backend is independently reviewed and integrated.

## Global Constraints

- No Career code, router, migration, manifest ownership or unrelated test changes under this task.
- Do not change counts until integrated code discovery confirms exactly three new Career route registrations, literal 578 + API-key 69 + handle 0 = total 647. If discovery differs, identify and account for the difference before editing.
- Local commits authorized; no push/merge/deploy. Preserve exact-total drift detection and unowned-route detection.

## Review Focus

- New current total 647 is explained as prior 644 plus three T08 Opportunity routes, not substituted into historical baseline prose.
- A fourth undeclared registration still fails the guard. Career continues to own only its route file.
- Focused guard and full Go suite pass on the same integrated code SHA.

---

### Task 1: Reconcile current Career route baseline

**Depends:** reviewed T08 backend integration. **Owner/validator:** mechanical_worker / backend_validator. **Files:** `tools/architectureguard/discovery_test.go`, `docs/architecture/moves/README.md` or dated addendum; focused guard test only if necessary. **Consumes:** exact integrated route discovery. **Produces:** reviewed current baseline and dated evidence.

- [ ] PRECHECK: Run `go test -count=1 ./tools/architectureguard -run 'TestGuardCleanAtHead|TestDiscoverRealRepoRouteTotals' -v` and `go run ./tools/architectureguard --root .`; save actual counts and diagnostics. Confirm only expected count mismatch and three new T08 registrations already covered by Career manifest.
- [ ] RED/GREEN: Adjust current literal/total expectation from 575/644 to 578/647 and document the T08 three-route extension. Keep API-key 69, handle 0, manifest 17 and other totals unchanged. Preserve prior dated history.
- [ ] VERIFY: Focused guard, `go test -count=1 ./tools/architectureguard/...`, `go test -count=1 ./...`, and `git diff --check`. Save exact commands/SHA/report; independent reviewer checks baseline accounting and drift enforcement before integration.

## Shared-file preflight

T08 backend owns route registrations; this task starts only after their reviewed code is integrated. T08 Web will own Career TypeScript and UI, not guard files. No implementation overlap is allowed in the guard worktree.
