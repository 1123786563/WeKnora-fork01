# Craft #107 Java Policy Fix 2 Report

## Scope and evidence

- Source: valid HIGH finding from independent review of Fix1: Java modular JAR/code can be made available through `--module-path`/`-p`, `--upgrade-module-path`, and `--patch-module` without the input execution policy screening path entries.
- Governing behavior: approved `docs/specs/2026-09-23-craft-web-artifact-spec.md`, User Story 7 and implementation decision that uploaded code is never directly executed.
- Java option syntax: Oracle Java 21 launcher docs for module path, upgrade module path and patch-module option.
- Plan/brief: `docs/plans/2026-09-29-craft-107-java-policy-fix2-plan.md` and `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix2/task-1-brief.md`.
- Worktree HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no staging or commits.

## Changes

- Screen colon-delimited entries for separated and attached `--module-path`, `-p`, and `--upgrade-module-path` options.
- Parse both `--patch-module module=path[:path]` and `--patch-module=module=path[:path]`; reject malformed selectors/path lists and deny if any entry resolves under the admitted inputs tree.
- Reuse the existing wrapper-aware interpreter identity for these checks.
- Permit Java's exact `-m` module-launch option, which is needed to preserve generated module execution. Uploaded paths remain denied at the module/patch path option. Existing Fix1 Java classpath and `-ea:` behavior is preserved.
- Added regressions for all forms, generated module paths, wrappers and patch-module paths.

## TDD and verification

- RED: `go test ./internal/modules/craft -run 'TestInputCodePolicy(ScreensJavaModulePathEntries|ScreensJavaPatchModulePathEntries|ScreensWrappedJavaModulePaths)' -count=1` — failed as expected: module/patch path input entries were allowed; legal generated module launch was refused on `-m`.
- GREEN/focused: `go test ./internal/modules/craft -run 'TestInputCodePolicy(.*Java|.*InterpreterLauncherFlags|StillDeniesProgramTextFlags)' -count=1` — PASS (`ok`, 1.900s).
- Broader policy regression: `go test ./internal/modules/craft -run 'TestInputCodePolicy' -count=1` — PASS (`ok`, 0.788s).
- `git diff --check` — PASS.
- No Docker tests or OCR were run. Parent performs independent review and validation.

## Incremental review package and hashes

- Package: `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix2/review-package-01.tar.gz`
- Package SHA-256: `6b1eac6a2d5717cea4392d6f01b979d25a0ac6159fbab25b0d66ff17364a8942`
- This package's `before/` bodies equal the Fix2 baseline (Fix1 checkpoint) frozen in `baseline-hashes.sha256`; `after/` bodies are the current complete files. `fix2.patch` therefore isolates only this round's additions.
- After SHA-256:
  - `internal/modules/craft/input_code.go`: `de3e0e35ef666cef316a0ef410c749f98ca8db2e5e2b124dbfc542e3c895bd7e`
  - `internal/modules/craft/input_code_round2_ocr_test.go`: `3bf433ed936ddf74e042ee09952822f1707d4470772c1aaa79667f5eb47e211d`

## Remaining limits

Only the module path/patch path execution boundary is addressed here. Full Craft acceptance and independent review remain pending.
