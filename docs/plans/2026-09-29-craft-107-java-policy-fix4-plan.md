# Craft #107 Java Policy Fix 4 Plan

Source: independent Fix3 review found Shell Java requests do not apply the `JDK_JAVA_OPTIONS` check to `InputExecutionRequest.Environment`, allowing generated `@argfile` launcher injection. Governing behavior: approved `docs/specs/2026-09-23-craft-web-artifact-spec.md`, User Story 7 and uploaded-code execution decision. Prior fix chain: `docs/plans/2026-09-29-craft-107-java-policy-fix3-report.md`.

## Task 1 — Apply Java environment guard to shell requests

- **Depends on:** Fix3 environment/argfile policy.
- **Owner:** backend implementer; parent retains independent review/validation.
- **Owned files:** `internal/modules/craft/input_code.go`, `internal/modules/craft/input_code_round2_ocr_test.go`, this plan and report.
- **Produces:** A shell request whose normalized token stream contains Java is denied when `Environment[JDK_JAVA_OPTIONS]` is non-empty, matching argv Java behavior. Ordinary shell Java launch requests without the variable remain unchanged.
- **TDD:** Add the specified regression with Shell=true, Java token, generated resolved target/digest and generated `JDK_JAVA_OPTIONS=@...`; prove RED, implement, prove GREEN.
- **Verification:** focused regression; `go test ./internal/modules/craft -run 'TestInputCodePolicy' -count=1`; `git diff --check`.
- **Failure handling:** Do not modify additional environment variables or non-Java behavior. No commit/staging.
