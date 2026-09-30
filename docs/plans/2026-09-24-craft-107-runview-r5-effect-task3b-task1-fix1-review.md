# Craft #107 R5 Task3b Task 1 Fix1 — independent re-review

Date: 2026-09-24. Scope: R5-3B-1 only, against the Fix1 plan, prior independent review and exact two-file checkpoint. No Docker/provider or migration run was repeated.

## Verdicts

- **Scoped Spec compliance: PASS.** `BeginEffectWithDigest` now rejects all three legacy kinds before entering `beginEffect` or opening a database transaction. A rejected mixed-API call writes no intent; the original `BeginEffect` still obtains the first permission and returns the same token without a second permission on exact replay. New network/probe kinds still require the digest API and reject missing/changed digests.
- **Code quality: PASS.** The three-line boundary guard is narrow, and the new repository test covers every legacy kind, zero row after rejection, first claim, empty digest and same-token replay. No new blocking finding was identified in this scoped fix.
- **R5 physical admission remains blocked.** This API correction does not prove canonicalization of real provider requests, one physical send, Docker receipts, or production DI.

## Evidence

The preimage archive, postimage archive and incremental patch SHA-256 values match the checkpoint: `fb017f01b288ad1044a15825f0da05a72bf8961e83d7ed9ab4f9b8aead9f8dbd`, `13990a8de4c12918f058130776ddb096f1c8b4c8013a32ae38e76d25ff15d59e`, and `9a9105c615734f5642a692a2dc36917732d6be71913744691ffb9fc87efe1db9`. The two archived preimage file hashes match the original Task3b Task1 postimages. Applying the patch to them reproduces both archived postimages and current integration files byte for byte: repository source `cd658d1c...` and test `e54aa74b...`.

At `craft_run_view_effect.go:150-161`, `BeginEffectWithDigest` checks `requiresRunViewEffectRequestDigest(kind)` and returns `ErrInvalidInput` before delegating to the transactional method. That predicate is true only for `docker_network_create` and `docker_probe`. The legacy `BeginEffect` path and its empty-digest behavior are unchanged. The new test at `craft_run_view_effect_test.go:217-248` verifies all three old kinds (`docker_create`, `docker_start`, `opencode_create`) and checks no intent row after the rejected call, then first `maySend=true` and exact same-token `maySend=false` replay. Existing new-kind tests continue to exercise lowercase digest syntax, drift conflict, unknown replay and exact receipt semantics.

I independently ran the focused command covering the new cross-API test plus the two new-kind authority tests; it passed. The report records the same focused selector and a race run passing. The prior Task1 migration evidence remains limited to SQLite behavior and isolated PostgreSQL migration 205; this Fix1 does not change those migration bytes.
