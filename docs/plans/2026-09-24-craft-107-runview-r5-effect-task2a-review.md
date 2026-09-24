# Craft #107 R5 effect Task2a — independent review

Date: 2026-09-24. Scope: the Task2a effect authority port/store/tests and SQLite 000121 / PostgreSQL 000200 migrations, against the approved Craft Spec, ADR-0008, `CONTEXT.md`, R5 fence plan/architecture, and the assigned Task2a plan. No source, tests, migrations, remote issue, or OCR operation was changed.

## Findings

### R5E-1 — High: provider claims can adopt an unfenced legacy allocation

**Evidence:** `AllocateAdmitted` refuses an existing `craft_run_views` row without a completed `allocate` intent (`craft_run_view_effect.go:92–108`), and `TestCraftRunViewEffectAuthorityDoesNotAdoptLegacyKeyOnlyAllocation` checks only that API (`craft_run_view_effect_test.go:208–219`). `BeginEffect` independently loads the RunView, checks generation/owner/session, then checks only for an existing intent of the *requested provider kind* before minting `maySend=true` (`craft_run_view_effect.go:169–202`). It never requires the matching completed allocation intent. The still-callable key-only `CraftRunViewStore.Allocate` creates exactly such a RunView without an effect intent (`craft_run_view.go:109–139`).

**Impact:** A caller with a currently valid Task/fence can turn an old key-only generation into a Docker/OpenCode send permission by calling `BeginEffect` directly, despite `AllocateAdmitted` rejecting that generation. This breaks the stated no-adoption contract and the required allocation-before-provider authority chain. Production remains default-off, so this is a contract defect to fix before Task3 dispatch, not evidence of a live provider call.

**Smallest correction:** Under the same Run transaction, require the exact `(tenant, Run, generation, allocate)` row with admitted identity, `state=finished`, `outcome=succeeded`, and `receipt=generation` before any provider-kind replay or insert. Share the check with `AllocateAdmitted`; add a regression that legacy key-only `Allocate` followed by direct `BeginEffect` returns a conflict and inserts no provider intent.

### R5E-2 — Medium: a successful provider effect can finish without an observable identity

**Evidence:** `FinishEffect` permits `RunViewEffectStateSucceeded` with `Receipt == ""`; it checks only length and NUL (`craft_run_view_effect.go:227–231`) and then persists `state=finished, outcome=succeeded` (`:252–265`). Both migration schemas allow the empty receipt. The R5 architecture requires `FinishEffect` to record the observed identity/outcome, and the R5 plan requires an observable exact identity for Docker create/start and OpenCode create. Existing success tests use a nonempty container receipt and do not cover empty-success rejection.

**Impact:** A sent operation whose exact provider identity was not observed may be durably marked complete, removing it from unresolved-intent checks and allowing downstream stages or lease recovery to proceed as if reconciliation were possible. An ambiguous send should remain `unknown` until an exact identity is established.

**Smallest correction:** Reject `succeeded` with an empty receipt at the authority seam, and validate the effect-specific receipt shape or association when Task3 defines provider identity. Add a test that empty-success leaves the intent pending/unknown and cannot be replayed as finished.

## Scoped verdicts

**Spec compliance: FAIL** because R5E-1 bypasses fenced allocation and R5E-2 permits a completed provider outcome without observed identity. **Code quality: FAIL** for those missing invariants. The core current-Run claim path otherwise checks the live owner/epoch/status/lease through `lockToolRun`, locks the session writer slot, checks actor/tenant/session/Workspace and recomputes the admitted snapshot digest before inserting a unique generation/kind token. Exact same-kind replay returns `maySend=false`; unknown is durable. Completed allocation can replay after a valid lease recovery without minting a new generation. Task2b's unresolved-intent transition guard and Task3's provider wiring are still absent by plan, so the route must remain default-off.

## Checkpoint and verification

The task-local patch SHA-256 matched `0ebf28d7b0d59a38403f6fcc290d9f12765186a30ba10c26b9a4425076859076`. I verified the preimage archive against its SHA manifest (including `ABSENT` entries), applied the patch in an isolated temporary directory, and compared all ten resulting files byte-for-byte with the current worktree and checkpoint post hashes. The postimage archive and SHA manifest also match all ten current files. This includes the ignored SQLite121 and PG200 up/down migration bytes; they are not omitted from review.

The report records a successful focused SQLite/PostgreSQL test run and migration up/down/up checks on the exact checkpoint. My independent rerun of `go test ./internal/application/repository -run 'TestCraftRunView(Effect|EffectAuthority|EffectIntent)' -count=1` did not reach these tests: the concurrently edited, unrelated `craft_docker_normal_input.go:128` fails compilation because it compares a struct containing `[]string`. I did not alter that file or count this as a Task2a test failure. No independent PostgreSQL rerun was performed.
