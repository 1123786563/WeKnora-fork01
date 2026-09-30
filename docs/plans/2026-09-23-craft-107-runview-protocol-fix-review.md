# RunView OpenCode redirect fix independent review

Date: 2026-09-23. Read-only review of the two changed OpenCode client files at integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Full-content SHA-256 matches the fix report: `internal/modules/agentruntime/agent/opencode/client.go` `809d9dab9d3e58f6ccf3cf2d2dce0f8cff9d3d849fd555e02a174b005284c0bd`, `client_test.go` `67678202fa5caf2d7e89919d6302a932e22bf333bda894ac185b26a172551934`. The owned two-file `git diff --binary` SHA-256 independently reproduces `2c8099fe748ef9a5ab585c0c956a151bff7c2a50caff9bd0aba115c907a1fe79`. `git diff --check` passed. The shared worktree contains unrelated concurrent changes; no source/test file was edited for this review.

Sources: approved Craft Spec, T01 per-Run isolation design and pinned OpenCode 1.18.4 query/header precedence, original RunView protocol plan/report and independent review, redirect-fix plan/report. No OCR or live OpenCode server probe was run.

## Verdict

- **Scoped Spec compliance: PASS.** The earlier Medium redirect override is closed for bound clients. After any custom `CheckRedirect`, the policy parses the final redirect URL and rejects malformed query encoding or any decoded `directory` parameter, even if the value equals the bound directory. Same-origin redirects without an override remain usable, cross-origin and stripped-header redirects remain denied, and unbound legacy clients retain their earlier query behavior.
- **Scoped code quality: PASS.** `url.ParseQuery` catches percent-encoded key names and duplicate `directory` keys without rewriting the URL. New `httptest` cases assert that the target is never reached for plain, same-value, encoded, duplicate and custom-callback-added overrides. I independently ran the full OpenCode client package tests; they passed. No blocking finding remains in the two-file fix.
- **T01/T05 isolation acceptance: NOT VERIFIED.** The bound header and redirect policy are routing context. The pinned `/event` stream is global, and no per-Run filesystem/process isolation, session-directory validation or integrated Run A→B exclusion test is established by this change.

## Evidence and limits

`directoryRedirectPolicy` rechecks origin and bound header after the prior/custom callback, then calls `url.ParseQuery(req.URL.RawQuery)` and rejects a `directory` key (`client.go:89-123`). The pinned server reads query `directory` ahead of `x-opencode-directory`, so rejecting the key removes the demonstrated precedence override. The tests in `client_test.go` cover target-not-called behavior for 307 same-origin redirects, including callback mutation, and confirm unbound compatibility. Full `go test ./internal/modules/agentruntime/agent/opencode -count=1` passed independently; this is fixture evidence, not a running pinned-server or sandbox proof. The fix report hash is `f1b0dcbaa26e74bfbbb25e0d2bbf8ba59591b6d59d4c0e18cedb31faab32e326`.
