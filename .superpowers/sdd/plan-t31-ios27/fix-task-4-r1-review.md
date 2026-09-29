# T31 R4 repair round 1 — independent scoped review

- Scope: BASE `eb821ee9d9e6f34dba49beb2d336c7e8396ff8de` → HEAD `1ebf2dfe55b4b0211c4753a82744e0a61cb86890`.
- Review package: `.superpowers/sdd/2026-09-29-t31-review-repairs/review-eb821ee9d..1ebf2dfe5.diff`; SHA-256 `c7b0770f29dd69bf1f1e906539bdc857d239a7671a99cfe3b6811f2312c2c4d1` (verified).
- Authority: round-1 brief, prior review, Issue #31 snapshot, approved mobile spec, ADR 0005, and `CONTEXT.md`.
- **Spec compliance: PASS for the scoped R4 repair.** The approved Issue, spec, and ADR do not prescribe dynamic `React.framework` packaging. The corrected brief explicitly allows the supported source configuration when the actual app launches with closed dynamic loads. This does not close Issue #31's staging login, OIDC, capability, or Android acceptance.
- **Code quality: FAIL.** The production checker can accept a declared framework executable elsewhere in `.app/Frameworks` even though the requested `@rpath/Foo.framework/Foo` path is absent (finding R1-1 below).

## Prior findings

1. **React.framework criterion — addressed by explicit ruling.** The round-1 brief and corrected R4 brief record the controller's change and its risk. Neither Issue #31, the approved spec, nor ADR 0005 requires a separately embedded `React.framework`. `apps/mobile/app.json` and generated `Podfile.properties.json` select source mode; the retained Release log ends with `BUILD SUCCEEDED`, `FRAMEWORK_CLOSURE_OK`, and an app path. The report says React is source linked and does not claim it is embedded. Actual simulator output records JS bundle evaluation, and the fresh screenshot shows sign-in below the status bar.
2. **App executable omitted — addressed.** `ios-release-build.sh:65` calls the checked-in `verify-ios-framework-closure.py`; checker lines 49 and 55–59 include the app executable in `otool -L` inspection. The direct missing dependency fixture at `ios-framework-closure.test.ts:48` exercises that entry point.
3. **Directory existence substituted for executable — substantially addressed, with a remaining gap.** Checker lines 15–34 parse `CFBundleExecutable`, resolve it, require a regular file inside `.app/Frameworks`, and lines 63–66 compare the requested binary name. The missing binary fixture at test line 57 fails. R1-1 describes the unresolved path identity case.
4. **Non-durable build and launch evidence — addressed.** Git tracks `xcodebuild-release-final.log.gz`, `simctl-install-launch.txt`, `simctl-launch-r1.txt`, and `ios27-release-launch-r1.png`. Verified compressed SHA-256 `4d182b68807f739504742a13bb1dfb3c62208f0629d36eb6184da41ce8fa435d`, decompressed SHA-256 `46f47407fb270ca1f8361a0442d32c5cbe184eaf7b1ed928420aa8afd1b3de83`, screenshot SHA-256 `6982dfa2728819aec625a1c85785a14271431e7d3135957d42fb2caab48cbd73`, and retained app executable SHA-256 `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`. The compressed log contains the successful build and checker output. Captured simulator output includes `ReactInstance: evaluateJavaScript() with JS bundle` and PID `15175`; the screenshot visibly places login content below the status bar. The report binds these artifacts to the unchanged executable hash. The ignored original `.log` files are no longer needed for durability.

## Open findings

### R1-1 — Medium — Framework executable containment is too broad

- **Evidence:** `verify-ios-framework-closure.py:54` passes the entire `.app/Frameworks` root to `executable_for`; lines 26–33 accept any resolved regular file under that root. Lines 63–66 then compare only `target.name` with the requested binary name. For example, `Foo.framework/Info.plist` can declare `CFBundleExecutable=../Bar.framework/Foo`, with a regular `Bar.framework/Foo`; a load of `@rpath/Foo.framework/Foo` passes even though `Foo.framework/Foo` is absent. Existing tests cover escape from the app, not this missing requested load path.
- **Impact:** The Release gate can print `FRAMEWORK_CLOSURE_OK` for an app that dyld cannot load, regressing the exact native closure guarantee R4 is meant to provide.
- **Smallest correction:** Require each declared executable to resolve within its own framework bundle and require the resolved load path for `@rpath/<framework>/<binary>` to equal that declared executable; add a negative integration fixture for an executable in another framework directory.

### R1-2 — Low — Mode output can misstate packaging

- **Evidence:** `verify-ios-framework-closure.py:70–71` prints `FRAMEWORK_MODE=source-expo-modules` solely when `ExpoModulesWorklets.framework` is absent. A precompiled Expo build that omits that module would be labeled source mode. The old release script's explicit Podfile source-mode/Worklets mismatch check was removed in this range. Current generated `Podfile.properties.json` does show source mode, so this is not evidence of a mislabeled retained app.
- **Impact:** Future release evidence may claim the wrong build mode; mode drift may go unnoticed even when dynamic closure happens to pass.
- **Smallest correction:** Derive the mode from effective generated build properties and use the framework set only as a consistency check, or emit an accurately named presence observation instead of a mode conclusion.

## Edge case assessed, currently nonblocking

`verify-ios-framework-closure.py:63–66` rejects a versioned framework load such as `@rpath/Foo.framework/Versions/A/Foo` because it compares `Foo` with the full suffix `Versions/A/Foo`. This is a false rejection for a valid versioned framework layout, not a false acceptance. The retained iOS app does not show this layout, so it does not block current R4 acceptance. If this checker is used for versioned frameworks, compare the fully resolved requested load path with the declared executable and cover that form in a fixture.

## Review limits

This was a read-only code and artifact review. No OCR, build, test, simulator command, or source edit was run. I inspected the tracked screenshot and verified artifact hashes and Git tracking; focused/full test counts, typecheck, and Expo alignment are reported by the implementer, not independently rerun here.
