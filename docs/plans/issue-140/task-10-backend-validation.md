# T10/#150 Backend Independent Validation

- **Result:** DONE_WITH_CONCERNS
- **Validated revision:** `d170cc02ea478f51a21804f6193d67a046424e0a` (`feat(career): add immutable evaluation endpoint`)
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-evaluation/WeKnora-fork01`
- **Scope:** Backend Office/API, auth and scope checks, persistence/replay, migrations, cancellation recovery and architecture registration. No source or test files changed during validation.

## Commands and results

1. `git rev-parse HEAD` → exact validated SHA above; `git status --short` → clean before validation.
2. `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` → PASS. Career, router, database, handler, handler/dto, handler/session and container all passed. Existing linker warning: `ignoring duplicate libraries: '-lc++'`.
3. `go test -count=1 ./internal/database -run '^TestCareerEvaluationSQLiteMigrationUpDownUp$'` → PASS. SQLite reaches schema 116, migrates down to 115 with prior Opportunity tables intact, then up and restores evaluation table/columns.
4. `go run ./tools/architectureguard --root .` → PASS: `literal=581 apiKeyRoute=69 handle=0 total=650`, zero violations.
5. `git diff --check d170cc02ea478f51a21804f6193d67a046424e0a^ d170cc02ea478f51a21804f6193d67a046424e0a` → PASS, no whitespace errors.
6. `TRPC_TEST_POSTGRES_DSN` availability check → unset; PostgreSQL migration/runtime validation could not be run.

## Acceptance evidence

- Focused tests cover 2026 versus 2027 graduation, matching and missing year, conflicting/unsupported requirements, punctuation boundaries, malformed/unconfirmed/contradictory facts, soft evidence that cannot override hard ineligibility, and omission of aggregate score/probability.
- Evidence returned by the read includes the pinned snapshot ID/raw text/hash and cited byte spans, plus confirmed fact key/value, profile revision, fact revision, source and confirmation metadata. Tests verify span-to-source identity and profile provenance on known rules.
- Exact replay after a profile edit returns the original receipt; a new request captures the new revision and the old evaluation remains unchanged/readable. Explicitly changed request intent conflicts, and future pinned revision conflicts.
- Office tests verify another user and another tenant cannot read evaluation/receipt; handler contract tests verify owner scope, the authenticated owner-only scope gate and rejection of client-supplied assessment fields. Routes are registered under the existing `/api/v1/career` group.
- Cancellation after persistence is injected and recovers the original receipt. Receipt reads are bounded by Tenant + owner scope.
- Evaluation creation reads an immutable opportunity snapshot and confirmed fact versions, persists the evaluation/receipt in one transaction, and makes no model/tool/network call. SQLite up/down/up is validated.

## Gaps and risks

- PostgreSQL schema application, transaction/row-lock behavior and cancellation recovery remain unverified because no test DSN is configured. The SQL migration exists and is structurally consistent with the SQLite schema, but this is not runtime evidence.
- The selected backend intentionally recognizes only the narrow `仅限 <year> 届` graduation rule. Other hard requirements stay `unknown`; overall evaluation is not complete until the downstream Web task renders unknown/ineligible and provenance without implying eligibility. This is a scoped parser limitation, not a validation failure.
- No acceptance gap found in the backend behaviors exercised at this revision. This report does not approve the independent code review or full T10/#150 completion.
