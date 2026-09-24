# Craft #107 R5 Task3b Task1 Fix1 Report

**Finding:** R5-3B-1, medium — `BeginEffectWithDigest` accepted a non-empty digest for legacy kinds, creating an intent that `BeginEffect` could not replay.

**Change:** `BeginEffectWithDigest` now admits only kinds that require request digests (`docker_network_create` and `docker_probe`). The gate runs before `beginEffect` and before any database transaction. Added a cross-API regression covering legacy `docker_create`, `docker_start`, and `opencode_create`: the digest API returns `ErrInvalidInput`, persists no intent, and the legacy API can then make its one first claim and replay the same token without another send. Existing digest-bound new-kind tests remain in the focused run.

## TDD and verification

- RED: `go test ./internal/application/repository -run '^TestCraftRunViewEffectRequestAuthorityRejectsDigestAPIForLegacyKinds$' -count=1` — failed as expected for all three legacy kinds because the digest API returned nil error.
- GREEN: `go test ./internal/application/repository -run '^TestCraftRunViewEffect(RequestAuthorityRejectsDigestAPIForLegacyKinds|RequestAuthorityBindsNetworkAndProbeRequests|RequestAuthorityRequiresReceiptAndCurrentFence)$' -count=1` — passed.
- Race: `go test -race ./internal/application/repository -run '^TestCraftRunViewEffect(RequestAuthorityRejectsDigestAPIForLegacyKinds|RequestAuthorityBindsNetworkAndProbeRequests|RequestAuthorityRequiresReceiptAndCurrentFence)$' -count=1` — passed.
- `gofmt -w internal/application/repository/craft_run_view_effect.go internal/application/repository/craft_run_view_effect_test.go` — completed.

No migrations, providers, coordinators, DI, or commits were changed. Docker tests were not run; they are outside this fix and subject to the separate E0/T14 release barrier.

## Exact uncommitted checkpoint

- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged)
- Preimage: `2026-09-24-craft-107-runview-r5-effect-task3b-task1-fix1-preimage.tar.gz`, SHA-256 `fb017f01b288ad1044a15825f0da05a72bf8961e83d7ed9ab4f9b8aead9f8dbd`
- Postimage: `2026-09-24-craft-107-runview-r5-effect-task3b-task1-fix1-postimage.tar.gz`, SHA-256 `13990a8de4c12918f058130776ddb096f1c8b4c8013a32ae38e76d25ff15d59e`
- Task-local patch: `2026-09-24-craft-107-runview-r5-effect-task3b-task1-fix1-task-local.patch`, SHA-256 `9a9105c615734f5642a692a2dc36917732d6be71913744691ffb9fc87efe1db9`
- Current owned file hashes: `craft_run_view_effect.go` `cd658d1c7490d1caae0cc5fcef014d37dc1eaae876f7ee65d4f6fd58d38a23f1`; `craft_run_view_effect_test.go` `e54aa74b9004398489a764095d3fead1ef0f477fe79de17ea57bd843c85770dd`.
- Machine-readable checkpoint: `2026-09-24-craft-107-runview-r5-effect-task3b-task1-fix1-checkpoint.json`.

**Remaining risk:** Independent reviewer re-review is pending with the parent. No known behavioral limitation within this narrow API fix.
