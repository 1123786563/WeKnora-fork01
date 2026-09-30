# T10 Task 1 backend implementation report

- **Task:** #150 / T10, Career Office evaluation and HTTP contract
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-evaluation/WeKnora-fork01`
- **Base:** `6ac8426b4b2a2d2f915c17f56c95a097f8d80d68`
- **Status:** implementation and targeted validation complete; independent review/integration remain with controller
- **Commit:** pending at report creation

## Files changed

- `internal/modules/career/evaluation.go`, `evaluation_test.go`
- `internal/modules/career/office.go`, `handler.go`
- `internal/router/routes_career.go`, `routes_career_test.go`
- `internal/database/career_migration_test.go`
- `migrations/sqlite/000116_career_evaluations.{up,down}.sql`
- `migrations/versioned/000195_career_evaluations.{up,down}.sql`

## Implementation

The Office creates a separate immutable `career_evaluations` row containing the original canonical request intent, request fingerprint, pinned Opportunity/snapshot identifiers, pinned profile revision, immutable receipt JSON and immutable evaluation JSON. It reads only scoped Opportunity evidence and append-only confirmed `career_fact_versions`; the current profile row is locked on PostgreSQL while capturing the revision. Facts are reconstructed as the latest version per key at or below that revision and must carry this owner's confirmation metadata. No profile mutation, model call, Task, URL fetch, Agent tool or external side effect occurs.

The deterministic parser recognizes one explicit `仅限 <year> 届` JD span. A matching confirmed year is eligible only when the remaining JD consists of whitespace or punctuation. An unambiguous mismatch is ineligible even when other requirements remain unparsed. Missing, unconfirmed, malformed, contradictory, ambiguous, bare-year and unsupported requirements remain unknown. Confirmed `graduation_year` and `education.graduation_year` accept a year, `届`/`年` suffix, or validated year/month/day date. Exact confirmed skill/project/preference values found in the JD are separate evidence-backed soft matches and cannot change hard status. No aggregate score or probability is emitted.

Receipt intent fingerprinting preserves omission versus an explicitly supplied profile revision. Replay is checked before reading the current revision, so retrying after profile edits returns the original receipt; changed intent returns `ErrIdempotencyConflict`. A fresh request captures the new revision, while old reads return their stored JSON. If transaction acknowledgement is uncertain, a bounded same-request lookup attempts recovery; otherwise the API returns `outcome_unknown` with the request ID. Evaluation reads and receipt recovery are scoped by both Tenant and owner.

## HTTP wire contract

Authenticated routes registered under `/api/v1/career`:

```text
POST /evaluations
GET  /evaluations/receipt?requestId=<requestId>
GET  /evaluations/:evaluationId
```

POST accepts only this body; unknown properties and trailing JSON are rejected with 400:

```json
{"requestId":"eval-1","opportunityId":"<id>","snapshotId":"<id>"}
```

Optional `profileRevision` may pin an existing historical revision. A revision above current returns 409 `revision_conflict`.

The POST and receipt lookup return:

```json
{"kind":"evaluation_created","requestId":"eval-1","evaluationId":"<id>","opportunityId":"<id>","snapshotId":"<id>","profileRevision":4,"status":"ineligible"}
```

GET by evaluation ID returns the same receipt fields at top level, plus `createdAt`, `rulesetVersion`, and:

```json
{
  "snapshot":{"opportunityId":"<id>","observationId":"<id>","snapshotId":"<id>","rawText":"...","rawSha256":"...","source":{"kind":"manual_paste"},"acquiredAt":"..."},
  "hard":{"overall":"ineligible","rules":[{
    "ruleId":"graduation_year","criterion":"graduation year","outcome":"ineligible","reasonCode":"graduation_year_mismatch",
    "jobEvidence":{"snapshotId":"<id>","observationId":"<id>","acquiredAt":"...","rawSha256":"...","spanStart":0,"spanEnd":13,"quotedText":"仅限2027届"},
    "profileEvidence":{"factKey":"education.graduation_year","value":"2026","revision":4,"factRevision":3,"source":{"kind":"manual"},"confirmation":{"userId":"<owner>","confirmedAt":"..."},"confirmedAt":"..."}
  }]},
  "soft":{"matches":[{"kind":"skill|project|intent","value":"...","jobEvidence":{...},"profileEvidence":{...}}]},
  "facts":[{"factKey":"...","value":"...","revision":4,"factRevision":3,"source":{...},"confirmation":{...},"confirmedAt":"..."}]
}
```

`soft.matches` and `facts` are empty arrays when no exact confirmed evidence is used. Missing profile evidence is omitted from the corresponding rule; the overall `profileRevision` remains present. All JD span offsets are UTF-8 byte offsets into the exact `snapshot.rawText`. No numeric aggregate field is returned.

Error mapping uses existing Career conventions: missing scoped evidence/receipt/evaluation is 404, unauthorized scope is 403, invalid input is 400, changed request intent or future pinned revision is 409, and unresolved commit outcome is 504 with `requestId`.

## RED/GREEN and verification evidence

RED was observed before implementation:

```text
go test ./internal/modules/career
FAIL: undefined EvaluationIneligible, Office.EvaluateOpportunity, EvaluateInput, Office.Evaluation, Evaluation
```

After implementing the seam and behavior, this exact targeted command passed:

```text
go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...
ok github.com/Tencent/WeKnora/internal/modules/career
ok github.com/Tencent/WeKnora/internal/router
ok github.com/Tencent/WeKnora/internal/database
ok github.com/Tencent/WeKnora/internal/handler
ok github.com/Tencent/WeKnora/internal/handler/dto
ok github.com/Tencent/WeKnora/internal/handler/session
ok github.com/Tencent/WeKnora/internal/container
```

The container test link emitted the existing linker warning `ignoring duplicate libraries: '-lc++'`; the package passed.

SQLite migration coverage runs from version 115 to 116, down to 115 (evaluation table removed and all T08 Opportunity tables preserved), then up to 116 with all evaluation columns restored. Both SQLite and PostgreSQL migration files are present; `TRPC_TEST_POSTGRES_DSN` was unset, so no PostgreSQL server migration run was available.

Runtime architecture scan:

```text
go run ./tools/architectureguard --root .
architectureguard: literal=581 apiKeyRoute=69 handle=0 total=650 | redis=23 lite=23 | hooks=58 | modules=17
architectureguard: OK (0 violations)
```

The route count includes the three new endpoint registrations; the historical guard baseline remains owned by the following Task 2.

Whitespace check:

```text
git diff --check
PASS (exit 0)
```

## Assumptions and remaining validation

- Ruleset `career-qualification-v1` deliberately supports only the explicit graduation-year requirement. Every other/partial/ambiguous hard condition stays unknown until a separately reviewed parser exists.
- Exact case-sensitive fact-value occurrence is the only soft match in this minimum backend slice; it is evidence, not a score or suitability conclusion.
- PostgreSQL SQL was reviewed structurally but could not be applied without a configured test DSN.
- Independent Spec/code review, independent backend validation, controller integration, and outer OCR remain pending.
