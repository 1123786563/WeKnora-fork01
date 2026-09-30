# Craft #107 Java Policy Fix 1 Report

## Scope and evidence

- Source finding: `docs/plans/craft-107-ocr-final-3.md`, HIGH at `input_code.go:418-421` (`--class-path` bypass) and MEDIUM at `input_code.go:939-945` (`-ea:` and wrapper interpreter offset mismatch).
- Acceptance basis: approved `docs/specs/2026-09-23-craft-web-artifact-spec.md`, User Story 7 and Implementation Decisions requiring uploaded code to remain read-only and never be directly executed.
- SDD plan/brief: `docs/plans/2026-09-29-craft-107-java-policy-fix1-plan.md` and `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix1/task-1-brief.md`.
- Workspace HEAD stayed `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no staging or commits.

## Changes

- Java `--class-path <list>` and `--class-path=<list>` now screen each colon-delimited path entry against the read-only inputs tree.
- Interpreter option classification now uses the interpreter located by `InterpreterPrefixStatus`'s already-validated offset, so `timeout ... java` and `env ... java` receive Java-specific handling consistently.
- Java `-ea:<package>` is treated as the documented benign assertion launcher flag. Existing `-cp`/`-classpath` filtering, generated-path allow behavior, and code-bearing program-text denial remain covered.
- Added focused tests for separated/attached long classpaths, both wrapper forms, and package-scoped assertion flags.

## TDD and verification evidence

- RED: `go test ./internal/modules/craft -run 'TestInputCodePolicy(ScreensLongJavaClasspathEntries|UsesDetectedJavaInterpreterAfterWrappers)' -count=1` — failed as expected. Both long-classpath forms and wrapped classpaths were allowed; wrapped `-ea:com.acme...` was denied.
- GREEN/focused: `go test ./internal/modules/craft -run 'TestInputCodePolicy(ScreensLongJavaClasspathEntries|UsesDetectedJavaInterpreterAfterWrappers|ScreensEveryJavaClasspathEntry|AllowsInterpreterLauncherFlags|StillDeniesProgramTextFlags|DeniesOptionBearingWrappers)' -count=1` — PASS (`ok`, 1.067s).
- Broader policy regression: `go test ./internal/modules/craft -run 'TestInputCodePolicy' -count=1` — PASS (`ok`, 0.795s).
- `git diff --check` — PASS.
- No Docker tests or OCR were run; OCR provider was quota-blocked as directed. Parent retains independent review and validation.

## Immutable incremental review package

- Package: `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix1/review-package-01.tar.gz`
- SHA-256: `d8a622527307e03e4464fcd6eecf5bc0f12f05018774fa272890779ccd08e245`
- Contains exact before/after source and regression test bodies, unified `fix1.patch`, and SHA-256 manifest.
- Before hashes are recorded in `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix1/baseline-hashes.sha256`.
- After SHA-256:
  - `internal/modules/craft/input_code.go`: `5f2d9c86579c0c8831bf0c12e463844793015b4b14cb15bc834b2fb8b83c0e16`
  - `internal/modules/craft/input_code_round2_ocr_test.go`: `cc669f901b9ebcd10d5df95b39df4c47d46225545a1e95b6c08ba54a6fa185f4`

## Limitations

The full Craft acceptance remains subject to independent review and runtime integration checks. This repair only closes the two scoped Java policy findings; it does not change broader T14 runtime acceptance or the OCR provider quota block.
