# T19 normal-output Task2a Fix1 Task1 independent review

Date: 2026-09-24. Scope: Fix1 incremental change to the ten owned repository, service, provider and SQLite124/PG203 migration files. Reviewed against the approved Craft #107 artifact spec, `CONTEXT.md`, T19 architecture, Fix1 plan and the preceding Task2a independent review. No source, test, requirement, issue or OCR change.

## Verdict

- **Scoped spec compliance: PASS.** F1 (unobservable/irrecoverable failed seal) and F2 (truncated exact-sequence replay) are closed for this checkpoint. The provider receives `Seal() error`, treats failure as partial and returns the cause. A repository operation is create-only for writer admission, so a process restart cannot reopen the same receipt after an uncertain cutoff. Exact original-input length and SHA-256 drive replay for both prefix truncation and zero-byte stored prefix.
- **Scoped code quality: PASS.** No new blocking finding in the ten-file Fix1 delta. The repository still serializes append and seal on the operation row; the service closes local admission and cancels active append contexts before attempting its bounded durable seal, and a later seal call can retry. Cursor reads use a separate read-only service path and do not mint a writer.
- **Broader completion: pending external gates.** PostgreSQL migration/runtime and DB-lock races were not exercised because `TRPC_TEST_POSTGRES_DSN` is unset. The ignored migration files must be deliberately included in final integration; no live Docker or production routing is in this task's scope.

## Evidence

- Verified all ten current file SHA-256 values against Fix1 postimage. In an isolated temporary directory, reverse-applied the incremental patch to those ten files and obtained every recorded preimage SHA-256 (eight Task2a store postimages plus two provider Fix3 postimages); forward-applying it reproduced every Fix1 postimage. Patch SHA-256 also matches the checkpoint. Both SQLite124/PG203 SQL pairs are present in this verification despite `.gitignore` excluding `migrations/`.
- Recorded final focused `go test` and `go test -race` runs pass for provider, repository and service. The recorded SQLite-only migration up/down/up x2 probe passes and includes receipt validation and original-input metadata constraints. I reviewed this evidence; I did not repeat the controller's OCR run.
- `internal/modules/execution/sandbox/docker_normal_exec.go:349-372`: `freezeAndSeal()` error prevents `TransportComplete`, yields `TransportPartial`, and returns an error. The attach path remains one physical start without a new retry.
- `internal/application/service/craft_docker_output.go:91-123`: `sealMu` serializes seal attempts, local writes close, active contexts are cancelled, a failed DB seal is returned, and a subsequent bounded seal can retry. `ReadAfter` remains available independently at lines 143-156.
- `internal/application/repository/craft_docker_output.go:132-141`: duplicate `Open` returns conflict rather than reopening a writer. The append transaction records original input metadata and a zero-length stored BLOB as a real sequence at lines 231-262; `outputReplayMatches` compares original stream/length/digest at lines 368-372.
- New focused tests cover provider seal failure classification, failed seal followed by rejected writer reopen and successful cursor read, local seal retry, prefix and zero-byte quota replay, and divergent input conflict.

## Findings

No new critical, high or medium findings in the reviewed Fix1 delta.

**Validation limitation (non-finding):** The failed-seal service test injects a database error rather than an actual DB lock timeout, and the blocked append test uses a cooperative mock. The real PostgreSQL serialization/cancellation path remains unverified until the configured PostgreSQL run. This limits the evidence for the timing guarantee but does not reveal a contradictory code path in this scoped review. Before a full delivery verdict, run the PostgreSQL migration and focused/race suite on the final checkpoint and confirm all four ignored migration SQL files are included in the final diff.
