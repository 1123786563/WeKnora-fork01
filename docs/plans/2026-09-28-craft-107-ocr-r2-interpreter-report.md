# OCR R2 Task Report — F37 interpreter launcher flags

## Scope

Fixed only F37 from `docs/plans/craft-107-ocr-final-1.md` (`internal/modules/craft/input_code.go:894-899` in the finding). The approved source is `docs/specs/2026-09-23-craft-web-artifact-spec.md`, story 7: uploaded code remains input material even when asked to run it. Existing T03 behavior uses `interpreter_input` denials for unreviewable program-text and module-loading flags.

## Change

Added interpreter-aware exact exceptions for Java `-jar`, `-cp`, `-classpath`, `-ea`; PowerShell `-File` (case-insensitive, for `pwsh` and `powershell`); and Bash `-e`. All other short options continue through `carriesProgramTextFlag`, preserving fail-closed rejection for program-text flags. Added policy tests covering the requested allowed flags and continued rejection of Python `-c`/`-m`, Node `-e`, PHP `-r`, and Ruby `-e`.

## Verification

Baseline HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`

RED evidence before implementation:

- Command: `go test ./internal/modules/craft -run 'TestInputCodePolicy(AllowsInterpreterLauncherFlags|StillDeniesProgramTextFlags)$' -count=1`
- Result: failed as expected; all seven named launcher cases were denied as `interpreter_input`.

Post-change commands and results:

- `gofmt -w internal/modules/craft/input_code.go internal/modules/craft/input_code_round2_ocr_test.go` — completed.
- `go test ./internal/modules/craft -run 'TestInputCodePolicy(AllowsInterpreterLauncherFlags|StillDeniesProgramTextFlags)$' -count=1` — PASS.
- `go test ./internal/modules/craft -count=1` — PASS (`ok`, 1.264s).

## Checkpoint evidence

HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159` (no commit made).

Changed source/test SHA-256 after verification:

- `internal/modules/craft/input_code.go`: `89c92ec31929390c7ac7e747483cb033b59ae7e5a5a137171baff03955687747`
- `internal/modules/craft/input_code_round2_ocr_test.go`: `f2fbee4e84a042b100d63ce4102db59db7a46596dc9e607dfb0f11caabcf97db`

No API, persistence, migration, authorization, or cancellation paths changed. No remaining task-local risks identified.

## Scoped re-review fix — Java classpath entries

Addressed Round 1/5 review finding F37: a separated Java `-cp`/`-classpath` value is now treated as a colon-separated classpath list. Each entry is canonicalized against the working directory and checked against the inputs tree individually, and the value is consumed as an option value rather than treated as the Java program operand.

RED evidence before implementation:

- Command: `go test ./internal/modules/craft -run 'TestInputCodePolicyScreensEveryJavaClasspathEntry$' -count=1`
- Result: failed as expected; `java -cp lib:/workspace/inputs/payload.jar Main` was allowed.

Post-fix verification:

- `gofmt -w internal/modules/craft/input_code.go internal/modules/craft/input_code_round2_ocr_test.go` — completed.
- `go test ./internal/modules/craft -run 'TestInputCodePolicy(ScreensEveryJavaClasspathEntry|AllowsInterpreterLauncherFlags|StillDeniesProgramTextFlags)$' -count=1` — PASS.
- `go test ./internal/modules/craft -count=1` — PASS (`ok`, 0.802s).

Current HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit made. Current changed file SHA-256:

- `internal/modules/craft/input_code.go`: `768cd91d46d7779b194dd6727d708e57ee68a77318463176e7510d204cd77fb9`
- `internal/modules/craft/input_code_round2_ocr_test.go`: `0b180c7582690521b34f3faaa75af7956e038b7bc39ba464902fd2e4dfc5b150`

Regression coverage confirms input-tree classpath entries are denied for both `-cp` and `-classpath`, generated classpath entries remain allowed, Java launcher flag tests remain allowed, and code-bearing interpreter flags remain denied. Ready for scoped re-review.

## Review cleanup — cover both Java classpath spellings in the unsafe case

Made the unsafe uploaded classpath regression table-driven over both `-cp` and `-classpath`, matching the existing generated-classpath cases and the report claim.

- Command: `gofmt -w internal/modules/craft/input_code_round2_ocr_test.go && go test ./internal/modules/craft -run 'TestInputCodePolicyScreensEveryJavaClasspathEntry$' -count=1`
- Output: `ok github.com/Tencent/WeKnora/internal/modules/craft 1.338s` (exit code 0).
- Updated test SHA-256: `internal/modules/craft/input_code_round2_ocr_test.go` — `32245693720f08e3f915632f271b88dd791b65c45e3a1478eea7cb2f32f803d9`.
- HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit made.
