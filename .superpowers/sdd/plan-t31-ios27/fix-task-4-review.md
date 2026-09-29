# Task R4 independent review

- Reviewer: `/root/review_t31_r4` (`reviewer`, read-only).
- Scope: BASE `003f12cc1456b0eba6578c6c164474c31fa9fc70` → HEAD `eb821ee9d9e6f34dba49beb2d336c7e8396ff8de`.
- Review package: `.superpowers/sdd/plan-t31-ios27/fix-task-4-review-package.patch`, SHA-256 `0e6c31b7f75fd2c61036356f0045317ef13f512b842c67502b8d785eae4a08e2`.
- Verdict: **Spec compliance FAIL; code quality FAIL**.

## Findings

1. **Medium — An explicit brief criterion was not met.** The original brief said the Release app must embed `React.framework`, while the implementation deliberately switches to source-built React and does not embed that framework. Source mode appears coherent with the runtime closure goal, but silently changing the acceptance was not allowed. Controller ruling: revise the brief because approved Issue #31/spec/ADR require a working runtime and do not prescribe a dynamic React framework. A coherent supported source build is acceptable. This ruling and its risk are recorded in the repair plan and round-1 brief.

2. **High — The release guard omits the app executable.** `ios-release-build.sh` inspected binaries inside embedded frameworks only. A missing `@rpath` dependency loaded directly by `WeKnora.app/WeKnora` could pass the gate and fail on launch.

3. **Medium — Framework directory presence is insufficient.** The guard counted `React.framework/` as an available dependency without ensuring its declared `CFBundleExecutable` exists and resolves to a regular file.

4. **Medium — Build/launch evidence was not durable.** The report cited `xcodebuild-release-final.log` and `launch.log`, but root `.gitignore:24` ignores `*.log`; HEAD tracked only screenshots. `launch.log` was a hand-written summary rather than captured simulator command output. The review requires tracked raw evidence tied to the built app executable hash.

## Scope and review notes

- New TypeScript tests exercised a separate helper, while the Release script used an inline Python implementation; they therefore did not cover findings 2 or 3.
- Reviewer confirmed `expo-build-properties` supports the two configuration options and generated Podfile properties selected source mode.
- Reviewer ran no tests/builds and made no edits.

## Repair reference

Round 1 brief: `.superpowers/sdd/plan-t31-ios27/fix-task-4-r1-brief.md`.
