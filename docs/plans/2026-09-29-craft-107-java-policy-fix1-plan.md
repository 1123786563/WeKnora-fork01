# Craft #107 Java Policy Fix 1 Plan

Source: [OCR Round 3](craft-107-ocr-final-3.md), valid HIGH finding for Java `--class-path` and MEDIUM finding for `-ea:` / wrapper interpreter detection. Governing behavior: [approved Craft Web Artifact Spec](../specs/2026-09-23-craft-web-artifact-spec.md), especially user story 7 and the decision that uploaded code is never directly executed. Prior policy context and integration state: [Craft #107 Ledger](2026-09-23-craft-107-ledger.md).

## Task 1 — Close Java classpath and wrapper flag gaps

- **Depends on:** OCR Round 3; current T03 input execution policy and its integrated prior Round 2 hardening.
- **Owner:** backend implementer; **validator/reviewer:** parent-assigned independent roles.
- **Owned files:** `internal/modules/craft/input_code.go`, `internal/modules/craft/input_code_round2_ocr_test.go`, this plan, `docs/plans/2026-09-29-craft-107-java-policy-fix1-report.md`. Do not modify other integration changes.
- **Consumes:** `InterpreterPrefixStatus` tri-state and its exact operand offset; `javaClasspathInputEntry`; existing deny/allow semantics.
- **Produces:** Java separated `--class-path <list>` and attached `--class-path=<list>` paths are split and checked entry-by-entry; Java launcher flags including `-ea:<package>` remain allowed; wrapper forms `timeout ... java` and `env ... java` use the detected interpreter; `-cp`/`-classpath` checks remain; code-bearing flags remain denied.
- **TDD:** Add focused regression cases first and show RED, then implement the smallest change and show GREEN.
- **Verification:** `go test ./internal/modules/craft -run 'TestInputCodePolicy(.*Java|.*InterpreterLauncherFlags|StillDeniesProgramTextFlags|DeniesOptionBearingWrappers)' -count=1`; `git diff --check`.
- **Failure handling:** Keep failures fail-closed; do not broaden benign flag exemptions or weaken code-bearing denial. No Docker/OCR rerun, commit, or staging.
