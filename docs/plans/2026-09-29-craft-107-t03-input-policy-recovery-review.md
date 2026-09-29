# Craft #107 T03 input execution policy — independent recovery review

Reviewed 2026-09-29 in the integration worktree. Read-only source review; no tests or OCR were run. This is supplementary review evidence, **not OCR coverage**. Facts used: approved `docs/specs/2026-09-23-craft-web-artifact-spec.md` (lines 61, 75, 87), `CONTEXT.md`, T03 acceptance in `docs/plans/2026-09-23-craft-107-dag.md` (lines 121–139), the current source and tests, and historical findings in `docs/plans/craft-107-ocr-t03.md`. No Craft-specific ADR was found under `docs/adr/`; the existing ADRs do not add a T03 launcher rule.

## Findings

### High — Java `--source` option value hides the actual uploaded program

- **Evidence:** `InputExecutionPolicy.Review` recognizes only a small set of Java options that consume a following value (`internal/modules/craft/input_code.go:426–440, 464–491`). For `[]string{"java", "--source", "21", "/workspace/inputs/Hello.java"}`, `--source` is skipped as a flag, `21` is marked as the first script at line 511, and the uploaded path is then treated as post-script data and skipped at lines 566–568. No target path or digest is supplied by the normal Docker adapter (`internal/application/service/craft_execution_policy.go:115–148`), so the policy returns `Allowed` and `craft.generated.execute` at line 615. Oracle's [Java launcher documentation](https://docs.oracle.com/en/java/javase/21/docs/specs/man/java.html) specifies that `--source` consumes a version and that the later source file is compiled and run. The new Java tests cover class/module paths and argfiles, but not `--source` or other value-taking launcher options.
- **Impact:** One direct command executes uploaded source bytes from the read-only tree, violating the approved no-direct-execution requirement and misclassifying the allowed audit event. This is a separate path from the documented copy-then-execute residual.
- **Smallest correction:** Model `--source` as a value-taking Java launcher option before choosing the script operand, then screen the subsequent source file. For any unsupported Java option whose arity can change that position, fail closed instead of treating the next nonflag token as the program. Add a regression for the exact command and a generated source-file counterpart.

### Medium — Allowed input reads are audited as generated execution

- **Evidence:** `readOnlyCommands` deliberately permits `cat /workspace/inputs/file` (`internal/modules/craft/input_code.go:626–642`), but every allowed request returns `AuditKindGeneratedExecute` (`:615`). `CraftMaterialPolicy.ReviewExecution` persists that kind as a success (`internal/application/service/craft_delegate.go:405–418`). `AuditInputRead` is a separate explicit method (`:422–431`); the normal exec adapter calls only `ReviewExecution` (`internal/application/service/craft_execution_policy.go:235`).
- **Impact:** A command that only reads uploaded data creates a `craft.generated.execute` audit row, weakening the T03 acceptance that audit evidence distinguish input reads from generated code execution. The row has no resolved generated target, so it cannot substantiate the claimed action.
- **Smallest correction:** Classify a permitted read-only command as `craft.input.read` (with its matched input identity) or use a distinct neutral command audit kind; reserve `craft.generated.execute` for an execution target whose generated provenance was actually established. Cover the normal exec audit path with a `cat inputs/...` behavioral test.

### Medium — The named production command-face assembly is not called by production code

- **Evidence:** `newCraftDockerExecCommandFaces` attaches the policy to normal and restricted services (`internal/container/craft_exec_policy_wiring.go:89–125`), but repository references to this symbol are its declaration and `internal/container/craft_exec_policy_wiring_test.go:157`. Searches of non-test code find the normal/restricted constructors only inside that unused assembly. The services now fail closed when no policy is attached (`internal/application/service/craft_docker_normal_exec.go:147–152`; `craft_docker_restricted_exec.go:106–112, 135–143`).
- **Impact:** The old “unwired gate” finding is only partly resolved: fail-closed guards prevent silent execution through these services, but the supposedly production assembly does not establish a reachable, functioning command face. T03 end-to-end acceptance and real audit persistence are not demonstrated by this wiring.
- **Smallest correction:** Connect the assembly to the actual container/runtime registration point and test that the runtime-resolved services carry the gate and audit sink. If these command faces intentionally remain dormant, document that as an integration blocker instead of treating construction in a test as production wiring.

## Historical OCR finding check

The historical report is context, not authority. Its wrapper `env` assignment, colon-list environment, inline program flags, `rg`/pager read-only exemptions, blank refusal text, orphan audit row, run lookup error, and restricted resume concerns have corresponding guards in the current source (`input_code.go:110–119, 265–274, 344–352, 442–463, 626–642`; `craft_execution_policy.go:77–81, 197–202`; `craft_docker_restricted_exec.go:135–143`). The former `-c` script-argument false positive is addressed by option-region handling in the module policy. The production wiring concern remains only partly resolved as described above. This review does not certify all historic cases dynamically.

## Verdict

- **Spec compliance: FAIL for T03.** The Java `--source` command provides a direct uploaded-code execution path, and the audit distinction is inaccurate for permitted input reads.
- **Code quality/security: FAIL.** One high and two medium findings remain. Existing tests cover the added Java classpath/module/argfile shapes but miss the value-taking `--source` path and normal-exec read audit behavior. No concurrency or data consistency defect was identified in the reviewed delta. OCR coverage remains outstanding because the controller's OCR runs timed out; this report does not replace them.
