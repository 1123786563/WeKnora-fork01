# Craft #107 T03 Java source and read-audit repair — independent review

Reviewed the frozen four-file checkpoint on 2026-09-29 against the approved web-artifact Spec (uploaded code remains read-only data and is never directly executed), `CONTEXT.md`, T03/#122 acceptance in the DAG, the recovery finding, and the assigned repair plan. ADR-0013 describes server-owned preflight but adds no Java launcher or Craft audit exception. This is a source and targeted-test review, not OCR coverage. The four supplied SHA-256 hashes matched at review start and end.

## Findings

### T03-FIX-R1 — High — An allowed read can proceed without a durable audit row

- **Evidence / symbol:** `CraftMaterialPolicy.writeAuditRow` returns when its audit sink is nil and logs then swallows `AuditLogService.Log` errors (`internal/application/service/craft_delegate.go:374–402`). `ReviewExecution` still returns `Allowed` after calling it for every `ReadEvent` (`:409–424`); `CraftDelegateExecutionPolicy.review` returns nil on that allowed decision (`internal/application/service/craft_execution_policy.go:230–239`), and normal exec then proceeds to send (`internal/application/service/craft_docker_normal_exec.go:153–167`). The new SQLite test (`craft_execution_policy_wiring_test.go:184–223`) covers successful inserts only.
- **Impact:** A missing sink, closed database, or failed insert permits an input read with no durable `craft.input.read` evidence. With multiple inputs, an insert failure can also leave a partial set of rows. The repair plan's durable per-input audit acceptance is not met under failure.
- **Smallest defensible correction:** Make the read-audit write result explicit at the gate and fail closed before dispatch when required persistence fails; persist the event set atomically or record a durable command event that identifies the complete read set. Add a failing-sink and multi-row failure regression through `ReviewNormalExec`.

### T03-FIX-R2 — Medium — Read-event extraction mistakes operands and collapses distinct inputs

- **Evidence / symbol:** `InputExecutionPolicy.readEvents` treats every nonflag argument after argv[0] as a read path and deduplicates by `Input.Ref` (`internal/modules/craft/input_code.go:699–725`). Thus `grep -e inputs/<digest>/one.csv /workspace/app/generated.txt` records `one.csv` as read although it is only the search pattern. Conversely, two admitted paths sharing a source ref but having different digest/name produce one event; `ValidateInputManifest` requires a nonempty ref but does not require unique refs (`internal/modules/craft/input.go:76–105`). A symlink alias outside the tree resolving to an admitted input is also not matched by the lexical path lookup.
- **Impact:** Audit rows can falsely assert input use or omit a distinct admitted file read. This weakens source and digest provenance even when database persistence succeeds.
- **Smallest defensible correction:** Parse file operands for each allowed reader (or admit only readers with unambiguous operand forms), key deduplication by canonical admitted input identity, and use resolved path evidence for aliases where available. Test grep pattern versus file, repeated refs with distinct digests, and alias behavior.

### T03-FIX-R3 — Medium — A preflight permission is recorded as a successful read before execution

- **Evidence / symbol:** `ReviewExecution` writes `AuditOutcomeSuccess` for read events in the policy gate (`internal/application/service/craft_delegate.go:415–418`); normal exec calls that gate before create/send (`internal/application/service/craft_docker_normal_exec.go:144–167`). The provider can subsequently fail or never run. The new service test invokes `MaterialPolicy.ReviewExecution` directly (`craft_execution_policy_wiring_test.go:202–223`), so it does not check a completed normal exec.
- **Impact:** A rejected/failed provider operation can leave `craft.input.read` with success outcome even though no read occurred. Retries can add success rows for attempts that produced no read.
- **Smallest defensible correction:** At preflight record an attempted/allowed observation, or defer the success read event until the observed command outcome is successful. Cover success, provider failure, and replay at the normal exec seam.

## Checks and verdicts

- **Java repair:** The exact `java --source 21 /workspace/inputs/<digest>/Hello.java` case is denied after `--source` consumes its version; a generated Workspace source remains allowed. Unknown Java options and missing modeled values fail closed. The four new focused module tests passed with `go test ./internal/modules/craft/... -run 'TestInputCodePolicy(ScreensJavaSourceAfterSourceVersion|FailsClosedForUnmodeledJavaOptionArity|AuditsExactInputsReadByReadOnlyCommand|DoesNotCallNonInputReadOnlyCommandGeneratedExecution)' -count=1` (exit 0). This does not establish a full Java option grammar or production dispatch coverage.
- **Spec compliance: FAIL for the repair's durable, accurate read-audit acceptance.** The Java source bypass is closed for the tested command, but R1–R3 prevent treating the audit evidence as a faithful record of input reads. The default-off T19/T20 production dispatch route remains the explicit integration blocker described in the brief.
- **Code quality/security: FAIL.** One high and two medium findings require correction or an explicit requirements ruling. Existing tests verify happy-path module decisions and direct SQLite insertion, not audit failure, operand semantics, or actual normal-exec outcomes. No additional concurrency finding was established from this delta.

