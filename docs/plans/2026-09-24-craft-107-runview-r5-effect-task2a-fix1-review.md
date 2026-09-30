# Craft #107 R5 effect Task2a Fix1 — independent review

Date: 2026-09-24. Scope: the two-file `craft_run_view_effect.go` and focused test increment against the approved Craft Spec, ADR-0008, `CONTEXT.md`, R5 Task2a plan, Fix1 plan/report/checkpoint and prior independent findings R5E-1/R5E-2. No production, test, migration, OCR or remote issue content was changed.

## Finding disposition

**R5E-1 closed.** `BeginEffect` now invokes `requireCraftRunViewAllocationIntent` inside the same transaction after `lockAndValidateRun` and the exact RunView generation/scope check, before replaying or inserting a provider-kind intent (`craft_run_view_effect.go:164–172`). The helper requires the exact tenant/Run/generation `allocate` row, current admitted tenant/owner/session/actor/snapshot identity, a stored nonempty writer owner and positive epoch, and `finished/succeeded` with `receipt == generation` (`:270–289`). A legacy key-only `Allocate` creates no such row and cannot yield `maySend=true`. Completed allocation replay after a newly valid lease remains possible because only the historical allocation's *identity* is compared, while the caller's current fence is checked separately under the Run lock. Provider-kind replay still checks its exact original owner/epoch and returns `maySend=false`.

**R5E-2 closed.** `FinishEffect(succeeded)` rejects an empty or whitespace-only receipt before opening its update transaction (`craft_run_view_effect.go:223–230`). The regression confirms an empty success leaves `state=pending`, empty outcome/receipt and the same replay token with `maySend=false`. Unknown remains unresolved; a different successful replay receipt still conflicts under the existing exact-outcome rule.

No new actionable finding was found in this two-file increment. The typed receipt remains opaque; provider-specific identity association belongs to Task3 as explicitly scoped in the Fix1 plan.

## Checkpoint and verification

Patch SHA-256 matched `7ef217a2d1dca63211f1cfd3ca38aec739ce2da9e6140cebff6baea7f1a0d04a`. I extracted the recorded preimage archive, verified both preimage hashes, applied the patch after `git apply --check`, and compared both resulting files byte-for-byte with the checkpoint post hashes and current worktree. The postimage archive matches them as well. The patch touches only these two owned files; no migration or concurrent file is attributed to Fix1.

I independently ran `go test ./internal/application/repository -run 'TestCraftRunView(Effect|EffectAuthority|EffectIntent)' -count=1` on the current worktree; it passed. The worker's exact-postimage SQLite/isolated PostgreSQL and race runs are recorded in the report but were not independently rerun with a PostgreSQL DSN here.

## Scoped verdicts

**Spec compliance: PASS for R5E-1/R5E-2. Code quality: PASS for the exact Fix1 checkpoint.** Task2b still must guard transition/reclaim/slot paths against unresolved intents, and Task3 must bind provider-specific receipts and route every physical mutation through this authority. Production remains default-off until those separate gates pass.
