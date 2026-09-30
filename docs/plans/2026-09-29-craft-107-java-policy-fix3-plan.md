# Craft #107 Java Policy Fix 3 Plan

Source: independent Fix2 review finding: Java expands `@argfile` launcher tokens before parsing options, so an uploaded argument file can supply otherwise-screened module/class paths. Governing behavior is approved `docs/specs/2026-09-23-craft-web-artifact-spec.md` User Story 7 and its decision that uploaded code is never directly executed. Prior accepted work: `docs/plans/2026-09-29-craft-107-java-policy-fix1-report.md` and `...-fix2-report.md`; execution history: `docs/plans/2026-09-23-craft-107-ledger.md`.

## Task 1 — Reject Java argfiles located in the read-only input tree

- **Depends on:** Fix2 validated wrapper-aware Java argv scan and module/class path screening.
- **Owner:** backend implementer; parent retains independent review/validation.
- **Owned files:** `internal/modules/craft/input_code.go`, `internal/modules/craft/input_code_round2_ocr_test.go`, this plan and `docs/plans/2026-09-29-craft-107-java-policy-fix3-report.md`.
- **Consumes:** `InterpreterPrefixStatus`, `shellTokens`, canonical path containment and Java interpreter identity.
- **Produces:** Java `@file` references under admitted inputs are denied, including wrapped Java commands and shell token forms after quote removal; benign ordinary Java arguments and generated/outside-input argfiles remain allowed. Wrapper environment assignments exposing `@file` are screened when Java is the launcher. Non-Java behavior is unchanged.
- **TDD:** Add RED regressions for direct, wrapped, shell-quoted and benign Java forms before implementation.
- **Verification:** `go test ./internal/modules/craft -run 'TestInputCodePolicy(.*Java|.*ArgFile)' -count=1`; `go test ./internal/modules/craft -run 'TestInputCodePolicy' -count=1`; `git diff --check`.
- **Failure handling:** Fail closed only when a Java argfile token resolves under the inputs tree; do not reject ordinary launcher flags or argfiles outside it. No commit/staging or OCR rerun.
