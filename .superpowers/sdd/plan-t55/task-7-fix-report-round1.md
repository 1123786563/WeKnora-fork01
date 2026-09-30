# Task 7 Fix Report — Review Round 1

Status: DONE_WITH_CONCERNS (live provider path blocked by environment)

## Review findings addressed

1. **Missing task branch handling in the live prepare path:** `BranchProtected` calls GitHub's branch endpoint before delivery preparation. GitHub returns 404 for a branch that does not exist yet. It now treats only a GitHub 404 as `protected=false`; all other API/transport errors remain errors, so authentication and provider failures fail closed. The GitHub wire emulator now returns 404 for absent branch refs (while recognizing extant/protected branches), and a regression test checks both missing branch 404 and invalid-token 401 behavior. The existing `BranchHead` missing-ref behavior also has explicit emulator coverage for 404 versus 401.
2. **Real baseline checkout:** the gated recovery loop reads the baseline commit tree and all blobs into the temporary repository workspace before writing a new marker path. It asserts the marker path is absent from the baseline, avoiding overwriting baseline content or representing the entire repository as deletions.
3. **Draft PR evidence:** after delivery, the real test reads the PR back from GitHub and asserts `Draft=true` before checking duplicate dispatch refusal and stable readback.

## TDD and verification evidence

- **RED — missing branch safety:** `go test ./internal/modules/codedelivery/ -run 'TestGitHubClientBranchProtectedTreatsOnlyNotFoundAsUnprotected' -count=1 -v` failed against the old implementation because `BranchProtected("missing")` returned `github api ... status 404: branch not found`.
- **GREEN — focused recovery and branch tests:** `go test ./internal/modules/codedelivery/ -run 'TestGitHub(ClientBranchHeadTreatsOnlyNotFoundAsMissing|ClientBranchProtectedTreatsOnlyNotFoundAsUnprotected|RealRecoveryLoopNoRepeatPush)' -count=1 -v` — PASS. BranchHead and BranchProtected emulator tests passed. The real provider recovery test skipped with the expected `blocked-env` message because required test environment opt-in/credentials were unavailable.
- **Package regression:** `go test ./internal/modules/codedelivery/ -count=1` — PASS.
- **Diff hygiene:** `git diff --check` — PASS.

## Changed code files

- `internal/modules/codedelivery/github_client.go`
- `internal/modules/codedelivery/github_real_test.go`
- `internal/modules/codedelivery/github_wire_test.go`

## Remaining limits

The live GitHub path, repository write, branch creation and PR draft readback were not exercised. Real credentials and explicit writable test-repository opt-in were unavailable (`blocked-env`); no live GitHub repository was changed. The test must run against a dedicated writable test repository to provide provider evidence. The pre-existing untracked `paseo.json` is outside the task and remains untouched.
