# Craft #107 Java Policy Fix4 — independent review

Reviewed: 2026-09-29. Scope: Fix4 plan/report and `.superpowers/sdd/2026-09-29-craft-107-java-policy-fix4/review-package-01.tar.gz`, against approved Craft Spec #107, `CONTEXT.md`, relevant ADRs, and the Fix3 independent finding. Read-only source review; no tests run. Tarball SHA-256 `547527ef489d4fdd166229c4c3db1637619f732addb34dec7784a25355f13dad`. Package before bodies match the Fix3 checkpoint (`ccac08b68f2ec09116ec75b0c7ce7ab6e16daaf29cc67c8bd907eb9a89cf02fe`, `2bb8e59f40410097390c9863b9697aeb82622394e2034f7d4d710127ad579917`); package after bodies, manifest and live source match exactly (`c2c8a860d155a2e07057f566439fdf09071d78df488a1872dfffec21b3fe248c`, `af9b5f1ea12d8fa75f8258e28dc07465cdbba9a4aa90be83511839db1ad1b4f6`).

## Evidence

- In the Java-detected shell branch, `Review` now checks `strings.TrimSpace(req.Environment["JDK_JAVA_OPTIONS"])` before raw shell assignment and argfile screening (`internal/modules/craft/input_code.go:290-310`). A non-empty value returns `interpreter_input` even if it points to a generated argfile outside the input tree. This closes the Fix3 shell-environment gap while leaving non-Java requests on their prior path.
- The new regression supplies `Shell: true`, `CommandText: java -jar app.jar`, generated resolved target/digest and `Environment["JDK_JAVA_OPTIONS"] = @/workspace/generated/launch.args`; it requires denial (`input_code_round2_ocr_test.go:348-356`). A matching shell Java launch without that environment value remains allowed (`:366-374`). The direct argv, `env` and `timeout` wrapper, quote-normalized shell argfile, and module path tests remain in the same file and production logic.
- The worker report records a RED result for the exact shell environment case, then focused and full `TestInputCodePolicy` passes and `git diff --check`. This reviewer did not rerun tests. The policy's documented choice to reject all Java argfiles and non-empty `JDK_JAVA_OPTIONS` has an intentional compatibility cost; the report preserves it.

## Verdict

- **Spec compliance: PASS for the Fix4 scope.** The reviewed shell environment route no longer admits uninspectable Java launcher options, and prior Java policy guards remain present.
- **Code quality/security: PASS for the Fix4 scope.** No new correctness, security, concurrency or data-consistency finding was identified in the small incremental patch. Broader Craft acceptance is outside this local verdict.
