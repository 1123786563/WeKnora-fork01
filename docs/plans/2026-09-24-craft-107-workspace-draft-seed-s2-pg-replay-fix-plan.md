# Craft #107 T01 S2 PostgreSQL Replay Fix Plan

> **For Codex:** Execute through SDD RED → GREEN → REFACTOR and independent Spec/quality review. Root cause was established using `superpowers:systematic-debugging` in `2026-09-24-craft-107-workspace-draft-seed-s2-pg-replay-rootcause.md`.

**Goal:** Make same-key Craft admission replay compare the caller's original nested JSON intent semantically after PostgreSQL JSONB normalizes object order, while still rejecting any actual changed prompt, input, selected KB, actor or Task identity. The frozen Workspace seed remains server-owned and excluded only from replay intent comparison.

**Sources:** Approved Craft Spec current Workspace continuation; S2 Task2 Fix1 review and PostgreSQL validation; root-cause report. The observed failure is nested `craft_knowledge_selection` object key order on PG readback (`query,knowledge_base_ids` vs Go input order).

## Global constraints

- Integration Worktree only, no commit/push. Preserve H2/T19 edits.
- Do not make replay compare only `RequestHash`, ignore arbitrary fields, or reread the current head. Exclude exactly `craft_workspace_seed` from the persisted snapshot and reject it in caller input; compare every other JSON value with object order ignored and array order/value retained.
- Avoid float64 precision loss for integer IDs/sizes. Fail closed on malformed or duplicate-key ambiguity; preserve generic Run replay behavior.

## Task 1 — Canonical nested intent comparison

**Role:** backend_implementer. **Owned files:** `internal/application/repository/agent_run.go` and `agent_run_craft_seed_test.go` only. **Consumes:** stored PG JSONB/SQLite snapshot and caller snapshot. **Produces:** true for semantically identical original intent regardless of nested object key order; false for any altered value/array/field or malformed data.

1. RED: explicit nested-key reorder replay case and the real PG17 test `TestAgentRunCraftSeedAdmissionFreezesAndReplaysOriginalHead/postgres` after D1→D2. Keep negatives for changed prompt/input/knowledge and add nested value/array mutations. Verify no seed/head reread on replay.
2. GREEN: recursively canonicalize JSON objects (not just outer `map[string]json.RawMessage`) with exact number handling such as `Decoder.UseNumber` and trailing-data rejection; remove only the top-level server seed. Compare canonical bytes. Ensure duplicate object keys cannot hide changed intent (or prove parser/test rejects them) and keep caller-supplied seed rejection.
3. Run focused SQLite/PG17 replay and markers, repository race, service StartRun, compile check, gofmt/diff-check. Record exact disposable PG version/setup/cleanup and task-local hashes/patch. Do not broaden full package suite unless a concrete regression remains.

**Acceptance:** the observed PG replay passes with original frozen D1 after D2; changed nested fields still conflict; SQLite remains green. Independent review must pass. **Failure handling:** keep S2/R4 gated and report any unresolved JSON canonicality limit.

## Review focus

Recursive semantic equality across dialects, exact top-level seed exclusion, no duplicate-key/number precision bypass, and actual PostgreSQL behavioral evidence.
