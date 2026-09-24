# Craft #107 T19 Docker send coordinator S2 Fix 1 — independent review

Reviewed 2026-09-24, read-only except this report. Sources: approved Craft web artifact spec (Task Budget and Run recovery), `CONTEXT.md`, ADR-0004, Docker exec recovery design, S2 and Fix 1 plans, prior S2 FAIL review, Fix 1 report, task-local patch, exact checkpoint, and the nine changed source/migration files. No OCR or provider call was run.

## Findings

No new blocking defect found in the scoped S2 Fix 1 checkpoint.

### S2-R3 — Medium, remaining S3 acceptance gate

- **Evidence:** `DockerSendPermission.Consume` in `internal/application/service/craft_docker_send_coordinator.go` returns a process-local receipt, while `TestCraftDockerSendCoordinatorClaimIsOneOwnerAndReplayCannotResend` counts token consumption. This slice has no Docker transport call or send callback. The Fix 1 report expressly leaves physical `ExecStart`, cancellation, and ambiguous-response proof to S3.
- **Impact:** The durable one-owner claim is verified here, but the physical one-send and bounded transport behavior required by the Docker recovery design and S2 plan cannot yet be accepted for T19.
- **Smallest defensible correction:** At the S3 adapter seam, run the claimed permission through the actual or fake physical send path under concurrent replay, crash, cancellation, and ambiguous response; assert at most one `ExecStart` and retained unresolved hold/fence. Include real-engine behavior before production wiring.

## S2-R1 disposition: closed for this checkpoint

`craft_budget.go:340-414` inserts the nullable `protocol='docker_coordinator'` marker in the same transaction as the journal intent and dispatched reservation, before `Prepare` returns for inert create. Existing rows remain `NULL`; both new and replayed coordinator operations require the marker (`craft_docker_send_coordinator.go:88-126`), as do bound recovery, observation, receipt bind, and claim. Legacy `BeginBinding`/`StartBinding` prepare only unmarked rows and encounter the unique activity key on a coordinator-owned row. Their resolution CAS requires `protocol IS NULL` and no provider, container, exec, or send claim (`craft_budget.go:222-239`). Thus an outstanding unmarked legacy callback cannot be adopted, while a stale legacy handle cannot resolve a marked row before or after claim. The cross-path regression exercises both orderings and checks the dispatched hold and `intent` fence (`craft_docker_send_coordinator_test.go:207-270`).

The mixed-version argument holds for this row protocol: an old binary can create only an unmarked row, which the new coordinator refuses; a marked row created first prevents the old prepare from reaching its callback through the existing unique key. An old callback already in flight has only its own unmarked row, which the coordinator refuses. Deploy the additive migrations before the new binary; the old binary cannot read the new column until that schema is applied. The paired PostgreSQL `000197` and SQLite `000118` migrations add a nullable, constrained column; their down migrations remove it. Migration tests check preservation of an unmarked row and rejection of an unrecognized value on both engines.

Repository bind and claim still require an executable current Run revision/status within a Run-row lock transaction and exact receipt CAS (`craft_docker_send_claim.go:49-132`). Replay after claim returns no permission. A failed or stale bind/claim does not clear the existing `intent` or dispatched reservation. The repository test covers concurrent owner selection, a competing Run pause, stale epoch/status, receipt mismatch, and retained hold. No SQL transaction encloses provider I/O in this slice.

## Evidence and limits

- All nine current file SHA-256 values and the task-local patch hash match `2026-09-24-craft-107-t19-docker-send-coordinator-s2-fix1-checkpoint.json`; HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`. The patch is based on the recorded S2 postimage.
- Independent rerun: `go test ./internal/application/service -run '^TestCraftDockerSendCoordinator(RejectsLegacyIntentAdoption|ClaimIsOneOwnerAndReplayCannotResend)$' -count=1` passed; `go test ./internal/application/repository -run '^TestCraftDockerSend(Claim|Protocol)' -count=1` passed; `git diff --check` produced no output.
- The Fix 1 report records disposable PostgreSQL **17.11** repository and coordinator test passes with the supported `app.skip_embedding=true` migration switch, including concurrent claim and competing Run pause. The tests are present and select PostgreSQL only with `TRPC_TEST_POSTGRES_DSN`; that DSN was unavailable in this reviewer session, so the PG run was assessed from its recorded checkpoint evidence rather than independently repeated. My repository rerun exercised SQLite and skipped the PG subtests. The PG service test proves protocol preparation/legacy exclusion; the repository PG tests prove claim and Run-lock behavior.
- No production Docker transport or runtime wiring is included. This report does not claim full T19 acceptance or resolve S2-R3.

## Verdict

**Scoped S2 Fix 1 spec compliance: PASS.** The S2-R1 cross-protocol safety failure is closed by durable ownership and conditional legacy resolution; S2-R2 has recorded PostgreSQL 17.11 migration/race evidence. **Scoped code quality: PASS.** The patch is narrow, the database guards align with the recovery protocol, and focused tests pass. **Full T19: not accepted yet.** S2-R3 physical transport proof and later production integration/recovery gates remain.
