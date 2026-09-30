# Craft #107 Java Policy Fix 2 Plan

Source: independent review of Fix1 identifying a valid HIGH module-path bypass. Governing behavior is approved `docs/specs/2026-09-23-craft-web-artifact-spec.md` User Story 7 and the decision that uploaded code is never directly executed. Prior accepted Java hardening is in `docs/plans/2026-09-29-craft-107-java-policy-fix1-report.md`; issue context is `docs/plans/2026-09-23-craft-107-ledger.md`. Java option grammar was cross-checked against Oracle Java 21 launcher documentation: `--module-path`/`-p`, `--upgrade-module-path`, `--patch-module`.

## Task 1 — Screen Java module and patch paths

- **Depends on:** Fix1, specifically validated interpreter offset, wrapper-aware Java detection and colon-list path screening.
- **Owner:** backend implementer; parent retains independent validation/review.
- **Owned files:** `internal/modules/craft/input_code.go`, `internal/modules/craft/input_code_round2_ocr_test.go`, this plan and `docs/plans/2026-09-29-craft-107-java-policy-fix2-report.md`.
- **Consumes:** Java-specific flag state handling and `javaClasspathInputEntry`; wrapper-resolved interpreter identity.
- **Produces:** Denials when an uploaded input path occurs in separated or attached `--module-path`/`-p`, `--upgrade-module-path`, and `--patch-module module=path[:path]` values; safe generated module paths remain allowed. Wrapper forms receive identical Java handling. Existing `-cp`, `-classpath`, `--class-path`, `-ea:` and non-Java behavior stay intact.
- **TDD:** Add explicit RED tests first, then implementation. Malformed or input-bearing module path values must fail closed; benign generated values remain allowed.
- **Verification:** `go test ./internal/modules/craft -run 'TestInputCodePolicy(.*Java|.*InterpreterLauncherFlags|StillDeniesProgramTextFlags)' -count=1`; `go test ./internal/modules/craft -run 'TestInputCodePolicy' -count=1`; `git diff --check`.
- **Failure handling:** Do not broaden exemptions for arbitrary Java flags. No OCR rerun (provider quota-blocked), commit or staging.
