# Craft #107 Java Policy Fix 4 Report

## Scope

- Source: independent Fix3 review found shell Java requests skip the fail-closed guard for `InputExecutionRequest.Environment[JDK_JAVA_OPTIONS]`.
- Approved behavior: `docs/specs/2026-09-23-craft-web-artifact-spec.md`, User Story 7 and uploaded-code execution decision. Prior policy rationale: `docs/plans/2026-09-29-craft-107-java-policy-fix3-report.md`.
- Plan/brief: `docs/plans/2026-09-29-craft-107-java-policy-fix4-plan.md` and `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix4/task-1-brief.md`.
- HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no staging or commits.

## Change

The shell branch now rejects non-empty `JDK_JAVA_OPTIONS` from the request environment whenever normalized shell tokens identify Java. Added the exact generated-target/generated-argfile regression requested by review, plus a control showing a shell `java -jar` remains allowed without that environment option. No other behavior changed.

## TDD and verification

- RED: `go test ./internal/modules/craft -run 'TestInputCodePolicyDeniesJavaArgFilesAndJDKOptions/shell_generated_JDK_argfile_environment' -count=1` — FAIL as expected; the request was allowed before the guard.
- GREEN/focused: `go test ./internal/modules/craft -run 'TestInputCodePolicy(DeniesJavaArgFilesAndJDKOptions|AllowsBenignJavaArgFilesAndLaunchers)' -count=1` — PASS (`ok`, 0.981s).
- Full policy regression: `go test ./internal/modules/craft -run 'TestInputCodePolicy' -count=1` — PASS (`ok`, 0.782s).
- `git diff --check` — PASS.
- No Docker tests or OCR were run. Parent retains independent review/validation.

## Incremental package and hashes

- Package: `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix4/review-package-01.tar.gz`
- Package SHA-256: `547527ef489d4fdd166229c4c3db1637619f732addb34dec7784a25355f13dad`
- Before bodies equal Fix3 checkpoint hashes in `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix4/baseline-hashes.sha256`; patch contains only Fix4 changes.
- After SHA-256:
  - `internal/modules/craft/input_code.go`: `c2c8a860d155a2e07057f566439fdf09071d78df488a1872dfffec21b3fe248c`
  - `internal/modules/craft/input_code_round2_ocr_test.go`: `af9b5f1ea12d8fa75f8258e28dc07465cdbba9a4aa90be83511839db1ad1b4f6`

## Remaining limits

The broader Fix3 compatibility decision remains: Java argfiles and non-empty `JDK_JAVA_OPTIONS` are refused because their contents can inject launcher behavior that this policy boundary cannot inspect.
