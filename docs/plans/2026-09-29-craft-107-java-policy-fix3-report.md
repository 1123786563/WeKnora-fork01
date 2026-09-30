# Craft #107 Java Policy Fix 3 Report

## Scope and decision

- Independent Fix2 review identified the HIGH bypass `java @/workspace/inputs/launch.args`: the launcher expands argfiles before normal option parsing.
- Approved behavior: `docs/specs/2026-09-23-craft-web-artifact-spec.md`, User Story 7 and the implementation decision that uploaded code remains read-only.
- Java launcher review confirmed `JDK_JAVA_OPTIONS` can also inject launcher options and argfiles. Because this policy seam cannot read an argfile's contents, the fail-closed decision (confirmed by parent) is to deny Java `@argfile` tokens regardless of file location and deny non-empty `JDK_JAVA_OPTIONS` when Java is the detected launcher.
- Compatibility cost: generated/outside-input Java argfiles and non-empty `JDK_JAVA_OPTIONS` are refused. Ordinary launcher options remain supported, including `-jar`, generated `--module-path` plus `-m`, and existing classpath/module/patch path behavior.
- Plan/brief: `docs/plans/2026-09-29-craft-107-java-policy-fix3-plan.md` and `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix3/task-1-brief.md`.
- HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no staging or commits.

## Changes

- For detected Java launchers, reject `@argfile` tokens in argv regardless of path, including timeout/env wrappers.
- Reject non-empty `JDK_JAVA_OPTIONS` from the process environment or an `env` wrapper assignment when Java is detected.
- For shell expressions, use normalized `shellTokens` to identify Java and quoted argfile tokens; inspect raw assignment fields as well because `shellTokens` intentionally strips `VAR=` prefixes.
- Added tests for absolute and relative input argfiles, wrapped launchers, wrapper/environment `JDK_JAVA_OPTIONS`, quoted shell argfile tokens, generated argfile fail-closed behavior, and ordinary launcher compatibility.

## TDD and verification

- RED: `go test ./internal/modules/craft -run 'TestInputCodePolicy(DeniesJavaArgFilesFromInputs|AllowsBenignJavaArgFilesAndLaunchers)' -count=1` — initially failed for direct absolute/relative argv argfiles and timeout-wrapped input argfiles (exit 1; 1.698s).
- RED follow-up: `go test ./internal/modules/craft -run 'TestInputCodePolicyDeniesJavaArgFilesFromInputs/shell_JDK_option_assignment' -count=1` — failed because quote-normalized shell tokenization had removed the environment assignment key.
- GREEN/focused: `go test ./internal/modules/craft -run 'TestInputCodePolicy(DeniesJavaArgFilesAndJDKOptions|AllowsBenignJavaArgFilesAndLaunchers)' -count=1` — PASS after the assignment scanner change (the renamed test is included in the final focused run below).
- Focused Java/argfile regression: `go test ./internal/modules/craft -run 'TestInputCodePolicy(.*Java|.*ArgFile|.*JDKOptions)' -count=1` — PASS (`ok`, 0.520s).
- Full input policy regressions: `go test ./internal/modules/craft -run 'TestInputCodePolicy' -count=1` — PASS (`ok`, 0.895s).
- `git diff --check` — PASS.
- No Docker tests or OCR were run. Parent retains independent review/validation.

## Incremental package and hashes

- Package: `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix3/review-package-02.tar.gz`
- Package SHA-256: `82ddb0a06b157f15294adb1bc7b5cf2dd15a59d49203c05d4d0e89544a1326cc`
- Package before bodies equal Fix2 checkpoint hashes in `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix3/baseline-hashes.sha256`; patch contains only Fix3 delta.
- After SHA-256:
  - `internal/modules/craft/input_code.go`: `ccac08b68f2ec09116ec75b0c7ce7ab6e16daaf29cc67c8bd907eb9a89cf02fe`
  - `internal/modules/craft/input_code_round2_ocr_test.go`: `2bb8e59f40410097390c9863b9697aeb82622394e2034f7d4d710127ad579917`

## Remaining limits

The policy denies all Java argfiles rather than parsing their contents. This intentionally trades argfile compatibility for a reviewable fail-closed boundary. Full Craft acceptance remains subject to parent review/validation.
