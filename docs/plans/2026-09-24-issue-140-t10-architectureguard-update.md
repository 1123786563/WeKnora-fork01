# T10 Career Evaluation route-count guard update

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep architectureguard's exact route count current after reviewed T10/#150 Evaluation endpoints are integrated.

**Architecture:** Career already owns `internal/router/routes_career.go` through its in-place manifest. Measure the actual integrated route registrations before changing only current count expectations and a dated architecture note; preserve historical baselines.

**Tech Stack:** Go guard tests and architecture documentation.

**Spec:** `docs/plans/2026-09-24-issue-140-t10-evaluation.md` Task 2, reviewed T10 backend report, `docs/architecture/moves/career.yaml` and `docs/architecture/moves/README.md`.

## Global Constraints

- Do not begin until T10 backend is independently reviewed and integrated. No Career code, router, migration or ownership manifest changes under this guard task.
- Current T10 backend worktree reports three new Career routes and runtime discovery `literal=581 apiKeyRoute=69 handle=0 total=650`; remeasure at integrated HEAD and account for any difference before editing tests.
- Keep historical 633, 644 and 647 route checkpoints and 17 manifest ownership facts. Local commit authorized; no push/merge/deploy.

## Review Focus

- Current total is exactly prior 647 plus audited T10 registrations, with zero coverage/ownership violations.
- An unexpected later route still fails exact-count drift detection.
- Focused guard and full Go suite pass at the reviewed integrated code SHA.

---

### Task 1: Reconcile current route baseline

**Depends:** reviewed T10 backend integration. **Owner/validator:** mechanical_worker / backend_validator. **Files:** `tools/architectureguard/discovery_test.go`, dated `docs/architecture/moves/README.md`. **Consumes:** runtime discovery. **Produces:** reviewed current exact count.

- [ ] PRECHECK: Run `go test -count=1 ./tools/architectureguard -run 'TestGuardCleanAtHead|TestDiscoverRealRepoRouteTotals' -v` and `go run ./tools/architectureguard --root .`; record exact mismatch and prove the new routes belong to Career.
- [ ] RED/GREEN: Update current literal/total expectation only if measured 581/650, preserving API-key 69 and handle 0; append a dated +3 Evaluation route explanation without replacing historical text.
- [ ] VERIFY: focused guard, `go test -count=1 ./tools/architectureguard/...`, `go test -count=1 ./...`, runtime guard, `git diff --check`; independent review and validator check before integration.

## Shared-file preflight

Backend route writer finishes and integrates before this mechanical task starts. Web contracts/UI own no guard file. No shared test DB or build directory is used by parallel implementation.
