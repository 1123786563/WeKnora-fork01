# OCR R2 Budget Pause Authorization Checkpoint

## Assignment

- Finding: F16 only — `BudgetPause` must allow TaskRead callers to read the persisted pause fact; extension authorization remains enforced on mutation/projection paths.
- Worktree: `/Users/wuyongjun/.codex/worktrees/ocr-budget-auth/WeKnora-fork01`
- Starting HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`
- Repair plan pointer: integration checkout `docs/plans/2026-09-28-craft-107-ocr-r2-plan.md` (not present in this worktree; the assigned F16 brief, OCR finding and referenced spec/ledger were used).
- References read: `docs/plans/craft-107-ocr-final-1.md` F16 at lines 219–232; `docs/specs/2026-09-23-craft-web-artifact-spec.md` budget and Task role requirements; `docs/plans/2026-09-23-craft-107-ledger.md` R1 finding disposition; `internal/handler/session/craft_budget_pause.go` TaskRead gate and `can_extend` projection.

## Checkpoint

- Changed files:
  - `internal/application/service/craft_budget.go`
  - `internal/application/service/craft_budget_t19_test.go`
- Test change: the durable T19 journey now verifies a non-owner TaskRead reader receives the same pause reason and logical call counts. It reuses the `member` identity for the forbidden `ExtendAndResume` assertion, preserving mutation authorization coverage.
- Implementation: removed only `authorizeBudgetActor` from `BudgetPause`; the handler still requires `TaskRead`, `MayExtendBudget` still projects extension authority, and `ExtendAndResume` still authorizes before extension and again reaches the authorized pause read.
- No schema, API shape, persistence, cancellation, or error mapping changes.

## TDD and verification evidence

Commands run from the worktree root:

1. RED: `go test ./internal/application/service -run '^TestCraftT19Journey$' -count=1`
   - Expected failure observed before production edit: `craft_budget_t19_test.go:53`, `Received unexpected error: craft forbidden`.
2. GREEN: `go test ./internal/application/service -run '^TestCraftT19Journey$' -count=1`
   - PASS: `ok github.com/Tencent/WeKnora/internal/application/service 2.151s`.
3. Focused suite: `go test ./internal/application/service -run 'CraftBudget|CraftT19' -count=1`
   - PASS: `ok github.com/Tencent/WeKnora/internal/application/service 21.082s`.
4. Static check: `go vet ./internal/application/service`
   - PASS, exit 0, no diagnostics.
5. Formatting: `gofmt -w internal/application/service/craft_budget.go internal/application/service/craft_budget_t19_test.go`
   - Completed before green verification.

## Snapshot evidence

- HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit created.
- Changed-file SHA-256 after verification:
  - `internal/application/service/craft_budget.go`: `3db0b031180d205c3b9621981e72652c7771c5bca516f496f2e6341ed0f07f82`
  - `internal/application/service/craft_budget_t19_test.go`: `5d6b93452cfe10b2290eff29b79f1d6797113acee163db43182f9e1a484114ee`
- Self-review: the production diff removes only the redundant read-side actor check and adds a comment describing the handler/mutation boundaries. The test pins collaborator read success and member mutation denial. No files outside the assigned service implementation, its focused service test, and this checkpoint report were changed.

## Scoped Review Fix 1/5 — HTTP TaskRead Journey

- Updated `TestCraftT20BudgetPauseHTTPJourney` to assert a viewer GET returns 200 and the expected run ID, exhausted reason, limit 1, used 1, and `can_extend=false`. The same journey continues to assert viewer POST is 403 and a same-tenant non-member GET is 403.
- Added a distinct `nonmember` test identity (`u3`) to `internal/handler/session/craft_test.go`; the old `admin` alias maps to the same `u2` identity as the viewer in this fixture, so it could not represent a non-member. Updated the sibling T20 journey's same-tenant non-member assertion to use this identity after the full package run exposed the same alias issue there.
- RED: with the old `authorizeBudgetActor` temporarily restored in the service, `go test ./internal/handler/session -run '^TestCraftT20BudgetPauseHTTPJourney$' -count=1` failed as expected at the viewer GET: expected 200, got 403 (`craft forbidden`). The temporary service edit was removed before GREEN; the service implementation remains at its approved prior checkpoint state.
- GREEN: `go test ./internal/handler/session -run '^TestCraftT20BudgetPauseHTTPJourney$' -count=1` passed (`ok .../internal/handler/session 2.271s`).
- Package check: the first `go test ./internal/handler/session -count=1` found the sibling journey still using `admin` for non-member and failing with 200; after correcting it to `nonmember`, the package passed (`ok .../internal/handler/session 22.215s`).
- Formatting: `gofmt -w internal/handler/session/craft_test.go internal/handler/session/craft_budget_pause_test.go internal/handler/session/craft_t20_journey_test.go`.
- Integration-checkout isolation: a mistakenly targeted one-line fixture edit was immediately reverted. `internal/handler/session/craft_test.go` is clean in `/Users/wuyongjun/trea/WeKnora-fork01`; no integration-checkout changes remain from this task.
- No commit created. HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`.
- Final SHA-256 for changed source/test files:
  - `internal/application/service/craft_budget.go`: `3db0b031180d205c3b9621981e72652c7771c5bca516f496f2e6341ed0f07f82`
  - `internal/application/service/craft_budget_t19_test.go`: `5d6b93452cfe10b2290eff29b79f1d6797113acee163db43182f9e1a484114ee`
  - `internal/handler/session/craft_budget_pause_test.go`: `39c5c1f694e464350a8c956fb058575206592ee1dbe7307c000b14c4644fd3ae`
  - `internal/handler/session/craft_test.go`: `de98d6833b39415b0421b39ac16cc9c2ec90a67854f84e3dc30e1c805a52c87d`
  - `internal/handler/session/craft_t20_journey_test.go`: `058bf8b15e2b757c801327f038b0f04fb378a35b8c10d536a5360cbc3684acc9`
- Self-review: the test response shape and literal pause values are asserted directly. The added fixture identity is same-tenant (`tenant=1`, `user=u3`) and is rejected by the existing Task checker; `viewer` remains `u2` and passes TaskRead. Both T20 journeys now use the non-member identity when checking 403. No production file changed in this fix round.
