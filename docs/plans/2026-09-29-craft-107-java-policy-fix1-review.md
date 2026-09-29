# Craft #107 Java Policy Fix 1 — independent review

Review scope: immutable `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix1/review-package-01.tar.gz`, the Fix 1 plan and report, OCR Round 3 Java findings, approved Craft Spec #107, `CONTEXT.md`, and relevant ADRs. This review did not run tests or change production or test source. Package SHA-256 `d8a622527307e03e4464fcd6eecf5bc0f12f05018774fa272890779ccd08e245`; its after-source hashes match the current `input_code.go` and `input_code_round2_ocr_test.go` (`5f2d9c86…` and `cc669f90…`).

## Finding

### HIGH — Java module path still executes uploaded code

- **Evidence:** `internal/modules/craft/input_code.go` recognizes only `-cp`, `-classpath`, and `--class-path` for `javaClasspathValueNext` (around line 418), and only `--class-path=` for attached path-list screening (around line 422). `--module-path` is absent from `longProgramTextOptions` and the Java path-list handling. For `java --module-path lib:/workspace/inputs/payload.jar --module payload.mod/com.example.Main`, the module-path value becomes the first non-flag `scriptSeen` operand and is canonicalized as one string instead of split at `:` (around line 441). The later `--module` flag and module name are in the post-script region and have no path match. The attached `--module-path=lib:/workspace/inputs/payload.jar` form likewise screens the unsplit value only. The [Oracle Java launcher reference](https://docs.oracle.com/en/java/javase/21/docs/specs/man/java.html) documents module-path elements as module files or directories and `--module` as a way to launch their main class.
- **Impact:** A modular JAR admitted as an immutable uploaded input can be selected from the inputs tree and executed by the JVM. This violates approved Spec #107, User Story 7 and its explicit rule that uploaded code is never directly executed. The original OCR `--class-path` bypass is closed, but the same path-list assumption leaves this adjacent Java launcher path open.
- **Smallest defensible correction:** Treat Java `--module-path` (and its `-p` alias) separated and `--module-path=` attached values as executable-material path lists, checking every entry against the input tree before any script operand is marked. Cover both forms with an uploaded modular JAR path, a generated module path, and a wrapped `env`/`timeout` form. Review the other Java executable-material path-list options (`--upgrade-module-path`, `--patch-module`) under the same rule before declaring the Java policy compliant.

## Scoped OCR finding disposition

- **OCR HIGH `--class-path`: closed for its stated separated and equals-attached forms.** The patch uses `javaClasspathInputEntry` for both, which splits each `:` entry and checks canonical containment. The new tests cover an input entry after a benign entry and generated path acceptance.
- **OCR MEDIUM `-ea:` / wrapper detection: closed for its stated forms.** `path.Base(req.Command[offset-1])` uses the interpreter position returned by `InterpreterPrefixStatus`; this correctly identifies Java after the tested `timeout` and `env` wrappers. `-ea:` is exempted while the existing `-c`, `-e`, `-r`, and `-m` program-text denials remain unchanged. Existing `-cp` and `-classpath` path-list handling remains in place. The focused and broader test passes are recorded in the implementer report, not independently rerun here.

## Verdict

- **Spec compliance: FAIL.** The module-path execution route above conflicts with the approved read-only uploaded-code boundary.
- **Code quality/security: FAIL.** The patch repairs its two reported OCR cases and preserves their relevant regressions, but executable Java path-list handling remains incomplete. No concurrency or data-consistency issue was found in this local policy patch. This is a source review; runtime execution was not tested in this review.
