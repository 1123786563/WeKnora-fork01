# FR1 independent scoped review — loader-relative framework closure

- **Exact range:** `1134dda075014f88e5d2976ee2dd269755b33e13..97cf8dc86cd307ef3d23f8701086f13038e7c729` in `/Users/wuyongjun/.paseo/worktrees/144ixsa6/t31-fr1-framework-closure` (implementation `ed9754492`, report `97cf8dc86`; reviewed HEAD `97cf8dc86cd307ef3d23f8701086f13038e7c729`). Only checker, its integration test, and FR1 report changed.
- **Spec compliance: FAIL.** The repair closes the original missing `@loader_path`/`@executable_path` cases, but two malformed or unsupported non-system framework loads can still reach `FRAMEWORK_CLOSURE_OK` (FR1-1 and FR1-2). This violates FR1's fail-closed acceptance.
- **Code quality: FAIL.** Both false passes were reproduced against the checked-in production checker with disposable fake-`otool` fixtures; no retained app failure is asserted.

## What is addressed

`verify-ios-framework-closure.py:82–103` classifies `@rpath`, `@loader_path`, and `@executable_path`, resolves owner/app-relative loads, requires a canonical match to the named framework's declared executable, and rejects a target outside the app. Existing `@rpath`/versioned-bundle behavior remains. The new integration tests cover missing and present loader-relative loads, a framework-owner sibling load, an unknown token, a non-system absolute framework path, and an absolute `/System/Library/Frameworks` load. The report records focused 18/18, full mobile suite 316 total / 302 pass / 14 gated skips, typecheck and Expo alignment passing, and an unchanged retained executable hash `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`. Those check counts are implementation-reported; I did not rerun the suite or native build.

## Findings

### FR1-1 — Medium — Framework load without a binary suffix is silently skipped

- **Evidence:** At `verify-ios-framework-closure.py:82–87`, the supported-form regex requires `.framework/<requested>`, and the fallback fails only if the load contains `.framework/`. A Mach-O load `@rpath/Missing.framework` has neither match; it is ignored. A disposable app with no `Missing.framework` returned exit 0 and `FRAMEWORK_CLOSURE_OK` (reproduction below).
- **Impact:** A malformed non-system framework install name can pass the Release gate even though the loader cannot resolve an executable at that path. FR1 brief acceptance 5 expressly requires malformed install names to fail.
- **Smallest correction:** Classify a framework component ending at `.framework` as malformed/unsupported even without a following slash, and add a negative production-checker fixture for that exact form.

### FR1-2 — Medium — Universal-binary header filter can discard a real dependency

- **Evidence:** `inspect()` at `verify-ios-framework-closure.py:48–50` removes every output line whose stripped text starts with the inspected binary's absolute pathname. This is broader than an `otool` architecture header. A dependency `/.../WeKnora.app/WeKnoraMissing.framework/Missing` starts with the app executable path `/.../WeKnora.app/WeKnora`; the checker drops that dependency and returned exit 0 plus `FRAMEWORK_CLOSURE_OK` in a disposable app with the target absent. The dependency is an absolute non-system framework load, which the same checker otherwise rejects at lines 84–86.
- **Impact:** The multi-arch parsing change creates a false pass for a non-system absolute load whose name shares an image-path prefix. This violates the explicit absolute/unknown fail-closed contract.
- **Smallest correction:** Skip only exact `otool` image headers (`<binary>:` or `<binary> (architecture <arch>):`), or parse header lines by their full shape while retaining indented load rows. Add a fixture with a path-prefix dependency plus two architecture headers.

## Reproduction and review limits

I ran a repository-external disposable Python fixture, equivalent to the following exact command, with source mode properties, a minimal `.app/Info.plist`, and fake `otool -L` output for each case:

```sh
python3 - <<'PY'
from pathlib import Path
import json, os, plistlib, subprocess, tempfile
checker = Path('apps/mobile/scripts/verify-ios-framework-closure.py').resolve()
for case in ('missing_binary_suffix', 'arch_header_prefix'):
    with tempfile.TemporaryDirectory(prefix='fr1-review-') as directory:
        root = Path(directory)
        app = root / 'WeKnora.app'
        app.mkdir()
        (app / 'Frameworks').mkdir()
        binary = app / 'WeKnora'
        binary.write_bytes(b'fixture')
        (app / 'Info.plist').write_bytes(plistlib.dumps({'CFBundleExecutable': 'WeKnora'}))
        properties = root / 'Podfile.properties.json'
        properties.write_text(json.dumps({'ios.buildReactNativeFromSource': 'true', 'EXPO_USE_PRECOMPILED_MODULES': 'false'}))
        bindir = root / 'bin'
        bindir.mkdir()
        dependency = '@rpath/Missing.framework' if case == 'missing_binary_suffix' else str(binary.resolve()) + 'Missing.framework/Missing'
        fake_otool = bindir / 'otool'
        fake_otool.write_text('#!/bin/sh\nprintf "%s (architecture arm64):\\n" "$2"\nprintf "    ' + dependency + ' (compatibility version 1.0, current version 1.0)\\n"\n')
        fake_otool.chmod(0o755)
        result = subprocess.run(['python3', str(checker), str(app), str(properties)], env={**os.environ, 'PATH': str(bindir) + os.pathsep + os.environ.get('PATH', '')}, capture_output=True, text=True)
        print(case, 'exit=' + str(result.returncode), 'stdout=' + repr(result.stdout.strip()), 'stderr=' + repr(result.stderr.strip()))
PY
```

Observed output:

```text
missing_binary_suffix exit=0 stdout='FRAMEWORK_MODE=source-expo-modules\nFRAMEWORK_CLOSURE_OK' stderr=''
arch_header_prefix exit=0 stdout='FRAMEWORK_MODE=source-expo-modules\nFRAMEWORK_CLOSURE_OK' stderr=''
```

Other read-only commands: `git rev-parse HEAD`, `git status --short`, `git diff --stat/--check/-- apps/mobile/scripts/verify-ios-framework-closure.py apps/mobile/src/ios-framework-closure.test.ts apps/mobile/scripts/ios-release-build.sh 1134dda075014f88e5d2976ee2dd269755b33e13..97cf8dc86cd307ef3d23f8701086f13038e7c729`, `cat` of FR1 brief/report, final review and repair plan, and `nl -ba` of checker/test. `git diff --check` had no errors. No production/test source, retained evidence, controller file, or OCR output was changed; no native build was run.
