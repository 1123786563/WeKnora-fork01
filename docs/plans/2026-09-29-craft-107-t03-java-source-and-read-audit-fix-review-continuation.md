# Craft #107 T03 repair checkpoint — independent continuation review

Reviewed 2026-09-29. Scope: the four uncommitted files frozen in the integration ledger, § “Recovery after user interruption.” The SHA-256 values matched before review: `input_code.go` `faa1c86cb6e467b39d8a866f5146147a8d902eccbda4837ad8ab1b169d9cb871`; `input_code_test.go` `bc5149458ab5cfbb41640577d25cc170efff6177e5bed25f9134ff08efcfb2d9`; `craft_delegate.go` `ef704ab689dd7c9d05bd6d813811c2a25ff3b58d22034e0862207db269abba92`; `craft_execution_policy_wiring_test.go` `49e2a6ca5b8c5d316f11cad89a74b0635e7e1f74627aa9471fe0304fb250fdbd`. The implementation report remained absent at review time, so its claims could not be considered. No OCR or tests were run for this review.

Authority: approved `docs/specs/2026-09-23-craft-web-artifact-spec.md` (uploaded code is read-only input and actual sources are recorded), T03/#122 acceptance in `docs/plans/2026-09-23-craft-107-dag.md`, `CONTEXT.md`, ADR-0013/0014, the assigned repair plan and brief, the recovery review, and the read-audit architecture note. The latter is guidance, not a replacement for the approved Spec. This review is limited to the T03 repair delta; production dispatch activation remains separately gated on T19/T20.

## Findings

### T03-FIX1-R1 — High — Required read audit can fail while dispatch remains allowed

- **Evidence / affected symbols:** `CraftMaterialPolicy.writeAuditRow` returns on a nil sink and swallows `AuditLogService.Log` errors (`internal/application/service/craft_delegate.go:374-402`). `ReviewExecution` writes each read row independently, then returns the unchanged allowed decision (`:409-424`). `CraftDelegateExecutionPolicy.review` returns nil for that decision (`internal/application/service/craft_execution_policy.go:225-239`), which permits normal exec to proceed (`internal/application/service/craft_docker_normal_exec.go:143-164`). The new SQLite test covers successful writes only.
- **Impact:** A missing or failing sink allows input reads with no durable evidence. A failure after the first of multiple rows leaves an incomplete read set. This violates the brief's durable read-audit acceptance and the Spec's actual-source evidence requirement.
- **Smallest defensible correction:** Make required preflight persistence return an error and stop dispatch on failure. Persist the complete canonical read set atomically, for example in one `accepted` row, and test nil/failing sinks plus a two-input failure through `ReviewNormalExec`.

### T03-FIX1-R2 — Medium — Reader operands are misclassified and distinct input identities are collapsed

- **Evidence / affected symbol:** `InputExecutionPolicy.readEvents` scans every nonflag token after argv[0] and deduplicates by `Input.Ref` (`internal/modules/craft/input_code.go:699-725`). For `grep -e inputs/<digest>/one.csv /workspace/app/generated.txt`, the input path is a pattern, yet it produces a read event. The attached file option `grep --file=inputs/<digest>/patterns.txt` reads the input but is skipped because its token starts with `-`. Two manifest files with different paths/digests but the same `Ref` produce one row; manifest validation does not require unique refs. The server-side path lookup is lexical and has no resolved alias evidence.
- **Impact:** Audit can report a source that was not opened or omit one that was named as a file operand, weakening provenance even when inserts succeed.
- **Smallest defensible correction:** Parse the admitted reader grammar, including option values and positional pattern rules; refuse ambiguous forms. Deduplicate by canonical admitted path and digest. Only attribute aliases when resolved-path evidence exists. Add focused `grep -e`, `grep -f`, repeated-ref, and alias cases.

### T03-FIX1-R3 — Medium — Preflight records successful reading before the process runs

- **Evidence / affected symbols:** `ReviewExecution` writes `craft.input.read` with `AuditOutcomeSuccess` during the policy gate (`internal/application/service/craft_delegate.go:415-418`). The normal executor invokes that gate before `prepareOrRecover`, provider create, claim, or start (`internal/application/service/craft_docker_normal_exec.go:143-167`). The new service test invokes `ReviewExecution` directly, with no provider result.
- **Impact:** Provider failure, cancellation, unknown outcome, and replay can leave success read facts for commands that never started or whose read was never established. This conflicts with the Spec's requirement to record actual sources.
- **Smallest defensible correction:** Record preflight as accepted intent, then write a separately identified completion observation only after sufficient provider and transport evidence. Do not assert byte-level reading from command success alone. Bind both facts to a durable activity/receipt identity and test failed/no-start, success, unknown, and replay paths.

## Verdict

- **Spec compliance: FAIL for the repair.** The direct `java --source 21 <uploaded source>` operand is screened, and an allowed generated Workspace source remains possible in the inspected policy path. The audit behavior does not yet establish accurate, durable input-read evidence (R1-R3). This checkpoint does not prove T19/T20 production dispatch coverage.
- **Code quality and security: FAIL.** One High and two Medium findings remain. The new tests cover Java parsing and happy-path direct audit insertion but miss sink failure, reader operand semantics, and observed execution outcomes. No further independent regression or concurrency finding was established in this scoped review.

No requirements, source, tests, remote issues, or OCR state were changed by this review.
