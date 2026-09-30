# T08/#146 backend integration and independent review

- Integration worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`.
- Starting integration HEAD before backend: `52c8cb97e7d9ec92b50901d10f6ef81c0fc24877`.
- Reviewed code sequence from isolated T08 worktree: `7a29b3336068f10d82d850e9f793db1d14a0970e`, `b1eba2892bcca03b0737258034dd492ade177630`, `8e54ac44c6379077a641aacd563c0f11decbbb0a`, `e37dcb2e860dd2d8be6d402fb7ab98531064f07f`.
- Cherry-picked integration commits: `d9b78b048`, `fb89d08a5`, `c10e86279`, `fbad0772c`. Report-only commits were not cherry-picked; their contents are copied into this persistent plan directory.

## Review sequence

1. Initial independent reviewer: partial Spec pass, quality changes needed. Medium findings were an unrecoverable ambiguous transaction outcome and unbounded JSON binding. Low observation append test is deferred until an append interface exists.
2. First repair reviewer: original findings resolved; medium issue found in classifying permanent first-read database errors as uncertain.
3. Second repair reviewer: permanent errors fixed; medium concurrent first-read SQLite lock still escaped as unrecoverable 500.
4. Third repair reviewer: **Spec PASS and quality PASS**, no actionable finding. Transient first-read lock now enters finite scoped recovery, permanent first-read errors remain ordinary, post-write ambiguity and body 413 remain correct. Reviewer verified five repeated contention tests and focused Career/database/router/handler/container suites.

Independent backend validators passed the initial and each repair revision. Final validator ran ten repeated same-ID contention tests, permanent first-read failure, post-write ambiguity, body limit, race checks, broader Go suites, and diff check at source code commit `e37dcb2e860dd2d8be6d402fb7ab98531064f07f`. Exact commands and caveats are in the copied implementation and validation reports.

## Integrated evidence and remaining work

At integrated `fbad0772c`, `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` passed. `go run ./tools/architectureguard --root .` found literal 578 + API-key 69 + handle 0 = 647 routes, 17 manifests and zero violations. The guard's exact-count tests still expected 575/644 and failed; the isolated T08 guard task owns that update. The current T08 Web acceptance, browser flow, and full parent OCR are outstanding. PostgreSQL migration execution was unavailable because no test DSN was configured; SQLite migration up/down/up passed.
