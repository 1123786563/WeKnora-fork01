# Task 2 Report — Publisher Custody and Transactional Admission

**Status:** Verified; ready for integration
**Base:** `93706830b78205de0c7d433097e89f33d9726513`

## Changes

- Added a shared repository discoverability predicate requiring the Public Listing to be listed with an approved current Release, the Publisher to remain verified, and the source tenant Listing to remain listed. Catalog and detail paths use this predicate; catalog SQL filters at source.
- Introduction now takes publisher and adopter tenant security guards in ascending tenant ID order, then repeats the eligibility read inside the same write transaction before ledger/adoption writes. This serializes admission against Publisher revocation and source Listing unlist.
- Publisher verification/revocation and source Listing lifecycle transitions use the established tenant guard. #64 release/dependency guard implementation was retained without weakening its admission checks.
- Added revocation/unlist custody assertions and deterministic callback/channel races; verified existing introduced bytes and release publisher/submission identity remain unchanged.

## RED evidence

Before implementation, `go test ./internal/application/repository -run 'TestPublicMarketplaceCustodyDeniesNewIntroductionAfterPublisherRevocation' -count=1` failed because Introduction returned nil error after Publisher revocation.

## Verification

- `go test ./internal/application/repository -run 'TestPublicMarketplace(RevocationGuard|SourceUnlistGuard)SerializesIntroduction' -count=10` — PASS (deterministic guard races).
- `go test ./internal/application/repository -run 'TestPublicMarketplace.*Custody|TestPublicMarketplace.*Introduce|TestPublicMarketplace.*Revok' -count=10` — PASS.
- `go test ./internal/application/repository -run 'TestTransitionListingStateIsCAS' -count=1` — PASS.
- `go test ./internal/application/service -run 'TestPublicMarketplace' -count=1` — PASS.
- `git diff --check` — PASS.
- `go build ./...` — PASS.

## Scope and risks

Changed seven owned source/test files. No schema change or migration required. The publisher/adopter guards are acquired in numeric order to avoid cross-tenant lock inversion. Security revocation and exact dependency admission remain enforced by the existing #64 guarded paths.

## Commit

`5688cfffd70edb0d268c0d85445a4efa5bb4dac0` (`Enforce publisher custody during public adoption`). The report is ignored by the repository's dotfile rule, so it remains at the requested worktree path and was not included in the source commit. Post-commit repository/service targeted tests and `git diff --check` passed; the worktree has no tracked changes.

## Review fix R1 (T2-R1-2 / T2-R1-3)

**Fix base:** `5688cfffd70edb0d268c0d85445a4efa5bb4dac0`

- T2-R1-2: added `publicListingEligibilityQuery(*gorm.DB)` and routed `IsPublicListingDiscoverable`, `ListPublicCatalog`, and transaction-local `IntroduceRelease` through the same verified Publisher + listed source + listed Public Listing + current Release predicates. Tenant guard ordering is unchanged.
- T2-R1-3: custody tests now re-read the persisted Adoption and assert active state and accepted introduced-release pointer; they also verify public listing/release IDs, digest, byte-for-byte bundle, public Release PublisherTenantID, and SubmissionID lineage remain preserved.
- T2-R1-1 was ruled not applicable by the controller: repository API search found no supported public Listing state writer. No public unlist API was added.

Characterization command before query-builder refactor:

`go test ./internal/application/repository -run 'TestPublicMarketplace(EligibilityPredicatesStayConsistent|CustodyDeniesNewIntroductionAfterPublisherRevocation|CustodyDeniesNewIntroductionAfterSourceUnlist)' -count=1` — PASS. The duplicated old clauses agreed behaviorally; R1-2 was a structural drift-risk finding, so this was a characterization pass rather than a behavior RED.

Exact requested fix verification:

- `go test ./internal/application/repository -run 'TestPublicMarketplace.*(Custody|Discoverable|Introduce|Revok)' -count=10` — PASS (`ok`, 3.805s).
- `go test ./internal/application/service -run '^TestPublicMarketplace' -count=1` — PASS (`ok`, 3.834s).
- `git diff --check` — PASS (exit 0, no output).
- `go build ./...` — PASS (exit 0); linker emitted only duplicate `-lc++` library warnings for `cmd/server` and `cmd/desktop`.

**Review-fix commit:** `e7c42c51bc31f0215f3f4f27850246ca55e773ba` (`Unify public marketplace custody eligibility`). Committed only the two owned files. Post-commit `git status --short` was empty.

## Independent focused fix validation evidence (verbatim)

Source: `.worktrees/issue30-b6-coordination/.superpowers/sdd/plan-t65-task2-review-fix-r1/task-1-validation-report.md`.

1. `git rev-parse HEAD && git status --short` — PASS; HEAD matched required revision and status was empty.
2. `go test ./internal/application/repository -run '^(TestPublicMarketplaceEligibilityPredicatesStayConsistent|TestPublicMarketplaceCustodyDeniesNewIntroductionAfterPublisherRevocation|TestPublicMarketplaceCustodyDeniesNewIntroductionAfterSourceUnlist)$' -count=1` — PASS; `ok github.com/Tencent/WeKnora/internal/application/repository 0.699s`.
3. `git diff --check` — PASS; exit code 0.
4. `git rev-parse HEAD && git diff --check; ... && git status --short` — PASS; required HEAD unchanged, diff check exit 0, status empty.

## Controller rerun: exact Task2 fix checks on `e7c42c51bc31f0215f3f4f27850246ca55e773ba`

- `go test ./internal/application/repository -run 'TestPublicMarketplace.*(Custody|Discoverable|Introduce|Revok)' -count=10` — exit 0; `ok   github.com/Tencent/WeKnora/internal/application/repository 2.028s`
- `go test ./internal/application/service -run '^TestPublicMarketplace' -count=1` — exit 0; `ok   github.com/Tencent/WeKnora/internal/application/service 4.505s`
- `git diff --check` — exit 0; no output
- `go build ./...` — exit 0; raw output:

```text
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
```
