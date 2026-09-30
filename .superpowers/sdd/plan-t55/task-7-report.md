# Task 7 Report — real GitHub recovery loop

## Scope

Implemented only `TestGitHubRealRecoveryLoopNoRepeatPush` and its env-backed credential adapter in `internal/modules/codedelivery/github_real_test.go`. The test requires `WEKNORA_GITHUB_TEST_TOKEN`, `WEKNORA_GITHUB_TEST_REPO`, and `WEKNORA_GITHUB_TEST_WRITABLE=1`; without all three it reports a `blocked-env` skip. The true-provider path constructs the delivery against a dedicated test repository and verifies successful delivery, refusal of duplicate dispatch (`ErrDeliveryState`), and stable delivered read state. Its comment documents that it pushes a task branch and creates a draft PR, that a dedicated test repo is required, and that deterministic real-provider PR failure injection is not available.

## Verification evidence

- `go test ./internal/modules/codedelivery/ -run 'TestGitHubRealRecoveryLoopNoRepeatPush' -count=1 -v` — PASS; the new test SKIPPED with the expected `blocked-env` reason because real write credentials/repository opt-in are unavailable in this environment. No credential values were inspected or printed.
- `go test ./internal/modules/codedelivery/ -count=1` — PASS (`ok ... 1.717s`); package regression suite passed, including the env-gated skip.
- `git diff --check` — PASS.

## Constraints and remaining limits

The real GitHub mutation path was not exercised because the required credential/repository environment was unavailable (`blocked-env`). No real repository was changed. The pre-existing untracked `paseo.json` is outside Task 7 and was not staged or committed.

## Review repair round 1

The repair is recorded in `task-7-fix-report-round1.md`. It handles GitHub's missing-task-branch 404 in `BranchProtected` without swallowing non-404 errors, verifies emulator 404/401 behavior, materializes all baseline blobs before adding a new marker file, and reads back the real PR draft flag. Local package checks pass; the live-provider path remains `blocked-env` because write credentials and explicit repository opt-in are unavailable.
