# T01 S2 PostgreSQL replay fix — independent Spec and quality review

## Scope and evidence

Reviewed the approved Craft web artifact Spec (current Workspace continuation, serialized writing Runs, reconnection without duplicate submission), `CONTEXT.md` Run/Workspace terms, ADR-0004/0008/0009, the S2 PG root-cause note, fix plan/report, task-local patch/checkpoint, prior S2 Fix1 PASS review, and the live admission and test paths. This is a review of the PG replay fix, not final S2 acceptance. No OCR, child agents, or source/test edits were used.

The live `agent_run.go` and `agent_run_craft_seed_test.go` SHA-256 values match the checkpoint postimage (`19291b4f…` and `4b8d4a5a…`). Patch, archive, and report hashes also match the checkpoint (`21308c79…`, `d261ca02…`, `08856efb…`). `git diff --check` for the owned files passed. The task report records successful focused SQLite, service, repository race, compile, and PostgreSQL 17.9 commands, including `TestAgentRunCraftSeedAdmissionFreezesAndReplaysOriginalHead/postgres`, and removal of the disposable container. I independently reran the focused repository canonicalization, exact-integer, SQLite D1→D2 replay, and caller-seed tests: exit 0. The PostgreSQL subtest skipped in my rerun because `TRPC_TEST_POSTGRES_DSN` was unset; PG behavior in this review relies on the task report's recorded PG17.9 run rather than a second live execution.

## Findings

**Low — malformed/trailing JSON behavior lacks a direct regression case.** `agent_run.go:625–638` rejects a second JSON value and syntax errors after the first value, and `decodeJSONValueToken` rejects duplicate keys recursively. `agent_run_craft_seed_test.go:59–132` covers duplicate keys and precision but does not submit trailing values or malformed nested JSON through `sameCraftAdmissionIntent` or `persistUsageBinding`. **Impact:** the fail-closed requirement is supported by code inspection but its regression protection is incomplete. **Smallest correction:** add table cases for trailing JSON and malformed nested JSON on both stored and incoming comparison paths, plus one admission-boundary rejection case. This is a test gap, not an observed bypass.

The prior Fix1 Low finding remains outside this narrow fix: `TestAgentRunCraftSeedAdmissionSerializesAgainstDraftAdvance` starts both contenders together and does not force both transaction orders. The task report's passing PG race run does not close that S2 concurrency evidence gap. Force admission-before-Advance and Advance-before-admission with controlled barriers before declaring full S2 acceptance.

## Verified boundaries

- `sameCraftAdmissionIntent` decodes both snapshots recursively, normalizes exact decimal numbers with `json.Number` and `big.Int`, then compares marshaled values. JSON object key order is ignored at every depth; arrays retain order and value. The focused tests cover nested reordering, nested value and array changes, `1e3`/`1000` equivalence, and distinct integers above 2^53.
- Only the top-level `craft_workspace_seed` is deleted for replay comparison (`agent_run.go:543–548`); a nested seed-like key remains compared. Admission rejects a caller-supplied top-level seed before replay lookup (`agent_run.go:325–331`), and the focused caller-seed test passed.
- The existing Run branch checks Session, driver, target, budget, assistant ID, and actor before comparing the Craft intent (`agent_run.go:358–390`). It returns the stored Run without calling the current-head reader; head selection occurs only in the fresh-Run branch (`agent_run.go:394–430`). The SQLite D1→D2 replay test passed and asserts the old D1 seed, changed prompt/input/KB conflicts, and a fresh D2 seed. The task report records that same test passing on PG17.9.
- Duplicate object keys at every depth and trailing JSON are rejected by the decoder; `persistUsageBinding` uses that decoder before persisting. This prevents float64 rounding and duplicate-key ambiguity at admission. Generic replay still uses its existing request-hash branch.

## Verdict

**Spec compliance: PASS for the narrow PostgreSQL replay fix.** The implementation preserves original-intent replay and the frozen server-owned seed while allowing PostgreSQL JSONB nested key normalization. PG17.9 acceptance is supported by the task report; my independent SQLite rerun passed.

**Code quality: PASS with one Low test gap.** No critical, high, or medium defect was found in this patch. Full S2 acceptance still needs the previously identified forced transaction-order evidence and the controller's broader gates.
